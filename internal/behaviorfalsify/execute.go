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
	"syscall"
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
	return execute(ctx, plan, approvedDigest, nil)
}

func execute(ctx context.Context, plan Plan, approvedDigest string, capture func(PlannedControl, AttemptResult, []byte)) (Report, error) {
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
	if isolation := plan.Request.Isolation; isolation != nil {
		clone := *isolation
		report.Isolation = &clone
		report.Limitations[2] = "persistent external state is a caller-declared shared server whose identity is reported by the caller's reset hook, not independently observed or sandboxed"
	}
	if plan.Request.Provider != nil {
		report.Provider = plan.Request.Provider.Name
	}
	if auth := plan.Request.Authentication; auth != nil {
		clone := *auth
		clone.StorageState = slices.Clone(auth.StorageState)
		report.Authentication = &clone
		report.Limitations = append(report.Limitations, "the authenticated account is caller-declared disposable; storage-state contents are pinned by digest and never retained")
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
				result = executeControl(ctx, plan, control, deadline, capture)
				if controlExecuted(result) {
					report.Executed++
				}
				workspaceReady = controlWorkspaceRestored(result)
			}
		}
		report.Results = append(report.Results, result)
		report.Counts[result.Status]++
		if result.Application == ApplicationNotApplied {
			report.InertControls = append(report.InertControls, result.ID)
		}
		if result.Status != StatusKilled {
			report.CoverageGaps = append(report.CoverageGaps, result.ID)
		}
		if err := ensureDocumentBound(report); err != nil {
			return Report{}, fmt.Errorf("report: %w", err)
		}
	}
	if hookReportedIsolation(plan, report.Results) {
		report.CoverageGaps = append(report.CoverageGaps, IsolationHookReportedGap)
	}
	report.CompleteVocabulary = completeVocabulary(plan.Controls)
	report.MutationScore = mutationScore(report.Counts)
	report.Digest = digestJSON(report)
	if err := ensureDocumentBound(report); err != nil {
		return Report{}, fmt.Errorf("report: %w", err)
	}
	if err := refuseAuthenticationMaterial(plan, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func executeControl(ctx context.Context, plan Plan, control PlannedControl, deadline time.Time, capture func(PlannedControl, AttemptResult, []byte)) ControlResult {
	result := ControlResult{ID: control.ID, Kind: control.Kind, Ordinal: control.Ordinal}
	for attempt := 1; attempt <= plan.Request.Attempts; attempt++ {
		if ctx.Err() != nil || time.Until(deadline) <= cleanupReserve {
			break
		}
		attemptResult := executeAttempt(ctx, plan, control, attempt, deadline, capture)
		result.Attempts = append(result.Attempts, attemptResult)
		if attemptResult.WorkspaceAfter == "" || attemptResult.WorkspaceAfter != attemptResult.WorkspaceBefore || ctx.Err() != nil {
			break
		}
	}
	result.Status, result.Reasons = aggregateAttempts(result.Attempts, plan.Request.Attempts)
	result.Application = aggregateApplication(control, result.Attempts, result.Status)
	return result
}

func executeAttempt(ctx context.Context, plan Plan, control PlannedControl, attempt int, deadline time.Time, capture func(PlannedControl, AttemptResult, []byte)) AttemptResult {
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
	reserve := cleanupReserve
	if plan.Request.Isolation != nil {
		reserve += isolationReserve
	}
	timeout := time.Duration(plan.Request.TimeoutSeconds) * time.Second
	if available := time.Until(deadline) - reserve; available < timeout {
		timeout = available
	}
	if timeout <= 0 {
		result.Status = StatusNotRun
		result.Reasons = []string{"wall-clock-budget-exhausted"}
		return result
	}
	runControl := true
	if plan.Request.Isolation != nil {
		// Reset the shared server and confirm its declared identity before the control (BBF-V0-020).
		process, identity, raw := runIsolationPhase(ctx, plan, control, attempt, isolationPhaseReset, timeout)
		result.Isolation = &IsolationObservation{ResetProcess: process, Before: identity, resetRaw: raw}
		failure := isolationFailure(plan, result)
		runControl = failure != "isolation-reset-failed" && failure != "isolation-identity-drift"
		// The reset hook ran with the caller's privileges, so the storage-state pins are rechecked
		// before the control can read them (BBF-V0-218).
		if runControl && plan.Request.Authentication != nil {
			if _, err := storageStateBytes(plan.Request); err != nil {
				runControl = false
				result.Reasons = append(result.Reasons, "authentication-input-drift")
			}
		}
		if available := time.Until(deadline) - reserve; available < timeout {
			timeout = available
		}
	}
	var hookRaw []byte
	if runControl && timeout > 0 {
		hookRaw = runControlHook(ctx, plan, control, attempt, input, timeout, deadline.Add(-reserve), &result)
	}
	// Capture after admission, before cleanup removes declared native reports (BBF-V0-018).
	if capture != nil {
		capture(control, result, hookRaw)
	}
	cleanupTimeout := minDuration(cleanupExecutionTimeout, time.Until(deadline)-processShutdownAllowance)
	if cleanupTimeout <= 0 {
		cleanupTimeout = time.Nanosecond
		result.Reasons = append(result.Reasons, "cleanup-budget-overrun")
	}
	_, cleanupProcess := runPinned(context.Background(), plan.Request.Cleanup, plan.Request.DisposableRoot, plan.Request.DeclaredEnvKeys, "cleanup", input, cleanupTimeout, 1<<20)
	result.CleanupProcess = cleanupProcess
	if runControl && plan.Request.Isolation != nil {
		// The declared identity must still hold after the control and its cleanup (BBF-V0-021).
		verifyTimeout := minDuration(isolationVerifyTimeout, time.Until(deadline)-processShutdownAllowance)
		process, identity, raw := runIsolationPhase(context.Background(), plan, control, attempt, isolationPhaseVerify, max(verifyTimeout, time.Nanosecond))
		result.Isolation.VerifyProcess, result.Isolation.After, result.Isolation.verifyRaw = &process, identity, raw
	}
	after, digestErr := workspaceDigest(plan.Request.DisposableRoot)
	result.WorkspaceAfter = after
	if digestErr != nil || after != before {
		result.Reasons = append(result.Reasons, "workspace-cleanup-mismatch")
	}
	result.Status, result.Reasons, result.Application = classifyAttempt(plan, control, result)
	return result
}

// runControlHook runs the control's pinned hook and admits its receipt: the hook's own receipt
// for the default profile, or one synthesized by the runner-neutral provider (BBF-V0-025).
func runControlHook(ctx context.Context, plan Plan, control PlannedControl, attempt int, input []byte, timeout time.Duration, budgetEnd time.Time, result *AttemptResult) []byte {
	limit := receiptOutputLimit(plan, control)
	if plan.Request.Provider != nil {
		limit = perturbationLimit
	}
	hookRaw, hookProcess := runPinned(ctx, *control.Hook, plan.Request.DisposableRoot, plan.Request.DeclaredEnvKeys, "control", input, timeout, limit)
	result.HookProcess = hookProcess
	switch {
	case !processSucceeded(hookProcess):
		result.Reasons = append(result.Reasons, "hook-process-failed")
	case plan.Request.Provider != nil:
		runProviderAttempt(ctx, plan, control, attempt, hookRaw, minDuration(timeout, time.Until(budgetEnd)), result)
	default:
		var receipt HookReceipt
		if err := Decode(hookRaw, &receipt); err != nil {
			result.Reasons = append(result.Reasons, "hook-output-invalid")
		} else {
			result.Receipt = &receipt
			artifacts, artifactReasons := validateArtifacts(plan.Request.DisposableRoot, receipt.Artifacts)
			result.RetainedArtifacts = artifacts
			result.Reasons = append(result.Reasons, artifactReasons...)
		}
	}
	return hookRaw
}

// classifyAttempt returns the attempt status, its reasons and, for a control that declares an
// application marker, its application classification (BBF-V0-015).
func classifyAttempt(plan Plan, control PlannedControl, result AttemptResult) (Status, []string, string) {
	reasons := slices.Clone(result.Reasons)
	// Shared-server identity drift or a failed reset is never evidence about the test (BBF-V0-021).
	if reason := isolationFailure(plan, result); reason != "" {
		return StatusInfrastructureFailed, uniqueReasons(append(reasons, reason)), ""
	}
	if !processSucceeded(result.HookProcess) {
		return StatusInfrastructureFailed, uniqueReasons(append(reasons, "execution-boundary-failed")), ""
	}
	if providerInfrastructureFailure(plan, result) {
		return StatusInfrastructureFailed, uniqueReasons(append(reasons, "provider-process-failed")), ""
	}
	if !processSucceeded(result.CleanupProcess) || result.WorkspaceAfter == "" || result.WorkspaceAfter != result.WorkspaceBefore {
		return StatusInvalidControl, uniqueReasons(append(reasons, "cleanup-invalid")), ""
	}
	if result.Receipt == nil || len(reasons) != 0 {
		return StatusInvalidControl, uniqueReasons(reasons), ""
	}
	receipt := *result.Receipt
	if receipt.Schema != HookSchema || receipt.PlanDigest != plan.Digest || receipt.PerturbationSHA256 != control.PerturbationSHA256 || receipt.Attempt != result.Attempt || receipt.Retry != 0 || !digestPattern.MatchString(receipt.NativeReceiptSHA256) {
		return StatusInvalidControl, []string{"receipt-binding-mismatch"}, ""
	}
	if !reflect.DeepEqual(receipt.Target, plan.Request.Target) || !reflect.DeepEqual(receipt.Runner, plan.Request.Runner) {
		return StatusInvalidControl, []string{"execution-identity-mismatch"}, ""
	}
	if !validMarkerObservations(receipt.Markers) {
		return StatusInvalidControl, []string{"marker-observation-invalid"}, ""
	}
	if receipt.Infrastructure != nil {
		if !validInfrastructureReceipt(plan, receipt) {
			return StatusInvalidControl, []string{"infrastructure-receipt-invalid"}, ""
		}
		return StatusInfrastructureFailed, []string{"playwright-infrastructure"}, ""
	}
	status, reasons := classifyObservations(plan, control, receipt)
	return classifyApplication(control, receipt, status, reasons)
}

// classifyApplication applies a declared application marker to an admitted receipt's outcome.
// A target outcome without this control's own marker is NOT-APPLIED and never counts as a kill
// or a survivor; an applied control whose required setup failed is APPLIED-EARLY-FAIL.
func classifyApplication(control PlannedControl, receipt HookReceipt, status Status, reasons []string) (Status, []string, string) {
	marker := control.ApplicationMarker
	if marker == nil {
		return status, reasons, ""
	}
	applied := false
	for _, observed := range receipt.Markers {
		if observed.Kind == marker.Kind && observed.Type == marker.Type && observed.ControlID == control.ID {
			applied = true
		}
	}
	earlyFailure := false
	for _, setup := range receipt.Setup {
		if setup.State == "failed" && slices.Contains(control.RequiredSetup, setup.ID) {
			earlyFailure = true
		}
	}
	switch {
	case !applied && (status == StatusKilled || status == StatusSurvived):
		return StatusInvalidControl, []string{"control-not-applied"}, ApplicationNotApplied
	case !applied:
		return status, reasons, ""
	case status == StatusKilled:
		return status, reasons, ApplicationAppliedFailed
	case status == StatusSurvived:
		return status, reasons, ApplicationAppliedPassed
	case earlyFailure:
		return status, reasons, ApplicationAppliedEarlyFail
	}
	return status, reasons, ""
}

func validMarkerObservations(markers []MarkerObservation) bool {
	if len(markers) > maxMarkerObservations {
		return false
	}
	for _, observed := range markers {
		if !validMarker(observed.Kind, observed.Type) || !idPattern.MatchString(observed.ControlID) {
			return false
		}
	}
	return true
}

func classifyObservations(plan Plan, control PlannedControl, receipt HookReceipt) (Status, []string) {
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
	staged, cleanup, err := stageVerified(command)
	if err != nil {
		return nil, ProcessEvidence{Error: err.Error()}
	}
	defer cleanup()
	return observeProcess(ctx, []string{staged}, dir, commandEnvironment(dir, keys, role), input, timeout, outputLimit)
}

// stageVerified reads the command once, checks those bytes against its pinned digest and writes
// exactly them to a private directory, so a replacement of the original after the check cannot
// run (BBF-V0-214). The caller runs the staged path and then calls cleanup.
func stageVerified(command Command) (string, func(), error) {
	executable, err := os.ReadFile(command.Path)
	if err != nil || digestBytes(executable) != command.ExecutableSHA256 {
		return "", nil, errors.New("command changed before staging")
	}
	scratch, err := os.MkdirTemp("", "corvint-behavior-control-")
	if err != nil {
		return "", nil, errors.New("command staging unavailable")
	}
	cleanup := func() { os.RemoveAll(scratch) }
	staged := filepath.Join(scratch, filepath.Base(command.Path))
	if err := os.WriteFile(staged, executable, 0700); err != nil {
		cleanup()
		return "", nil, errors.New("command staging failed")
	}
	return staged, cleanup, nil
}

// runProcessGroup is procgroup.Run; tests replace it to inject start failures.
var runProcessGroup = procgroup.Run

// textBusyRetries bounds the start retries of a just-staged executable (see startProcess).
var textBusyRetries = []time.Duration{5 * time.Millisecond, 20 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}

// startProcess runs one process group, retrying a start that failed with ETXTBSY. stageVerified
// closes its write descriptor before returning, but a fork made concurrently by another goroutine
// can inherit that descriptor and keep it open until the child's own exec closes it (close-on-exec),
// so exec of the staged copy can briefly fail "text file busy" on Linux. Nothing started in that
// case, so a bounded retry is safe; any other failure, or a start, returns at once.
func startProcess(ctx context.Context, spec procgroup.Spec) procgroup.Observation {
	observation := runProcessGroup(ctx, spec)
	for _, delay := range textBusyRetries {
		if observation.Started || !errors.Is(observation.Err, syscall.ETXTBSY) {
			break
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return observation
		case <-timer.C:
		}
		observation = runProcessGroup(ctx, spec)
	}
	return observation
}

// observeProcess runs one contained process group and returns its stdout and bounded evidence.
func observeProcess(ctx context.Context, argv []string, dir string, env []string, input []byte, timeout time.Duration, outputLimit int) ([]byte, ProcessEvidence) {
	observation := startProcess(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Stdin: input, Timeout: timeout, ShutdownTimeout: processShutdownAllowance, InputLimit: maxDocumentBytes, OutputLimit: outputLimit, StderrLimit: 1 << 20, ObserveDescendants: true})
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
	if root := acceptanceTempRoot(); root != "" {
		values["TMPDIR"] = root
	}
	for key, value := range declaredEnvironment(keys) {
		values[key] = value
	}
	return environmentList(values)
}

func environmentList(values map[string]string) []string {
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

// aggregateApplication is a marked control's application over its attempts: a kill or survivor
// carries its applied outcome; otherwise any NOT-APPLIED attempt makes the control inert.
func aggregateApplication(control PlannedControl, attempts []AttemptResult, status Status) string {
	if control.ApplicationMarker == nil {
		return ""
	}
	switch status {
	case StatusKilled:
		return ApplicationAppliedFailed
	case StatusSurvived:
		return ApplicationAppliedPassed
	}
	application := ""
	for _, attempt := range attempts {
		if attempt.Application == ApplicationNotApplied {
			return ApplicationNotApplied
		}
		if attempt.Application == ApplicationAppliedEarlyFail {
			application = ApplicationAppliedEarlyFail
		}
	}
	return application
}

func controlExecuted(result ControlResult) bool {
	for _, attempt := range result.Attempts {
		if attempt.HookProcess.Started {
			return true
		}
	}
	return false
}

func receiptOutputLimit(plan Plan, control PlannedControl) int {
	slots := 0
	for _, planned := range plan.Controls {
		if planned.Disposition == "run" {
			slots += plan.Request.Attempts
		}
	}
	// Leave room for Unicode escaping, validated artifact metadata, process evidence and framing.
	share := maxDocumentBytes / 8
	if slots >= 1 {
		share = maxDocumentBytes / (8 * slots)
	}
	// A receipt the validators accept always fits; the report bound still fails closed on the total.
	return max(share, acceptedReceiptBound(control))
}

func acceptedReceiptBound(control PlannedControl) int {
	// Every string the validators accept is at most 4096 bytes: 15 target, runner and
	// target-observation identities, one infrastructure detail, 64 artifact paths, two IDs per
	// unrelated observation, one per setup observation and three per marker observation. The factor two leaves room for JSON
	// framing, digests and escaping.
	strings := 15 + 1 + 64 + 2*len(control.UnrelatedCriteria) + len(control.RequiredSetup) + 3*maxMarkerObservations
	return 2 * strings * 4096
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
		if control.Disposition == "run" {
			seen[control.Kind] = true
		}
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

// acceptanceTempRoot recognizes the closed transport environment, not arbitrary
// inherited TMPDIR. Standalone callers retain the declared-environment policy.
func acceptanceTempRoot() string {
	if len(os.Environ()) != 4 || os.Getenv("PATH") != "/opt/homebrew/bin:/usr/bin:/bin" || os.Getenv("LANG") != "C" || os.Getenv("LC_ALL") != "C" {
		return ""
	}
	root := os.Getenv("TMPDIR")
	if root == "/tmp" {
		return ""
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || len(root) > 4096 || strings.ContainsAny(root, "\x00\r\n") {
		return ""
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return ""
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return ""
	}
	return root
}
