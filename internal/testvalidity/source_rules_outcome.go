package testvalidity

// Outcome, cleanup, input and vacuity source rules (docs/specs/test-source-analysis-v0.md,
// TSA-V0-160..176). internal/testsource reads them from committed source; this file holds only the
// vocabulary. An invalid rule here uses its rule ID as the hygiene reason (TSA-V0-051).
const (
	// RuleEitherOrOutcome: one assertion accepts any of several distinct literal outcomes, so it
	// cannot detect a regression from one to another (TSA-V0-160). Invalid.
	RuleEitherOrOutcome = "EITHER-OR-OUTCOME"
	// RuleCleanupHandleOutlived: a cleanup closure registered on a worker-scoped registry captures
	// a test-scoped fixture handle that is disposed before the registry runs (TSA-V0-165).
	// Advisory.
	RuleCleanupHandleOutlived = "CLEANUP-HANDLE-OUTLIVED"
	// RuleUnstubbedOutboundInput: a test input names a value of a stubbed integration's parameter
	// that no stub in the file matches (TSA-V0-169). Advisory.
	RuleUnstubbedOutboundInput = "UNSTUBBED-OUTBOUND-INPUT"
	// RuleVacuousEmptyEquality: a count or equality assertion compares two collection-derived
	// operands with no non-empty precondition, so it passes when nothing was seeded
	// (TSA-V0-172). Advisory.
	RuleVacuousEmptyEquality = "VACUOUS-EMPTY-EQUALITY"
)

// ValidityAllowMarker opens a justification annotation, `corvint-validity-allow RULE: REASON`,
// that exempts one flagged line from an annotatable rule (TSA-V0-163).
const ValidityAllowMarker = "corvint-validity-allow"
