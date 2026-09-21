package behaviorfalsify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const helperEnvironment = "CORVINT_BEHAVIOR_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnvironment) == "1" {
		runTestHelper()
		return
	}
	os.Exit(m.Run())
}

func runTestHelper() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(2)
	}
	var invocation Invocation
	if json.Unmarshal(data, &invocation) != nil {
		os.Exit(2)
	}
	scenario := invocation.Control.Definition["scenario"]
	if os.Getenv("CORVINT_BEHAVIOR_COMMAND_ROLE") == "cleanup" {
		_ = os.Remove("artifact.json")
		_ = os.Remove("started")
		if scenario == "cleanup-fail" {
			os.Exit(7)
		}
		return
	}
	switch scenario {
	case "timeout":
		time.Sleep(2 * time.Second)
		return
	case "wait-cancel":
		if os.WriteFile("started", []byte("started\n"), 0600) != nil {
			os.Exit(2)
		}
		time.Sleep(30 * time.Second)
		return
	case "slow-termination":
		signal.Ignore(syscall.SIGTERM)
		time.Sleep(30 * time.Second)
		return
	case "crash":
		os.Exit(9)
	case "overflow":
		chunk := bytes.Repeat([]byte("x"), 1<<20)
		for index := 0; index < 33; index++ {
			_, _ = os.Stdout.Write(chunk)
		}
		return
	}
	artifactData := []byte(`{"trace":"synthetic"}`)
	if os.WriteFile("artifact.json", artifactData, 0600) != nil {
		os.Exit(2)
	}
	receipt := matchingReceipt(invocation)
	receipt.Artifacts = []Artifact{{Path: "artifact.json", SHA256: hashBytes(artifactData)}}
	switch scenario {
	case "survived":
		receipt.TestOutcome = "passed"
		receipt.TargetObservation.State = "passed"
		receipt.TargetObservation.FailureKind = ""
	case "unrelated-failure":
		receipt.Unrelated[0].State = "failed"
		receipt.Unrelated[0].FailureKind = "assertion"
	case "wrong-assertion":
		receipt.TargetObservation.AssertionID = "assertion:other"
	case "retry":
		receipt.Retry = 1
	case "stale":
		receipt.Target.TestRevision = strings.Repeat("b", 40)
	case "stale-artifact":
		receipt.Artifacts[0].SHA256 = digestN("stale-artifact")
	case "html-expansion":
		receipt.Unrelated[0].AssertionID = strings.Repeat("<", 2<<20)
	}
	encoded, err := Encode(receipt)
	if err != nil {
		os.Exit(2)
	}
	if _, err := os.Stdout.Write(encoded); err != nil {
		os.Exit(2)
	}
}

func TestBBFV0001PlanIsCanonicalAndRequiresApproval(t *testing.T) {
	request := testRequest(t, []ControlSpec{
		{ID: "z", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "no hook"}},
		{ID: "a", Kind: WrongExpectedValue, Disposition: "not_run", Definition: map[string]string{"reason": "deferred"}},
	})
	first, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildPlan(request)
	t.Run("BBF-V0-001 canonical plan requires exact approval", func(t *testing.T) {
		if err != nil || first.Digest != second.Digest || first.Controls[0].ID != "a" || first.Controls[1].ID != "z" {
			t.Fatalf("canonical plan mismatch: err=%v first=%+v second=%+v", err, first, second)
		}
	})
	if _, err := Execute(context.Background(), first, ""); err == nil || err.Error() != "operator-authorization-required" {
		t.Fatalf("missing approval error = %v", err)
	}
	encoded, err := Encode(first)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Plan
	if err := Decode(encoded, &decoded); err != nil || !samePlan(first, decoded) {
		t.Fatalf("plan round-trip: err=%v decoded=%+v", err, decoded)
	}
	report, err := Execute(context.Background(), first, first.Digest)
	if err != nil || report.Counts[StatusNotRun] != 1 || report.Counts[StatusNotSupported] != 1 || report.Fallback != "full-relevant-suite" || report.MutationScore.Defined {
		t.Fatalf("report = %+v, err=%v", report, err)
	}
}

func TestBBFV0003ClosedControlVocabulary(t *testing.T) {
	t.Run("BBF-V0-003 closed controls never convert unsupported work to kills", func(t *testing.T) {
		kinds := []ControlKind{
			WrongLocator, WrongExpectedValue, OmittedAssertion, OmittedEvent,
			ReorderedEvent, WrongProject, SuppressedPersistence, OppositeBranch,
			ChangedFixtureValue,
		}
		controls := make([]ControlSpec, 0, len(kinds))
		for _, kind := range kinds {
			controls = append(controls, ControlSpec{
				ID: string(kind), Kind: kind, Disposition: "not_supported",
				Definition: map[string]string{"reason": "no caller hook"},
			})
		}
		request := testRequest(t, controls)
		plan, err := BuildPlan(request)
		if err != nil {
			t.Fatal(err)
		}
		report, err := Execute(context.Background(), plan, plan.Digest)
		if err != nil || report.Counts[StatusNotSupported] != len(kinds) || report.Counts[StatusKilled] != 0 || report.Fallback != "full-relevant-suite" {
			t.Fatalf("unsupported controls report = %+v, err=%v", report, err)
		}
		request.Controls[0].Kind = "outside-closed-vocabulary"
		if _, err := BuildPlan(request); err == nil {
			t.Fatal("unknown control kind unexpectedly accepted")
		}
	})
}

func TestBBFV0001PlanDriftRefused(t *testing.T) {
	request := testRequest(t, []ControlSpec{{ID: "unsupported", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "fixture"}}})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Plan){
		func(candidate *Plan) { candidate.Controls[0].Ordinal = 2 },
		func(candidate *Plan) { candidate.Controls = nil },
	} {
		candidate := plan
		candidate.Controls = append([]PlannedControl(nil), plan.Controls...)
		mutate(&candidate)
		if _, err := Execute(context.Background(), candidate, plan.Digest); err == nil {
			t.Fatalf("drift error = %v", err)
		}
	}
}

func TestBBFV0011SyntheticConformance(t *testing.T) {
	plan := classificationPlan()
	control := plan.Controls[0]
	success := successfulProcess()
	base := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: success, CleanupProcess: success, WorkspaceBefore: digestN("workspace"), WorkspaceAfter: digestN("workspace")}
	tests := []struct {
		name   string
		mutate func(*AttemptResult)
		want   Status
	}{
		{name: "BBF-V0-005 expected criterion kill", mutate: func(*AttemptResult) {}, want: StatusKilled},
		{name: "BBF-V0-006 unrelated failure is invalid", mutate: func(result *AttemptResult) { result.Receipt.Unrelated[0].State = "failed" }, want: StatusInvalidControl},
		{name: "wrong assertion", mutate: func(result *AttemptResult) { result.Receipt.TargetObservation.AssertionID = "assertion:other" }, want: StatusInvalidControl},
		{name: "selector error", mutate: func(result *AttemptResult) { result.Receipt.TargetObservation.FailureKind = "selector" }, want: StatusInvalidControl},
		{name: "timeout", mutate: func(result *AttemptResult) { result.HookProcess.TimedOut = true }, want: StatusInfrastructureFailed},
		{name: "output overflow", mutate: func(result *AttemptResult) { result.HookProcess.Overflow = true }, want: StatusInfrastructureFailed},
		{name: "descendant cleanup unavailable", mutate: func(result *AttemptResult) { result.HookProcess.DescendantsGone = false }, want: StatusInfrastructureFailed},
		{name: "cleanup failure", mutate: func(result *AttemptResult) { result.CleanupProcess.Exit = 1 }, want: StatusInvalidControl},
		{name: "retry hides first outcome", mutate: func(result *AttemptResult) { result.Receipt.Retry = 1 }, want: StatusInvalidControl},
		{name: "BBF-V0-011 stale revision is invalid", mutate: func(result *AttemptResult) { result.Receipt.Target.TestRevision = strings.Repeat("d", 40) }, want: StatusInvalidControl},
		{name: "stale perturbation digest", mutate: func(result *AttemptResult) { result.Receipt.PerturbationSHA256 = digestN("stale") }, want: StatusInvalidControl},
		{name: "stale artifact digest", mutate: func(result *AttemptResult) { result.Reasons = []string{"artifact-digest-mismatch"} }, want: StatusInvalidControl},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := cloneAttempt(base)
			test.mutate(&result)
			got, _ := classifyAttempt(plan, control, result)
			if got != test.want {
				t.Fatalf("status = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBBFV0011DistinctSurvivorFixtures(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		kind       ControlKind
		definition map[string]string
	}{
		{name: "BBF-V0-011 tautological assertion", kind: OmittedAssertion, definition: map[string]string{"assertion": "always-true"}},
		{name: "hidden duplicate element", kind: WrongLocator, definition: map[string]string{"locator": "duplicate-hidden"}},
		{name: "wrong value", kind: WrongExpectedValue, definition: map[string]string{"expected": "fixture-wrong"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			plan := classificationPlan()
			control := plan.Controls[0]
			control.Kind = fixture.kind
			control.Definition = fixture.definition
			control.PerturbationSHA256 = digestJSON(fixture)
			result := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), WorkspaceBefore: digestN("workspace"), WorkspaceAfter: digestN("workspace")}
			result.Receipt.NativeReceiptSHA256 = digestN(fixture.name)
			survive(&result)
			if got, _ := classifyAttempt(plan, control, result); got != StatusSurvived {
				t.Fatalf("fixture %q status = %q", fixture.name, got)
			}
		})
	}
}

func TestBBFV0006InfrastructureReceiptIsClosed(t *testing.T) {
	plan := classificationPlan()
	control := plan.Controls[0]
	base := AttemptResult{Attempt: 1, Receipt: receiptPointer(classificationReceipt(plan, control)), HookProcess: successfulProcess(), CleanupProcess: successfulProcess(), WorkspaceBefore: digestN("workspace"), WorkspaceAfter: digestN("workspace")}
	makeInfrastructure := func() AttemptResult {
		result := cloneAttempt(base)
		result.Receipt.TestOutcome = "infrastructure_failed"
		result.Receipt.TargetObservation.State = "not_run"
		result.Receipt.TargetObservation.FailureKind = ""
		result.Receipt.Unrelated = nil
		result.Receipt.Setup = nil
		result.Receipt.Infrastructure = &InfrastructureFailure{Reason: "browser-unavailable"}
		return result
	}
	valid := makeInfrastructure()
	if got, _ := classifyAttempt(plan, control, valid); got != StatusInfrastructureFailed {
		t.Fatalf("valid infrastructure status = %q", got)
	}
	for _, mutate := range []func(*AttemptResult){
		func(result *AttemptResult) { result.Receipt.Infrastructure.Reason = "caller-controlled" },
		func(result *AttemptResult) { result.Receipt.TargetObservation.AssertionID = "assertion:other" },
		func(result *AttemptResult) { result.Receipt.TargetObservation.State = "passed" },
		func(result *AttemptResult) { result.Receipt.TestOutcome = "passed" },
	} {
		candidate := makeInfrastructure()
		mutate(&candidate)
		if got, _ := classifyAttempt(plan, control, candidate); got != StatusInvalidControl {
			t.Fatalf("contradictory infrastructure status = %q", got)
		}
	}
}

func TestBBFV0004LiveHookRestoresDisposableWorkspace(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := testRequest(t, []ControlSpec{{
		ID: "wrong-value", Kind: WrongExpectedValue, Disposition: "run",
		Definition: map[string]string{"scenario": "killed"}, Hook: testCommand(t),
		UnrelatedCriteria: []string{"criterion:unrelated"}, RequiredSetup: []string{"setup:page"},
	}})
	request.DeclaredEnvKeys = []string{helperEnvironment}
	request.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(request.DeclaredEnvKeys))
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	result := report.Results[0]
	t.Run("BBF-V0-004 approved hook restores disposable workspace", func(t *testing.T) {
		if result.Status != StatusKilled || len(result.Attempts) != 1 || result.Attempts[0].WorkspaceBefore != result.Attempts[0].WorkspaceAfter || !processSucceeded(result.Attempts[0].HookProcess) || !processSucceeded(result.Attempts[0].CleanupProcess) {
			t.Fatalf("live result = %+v", result)
		}
	})
	if _, err := os.Stat(filepath.Join(request.DisposableRoot, "artifact.json")); !os.IsNotExist(err) {
		t.Fatalf("artifact survived cleanup: %v", err)
	}
}

func TestBBFV0005LiveTimeoutIsInfrastructureFailure(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := testRequest(t, []ControlSpec{{
		ID: "timeout", Kind: OmittedEvent, Disposition: "run",
		Definition: map[string]string{"scenario": "timeout"}, Hook: testCommand(t),
		UnrelatedCriteria: []string{"criterion:unrelated"}, RequiredSetup: []string{"setup:page"},
	}})
	request.DeclaredEnvKeys = []string{helperEnvironment}
	request.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(request.DeclaredEnvKeys))
	request.TimeoutSeconds = 1
	request.WallClockSeconds = 12
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	attempt := report.Results[0].Attempts[0]
	if report.Results[0].Status != StatusInfrastructureFailed || !attempt.HookProcess.TimedOut || !attempt.HookProcess.OwnedCleanup || attempt.WorkspaceBefore != attempt.WorkspaceAfter {
		t.Fatalf("timeout result = %+v", report.Results[0])
	}
}

func TestBBFV0010SlowTerminationPreservesCleanupBudget(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := liveRequest(t, []ControlSpec{liveControl(t, "slow-termination", "slow-termination")})
	request.TimeoutSeconds = 1
	request.WallClockSeconds = 12
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	attempt := report.Results[0].Attempts[0]
	if report.Results[0].Status != StatusInfrastructureFailed || !attempt.HookProcess.TimedOut || !attempt.CleanupProcess.Started || !processSucceeded(attempt.CleanupProcess) || attempt.WorkspaceBefore != attempt.WorkspaceAfter {
		t.Fatalf("slow termination report = %+v", report.Results[0])
	}
}

func TestBBFV0004LiveFailureMatrix(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	for _, test := range []struct {
		scenario string
		want     Status
	}{
		{"crash", StatusInfrastructureFailed},
		{"overflow", StatusInfrastructureFailed},
		{"cleanup-fail", StatusInvalidControl},
		{"stale-artifact", StatusInvalidControl},
		{"html-expansion", StatusInvalidControl},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			request := liveRequest(t, []ControlSpec{liveControl(t, test.scenario, test.scenario)})
			plan, err := BuildPlan(request)
			if err != nil {
				t.Fatal(err)
			}
			report, err := Execute(context.Background(), plan, plan.Digest)
			if err != nil || report.Results[0].Status != test.want || report.Executed != 1 {
				t.Fatalf("scenario=%s report=%+v err=%v", test.scenario, report, err)
			}
		})
	}
}

func TestBBFV0007CancellationStopsFurtherExecution(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := liveRequest(t, []ControlSpec{
		liveControl(t, "first", "wait-cancel"),
		liveControl(t, "second", "killed"),
	})
	request.Attempts = 16
	request.TimeoutSeconds = 30
	request.WallClockSeconds = 60
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	type execution struct {
		report Report
		err    error
	}
	finished := make(chan execution, 1)
	go func() {
		report, runErr := Execute(ctx, plan, plan.Digest)
		finished <- execution{report: report, err: runErr}
	}()
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(request.DisposableRoot, "started")); err == nil {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatal("control hook did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	outcome := <-finished
	report, err := outcome.report, outcome.err
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-007 cancellation preserves first attempt and later ordinal", func(t *testing.T) {
		if report.Executed != 1 || report.Results[0].Status != StatusInfrastructureFailed || len(report.Results[0].Attempts) != 1 || !report.Results[0].Attempts[0].CleanupProcess.Started || report.Results[1].Status != StatusNotRun || len(report.Results[1].Attempts) != 0 {
			t.Fatalf("cancellation report = %+v", report)
		}
	})
}

func TestBBFV0009AggregateCountsAndFallback(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := liveRequest(t, []ControlSpec{
		liveControl(t, "killed", "killed"),
		{ID: "unsupported", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "fixture"}},
		{ID: "deferred", Kind: OmittedAssertion, Disposition: "not_run", Definition: map[string]string{"reason": "fixture"}},
	})
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("BBF-V0-008 partial controls retain gaps and suite fallback", func(t *testing.T) {
		if report.CompleteVocabulary || report.Fallback != "full-relevant-suite" || len(report.CoverageGaps) != 2 {
			t.Fatalf("coverage report = %+v", report)
		}
	})
	t.Run("BBF-V0-009 raw status counts and mutation denominator", func(t *testing.T) {
		if report.Requested != 3 || report.Supported != 1 || report.Executed != 1 || report.Counts[StatusKilled] != 1 || report.Counts[StatusNotSupported] != 1 || report.Counts[StatusNotRun] != 1 || !report.MutationScore.Defined || report.MutationScore.Killed != 1 || report.MutationScore.Denominator != 1 || len(report.Counts) != 6 {
			t.Fatalf("aggregate report = %+v", report)
		}
	})
	t.Run("BBF-V0-012 observations never authorize suite narrowing", func(t *testing.T) {
		if report.Fallback != "full-relevant-suite" || len(report.Limitations) != 3 || report.Limitations[0] != "caller-owned hook semantics are not authenticated" || report.Limitations[1] != "results cover only the exact approved controls and never authorize suite narrowing" {
			t.Fatalf("limitations report = %+v", report)
		}
	})
}

func TestBBFV0010EncodingIsBounded(t *testing.T) {
	t.Run("BBF-V0-010 bounded report rejects oversized output", func(t *testing.T) {
		if _, err := Encode(Report{Limitations: []string{strings.Repeat("x", maxDocumentBytes)}}); err == nil {
			t.Fatal("oversized report unexpectedly encoded")
		}
	})
	data, err := Encode(Report{Limitations: []string{strings.Repeat("<", 6<<20)}})
	if err != nil || len(data) >= maxDocumentBytes {
		t.Fatalf("HTML-safe content expanded beyond wire bound: bytes=%d err=%v", len(data), err)
	}
}

func TestBBFV0010InvocationInputUsesDocumentBound(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	input := bytes.Repeat([]byte("x"), (16<<20)+1)
	_, evidence := runPinned(context.Background(), *testCommand(t), t.TempDir(), []string{helperEnvironment}, "control", input, 5*time.Second, 1<<20)
	if !evidence.Started || strings.Contains(evidence.Error, "stdin exceeds") {
		t.Fatalf("17 MiB invocation rejected at process boundary: %+v", evidence)
	}
}

func TestBBFV0010ExecutionOutputOverflowRemainsReportable(t *testing.T) {
	t.Setenv(helperEnvironment, "1")
	request := liveRequest(t, []ControlSpec{liveControl(t, "overflow", "overflow")})
	request.Attempts = 2
	request.WallClockSeconds = 60
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Execute(context.Background(), plan, plan.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Results[0].Status != StatusInfrastructureFailed || len(report.Results[0].Attempts) != 2 {
		t.Fatalf("overflow report = %+v", report.Results[0])
	}
	for _, attempt := range report.Results[0].Attempts {
		if !attempt.HookProcess.Overflow {
			t.Fatalf("overflow attempt = %+v", attempt)
		}
	}
}

func TestBBFV0002HookDriftRefused(t *testing.T) {
	copyPath := filepath.Join(t.TempDir(), "hook")
	executable, err := os.ReadFile(testCommand(t).Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, executable, 0700); err != nil {
		t.Fatal(err)
	}
	request := liveRequest(t, []ControlSpec{liveControl(t, "drift", "killed")})
	request.Controls[0].Hook = &Command{Path: copyPath, ExecutableSHA256: hashBytes(executable)}
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, append(executable, 0), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), plan, plan.Digest); err == nil {
		t.Fatal("drifted hook unexpectedly executed")
	}
}

func TestBBFV0004StaleArtifactDigestRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), []byte("actual"), 0600); err != nil {
		t.Fatal(err)
	}
	if retained, reasons := validateArtifacts(root, []Artifact{{Path: "artifact.json", SHA256: digestN("stale")}}); len(retained) != 0 || len(reasons) != 1 || reasons[0] != "artifact-digest-mismatch" {
		t.Fatalf("retained=%v reasons=%v", retained, reasons)
	}
}

func TestBBFV0004ArtifactParentSymlinkIsRefused(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	data := []byte("escaped")
	if err := os.WriteFile(filepath.Join(outside, "trace.zip"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	retained, reasons := validateArtifacts(root, []Artifact{{Path: filepath.Join("link", "trace.zip"), SHA256: hashBytes(data)}})
	if len(retained) != 0 || len(reasons) != 1 || reasons[0] != "artifact-path-invalid" {
		t.Fatalf("retained=%v reasons=%v", retained, reasons)
	}
}

func TestBBFV0002PlanRejectsStaleRepositoryBindings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "BBF-V0-002 stale revision binding", mutate: func(request *Request) { request.Target.TestRevision = strings.Repeat("d", 40) }},
		{name: "contract digest", mutate: func(request *Request) { request.Target.ContractSHA256 = digestN("stale-contract") }},
		{name: "config digest", mutate: func(request *Request) { request.Runner.ConfigSHA256 = digestN("stale-config") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := testRequest(t, []ControlSpec{{ID: "unsupported", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "fixture"}}})
			test.mutate(&request)
			if _, err := BuildPlan(request); err == nil {
				t.Fatal("stale binding unexpectedly accepted")
			}
		})
	}
}

func TestBBFV0002PlanIsolatedFromAmbientGit(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "poison"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.bare")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	request := testRequest(t, []ControlSpec{{ID: "unsupported", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "fixture"}}})
	if _, err := BuildPlan(request); err != nil {
		t.Fatalf("ambient Git settings changed repository binding: %v", err)
	}
}

func TestBBFV0004ReservedEnvironmentCannotOverrideBoundary(t *testing.T) {
	request := testRequest(t, []ControlSpec{{ID: "unsupported", Kind: WrongLocator, Disposition: "not_supported", Definition: map[string]string{"reason": "fixture"}}})
	request.DeclaredEnvKeys = []string{"CORVINT_BEHAVIOR_COMMAND_ROLE"}
	request.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(request.DeclaredEnvKeys))
	if _, err := BuildPlan(request); err == nil {
		t.Fatal("reserved runner environment unexpectedly accepted")
	}
}

func matchingReceipt(invocation Invocation) HookReceipt {
	unrelated := make([]CriterionObservation, 0, len(invocation.Control.UnrelatedCriteria))
	for _, criterion := range invocation.Control.UnrelatedCriteria {
		unrelated = append(unrelated, CriterionObservation{CriterionID: criterion, AssertionID: "assertion:" + criterion, State: "passed"})
	}
	setup := make([]SetupObservation, 0, len(invocation.Control.RequiredSetup))
	for _, id := range invocation.Control.RequiredSetup {
		setup = append(setup, SetupObservation{ID: id, State: "passed"})
	}
	return HookReceipt{
		Schema: HookSchema, PlanDigest: invocation.PlanDigest, PerturbationSHA256: invocation.Control.PerturbationSHA256,
		Target: invocation.Target, Runner: invocation.Runner, Attempt: invocation.Attempt, Retry: 0,
		NativeReceiptSHA256: digestN("native"), TestOutcome: "failed",
		TargetObservation: CriterionObservation{CriterionID: invocation.Target.CriterionID, AssertionID: invocation.Target.AssertionID, State: "failed", FailureKind: "assertion"},
		Unrelated:         unrelated, Setup: setup, Artifacts: []Artifact{},
	}
}

func testRequest(t *testing.T, controls []ControlSpec) Request {
	t.Helper()
	workspace := t.TempDir()
	marker := filepath.Join(workspace, MarkerName)
	if err := os.WriteFile(marker, []byte("disposable browser fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	repository := repositoryRoot(t)
	head := strings.TrimSpace(string(mustGitBytes(t, repository, "rev-parse", "HEAD")))
	contractFile := "docs/specs/browser-behavior-falsification-v0.md"
	testFile := "internal/jstestprovider/external_test.go"
	configFile := "go.mod"
	return Request{
		Target: TargetIdentity{
			ContractID: "contract:checkout", ContractSHA256: hashBytes(mustGitBytes(t, repository, "cat-file", "blob", head+":"+contractFile)), CriterionID: "criterion:total", AssertionID: "assertion:total",
			ApplicationRevision: head, TestRevision: head, DocumentationRevision: head, ContractFile: contractFile,
			TestID: "test:checkout", TestFile: testFile, TestLine: 12, TestTitle: "checkout total", Project: "chromium",
		},
		Runner:   RunnerIdentity{Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "153.0.8010.48", ConfigFile: configFile, ConfigSHA256: hashBytes(mustGitBytes(t, repository, "cat-file", "blob", head+":"+configFile)), EnvironmentSHA256: digestJSON(map[string]string{})},
		Controls: controls, Cleanup: *testCommand(t), DisposableRoot: workspace, Repositories: RepositoryRoots{Application: repository, Test: repository, Documentation: repository}, MarkerSHA256: mustFileDigest(t, marker), Attempts: 1, TimeoutSeconds: 5, WallClockSeconds: 30, ExternalState: "none",
	}
}

func liveRequest(t *testing.T, controls []ControlSpec) Request {
	t.Helper()
	request := testRequest(t, controls)
	request.DeclaredEnvKeys = []string{helperEnvironment}
	request.Runner.EnvironmentSHA256 = digestJSON(declaredEnvironment(request.DeclaredEnvKeys))
	return request
}

func liveControl(t *testing.T, id, scenario string) ControlSpec {
	t.Helper()
	return ControlSpec{
		ID: id, Kind: WrongExpectedValue, Disposition: "run",
		Definition: map[string]string{"scenario": scenario}, Hook: testCommand(t),
		UnrelatedCriteria: []string{"criterion:unrelated"}, RequiredSetup: []string{"setup:page"},
	}
}

func testCommand(t *testing.T) *Command {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := digestFile(path, maxArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	return &Command{Path: path, ExecutableSHA256: digest}
}

func classificationPlan() Plan {
	target := TargetIdentity{ContractID: "contract:x", ContractSHA256: digestN("contract"), CriterionID: "criterion:target", AssertionID: "assertion:target", ApplicationRevision: strings.Repeat("a", 40), TestRevision: strings.Repeat("b", 40), DocumentationRevision: strings.Repeat("c", 40), ContractFile: "contract.json", TestID: "test:x", TestFile: "x.spec.ts", TestLine: 1, TestTitle: "x", Project: "chromium"}
	runner := RunnerIdentity{Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "153", ConfigFile: "playwright.config.ts", ConfigSHA256: digestN("config"), EnvironmentSHA256: digestN("env")}
	control := PlannedControl{ControlSpec: ControlSpec{ID: "control:x", Kind: WrongLocator, Disposition: "run", Definition: map[string]string{"locator": "wrong"}, UnrelatedCriteria: []string{"criterion:unrelated"}, RequiredSetup: []string{"setup:page"}}, Ordinal: 1, PerturbationSHA256: digestN("perturbation")}
	return Plan{Schema: PlanSchema, Request: Request{Target: target, Runner: runner, Attempts: 1}, Controls: []PlannedControl{control}, Digest: digestN("plan")}
}

func classificationReceipt(plan Plan, control PlannedControl) HookReceipt {
	return matchingReceipt(Invocation{PlanDigest: plan.Digest, Attempt: 1, Target: plan.Request.Target, Runner: plan.Request.Runner, Control: control})
}

func successfulProcess() ProcessEvidence {
	return ProcessEvidence{Exit: 0, Started: true, Completed: true, OwnedCleanup: true, DescendantsGone: true, StdoutSHA256: digestN("stdout"), StderrSHA256: digestN("stderr")}
}

func survive(result *AttemptResult) {
	result.Receipt.TargetObservation.State = "passed"
	result.Receipt.TargetObservation.FailureKind = ""
	result.Receipt.TestOutcome = "passed"
}

func cloneAttempt(in AttemptResult) AttemptResult {
	out := in
	receipt := *in.Receipt
	receipt.Unrelated = append([]CriterionObservation(nil), in.Receipt.Unrelated...)
	receipt.Setup = append([]SetupObservation(nil), in.Receipt.Setup...)
	receipt.Artifacts = append([]Artifact(nil), in.Receipt.Artifacts...)
	out.Receipt = &receipt
	out.Reasons = append([]string(nil), in.Reasons...)
	return out
}

func receiptPointer(receipt HookReceipt) *HookReceipt { return &receipt }

func digestN(value string) string { return hashBytes([]byte(value)) }

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func mustFileDigest(t *testing.T, path string) string {
	t.Helper()
	digest, err := digestFile(path, maxArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return resolvePath(directory)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root not found")
		}
		directory = parent
	}
}

func mustGitBytes(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	data, err := gitBytes(root, args...)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
