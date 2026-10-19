package testvalidity

// Intra-example stub and value-origin rule IDs (docs/specs/test-source-analysis-v0.md,
// TSA-V0-120..130). internal/testsource reads them from committed RSpec, Jest and Vitest source.
// Every finding of these rules is advisory: an intra-example trace never sees the code under test,
// so it never makes a test INELIGIBLE (TSA-V0-120).
const (
	// RuleStubOnPath: a stub on a first-party collaborator returns the value an assertion checks
	// on the subject's result, or the assertion calls the stub itself (TSA-V0-122).
	RuleStubOnPath = "STUB-ON-PATH"
	// RuleStubNeverInvoked: a silent stub is set on an object the test created and never hands to
	// any code, so the action cannot invoke it (static form, TSA-V0-124).
	RuleStubNeverInvoked = "STUB-NEVER-INVOKED"
	// RuleAssertedValueNotFromSubject: an assertion's actual value comes only from values the test
	// built itself, or a call expectation is met by the test's own call (TSA-V0-126..128).
	RuleAssertedValueNotFromSubject = "ASSERTED-VALUE-NOT-FROM-SUBJECT"
)
