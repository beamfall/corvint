package playwrightminimize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"time"
)

func Execute(ctx context.Context, plan Plan, authorization Authorization, runner Runner) (Report, error) {
	return executeWithClock(ctx, plan, authorization, runner, time.Now)
}

func executeWithClock(ctx context.Context, plan Plan, authorization Authorization, runner Runner, now func() time.Time) (Report, error) {
	if !authorization.OperatorApproved || authorization.PlanDigest != plan.Digest || plan.Digest != planDigest(plan) {
		return Report{}, errors.New("operator-authorization-required")
	}
	if runner == nil {
		return Report{}, errors.New("trial-runner-required")
	}
	report := Report{
		Schema: Schema, PlanDigest: plan.Digest, Status: "incomplete", Diagnosis: "none",
		Blockers: slices.Clone(plan.Blockers), Trials: []TrialResult{}, ObservedFailures: []ObservedFailure{},
		Limitations: []string{"findings are limited to the exact planned universe and retained trials", "smallest observed never means globally minimal", "failure observations are preserved without causal adjudication"},
	}
	started := now()
	reproduction := runKind(ctx, &report, plan, TrialReproduction, runner, now, started)
	if !kindExecutionComplete(plan, reproduction, TrialReproduction) {
		report.Blockers = appendUnique(report.Blockers, "reproduction-incomplete")
		return report, nil
	}
	switch groupState(reproduction) {
	case "passed":
		report.Status = "not_reproduced"
		return report, nil
	case "mixed":
		report.Status = "complete"
		report.Diagnosis = "nondeterministic"
		report.Blockers = appendUnique(report.Blockers, "reproduction-nondeterministic")
		return report, nil
	case "invalid":
		report.Blockers = appendUnique(report.Blockers, "reproduction-invalid")
		return report, nil
	case "failed":
	}
	isolation := runKind(ctx, &report, plan, TrialIsolation, runner, now, started)
	if !kindExecutionComplete(plan, isolation, TrialIsolation) {
		report.Blockers = appendUnique(report.Blockers, "isolation-incomplete")
		return report, nil
	}
	switch groupState(isolation) {
	case "failed":
		report.Status = "complete"
		report.Diagnosis = isolatedDiagnosis(isolation)
		report.Blockers = appendUnique(report.Blockers, "isolated-test-no-longer-passes")
		return report, nil
	case "mixed":
		report.Status = "complete"
		report.Diagnosis = "nondeterministic"
		report.Blockers = appendUnique(report.Blockers, "isolation-nondeterministic")
		return report, nil
	case "invalid":
		report.Blockers = appendUnique(report.Blockers, "isolation-invalid")
		return report, nil
	case "passed":
	}
	runKind(ctx, &report, plan, TrialOrdered, runner, now, started)
	runKind(ctx, &report, plan, TrialLoad, runner, now, started)
	report.Ordered = buildFinding(plan, report.Trials, TrialOrdered)
	report.Load = buildFinding(plan, report.Trials, TrialLoad)
	report.Diagnosis = diagnosis(report.Ordered, report.Load)
	if hasNondeterminism(report.Trials) {
		report.Diagnosis = "ambiguous"
		report.Blockers = appendUnique(report.Blockers, "candidate-nondeterminism")
	}
	allExecuted := len(report.Trials) == len(plan.Trials)
	allValid := everyResultValid(report.Trials)
	withinBudget := !slices.Contains(report.Blockers, "wall-clock-budget-exhausted")
	if allExecuted && allValid && plan.Complete && withinBudget {
		report.Status = "complete"
	} else {
		report.Blockers = appendUnique(report.Blockers, "bounded-search-incomplete")
	}
	report.Confident = report.Status == "complete" && report.Diagnosis != "ambiguous" && report.Diagnosis != "unresolved" && len(report.Blockers) == 0
	return report, nil
}

func runKind(ctx context.Context, report *Report, plan Plan, kind TrialKind, runner Runner, now func() time.Time, started time.Time) []TrialResult {
	var results []TrialResult
	for _, trial := range plan.Trials {
		if trial.Kind != kind {
			continue
		}
		if ctx.Err() != nil {
			report.Blockers = appendUnique(report.Blockers, "execution-context-cancelled")
			break
		}
		if now().Sub(started) >= plan.Request.Limits.WallClock {
			report.Blockers = appendUnique(report.Blockers, "wall-clock-budget-exhausted")
			break
		}
		remaining := plan.Request.Limits.WallClock - now().Sub(started)
		trialCtx, cancel := context.WithTimeout(ctx, remaining)
		receipt, err := runner.Run(trialCtx, cloneTrial(trial))
		trialContextErr := trialCtx.Err()
		cancel()
		result := validateReceipt(trial, receipt, err)
		if receipt.Outcome == OutcomeFailed && trial.Kind != TrialIsolation && !slices.Equal(receiptFailureClasses(receipt), receiptFailureClasses(TrialReceipt{Failures: plan.Request.OriginalFailure.Failures})) {
			result.Valid = false
			result.InvalidReasons = append(result.InvalidReasons, "original-failure-signature-mismatch")
			report.Blockers = appendUnique(report.Blockers, "original-failure-signature-mismatch")
		}
		contextReason := ""
		if trialContextErr != nil {
			contextReason = "execution-context-cancelled"
			if errors.Is(trialContextErr, context.DeadlineExceeded) {
				contextReason = "wall-clock-budget-exhausted"
			}
			result.Valid = false
			result.InvalidReasons = append(result.InvalidReasons, contextReason)
		}
		overrun := now().Sub(started) >= plan.Request.Limits.WallClock
		if overrun {
			result.Valid = false
			result.InvalidReasons = append(result.InvalidReasons, "wall-clock-budget-exhausted")
		}
		if result.Receipt != nil && receiptDigestSeen(report.Trials, result.Receipt.Digest) {
			result.Valid = false
			result.InvalidReasons = append(result.InvalidReasons, "duplicate-receipt-digest")
		}
		if contextReason != "" {
			report.Blockers = appendUnique(report.Blockers, contextReason)
		}
		report.Trials = append(report.Trials, result)
		results = append(results, result)
		if result.Receipt != nil {
			for _, failure := range result.Receipt.Failures {
				report.ObservedFailures = append(report.ObservedFailures, ObservedFailure{TrialID: trial.ID, ReceiptDigest: result.Receipt.Digest, Observation: failure})
			}
		}
		if overrun {
			report.Blockers = appendUnique(report.Blockers, "wall-clock-budget-exhausted")
			break
		}
	}
	return results
}

func validateReceipt(trial Trial, receipt TrialReceipt, runErr error) TrialResult {
	result := TrialResult{Trial: trial, Receipt: &receipt, Valid: true}
	if runErr != nil {
		result.Valid = false
		result.ExecutionError = runErr.Error()
		return result
	}
	if receipt.TrialID != trial.ID {
		result.InvalidReasons = append(result.InvalidReasons, "trial-identity-mismatch")
	}
	if !reflect.DeepEqual(receipt.Identity, trial.Identity) || !reflect.DeepEqual(receipt.ResetPolicy, trial.ResetPolicy) {
		result.InvalidReasons = append(result.InvalidReasons, "run-identity-mismatch")
	}
	if !digestPattern.MatchString(receipt.Digest) || receipt.Digest != receiptDigest(receipt) {
		result.InvalidReasons = append(result.InvalidReasons, "receipt-digest-invalid")
	}
	if !receipt.ResetSucceeded {
		result.InvalidReasons = append(result.InvalidReasons, "reset-failed")
	}
	if !receipt.CleanupSucceeded {
		result.InvalidReasons = append(result.InvalidReasons, "cleanup-failed")
	}
	attestation := trial.Identity.Fixed.ApplicationAttestationDigest
	if receipt.ApplicationAttestationStart != attestation {
		result.InvalidReasons = append(result.InvalidReasons, "application-attestation-mismatch")
	}
	if receipt.ApplicationAttestationPublish != attestation {
		result.InvalidReasons = append(result.InvalidReasons, "application-restarted")
	}
	if receipt.Outcome != OutcomePassed && receipt.Outcome != OutcomeFailed {
		result.InvalidReasons = append(result.InvalidReasons, "outcome-invalid")
	}
	if receipt.Outcome == OutcomeFailed && len(receipt.Failures) == 0 {
		result.InvalidReasons = append(result.InvalidReasons, "failure-evidence-missing")
	}
	if receipt.Outcome == OutcomePassed && len(receipt.Failures) != 0 {
		result.InvalidReasons = append(result.InvalidReasons, "passing-trial-has-failure")
	}
	if !validEvidence(receipt.Evidence) {
		result.InvalidReasons = append(result.InvalidReasons, "trial-evidence-incomplete")
	}
	for _, failure := range receipt.Failures {
		if !validFailure(failure) {
			result.InvalidReasons = append(result.InvalidReasons, "failure-evidence-invalid")
			break
		}
	}
	result.Valid = len(result.InvalidReasons) == 0
	return result
}

func validEvidence(evidence TrialEvidence) bool {
	groups := [][]EvidenceRef{evidence.Setup, evidence.PageAssertions, evidence.Retries, evidence.Cleanup, evidence.ServerHealth, evidence.Resources}
	for _, group := range groups {
		if len(group) == 0 {
			return false
		}
		for _, item := range group {
			if !digestPattern.MatchString(item.Digest) || item.Detail == "" {
				return false
			}
		}
	}
	return true
}

func validFailure(failure FailureObservation) bool {
	switch failure.Class {
	case FailureAssertion, FailureSynchronization, FailureFixture, FailureProduct, FailureInfrastructure, FailureRestart, FailureResourceExhaustion:
		return digestPattern.MatchString(failure.EvidenceDigest) && failure.Summary != ""
	default:
		return false
	}
}

func groupState(results []TrialResult) string {
	if len(results) == 0 {
		return "invalid"
	}
	for _, result := range results {
		if !result.Valid || result.Receipt == nil {
			return "invalid"
		}
	}
	passed, failed := false, false
	var classes []FailureClass
	for _, result := range results {
		if result.Receipt.Outcome == OutcomePassed {
			passed = true
		} else {
			failed = true
			current := receiptFailureClasses(*result.Receipt)
			if classes == nil {
				classes = current
			} else if !slices.Equal(classes, current) {
				return "mixed"
			}
		}
	}
	if passed && failed {
		return "mixed"
	}
	if failed {
		return "failed"
	}
	return "passed"
}

func receiptFailureClasses(receipt TrialReceipt) []FailureClass {
	var classes []FailureClass
	for _, failure := range receipt.Failures {
		if !slices.Contains(classes, failure.Class) {
			classes = append(classes, failure.Class)
		}
	}
	slices.Sort(classes)
	return classes
}

func isolatedDiagnosis(results []TrialResult) string {
	for _, result := range results {
		if result.Receipt == nil || !result.Valid {
			return "isolated-failure-observed"
		}
		classes := receiptFailureClasses(*result.Receipt)
		if !slices.Equal(classes, []FailureClass{FailureProduct}) {
			return "isolated-failure-observed"
		}
	}
	return "isolated-product-regression"
}

func buildFinding(plan Plan, results []TrialResult, kind TrialKind) *Finding {
	groups := resultGroups(results, kind)
	var best *candidateGroup
	searchValid := true
	for i := range groups {
		state := candidateState(groups[i].results, plan.Request.Limits.Repetitions)
		if state == "invalid" || state == "mixed" {
			searchValid = false
		}
		if state != "failed" {
			continue
		}
		if best == nil || len(groups[i].context) < len(best.context) {
			copy := groups[i]
			best = &copy
		}
	}
	if best == nil {
		return nil
	}
	complete := searchValid && kindExecutionComplete(plan, results, kind)
	if kind == TrialOrdered {
		complete = complete && plan.OrderedComplete
	} else {
		complete = complete && plan.LoadComplete
	}
	finding := &Finding{Kind: kind, Smallest: slices.Clone(best.context), SearchComplete: complete, Minimality: "smallest-observed"}
	if complete {
		finding.Minimality = "proved-within-bounded-universe"
	}
	for _, result := range resultsOfKind(results, kind) {
		if result.Receipt != nil {
			finding.ReceiptDigests = appendUnique(finding.ReceiptDigests, result.Receipt.Digest)
		}
	}
	if isolationMatchesTopology(plan, best.results) {
		for _, result := range resultsOfKind(results, TrialIsolation) {
			if result.Receipt != nil {
				finding.ReceiptDigests = appendUnique(finding.ReceiptDigests, result.Receipt.Digest)
			}
		}
	}
	for index, member := range best.context {
		without := append(slices.Clone(best.context[:index]), best.context[index+1:]...)
		status := "unproven"
		if len(without) == 0 && isolationMatchesTopology(plan, best.results) && groupState(resultsOfKind(results, TrialIsolation)) == "passed" {
			status = "necessary-in-observed-universe"
		} else if candidateState(findGroup(groups, without), plan.Request.Limits.Repetitions) == "passed" {
			status = "necessary-in-observed-universe"
		}
		finding.Members = append(finding.Members, MemberAssessment{Member: member, Status: status})
	}
	return finding
}

func candidateState(results []TrialResult, repetitions int) string {
	if len(results) != repetitions {
		return "invalid"
	}
	return groupState(results)
}

func isolationMatchesTopology(plan Plan, results []TrialResult) bool {
	if len(results) == 0 {
		return false
	}
	return reflect.DeepEqual(results[0].Trial.Identity.WorkerTopology, plan.Request.IsolatedPass.Identity.WorkerTopology)
}

func receiptDigestSeen(results []TrialResult, digest string) bool {
	for _, result := range results {
		if result.Receipt != nil && result.Receipt.Digest == digest {
			return true
		}
	}
	return false
}

func receiptDigest(receipt TrialReceipt) string {
	receipt.Digest = ""
	encoded, _ := json.Marshal(receipt)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneTrial(trial Trial) Trial {
	trial.Context = slices.Clone(trial.Context)
	trial.Identity = cloneIdentity(trial.Identity)
	return trial
}

func kindExecutionComplete(plan Plan, results []TrialResult, kind TrialKind) bool {
	planned, observed := 0, 0
	for _, trial := range plan.Trials {
		if trial.Kind == kind {
			planned++
		}
	}
	for _, result := range results {
		if result.Trial.Kind == kind {
			observed++
		}
	}
	return planned == observed
}

type candidateGroup struct {
	id      string
	context []string
	results []TrialResult
}

func resultGroups(results []TrialResult, kind TrialKind) []candidateGroup {
	var groups []candidateGroup
	for _, result := range results {
		if result.Trial.Kind != kind {
			continue
		}
		if len(groups) == 0 || groups[len(groups)-1].id != result.Trial.CandidateID {
			groups = append(groups, candidateGroup{id: result.Trial.CandidateID, context: slices.Clone(result.Trial.Context)})
		}
		groups[len(groups)-1].results = append(groups[len(groups)-1].results, result)
	}
	return groups
}

func findGroup(groups []candidateGroup, context []string) []TrialResult {
	for _, group := range groups {
		if slices.Equal(group.context, context) {
			return group.results
		}
	}
	return nil
}

func resultsOfKind(results []TrialResult, kind TrialKind) []TrialResult {
	var selected []TrialResult
	for _, result := range results {
		if result.Trial.Kind == kind {
			selected = append(selected, result)
		}
	}
	return selected
}

func hasNondeterminism(results []TrialResult) bool {
	for _, kind := range []TrialKind{TrialOrdered, TrialLoad} {
		for _, group := range resultGroups(results, kind) {
			if groupState(group.results) == "mixed" {
				return true
			}
		}
	}
	return false
}

func diagnosis(ordered, load *Finding) string {
	if ordered != nil && load != nil {
		return "ambiguous"
	}
	if ordered != nil {
		return "ordered-predecessor-observed"
	}
	if load != nil {
		return "unordered-load-observed"
	}
	return "unresolved"
}

func everyResultValid(results []TrialResult) bool {
	for _, result := range results {
		if !result.Valid {
			return false
		}
	}
	return true
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}
