package playwrightminimize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestPSMV0001PlanIsExactBoundedAndReadOnly(t *testing.T) {
	request := qualifiedRequest()
	before := cloneRequest(request)
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request, before) {
		t.Fatal("BuildPlan mutated its request")
	}
	if len(plan.Trials) != request.Limits.MaxTrials || !plan.Complete || plan.Digest != planDigest(plan) {
		t.Fatalf("unexpected bounded plan: trials=%d complete=%v digest=%q", len(plan.Trials), plan.Complete, plan.Digest)
	}
	for index, trial := range plan.Trials {
		if trial.Ordinal != index+1 || trial.ID == "" || trial.Identity.Fixed != request.OriginalFailure.Identity.Fixed || trial.ResetPolicy != request.ResetPolicy {
			t.Fatalf("trial %d is not exact: %#v", index, trial)
		}
	}
}

func TestPSMV0002ExecutionRequiresSeparateAuthorization(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	calls := 0
	_, err := Execute(context.Background(), plan, Authorization{PlanDigest: plan.Digest}, RunnerFunc(func(context.Context, Trial) (TrialReceipt, error) {
		calls++
		return TrialReceipt{}, nil
	}))
	if err == nil || calls != 0 {
		t.Fatalf("unauthorized execution reached runner: err=%v calls=%d", err, calls)
	}
}

func TestPSMV0003TruePredecessorLeakQualification(t *testing.T) {
	plan := mustPlan(t, requestWithClasses(FailureFixture))
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		failed := trial.Kind == TrialReproduction || (trial.Kind == TrialOrdered && slices.Equal(trial.Context, []string{"leak"}))
		if failed {
			return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureFixture, EvidenceDigest: digest("leak"), Summary: "predecessor leaked browser state"})
		}
		return receiptFor(trial, OutcomePassed)
	})
	if report.Ordered == nil || !slices.Equal(report.Ordered.Smallest, []string{"leak"}) || report.Load != nil || report.Diagnosis != "ordered-predecessor-observed" || !report.Confident {
		t.Fatalf("unexpected predecessor diagnosis: %#v", report)
	}
	if report.Ordered.Minimality != "proved-within-bounded-universe" || report.Ordered.Members[0].Status != "necessary-in-observed-universe" {
		t.Fatalf("unexpected minimality: %#v", report.Ordered)
	}
}

func TestPSMV0004LoadOnlyQualification(t *testing.T) {
	plan := mustPlan(t, requestWithClasses(FailureResourceExhaustion, FailureInfrastructure))
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		failed := trial.Kind == TrialReproduction || (trial.Kind == TrialLoad && len(trial.Context) == 2)
		if failed {
			return receiptFor(trial, OutcomeFailed,
				FailureObservation{Class: FailureResourceExhaustion, EvidenceDigest: digest("resource"), Summary: "worker memory exhausted"},
				FailureObservation{Class: FailureInfrastructure, EvidenceDigest: digest("infra"), Summary: "browser process exited"},
			)
		}
		return receiptFor(trial, OutcomePassed)
	})
	if report.Ordered != nil || report.Load == nil || !slices.Equal(report.Load.Smallest, []string{"load-a", "load-b"}) || report.Diagnosis != "unordered-load-observed" || !report.Confident {
		t.Fatalf("unexpected load diagnosis: %#v", report)
	}
	if countClass(report.ObservedFailures, FailureResourceExhaustion) == 0 || countClass(report.ObservedFailures, FailureInfrastructure) == 0 {
		t.Fatal("observed classifications were not preserved")
	}
}

func TestPSMV0005ApplicationRestartInvalidatesTrial(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		receipt := receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureRestart, EvidenceDigest: digest("restart"), Summary: "application instance changed"})
		receipt.ApplicationAttestationPublish = digest("new-instance")
		receipt.Digest = receiptDigest(receipt)
		return receipt
	})
	if report.Status != "incomplete" || report.Diagnosis != "none" || !hasReason(report.Trials, "application-restarted") || countClass(report.ObservedFailures, FailureRestart) == 0 {
		t.Fatalf("restart was not retained as an invalid trial: %#v", report)
	}
}

func TestPSMV0006CleanupFailureInvalidatesTrial(t *testing.T) {
	for _, test := range []struct {
		name   string
		reason string
		alter  func(*TrialReceipt)
	}{
		{name: "cleanup", reason: "cleanup-failed", alter: func(receipt *TrialReceipt) { receipt.CleanupSucceeded = false }},
		{name: "reset", reason: "reset-failed", alter: func(receipt *TrialReceipt) { receipt.ResetSucceeded = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := mustPlan(t, qualifiedRequest())
			report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
				receipt := receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureFixture, EvidenceDigest: digest(test.name + "-observation"), Summary: "fixture lifecycle failure"})
				test.alter(&receipt)
				receipt.Digest = receiptDigest(receipt)
				return receipt
			})
			if report.Status != "incomplete" || !hasReason(report.Trials, test.reason) || len(report.ObservedFailures) == 0 {
				t.Fatalf("%s was not invalidated and retained: %#v", test.name, report)
			}
		})
	}
}

func TestPSMV0007NondeterministicReproductionStopsMinimization(t *testing.T) {
	plan := mustPlan(t, requestWithClasses(FailureSynchronization))
	calls := 0
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		calls++
		if trial.Kind == TrialReproduction && trial.Repetition == 1 {
			return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureSynchronization, EvidenceDigest: digest("timing"), Summary: "timing window"})
		}
		return receiptFor(trial, OutcomePassed)
	})
	if report.Diagnosis != "nondeterministic" || calls != plan.Request.Limits.Repetitions || report.Confident {
		t.Fatalf("nondeterminism did not stop minimization: calls=%d report=%#v", calls, report)
	}
}

func TestPSMV0008IsolatedProductRegressionStopsMinimization(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	calls := 0
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		calls++
		class := FailureAssertion
		if trial.Kind == TrialIsolation {
			class = FailureProduct
		}
		return receiptFor(trial, OutcomeFailed, FailureObservation{Class: class, EvidenceDigest: digest(string(class)), Summary: "isolated failure"})
	})
	if report.Diagnosis != "isolated-product-regression" || calls != 2*plan.Request.Limits.Repetitions || report.Confident {
		t.Fatalf("isolated regression did not stop minimization: calls=%d report=%#v", calls, report)
	}
}

func TestPSMV0009NotReproducedHasNoMinimizationOrClassification(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	calls := 0
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		calls++
		return receiptFor(trial, OutcomePassed)
	})
	if report.Status != "not_reproduced" || report.Diagnosis != "none" || calls != plan.Request.Limits.Repetitions || len(report.ObservedFailures) != 0 || report.Ordered != nil || report.Load != nil {
		t.Fatalf("not-reproduced result overclaimed: calls=%d report=%#v", calls, report)
	}
}

func TestPSMV0010MissingComposedQualificationsBlocksConfidence(t *testing.T) {
	request := requestWithClasses(FailureFixture)
	request.Qualification.RunnerQualified = false
	request.Qualification.RunnerReceiptDigest = ""
	request.Qualification.StabilityQualified = false
	request.Qualification.StabilityReceiptDigest = ""
	request.Qualification.ApplicationAttested = false
	request.Qualification.ApplicationReceiptDigest = ""
	plan := mustPlan(t, request)
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		if trial.Kind == TrialReproduction || (trial.Kind == TrialOrdered && slices.Equal(trial.Context, []string{"leak"})) {
			return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureFixture, EvidenceDigest: digest("missing-deps"), Summary: "synthetic leak"})
		}
		return receiptFor(trial, OutcomePassed)
	})
	if report.Ordered == nil || report.Confident || !slices.Contains(report.Blockers, "runner-qualification-missing") || !slices.Contains(report.Blockers, "stability-identity-missing") || !slices.Contains(report.Blockers, "application-attestation-missing") {
		t.Fatalf("missing composed contracts did not block confidence: %#v", report)
	}
}

func TestPSMV0011WallClockAndTrialBoundsStayVisible(t *testing.T) {
	request := qualifiedRequest()
	request.Limits.MaxTrials = 8
	plan := mustPlan(t, request)
	if plan.Complete {
		t.Fatal("truncated plan claimed complete")
	}
	now := time.Unix(0, 0)
	clock := func() time.Time { return now }
	runner := RunnerFunc(func(_ context.Context, trial Trial) (TrialReceipt, error) {
		now = now.Add(plan.Request.Limits.WallClock)
		return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureAssertion, EvidenceDigest: digest("bounded"), Summary: "original failure"}), nil
	})
	report, err := executeWithClock(context.Background(), plan, Authorization{OperatorApproved: true, PlanDigest: plan.Digest}, runner, clock)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "incomplete" || !slices.Contains(report.Blockers, "wall-clock-budget-exhausted") {
		t.Fatalf("budget exhaustion was hidden: %#v", report)
	}
}

func TestPSMV0012IncompleteReproductionCannotBecomeNotReproduced(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	now := time.Unix(0, 0)
	runner := RunnerFunc(func(_ context.Context, trial Trial) (TrialReceipt, error) {
		now = now.Add(plan.Request.Limits.WallClock)
		return receiptFor(trial, OutcomePassed), nil
	})
	report, err := executeWithClock(context.Background(), plan, Authorization{OperatorApproved: true, PlanDigest: plan.Digest}, runner, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if report.Status == "not_reproduced" || !slices.Contains(report.Blockers, "reproduction-incomplete") || !hasReason(report.Trials, "wall-clock-budget-exhausted") {
		t.Fatalf("partial reproduction became conclusive: %#v", report)
	}
}

func TestPSMV0013RunnerGetsDeadlineAndErrorReceiptIsRetained(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	report, err := Execute(context.Background(), plan, Authorization{OperatorApproved: true, PlanDigest: plan.Digest}, RunnerFunc(func(ctx context.Context, trial Trial) (TrialReceipt, error) {
		if _, present := ctx.Deadline(); !present {
			t.Fatal("runner context has no wall-clock deadline")
		}
		receipt := receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureInfrastructure, EvidenceDigest: digest("runner-error"), Summary: "runner returned evidence and error"})
		return receipt, errors.New("synthetic runner error")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Trials) != plan.Request.Limits.Repetitions || report.Trials[0].Receipt == nil || report.Trials[0].ExecutionError == "" || countClass(report.ObservedFailures, FailureInfrastructure) != plan.Request.Limits.Repetitions {
		t.Fatalf("runner error erased its receipt: %#v", report)
	}
}

func TestPSMV0014NecessityRequiresMatchingTopologyAndSupportingReceipts(t *testing.T) {
	request := requestWithClasses(FailureFixture)
	request.IsolatedPass.Identity.WorkerTopology = WorkerTopology{Workers: 2, Policy: "different-isolation"}
	plan := mustPlan(t, request)
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		if trial.Kind == TrialReproduction || (trial.Kind == TrialOrdered && slices.Equal(trial.Context, []string{"leak"})) {
			return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureFixture, EvidenceDigest: digest("topology-leak"), Summary: "predecessor leak"})
		}
		return receiptFor(trial, OutcomePassed)
	})
	if report.Ordered == nil || report.Ordered.Members[0].Status != "unproven" {
		t.Fatalf("necessity crossed worker topologies: %#v", report.Ordered)
	}
	passingDigest := ""
	for _, result := range report.Trials {
		if result.Trial.Kind == TrialOrdered && result.Receipt != nil && result.Receipt.Outcome == OutcomePassed {
			passingDigest = result.Receipt.Digest
			break
		}
	}
	if passingDigest == "" || !slices.Contains(report.Ordered.ReceiptDigests, passingDigest) {
		t.Fatalf("minimality-supporting pass receipt missing: %#v", report.Ordered)
	}
}

func TestPSMV0015DuplicateBaselineDigestRefuses(t *testing.T) {
	request := qualifiedRequest()
	request.IsolatedPass.ReceiptDigest = request.OriginalFailure.ReceiptDigest
	if _, err := BuildPlan(request); err == nil {
		t.Fatal("contradictory baselines shared one immutable receipt digest")
	}
}

func TestPSMV0016InfrastructureIsolationIsNotProductAttribution(t *testing.T) {
	plan := mustPlan(t, qualifiedRequest())
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		class := FailureAssertion
		if trial.Kind == TrialIsolation {
			class = FailureInfrastructure
		}
		return receiptFor(trial, OutcomeFailed, FailureObservation{Class: class, EvidenceDigest: digest(string(class)), Summary: "retained failure"})
	})
	if report.Diagnosis != "isolated-failure-observed" {
		t.Fatalf("infrastructure failure was attributed to product: %#v", report)
	}
}

func TestPSMV0017CancellationDuringFinalTrialCannotPublishConfidence(t *testing.T) {
	plan := mustPlan(t, requestWithClasses(FailureFixture))
	ctx, cancel := context.WithCancel(context.Background())
	report, err := Execute(ctx, plan, Authorization{OperatorApproved: true, PlanDigest: plan.Digest}, RunnerFunc(func(_ context.Context, trial Trial) (TrialReceipt, error) {
		if trial.Ordinal == len(plan.Trials) {
			cancel()
		}
		if trial.Kind == TrialReproduction || (trial.Kind == TrialOrdered && slices.Equal(trial.Context, []string{"leak"})) {
			return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureFixture, EvidenceDigest: digest("cancel-leak"), Summary: "predecessor leak"}), nil
		}
		return receiptFor(trial, OutcomePassed), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if report.Status == "complete" || report.Confident || !slices.Contains(report.Blockers, "execution-context-cancelled") || !hasReason(report.Trials, "execution-context-cancelled") {
		t.Fatalf("cancelled final trial published confidence: %#v", report)
	}
}

func TestPSMV0018InvalidRepetitionOutranksMixedClasses(t *testing.T) {
	request := qualifiedRequest()
	request.Limits.Repetitions = 3
	request.Limits.MaxTrials = 27
	plan := mustPlan(t, request)
	report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
		class := FailureFixture
		if trial.Repetition > 1 {
			class = FailureInfrastructure
		}
		receipt := receiptFor(trial, OutcomeFailed, FailureObservation{Class: class, EvidenceDigest: digest(string(class)), Summary: "retained reproduction failure"})
		if trial.Kind == TrialReproduction && trial.Repetition == 3 {
			receipt.CleanupSucceeded = false
			receipt.Digest = receiptDigest(receipt)
		}
		return receipt
	})
	if report.Status != "incomplete" || report.Diagnosis != "none" || !hasReason(report.Trials, "cleanup-failed") || !slices.Contains(report.Blockers, "reproduction-invalid") {
		t.Fatalf("mixed classes masked invalid repetition: %#v", report)
	}
}

func TestPSMV0019FailureSignatureMismatchCannotMinimize(t *testing.T) {
	for _, kind := range []TrialKind{TrialReproduction, TrialOrdered, TrialLoad} {
		for _, class := range []FailureClass{FailureFixture, FailureSynchronization} {
			t.Run(string(kind)+"/"+string(class), func(t *testing.T) {
				plan := mustPlan(t, qualifiedRequest())
				report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
					if trial.Kind == kind {
						return receiptFor(trial, OutcomeFailed, FailureObservation{Class: class, EvidenceDigest: digest("new-run"), Summary: "different failure"})
					}
					if trial.Kind == TrialReproduction {
						return receiptFor(trial, OutcomeFailed, FailureObservation{Class: FailureAssertion, EvidenceDigest: digest("fresh-assertion"), Summary: "fresh observation"})
					}
					return receiptFor(trial, OutcomePassed)
				})
				if report.Confident || report.Status != "incomplete" || report.Ordered != nil || report.Load != nil || !slices.Contains(report.Blockers, "original-failure-signature-mismatch") || countClass(report.ObservedFailures, class) == 0 {
					t.Fatalf("changed failure was minimized or erased: %#v", report)
				}
			})
		}
	}
}

func TestPSMV0020FailureSignatureUsesExactClassSetNotEvidenceDigest(t *testing.T) {
	for _, classes := range [][]FailureClass{{FailureAssertion}, {FailureAssertion, FailureFixture}, {FailureFixture, FailureAssertion, FailureFixture}, {FailureAssertion, FailureFixture, FailureSynchronization}} {
		request := requestWithClasses(FailureAssertion, FailureFixture)
		plan := mustPlan(t, request)
		report := runSynthetic(t, plan, func(trial Trial) TrialReceipt {
			if trial.Kind != TrialReproduction && !(trial.Kind == TrialOrdered && slices.Equal(trial.Context, []string{"leak"})) {
				return receiptFor(trial, OutcomePassed)
			}
			var failures []FailureObservation
			for _, class := range classes {
				failures = append(failures, FailureObservation{Class: class, EvidenceDigest: digest("new-" + string(class)), Summary: "new evidence"})
			}
			return receiptFor(trial, OutcomeFailed, failures...)
		})
		if report.Confident != (len(classes) > 1 && !slices.Contains(classes, FailureSynchronization)) {
			t.Fatalf("class-set comparison incorrect: classes=%v report=%#v", classes, report)
		}
	}
}

func requestWithClasses(classes ...FailureClass) Request {
	request := qualifiedRequest()
	request.OriginalFailure.Failures = nil
	for _, class := range classes {
		request.OriginalFailure.Failures = append(request.OriginalFailure.Failures, FailureObservation{Class: class, EvidenceDigest: digest("original-" + string(class)), Summary: "original retained observation"})
	}
	return request
}

func qualifiedRequest() Request {
	fixed := FixedIdentity{
		TestRevision: digestOID("tests"), ApplicationRevision: digestOID("app"), ConfigDigest: digest("config"),
		Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "140.0", Project: "desktop",
		FixtureSchema: "fixture/1", FixtureDigest: digest("fixture"), SeedIdentity: digest("seed"), ApplicationAttestationDigest: digest("instance"),
	}
	originalIdentity := RunIdentity{Fixed: fixed, Order: []string{"noise", "leak", "target"}, WorkerTopology: WorkerTopology{Workers: 1, Policy: "serial"}}
	isolatedIdentity := RunIdentity{Fixed: fixed, Order: []string{"target"}, WorkerTopology: WorkerTopology{Workers: 1, Policy: "serial"}}
	return Request{
		Target: "target", Predecessors: []string{"noise", "leak"}, LoadMembers: []string{"load-a", "load-b"},
		OriginalFailure: Baseline{ReceiptDigest: digest("original"), Identity: originalIdentity, Outcome: OutcomeFailed, Failures: []FailureObservation{{Class: FailureAssertion, EvidenceDigest: digest("original-failure"), Summary: "original retained failure"}}},
		IsolatedPass:    Baseline{ReceiptDigest: digest("isolated"), Identity: isolatedIdentity, Outcome: OutcomePassed},
		LoadTopology:    WorkerTopology{Workers: 4, FullyParallel: true, Policy: "parallel-load"},
		ResetPolicy:     ResetPolicy{Name: "fresh-context", Digest: digest("reset"), RestartApplication: true, ClearBrowserState: true},
		Limits:          Limits{MaxTrials: 18, WallClock: time.Minute, Repetitions: 2},
		Qualification:   Qualification{RunnerQualified: true, RunnerReceiptDigest: digest("runner-qualified"), StabilityQualified: true, StabilityReceiptDigest: digest("stability-qualified"), ApplicationAttested: true, ApplicationReceiptDigest: digest("app-qualified")},
	}
}

func receiptFor(trial Trial, outcome Outcome, failures ...FailureObservation) TrialReceipt {
	evidence := []EvidenceRef{{Digest: digest(trial.ID + "-evidence"), Detail: "synthetic observation"}}
	receipt := TrialReceipt{
		TrialID: trial.ID, Identity: cloneIdentity(trial.Identity), ResetPolicy: trial.ResetPolicy,
		ResetSucceeded: true, CleanupSucceeded: true, ApplicationAttestationStart: trial.Identity.Fixed.ApplicationAttestationDigest,
		ApplicationAttestationPublish: trial.Identity.Fixed.ApplicationAttestationDigest, Outcome: outcome,
		Evidence: TrialEvidence{Setup: evidence, PageAssertions: evidence, Retries: evidence, Cleanup: evidence, ServerHealth: evidence, Resources: evidence}, Failures: failures,
	}
	receipt.Digest = receiptDigest(receipt)
	return receipt
}

func runSynthetic(t *testing.T, plan Plan, synthetic func(Trial) TrialReceipt) Report {
	t.Helper()
	report, err := Execute(context.Background(), plan, Authorization{OperatorApproved: true, PlanDigest: plan.Digest}, RunnerFunc(func(_ context.Context, trial Trial) (TrialReceipt, error) {
		if synthetic == nil {
			return TrialReceipt{}, errors.New("missing synthetic fixture")
		}
		return synthetic(trial), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func mustPlan(t *testing.T, request Request) Plan {
	t.Helper()
	plan, err := BuildPlan(request)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func digestOID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:20])
}

func countClass(failures []ObservedFailure, class FailureClass) int {
	count := 0
	for _, failure := range failures {
		if failure.Observation.Class == class {
			count++
		}
	}
	return count
}

func hasReason(results []TrialResult, reason string) bool {
	for _, result := range results {
		if slices.Contains(result.InvalidReasons, reason) {
			return true
		}
	}
	return false
}
