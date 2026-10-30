package testvalidity

// Assertion-parity rule vocabulary (docs/specs/test-source-analysis-v0.md, TSA-V0-670..677).
// internal/testsource compares the normalised expectation tuples of a new test with those of the
// legacy test it is linked to. Every rule here is advisory: it never changes a hygiene or strength
// axis or the exit status.

// Assertion-parity rule IDs.
const (
	// RuleAssertionParityUnmatchedLegacy: a legacy expectation has no new expectation with the
	// same expected value and polarity (TSA-V0-674).
	RuleAssertionParityUnmatchedLegacy = "ASSERTION-PARITY-UNMATCHED-LEGACY"
	// RuleAssertionParityNewWithoutOrigin: a new expectation matches no legacy expectation
	// (TSA-V0-674).
	RuleAssertionParityNewWithoutOrigin = "ASSERTION-PARITY-NEW-WITHOUT-ORIGIN"
	// RuleAssertionParityWeakened: a legacy expectation is matched only by a new expectation of a
	// weaker matcher class, for example exact equality by containment or presence (TSA-V0-675).
	RuleAssertionParityWeakened = "ASSERTION-PARITY-WEAKENED"
	// RuleAssertionParityNotAnalysed: one side's expectations could not all be read, so assertion
	// parity is not reported (TSA-V0-676).
	RuleAssertionParityNotAnalysed = "ASSERTION-PARITY-NOT-ANALYSED"
)

// Assertion-parity statuses (TSA-V0-673). Parity is the correspondence of the normalised
// expectation tuples only, never behavioural parity.
const (
	AssertionParityStatusParity      = "parity"
	AssertionParityStatusDiverges    = "diverges"
	AssertionParityStatusNotAnalysed = "not-analysed"
)
