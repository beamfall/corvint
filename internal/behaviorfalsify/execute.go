package behaviorfalsify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	cleanupExecutionTimeout  = 5 * time.Second
	processShutdownAllowance = 2 * time.Second
	cleanupSchedulingSlack   = 1 * time.Second
	cleanupReserve           = processShutdownAllowance + cleanupExecutionTimeout + processShutdownAllowance + cleanupSchedulingSlack
)

var infrastructureReasons = map[string]bool{
	"browser-disconnected":    true,
	"browser-unavailable":     true,
	"cancelled":               true,
	"environment-unavailable": true,
	"runner-unavailable":      true,
	"timeout":                 true,
}

func Execute(ctx context.Context, plan Plan, approvedDigest string) (Report, error) {
	if approvedDigest == "" || approvedDigest != plan.Digest {
		return Report{}, errors.New("operator-authorization-required")
	}
	request := plan.Request
	request.Controls = make([]ControlSpec, 0, len(plan.Controls))
	for _, control := range plan.Controls {
		request.Controls = append(request.Controls, control.ControlSpec)
	}
	rebuilt, err := BuildPlan(request)
	if err != nil {
		return Report{}, err
	}
	if !samePlan(rebuilt, plan) {
		return Report{}, errors.New("approved-plan-drift")
	}
	report := Report{
		Schema:      ReportSchema,
		PlanDigest:  plan.Digest,
		Counts:      allStatusCounts(),
		Requested:   len(plan.Controls),
		Fallback:    "full-relevant-suite",
		Limitations: []string{"caller-owned hook semantics are not authenticated", "results cover only the exact approved controls and never authorize suite narrowing", "persistent external state is caller-declared absent, not independently sandboxed"},
	}
	deadline := time.Now().Add(time.Duration(plan.Request.WallClockSeconds) * time.Second)
	workspaceReady := true
	for _, control := range plan.Controls {
		result := ControlResult{ID: control.ID, Kind: control.Kind, Ordinal: control.Ordinal}
		switch control.Disposition {
		case "not_supported":
			result.Status = StatusNotSupported
			result.Reasons = []string{"caller-hook-not-supported"}
		case "not_run":
			result.Status = StatusNotRun
			result.Reasons = []string{"caller-deferred-control"}
		case "run":
			report.Supported++
			if ctx.Err() != nil {
				result.Status = StatusNotRun
				result.Reasons = []string{"execution-cancelled"}
			} else if !workspaceReady || time.Until(deadline) <= cleanupReserve {
				result.Status = StatusNotRun
				result.Reasons = []string{"workspace-or-wall-clock-boundary-unavailable"}
			} else {
				result = executeControl(ctx, plan, control, deadline)
				if controlExecuted(result) {
					report.Executed++
				}
				workspaceReady = controlWorkspaceRestored(result)
			}
		}
		report.Results = append(report.Results, result)
		report.Counts[result.Status]++
		if result.Status != StatusKilled {
			report.CoverageGaps = append(report.CoverageGaps, result.ID)
		}
		if err := ensureDocumentBound(report); err != nil {
			return Report{}, fmt.Errorf("report: %w", err)
		}
	}
	report.CompleteVocabulary = completeVocabulary(plan.Controls)
	report.MutationScore = mutationScore(report.Counts)
	report.Digest = digestJSON(report)
	if err := ensureDocumentBound(report); err != nil {
		return Report{}, fmt.Errorf("report: %w", err)
	}
	return report, nil
}

func executeControl(ctx context.Context, plan Plan, control PlannedControl, deadline time.Time) ControlResult {
	result := ControlResult{ID: control.ID, Kind: control.Kind, Ordinal: control.Ordinal}
	for attempt := 1; attempt <= plan.Request.Attempts; attempt++ {
		if ctx.Err() != nil || time.Until(deadline) <= cleanupReserve {
			break
		}
		attemptResult := executeAttempt(ctx, plan, control, attempt, deadline)
		result.Attempts = append(result.Attempts, attemptResult)
		if attemptResult.WorkspaceAfter == "" || attemptResult.WorkspaceAfter != attemptResult.WorkspaceBefore || ctx.Err() != nil {
			break
		}
	}
	result.Status, result.Reasons = aggregateAttempts(result.Attempts, plan.Request.Attempts)
	return result
}

func executeAttempt(ctx context.Context, plan Plan, control PlannedControl, attempt int, deadline time.Time) AttemptResult {
	result := AttemptResult{Attempt: attempt}
	before, err := workspaceDigest(plan.Request.DisposableRoot)
	if err != nil || before != plan.WorkspaceSHA256 {
		result.Status = StatusInvalidControl
		result.Reasons = []string{"workspace-baseline-drift"}
		return result
	}
	result.WorkspaceBefore = before
	invocation := Invocation{Schema: InvocationSchema, PlanDigest: plan.Digest, Attempt: attempt, Target: plan.Request.Target, Runner: plan.Request.Runner, Control: control}
	input, _ := marshalJSON(invocation)
	timeout := time.Duration(plan.Request.TimeoutSeconds) * time.Second
	if available := time.Until(deadline) - cleanupReserve; available < timeout {
		timeout = available
	}
	if timeout <= 0 {
		result.Status = StatusNotRun
		result.Reasons = []string{"wall-clock-budget-exhausted"}
		return result
	}
	hookRaw, hookProcess := runPinned(ctx, *control.Hook, plan.Request.DisposableRoot, plan.Request.DeclaredEnvKeys, "control", input, timeout, receiptOutputLimit(plan))
	result.HookProcess = hookProcess
	var receipt HookReceipt
	if processSucceeded(hookProcess) {
		if err := Decode(hookRaw, &receipt); err != nil {
			result.Reasons = append(result.Reasons, "hook-output-invalid")
		} else {
			result.Receipt = &receipt
			artifacts, artifactReasons := validateArtifacts(plan.Request.DisposableRoot, receipt.Artifacts)
			result.RetainedArtifacts = artifacts
			result.Reasons = append(result.Reasons, artifactReasons...)
		}
	} else {
		result.Reasons = append(result.Reasons, "hook-process-failed")
	}
	cleanupTimeout := minDuration(cleanupExecutionTimeout, time.Until(deadline)-processShutdownAllowance)
	if cleanupTimeout <= 0 {
		cleanupTimeout = time.Nanosecond
		result.Reasons = append(result.Reasons, "cleanup-budget-overrun")
	}
	_, cleanupProcess := runPinned(context.Background(), plan.Request.Cleanup, plan.Request.DisposableRoot, plan.Request.DeclaredEnvKeys, "cleanup", input, cleanupTimeout, 1<<20)
	result.CleanupProcess = cleanupProcess
	after, digestErr := workspaceDigest(plan.Request.DisposableRoot)
	result.WorkspaceAfter = after
	if digestErr != nil || after != before {
		result.Reasons = append(result.Reasons, "workspace-cleanup-mismatch")
	}
	result.Status, result.Reasons = classifyAttempt(plan, control, result)
	return result
}

func classifyAttempt(plan Plan, control PlannedControl, result AttemptResult) (Status, []string) {
	reasons := slices.Clone(result.Reasons)
	if !processSucceeded(result.HookProcess) {
		return StatusInfrastructureFailed, uniqueReasons(append(reasons, "execution-boundary-failed"))
	}
	if !processSucceeded(result.CleanupProcess) || result.WorkspaceAfter == "" || result.WorkspaceAfter != result.WorkspaceBefore {
		return StatusInvalidControl, uniqueReasons(append(reasons, "cleanup-invalid"))
	}
	if result.Receipt == nil || len(reasons) != 0 {
		return StatusInvalidControl, uniqueReasons(reasons)
	}
	receipt := *result.Receipt
	if receipt.Schema != HookSchema || receipt.PlanDigest != plan.Digest || receipt.PerturbationSHA256 != control.PerturbationSHA256 || receipt.Attempt != result.Attempt || receipt.Retry != 0 || !digestPattern.MatchString(receipt.NativeReceiptSHA256) {
		return StatusInvalidControl, []string{"receipt-binding-mismatch"}
	}
	if !reflect.DeepEqual(receipt.Target, plan.Request.Target) || !reflect.DeepEqual(receipt.Runner, plan.Request.Runner) {
		return StatusInvalidControl, []string{"execution-identity-mismatch"}
	}
	if receipt.Infrastructure != nil {
		if !validInfrastructureReceipt(plan, receipt) {
			return StatusInvalidControl, []string{"infrastructure-receipt-invalid"}
		}
		return StatusInfrastructureFailed, []string{"playwright-infrastructure"}
	}
	if !observationsMatch(receipt.Unrelated, control.UnrelatedCriteria) {
		return StatusInvalidControl, []string{"unrelated-criterion-failed-or-missing"}
	}
	if !setupMatches(receipt.Setup, control.RequiredSetup) {
		return StatusInvalidControl, []string{"required-setup-failed-or-missing"}
	}
	target := receipt.TargetObservation
	if target.CriterionID != plan.Request.Target.CriterionID || target.AssertionID != plan.Request.Target.AssertionID {
		return StatusInvalidControl, []string{"wrong-assertion-identity"}
	}
	switch {
	case target.State == "failed" && target.FailureKind == "assertion" && receipt.TestOutcome == "failed":
		return StatusKilled, []string{"expected-assertion-failed"}
	case target.State == "passed" && target.FailureKind == "" && receipt.TestOutcome == "passed":
		return StatusSurvived, []string{"target-criterion-survived"}
	default:
		return StatusInvalidControl, []string{"target-failed-for-unexpected-reason"}
	}
}

func observationsMatch(observations []CriterionObservation, expected []string) bool {
	if len(observations) != len(expected) {
		return false
	}
	actual := make([]string, 0, len(observations))
	for _, observation := range observations {
		if observation.State != "passed" || observation.AssertionID == "" || len(observation.AssertionID) > 4096 || observation.FailureKind != "" {
			return false
		}
		actual = append(actual, observation.CriterionID)
	}
	slices.Sort(actual)
	return slices.Equal(actual, expected)
}

func setupMatches(observations []SetupObservation, expected []string) bool {
	if len(observations) != len(expected) {
		return false
	}
	actual := make([]string, 0, len(observations))
	for _, observation := range observations {
		if observation.State != "passed" {
			return false
		}
		actual = append(actual, observation.ID)
	}
	slices.Sort(actual)
	return slices.Equal(actual, expected)
}

func validateArtifacts(root string, declared []Artifact) ([]Artifact, []string) {
	if len(declared) > 64 {
		return nil, []string{"artifact-count-exceeded"}
	}
	retained := make([]Artifact, 0, len(declared))
	seen := map[string]bool{}
	for _, artifact := range declared {
		if seen[artifact.Path] || len(artifact.Path) > 4096 || filepath.IsAbs(artifact.Path) || filepath.Clean(artifact.Path) != artifact.Path || artifact.Path == ".." || strings.HasPrefix(artifact.Path, ".."+string(filepath.Separator)) || !digestPattern.MatchString(artifact.SHA256) {
			return retained, []string{"artifact-identity-invalid"}
		}
		seen[artifact.Path] = true
		path, err := containedRegularFile(root, artifact.Path)
		if err != nil {
			return retained, []string{"artifact-path-invalid"}
		}
		digest, err := digestFile(path, maxArtifactBytes)
		if err != nil || digest != artifact.SHA256 {
			return retained, []string{"artifact-digest-mismatch"}
		}
		retained = append(retained, artifact)
	}
	return retained, nil
}

func containedRegularFile(root, relative string) (string, error) {
	current := root
	parts := strings.Split(filepath.Clean(relative), string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("artifact path escapes through a symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", errors.New("artifact parent is not a directory")
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", errors.New("artifact is not a regular file")
		}
	}
	return current, nil
}

func runPinned(ctx context.Context, command Command, dir string, keys []string, role string, input []byte, timeout time.Duration, outputLimit int) ([]byte, ProcessEvidence) {
	if timeout <= 0 {
		return nil, ProcessEvidence{Error: "command budget exhausted"}
	}
	if err := validateCommand(command); err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	executable, err := os.ReadFile(command.Path)
	if err != nil || digestBytes(executable) != command.ExecutableSHA256 {
		return nil, ProcessEvidence{Error: "command changed before staging"}
	}
	scratch, err := os.MkdirTemp("", "corvint-behavior-control-")
	if err != nil {
		return nil, ProcessEvidence{Error: "command staging unavailable"}
	}
	defer os.RemoveAll(scratch)
	staged := filepath.Join(scratch, "command")
	if err := os.WriteFile(staged, executable, 0700); err != nil {
		return nil, ProcessEvidence{Error: "command staging failed"}
	}
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: []string{staged}, Dir: dir, Env: commandEnvironment(dir, keys, role), Stdin: input, Timeout: timeout, ShutdownTimeout: processShutdownAllowance, InputLimit: maxDocumentBytes, OutputLimit: outputLimit, StderrLimit: 1 << 20, ObserveDescendants: true})
	evidence := ProcessEvidence{
		Exit: observation.ExitStatus, Started: observation.Started,
		Completed: observation.WaitCompleted && observation.ExitObserved,
		Cancelled: observation.Cancelled, TimedOut: observation.TimedOut,
		Overflow:     observation.OutputOverflow || observation.StdoutOverflow || observation.StderrOverflow,
		OwnedCleanup: observation.OwnedProcessGroupCleanup,
		StdoutSHA256: digestBytes(observation.Stdout), StderrSHA256: digestBytes(observation.Stderr),
	}
	if observation.DescendantObservation != nil {
		evidence.DescendantsGone = observation.DescendantObservation.Absent
	}
	if observation.Err != nil {
		evidence.Error = observation.Err.Error()
	}
	return observation.Stdout, evidence
}

func commandEnvironment(root string, keys []string, role string) []string {
	values := map[string]string{"HOME": root, "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", "CORVINT_BEHAVIOR_COMMAND_ROLE": role}
	for key, value := range declaredEnvironment(keys) {
		values[key] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	environment := make([]string, 0, len(names))
	for _, name := range names {
		environment = append(environment, name+"="+values[name])
	}
	return environment
}

func processSucceeded(evidence ProcessEvidence) bool {
	return evidence.Started && evidence.Completed && evidence.Exit == 0 && !evidence.Cancelled && !evidence.TimedOut && !evidence.Overflow && evidence.OwnedCleanup && evidence.DescendantsGone && evidence.Error == ""
}

func aggregateAttempts(attempts []AttemptResult, expected int) (Status, []string) {
	for _, attempt := range attempts {
		if attempt.Status == StatusSurvived {
			return StatusSurvived, []string{"one-or-more-attempts-survived"}
		}
	}
	for _, attempt := range attempts {
		if attempt.Status == StatusInfrastructureFailed {
			return StatusInfrastructureFailed, []string{"one-or-more-attempts-lost-to-infrastructure"}
		}
	}
	for _, attempt := range attempts {
		if attempt.Status == StatusInvalidControl {
			return StatusInvalidControl, []string{"one-or-more-attempts-invalid"}
		}
	}
	if len(attempts) != expected {
		return StatusNotRun, []string{"attempt-set-incomplete"}
	}
	for _, attempt := range attempts {
		if attempt.Status == StatusNotRun {
			return StatusNotRun, []string{"one-or-more-attempts-not-run"}
		}
	}
	return StatusKilled, []string{"every-attempt-killed"}
}

func controlExecuted(result ControlResult) bool {
	for _, attempt := range result.Attempts {
		if attempt.HookProcess.Started {
			return true
		}
	}
	return false
}

func receiptOutputLimit(plan Plan) int {
	slots := 0
	for _, control := range plan.Controls {
		if control.Disposition == "run" {
			slots += plan.Request.Attempts
		}
	}
	if slots < 1 {
		return maxDocumentBytes / 8
	}
	// Leave room for Unicode escaping, validated artifact metadata, process evidence and framing.
	return maxDocumentBytes / (8 * slots)
}

func validInfrastructureReceipt(plan Plan, receipt HookReceipt) bool {
	failure := receipt.Infrastructure
	return failure != nil && infrastructureReasons[failure.Reason] && len(failure.Detail) <= 4096 &&
		receipt.TestOutcome == "infrastructure_failed" && receipt.TargetObservation.CriterionID == plan.Request.Target.CriterionID &&
		receipt.TargetObservation.AssertionID == plan.Request.Target.AssertionID && receipt.TargetObservation.State == "not_run" &&
		receipt.TargetObservation.FailureKind == "" && len(receipt.Unrelated) == 0 && len(receipt.Setup) == 0
}

func controlWorkspaceRestored(result ControlResult) bool {
	for _, attempt := range result.Attempts {
		if attempt.WorkspaceBefore == "" || attempt.WorkspaceAfter != attempt.WorkspaceBefore {
			return false
		}
	}
	return true
}

func completeVocabulary(controls []PlannedControl) bool {
	seen := map[ControlKind]bool{}
	for _, control := range controls {
		seen[control.Kind] = true
	}
	return len(seen) == len(controlKinds)
}

func mutationScore(counts map[Status]int) MutationScore {
	denominator := counts[StatusKilled] + counts[StatusSurvived]
	return MutationScore{Defined: denominator > 0, Killed: counts[StatusKilled], Denominator: denominator}
}

func allStatusCounts() map[Status]int {
	return map[Status]int{StatusKilled: 0, StatusSurvived: 0, StatusNotSupported: 0, StatusNotRun: 0, StatusInfrastructureFailed: 0, StatusInvalidControl: 0}
}

func uniqueReasons(reasons []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if reason != "" && !seen[reason] {
			seen[reason] = true
			out = append(out, reason)
		}
	}
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
