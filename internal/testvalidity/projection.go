// Package testvalidity defines one shared test-validity projection consumed
// by both the Go CLI/MCP surface and the VS Code extension's TypeScript
// mirror (extensions/vscode/src/testvalidity.ts). A projection composes
// facts already produced by internal/tcq (association, hygiene, reported
// execution) and internal/liveverify/mutate (measured strength), plus one
// execution/freshness report shaped by the frozen LPCV "Result axes"
// vocabulary (docs/specs/live-proof-carrying-verification-v0.md) and the
// GLTP INCOMPLETE-cause vocabulary (docs/specs/go-live-test-provider-v0.md).
//
// Five axes stay independent: association (which source the test covers),
// hygiene (empty/always-skipped/wrong target), freshness (stale execution vs
// current source), execution (pass/fail/skipped/infrastructure/cancelled),
// and measured strength (mutation witness: a killed mutation establishes
// only the witnessed distinction, never general adequacy). No axis rewrites
// another. Passing never implies adequacy, and an unsupported case never
// collapses into a universal "valid" boolean: Project always returns every
// axis, stated UNSUPPORTED with a reason when its producer supplied nothing,
// rather than a derived pass/fail summary field.
package testvalidity

import (
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/tcq"
)

// StateUnsupported is the shared sentinel every axis uses when its own input
// was not supplied at all. It is not TCQ's or LPCV's frozen enum value; it is
// this projection's explicit "no fact to project" state, always carrying a
// reason so it cannot be mistaken for a positive result.
const StateUnsupported = "UNSUPPORTED"

// Association axis states, reused verbatim from TCQ-V0-006.
const (
	AssociationAssociated  = tcq.AssociationAssociated
	AssociationAbstained   = tcq.AssociationAbstained
	AssociationUnsupported = StateUnsupported
)

// Hygiene axis states, reused verbatim from TCQ-V0-006.
const (
	HygieneEligible    = tcq.HygieneEligible
	HygieneIneligible  = tcq.HygieneIneligible
	HygieneAbstained   = tcq.HygieneAbstained
	HygieneUnsupported = StateUnsupported
)

// HygieneReasonWrongTarget is a testvalidity-level hygiene reason beyond
// TCQ's frozen empty-body/unconditional-skip pair: the associated unit does
// not target what its claim declares. TCQ's claim-only boundary does not
// classify this; it is contributed by whatever caller populates ClaimFacts.
const HygieneReasonWrongTarget = "wrong-target"

// Freshness axis states: stale execution vs current source (LPCV "Result
// axes" currency term).
const (
	FreshnessCurrent     = "CURRENT"
	FreshnessStale       = "STALE"
	FreshnessUnknown     = "UNKNOWN"
	FreshnessUnsupported = StateUnsupported
)

// Execution axis states: pass/fail/skipped/infrastructure/cancelled, plus
// not-matched for a report with no corresponding row and error for a
// caller-reported harness error.
const (
	ExecutionPassed         = "PASSED"
	ExecutionFailed         = "FAILED"
	ExecutionSkipped        = "SKIPPED"
	ExecutionError          = "ERROR"
	ExecutionNotMatched     = "NOT_MATCHED"
	ExecutionInfrastructure = "INFRASTRUCTURE"
	ExecutionCancelled      = "CANCELLED"
	ExecutionUnsupported    = StateUnsupported
)

// Strength axis states: a mutation witness. NotMeasured means no mutation
// run was attempted; it is not evidence of adequacy either way.
const (
	StrengthKilled      = string(mutate.Killed)
	StrengthSurvived    = string(mutate.Survived)
	StrengthNotMeasured = "NOT_MEASURED"
	StrengthUnsupported = StateUnsupported
)

// Axis is one independent facet of a test-validity projection: a state, the
// reason it holds, and the content anchors that ground it. No axis is a
// confidence, coverage, proof, or generic pass/fail score (TCQ-V0-006,
// extended here to freshness/execution/strength).
type Axis struct {
	State   string   `json:"state"`
	Reason  string   `json:"reason"`
	Anchors []string `json:"anchors"`
}

// Projection is the one shared native test-validity shape. It has no boolean
// summary field by design.
type Projection struct {
	Association Axis `json:"association"`
	Hygiene     Axis `json:"hygiene"`
	Freshness   Axis `json:"freshness"`
	Execution   Axis `json:"execution"`
	Strength    Axis `json:"strength"`
}

// ClaimFacts is the subset of a TCQ claim result a projection draws from:
// association, hygiene, and reported-execution state plus their reasons and
// anchors. FromClaimResult adapts the real TCQ producer type unchanged; a
// caller may also build one directly, since ClaimFacts.Reasons is an open
// vocabulary a superset of TCQ's own frozen reason list (see
// HygieneReasonWrongTarget).
type ClaimFacts struct {
	AssociationState string
	HygieneState     string
	ReportState      string
	Reasons          []string
	Anchors          []string
}

// FromClaimResult projects the three TCQ-V0-006 axes out of one real TCQ
// claim result. TCQ's own frozen reason vocabulary is carried through
// verbatim; this adapter adds no reason of its own.
func FromClaimResult(claim tcq.ClaimResult) ClaimFacts {
	anchors := make([]string, 0, 4)
	for _, anchor := range []string{claim.ObligationID, claim.ClaimID, claim.TestUnitID, claim.AnchorProfile} {
		if anchor != "" {
			anchors = append(anchors, anchor)
		}
	}
	return ClaimFacts{
		AssociationState: claim.AssociationState,
		HygieneState:     claim.HygieneState,
		ReportState:      claim.ReportState,
		Reasons:          append([]string(nil), claim.Reasons...),
		Anchors:          anchors,
	}
}

// ExecutionFacts is the caller-normalized shape of one LPCV/GLTP execution
// result: LPCV's frozen "Result axes" currency term plus GLTP's
// executionOutcome cause vocabulary. A projector consumes this decoded shape
// rather than re-parsing receipt wire bytes.
type ExecutionFacts struct {
	// Outcome is PASSED, FAILED, SKIPPED, INCOMPLETE, or "" when no report
	// exists. SKIPPED is a test the runner reported as skipped: it projects
	// the execution state SKIPPED, never PASSED (LPCV-V0-052).
	Outcome string
	// Cause qualifies a FAILED or INCOMPLETE outcome: BUILD,
	// ASSERTION_OR_TEST, STALE, TIMEOUT, INFRASTRUCTURE, CANCELLATION, or "".
	Cause string
	// Currency is CURRENT, STALE, UNKNOWN, or "" (LPCV "Result axes").
	Currency string
	Anchors  []string
}

// MutationFacts adapts one real mutate.Report into the strength axis.
type MutationFacts struct {
	Verdict string
	Detail  string
	Witness *mutate.Witness
}

// FromMutationReport adapts a real mutate.Report unchanged.
func FromMutationReport(report mutate.Report) MutationFacts {
	return MutationFacts{Verdict: string(report.Verdict), Detail: report.Detail, Witness: report.Witness}
}

// Input is every fact a projection may draw from. A nil member means that
// producer supplied nothing at all; Project states UNSUPPORTED for the axes
// with no input rather than inventing a state.
type Input struct {
	Claim     *ClaimFacts
	Execution *ExecutionFacts
	Mutation  *MutationFacts
}

// Project composes one Projection from whatever facts Input carries. It
// never errors and never collapses to a boolean: a completely empty Input
// yields every axis UNSUPPORTED with reason "no-input-supplied", the
// unsupported-input case a caller must not read as a universal "valid".
func Project(input Input) Projection {
	if input.Claim == nil && input.Execution == nil && input.Mutation == nil {
		unsupported := Axis{State: StateUnsupported, Reason: "no-input-supplied"}
		return Projection{
			Association: unsupported,
			Hygiene:     unsupported,
			Freshness:   unsupported,
			Execution:   unsupported,
			Strength:    unsupported,
		}
	}
	return Projection{
		Association: associationAxis(input.Claim),
		Hygiene:     hygieneAxis(input.Claim),
		Freshness:   freshnessAxis(input.Execution),
		Execution:   executionAxis(input.Claim, input.Execution),
		Strength:    strengthAxis(input.Mutation),
	}
}

func associationAxis(claim *ClaimFacts) Axis {
	if claim == nil {
		return Axis{State: StateUnsupported, Reason: "no-association-input"}
	}
	reason := pickReason(claim.Reasons, "claim-association-missing", "claim-association-ambiguous", "unsupported-anchor-profile")
	return Axis{State: claim.AssociationState, Reason: reason, Anchors: claim.Anchors}
}

func hygieneAxis(claim *ClaimFacts) Axis {
	if claim == nil {
		return Axis{State: StateUnsupported, Reason: "no-association-input"}
	}
	reason := pickReason(claim.Reasons, "empty-body", "unconditional-skip", HygieneReasonWrongTarget, "unsupported-anchor-profile")
	return Axis{State: claim.HygieneState, Reason: reason, Anchors: claim.Anchors}
}

func freshnessAxis(execution *ExecutionFacts) Axis {
	if execution == nil || execution.Currency == "" {
		return Axis{State: FreshnessUnknown, Reason: "no-execution-report"}
	}
	reason := ""
	if execution.Currency == FreshnessStale {
		reason = "workspace-execution-identity-mismatch"
	}
	return Axis{State: execution.Currency, Reason: reason, Anchors: execution.Anchors}
}

func executionAxis(claim *ClaimFacts, execution *ExecutionFacts) Axis {
	if execution != nil && execution.Outcome != "" {
		return executionFromReport(*execution)
	}
	if claim != nil && claim.ReportState != "" {
		return executionFromClaim(*claim)
	}
	return Axis{State: StateUnsupported, Reason: "no-execution-input"}
}

func executionFromReport(execution ExecutionFacts) Axis {
	switch execution.Outcome {
	case "PASSED":
		return Axis{State: ExecutionPassed, Anchors: execution.Anchors}
	case "FAILED":
		return Axis{State: ExecutionFailed, Reason: execution.Cause, Anchors: execution.Anchors}
	case "SKIPPED":
		// A skip carries no cause; one that claims a cause is outside the
		// vocabulary and abstains rather than being read as either.
		if execution.Cause != "" {
			return Axis{State: StateUnsupported, Reason: "unsupported-execution-outcome", Anchors: execution.Anchors}
		}
		return Axis{State: ExecutionSkipped, Reason: "test-skipped", Anchors: execution.Anchors}
	case "INCOMPLETE":
		if execution.Cause == "CANCELLATION" {
			return Axis{State: ExecutionCancelled, Reason: execution.Cause, Anchors: execution.Anchors}
		}
		return Axis{State: ExecutionInfrastructure, Reason: execution.Cause, Anchors: execution.Anchors}
	default:
		return Axis{State: StateUnsupported, Reason: "unsupported-execution-outcome", Anchors: execution.Anchors}
	}
}

func executionFromClaim(claim ClaimFacts) Axis {
	reason := pickReason(claim.Reasons, "test-not-matched", "test-skipped", "test-failed", "test-error", "repeated-test-rows", "execution-identity-ambiguous")
	switch claim.ReportState {
	case tcq.ReportPassed:
		return Axis{State: ExecutionPassed, Reason: reason, Anchors: claim.Anchors}
	case tcq.ReportFailed:
		return Axis{State: ExecutionFailed, Reason: reason, Anchors: claim.Anchors}
	case tcq.ReportSkipped:
		return Axis{State: ExecutionSkipped, Reason: reason, Anchors: claim.Anchors}
	case tcq.ReportError:
		return Axis{State: ExecutionError, Reason: reason, Anchors: claim.Anchors}
	case tcq.ReportNotMatched, tcq.ReportAmbiguous:
		return Axis{State: ExecutionNotMatched, Reason: reason, Anchors: claim.Anchors}
	default:
		return Axis{State: StateUnsupported, Reason: "unsupported-report-state", Anchors: claim.Anchors}
	}
}

func strengthAxis(mutation *MutationFacts) Axis {
	if mutation == nil {
		return Axis{State: StrengthNotMeasured, Reason: "no-mutation-run"}
	}
	switch mutation.Verdict {
	case string(mutate.Killed):
		var anchors []string
		if mutation.Witness != nil {
			anchors = []string{mutation.Witness.Operator, mutation.Witness.KillingTest}
		}
		return Axis{State: StrengthKilled, Reason: "witnessed-kill: " + mutation.Detail, Anchors: anchors}
	case string(mutate.Survived):
		return Axis{State: StrengthSurvived, Reason: mutation.Detail}
	default:
		return Axis{State: StrengthNotMeasured, Reason: mutation.Detail}
	}
}

// pickReason returns the first candidate present in reasons, in candidate
// priority order, or "" when none apply.
func pickReason(reasons []string, candidates ...string) string {
	present := make(map[string]bool, len(reasons))
	for _, reason := range reasons {
		present[reason] = true
	}
	for _, candidate := range candidates {
		if present[candidate] {
			return candidate
		}
	}
	return ""
}
