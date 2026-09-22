// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

// Per-case satisfiability.
//
// The fixture tree is an adversarial matrix, and the matrix deliberately
// includes rows whose universe no caller of the public entry point can build:
// a timeout has to be injected, an LRF `relevance-bound-exceeded` document has
// to be returned by an evaluator that will not return one over a real
// repository, a 2,048-hunk result has to be counted rather than committed.
//
// A runner that fails those cases is reporting a defect in itself, and a suite
// that skips ALL cases because a few are unbuildable reports nothing at all.
// So each case declares the capabilities its universe needs, a bound runner
// declares which it has, and a case whose requirements are unmet skips with the
// SPECIFIC reason its universe is out of reach. "cannot inject a timeout
// through the public entry point" is a result; "the seam is unbound" is not.
//
// Requirements are DERIVED from the declared universe rather than listed per
// case, so a fixture that grows a declared fault cannot quietly acquire an
// executing case that silently does nothing.

// Capability is one materialization ability a fixture case needs from a bound
// runner.
type Capability string

const (
	// CapFaultInjection is a declared `injected` fault: a timeout, an
	// allocation failure, an interruption, an unknown upstream code, or an
	// uncatchable termination.
	CapFaultInjection Capability = "fault-injection"
	// CapLRFResultOverride is a declared LRF result other than a normal one.
	CapLRFResultOverride Capability = "lrf-result-override"
	// CapTCQResultOverride is a declared TCQ outcome other than a normal one.
	CapTCQResultOverride Capability = "tcq-result-override"
	// CapDynamicTestTuple is a complete CF-V0-003 command/observation/report
	// tuple that a real TCQ evaluation accepts and matches.
	CapDynamicTestTuple Capability = "dynamic-test-tuple"
	// CapTCQDiagnosticSet is a declared set of TCQ edge diagnostics wider than
	// the one a real static evaluation of the declared claims produces.
	CapTCQDiagnosticSet Capability = "tcq-diagnostic-set"
	// CapDeclaredItemCounts is a declared CF-V0-023 count: a universe sized to
	// exactly N or N+1 of a bounded thing, which a runner must synthesize at
	// that scale rather than read out of an enumerated fixture.
	CapDeclaredItemCounts Capability = "declared-item-counts"
	// CapUnreachableItemCount is a declared CF-V0-023 count no universe can
	// reach, because the artifact that feeds the count is capped at or below
	// the Frontier bound. The blocker is per case, so its message comes from
	// PlanCounts rather than from one shared sentence.
	CapUnreachableItemCount Capability = "unreachable-item-count"
	// CapReasonUnionBeyondAlgebra is a single item carrying more reasons than
	// the closed CF-V0-010/CF-V0-016 unions can ever hold. See
	// ReasonAlgebraCeilings.
	CapReasonUnionBeyondAlgebra Capability = "reason-union-beyond-algebra"
	// CapCandidateDocument is a candidate Frontier document presented to the
	// verifier rather than a universe presented to the producer. These cases
	// check refusal of an almost-right document, so they need the CF-V0-018
	// document-verification seam and not the producing one.
	CapCandidateDocument Capability = "candidate-document"
	// CapSiblingRepository is an object that resolves only in a repository
	// other than the one under verification.
	CapSiblingRepository Capability = "sibling-repository"
	// CapUncommittableOCMReferences is an obligation whose reference arrays a
	// canonical OCM artifact cannot carry at all.
	CapUncommittableOCMReferences Capability = "uncommittable-ocm-references"
	// CapObligationFreeUniverse is a verified universe carrying no OCM
	// obligation at all.
	CapObligationFreeUniverse Capability = "obligation-free-universe"
)

// capabilityReasons names, for each capability, WHY a runner may not have it.
// The text is the skip message, so it must say what is out of reach and not
// merely that something is.
var capabilityReasons = map[Capability]string{
	CapFaultInjection: "the declared fault (timeout, allocation failure, interruption, unknown upstream " +
		"code, or uncatchable termination) cannot be injected through the public entry point: " +
		"CF-V0-026 keeps the library boundary free of a fault seam, so no caller can produce this " +
		"universe. Frontier's own translation of these codes is covered by internal/frontier's " +
		"unit tests, which reach the translation directly.",
	CapLRFResultOverride: "a normal LRF evaluation over a real repository never returns the declared " +
		"result: CF-V0-002 makes Frontier recompute LRF rather than accept one, so there is no input " +
		"through which a caller can hand it an aggregate bound document or an unsupported LRF context.",
	CapTCQResultOverride: "a real TCQ recomputation over the declared universe never returns the declared " +
		"outcome: CF-V0-002 forbids accepting a caller-supplied TCQ result, so a forged authority class, " +
		"an unknown relation, a resource exhaustion, or a synthetic validation error cannot be presented " +
		"through the entry point.",
	CapDynamicTestTuple: "the declared universe needs a complete CF-V0-003 command/observation/report " +
		"tuple that a real TCQ evaluation accepts and matches against the committed test blob; this " +
		"runner builds real repositories but does not execute a test command to produce an attested " +
		"observation and a matching JUnit report.",
	CapTCQDiagnosticSet: "the declared TCQ edge diagnostics are wider than a real static evaluation of " +
		"the declared claims produces; presenting them would mean handing Frontier a TCQ result, which " +
		"CF-V0-002 forbids.",
	CapDeclaredItemCounts: "the declared CF-V0-023 count has to be materialized as that many real CEM " +
		"hunks, OCM obligations, or evidence citations in a real repository, derived from the bound " +
		"being driven rather than enumerated by the fixture; a runner without this synthesis cannot " +
		"build a universe at that scale. The bounds themselves are also enforced at the seal boundary " +
		"and covered by internal/frontier's own limit tests.",
	CapUnreachableItemCount: "the declared CF-V0-023 count is not reachable by any universe: the " +
		"artifact that feeds the count is itself capped at or below the Frontier bound, so the Frontier " +
		"ceiling is defence in depth over a narrower algebra. PlanCounts records the derivation for the " +
		"exact bound this case drives, and SkipReason reports it in place of this sentence.",
	CapReasonUnionBeyondAlgebra: "no universe can put this many reasons on one item. CF-V0-010 and " +
		"CF-V0-016 are CLOSED unions with unique members, so an item's reason count is bounded by its " +
		"kind's table, not by CF-V0-023: see ReasonAlgebraCeilings for the derivation and the reachable " +
		"maximum, which internal/frontier tests directly.",
	CapCandidateDocument: "this case presents a candidate Frontier document to be refused, not a universe " +
		"to be computed; it needs the CF-V0-018 document-verification seam rather than the producing one.",
	CapSiblingRepository: "the declared universe needs an object that resolves only in a sibling " +
		"repository, which requires a second repository plus an alternates or worktree arrangement this " +
		"runner does not build.",
	CapUncommittableOCMReferences: "the declared obligation references cannot be committed as a canonical " +
		"OCM artifact: reference arrays are sorted and unique, a linked obligation may only name hunks the " +
		"CEM verified as `supported`, and a claim's anchor must carry the exact obligation ID so one claim " +
		"cannot be shared by two obligations. The declared universe breaks at least one of those, so it " +
		"cannot be built and handed to the entry point.",
	CapObligationFreeUniverse: "no verified universe can carry zero obligations, so the CF-V0-004 " +
		"EMPTY state cannot be produced from any repository. See IntentItemsPerObligation for the four " +
		"links and TestEmptyStateIsUnreachableThroughTheEntryPoint, which drives the refusal for real " +
		"rather than asserting it, so this skip stops being true the moment the refusal is relaxed. " +
		"This is a recorded spec gap, not a Frontier defect and not a fixture defect: the refusal is " +
		"shared by BOTH runtimes (internal/lrfrepo/ocm.go and the frozen oracle src/context_corvint_ocm.py), " +
		"while OCM-V0-001 does not itself demand a non-empty `## Requirements` section — and " +
		"conformance/divergence-register.md adjudicates a spec-silent observable as `spec-gap`: amend the " +
		"owning spec first, neither runtime wins by default.",
}

// IntentItemsPerObligation is the minimum number of items one obligation of
// each OCM disposition contributes, read from the clauses rather than from the
// implementation. It is the last link of the derivation below, and a zero here
// would mean the CF-V0-004 EMPTY state had become reachable.
//
// That derivation is recorded here so the next reader of the sixteen
// obligation-free cases does not have to rediscover it. Each link is the clause
// or code that forces the next one:
//
//  1. CF-V0-001 requires "exactly one canonical `ocm/0.1-experimental` artifact
//     bound to it", so every universe carries an OCM artifact the OCM verifier
//     accepts.
//  2. That verifier refuses an intent scope whose `## Requirements` section
//     enumerates no requirement, with the exact code `missing-requirements`.
//     Both runtimes do it: internal/lrfrepo/ocm.go and the frozen Python oracle
//     src/context_corvint_ocm.py. It is NOT Go-side strictness.
//  3. OCM-V0-003 requires "exactly one obligation row for every enumerated
//     requirement", so a universe has at least one obligation.
//  4. Every obligation emits at least one intent item: CF-V0-011 gives an OCM
//     `unknown` obligation exactly one INTENT_CHANGE item, and CF-V0-012 plus
//     CF-V0-015 give a linked one exactly one of each intent kind — the map
//     below.
//
// So no repository produces a result with zero items, and exit code 0 — which
// CF-V0-004 defines and CF-V0-018 gives a shape — is unreachable.
//
// Decision 0012 closes the former `spec-gap`: OCM-V0-001 now requires at least
// one enumerated requirement and names exact `missing-requirements` refusal for
// an empty enumeration. CF-V0-004 and CF-V0-018 retain EMPTY and exit 0 only as
// a canonical verification/refusal shape unreachable through the V0 entry
// point. The sixteen obligation-free cases therefore remain skipped without
// relaxing either runtime's existing refusal.
var IntentItemsPerObligation = map[string]int{
	"unknown": 1, // CF-V0-011: exactly one INTENT_CHANGE, no INTENT_TEST.
	"linked":  2, // CF-V0-012 and CF-V0-015: exactly one of each intent kind.
}

// ObligationFreeCaseCount is the number of fixture cases whose skip reason IS
// this derivation; a case that also carries a more specific blocker is counted
// under that one instead. It is asserted against the live fixture tree, so the
// recorded cost of the gap cannot drift away from the tree it describes.
const ObligationFreeCaseCount = 16

// ReasonAlgebraCeilings records why CF-V0-023's 32-reasons-per-item bound is
// unreachable, so the next reader of the resource-limits fixture does not have
// to rediscover it.
//
// CF-V0-016 fixes INTENT_TEST's reason table at 13 rows and says reasons are
// UNIQUE and stored in that order after taking the union across selected claim
// edges. CF-V0-010 fixes HUNK_BASIS's union; CF-V0-016 says INTENT_CHANGE "has
// exactly one reason". So an item's reason count is bounded by its kind's
// closed table:
//
//   - HUNK_BASIS: a CEM `unknown` hunk carries exactly one mapped reason, and
//     a `supported` hunk carries the union of the five CF-V0-010 basis
//     diagnostics. Ceiling 5.
//   - INTENT_CHANGE: exactly one, by clause text. Ceiling 1.
//   - INTENT_TEST: one relation reason plus the twelve distinct reasons the
//     CF-V0-016 TCQ mapping can produce. Ceiling 13.
//
// 13 < 32, so no valid document can hold an item at the declared bound, and no
// universe can hold one at bound+1 either. The bound is still real and still
// enforced — it is a defence-in-depth ceiling on a union whose own algebra is
// already narrower — which is why the pair is marked unsatisfiable by
// construction rather than deleted.
var ReasonAlgebraCeilings = map[string]int{
	"HUNK_BASIS":    5,
	"INTENT_CHANGE": 1,
	"INTENT_TEST":   13,
}

// ReachableReasonMaximum is the largest reason count any item kind can hold.
func ReachableReasonMaximum() int {
	maximum := 0
	for _, ceiling := range ReasonAlgebraCeilings {
		if ceiling > maximum {
			maximum = ceiling
		}
	}
	return maximum
}

// SkipReason is the message one case skips with. It is CapabilityReason plus,
// where the blocker is per case rather than per capability, that case's own
// derivation: the CF-V0-023 counts cases are each out of reach for a DIFFERENT
// reason — a CEM hunk cap, an OCM obligation cap, a reference-array cap, an
// item-count arithmetic — and one shared sentence for all of them would be
// exactly the "the seam is unbound" non-answer this mechanism exists to avoid.
func SkipReason(c Case, capability Capability) string {
	if capability != CapUnreachableItemCount {
		return CapabilityReason(capability)
	}
	if _, unreachable := PlanCounts(c); unreachable != "" {
		return string(capability) + ": " + unreachable
	}
	return CapabilityReason(capability)
}

// CapabilityReason returns the skip message for a capability.
func CapabilityReason(capability Capability) string {
	if reason, found := capabilityReasons[capability]; found {
		return string(capability) + ": " + reason
	}
	return string(capability) + ": no reason was recorded for this capability, which is itself a defect"
}

// RequiredCapabilities derives what a runner must be able to materialize for
// one case. The derivation reads the declared universe, never the case ID,
// except where the fixture's whole subject is a different seam.
func RequiredCapabilities(f Fixture, c Case) []Capability {
	required := map[Capability]bool{}
	declared := c.Declared

	if declared.Injected != "" {
		required[CapFaultInjection] = true
	}
	if declared.LRF != "normal" {
		required[CapLRFResultOverride] = true
	}
	if declared.TCQ != "normal" && !callerProducibleTCQOutcome(declared) {
		required[CapTCQResultOverride] = true
	}
	if declared.DynamicTuple == "complete" {
		required[CapDynamicTestTuple] = true
	}
	if len(declared.Counts) > 0 {
		// The reasons-per-item bound is not a matter of scale: the closed reason
		// unions cannot reach it at any size, so it names its own reason rather
		// than the generic count one.
		switch {
		case declared.Counts["reasonsPerItem"] != "":
			required[CapReasonUnionBeyondAlgebra] = true
		default:
			// The universe of a counts case is DERIVED from the bound, not read
			// off the fixture, so whether it is satisfiable is a property of the
			// derivation. PlanCounts either produces one to build or names why
			// that size is out of reach.
			if _, unreachable := PlanCounts(c); unreachable != "" {
				required[CapUnreachableItemCount] = true
			} else {
				required[CapDeclaredItemCounts] = true
			}
		}
	}
	for _, obligation := range declared.Obligations {
		// A TCQ relation only exists once a report has been matched, which is
		// the dynamic tuple's job.
		if obligation.TCQRelation != "" {
			required[CapDynamicTestTuple] = true
		}
		if !staticTCQReasons(obligation.TCQReasons) {
			required[CapTCQDiagnosticSet] = true
		}
		if hasDuplicate(obligation.HunkIDs) || hasDuplicate(obligation.ClaimIDs) {
			required[CapUncommittableOCMReferences] = true
		}
		if obligation.Disposition == "linked" && linksUnsupportedHunk(declared, obligation) {
			required[CapUncommittableOCMReferences] = true
		}
	}
	// A counts case is excluded: it enumerates no obligation because it declares
	// a SIZE, and PlanCounts supplies the obligations that size implies. Reading
	// the empty array as an obligation-free universe would report the wrong
	// blocker for a case that needs 256 of them.
	if len(declared.Obligations) == 0 && len(declared.Counts) == 0 && c.Expect.Kind == "result" {
		required[CapObligationFreeUniverse] = true
	}
	if sharesAClaim(declared.Obligations) {
		required[CapUncommittableOCMReferences] = true
	}
	// CF-V0-022's `repository-object-unavailable` row is the one case whose
	// universe spans two repositories.
	if c.Expect.ErrorCode == "repository-object-unavailable" {
		required[CapSiblingRepository] = true
	}
	// The refusal fixture's subject is a candidate document, not a universe.
	if f.ID == "noncanonical-frontier-refusals" {
		required[CapCandidateDocument] = true
	}

	ordered := make([]Capability, 0, len(required))
	for _, capability := range capabilityOrder {
		if required[capability] {
			ordered = append(ordered, capability)
		}
	}
	if len(ordered) != len(required) {
		// A capability with no place in the order would sort arbitrarily and
		// could shadow a more specific reason, so surface it instead.
		for capability := range required {
			if !containsCapability(ordered, capability) {
				ordered = append(ordered, capability)
			}
		}
	}
	return ordered
}

// capabilityOrder decides which reason a case skips with when more than one
// applies. The most SPECIFIC reason wins: a case that cannot exist because the
// reason algebra forbids it should not be reported as merely too large.
var capabilityOrder = []Capability{
	CapReasonUnionBeyondAlgebra,
	CapUnreachableItemCount,
	CapCandidateDocument,
	CapFaultInjection,
	CapLRFResultOverride,
	CapTCQDiagnosticSet,
	CapDynamicTestTuple,
	CapTCQResultOverride,
	CapUncommittableOCMReferences,
	CapSiblingRepository,
	CapDeclaredItemCounts,
	CapObligationFreeUniverse,
}

func containsCapability(values []Capability, target Capability) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// callerProducibleTCQOutcome reports whether a declared non-normal TCQ outcome
// is one a REAL evaluation returns over caller-supplied bytes, rather than one
// that would have to be handed to Frontier.
//
// `invalid-junit` is the only such outcome. The JUnit report is the caller's
// own raw evidence — TCQ-V0-044 makes every dynamic artifact caller-supplied
// immutable bytes — so a caller can present a malformed report and TCQ reaches
// the refusal by parsing it. A forged authority class, an unknown relation, a
// resource exhaustion, or a synthetic validation error are values only the
// producer decides, so declaring them means handing Frontier a TCQ result,
// which CF-V0-002 forbids. Without the rest of the tuple the report is never
// read at all (the partial combination is refused at cascade stage 1), so the
// complete tuple is part of the condition.
func callerProducibleTCQOutcome(declared Declared) bool {
	return declared.TCQ == "invalid-junit" && declared.DynamicTuple == "complete"
}

// staticTCQReasons reports whether a declared diagnostic set is one a real
// static TCQ evaluation of the declared claims produces. With no dynamic
// tuple, every selected claim edge is unmatched, which TCQ reports as
// `test-not-matched` and CF-V0-016 maps to TEST_NOT_MATCHED.
func staticTCQReasons(reasons []string) bool {
	for _, reason := range reasons {
		if reason != "test-not-matched" {
			return false
		}
	}
	return true
}

// linksUnsupportedHunk reports whether a linked obligation names a hunk the
// CEM does not declare `supported`. The canonical OCM verifier refuses that
// (`unknown-hunk-reference`), so the pair cannot exist in one artifact.
func linksUnsupportedHunk(declared Declared, obligation DeclaredOblig) bool {
	supported := map[string]bool{}
	for _, hunk := range declared.Hunks {
		if hunk.Disposition == "supported" || hunk.Disposition == "deletion" {
			supported[hunk.ID] = true
		}
	}
	for _, hunkID := range obligation.HunkIDs {
		if !supported[hunkID] {
			return true
		}
	}
	return false
}

// sharesAClaim reports whether one claim is referenced by two obligations. A
// claim's anchor must contain the exact obligation ID, and the admitted anchor
// grammar has no room for two, so no committable artifact can do this.
func sharesAClaim(obligations []DeclaredOblig) bool {
	owner := map[string]string{}
	for _, obligation := range obligations {
		for _, claimID := range obligation.ClaimIDs {
			if previous, found := owner[claimID]; found && previous != obligation.ID {
				return true
			}
			owner[claimID] = obligation.ID
		}
	}
	return false
}

func hasDuplicate(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}
