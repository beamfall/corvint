package testvalidity

// JS/TS example hygiene and strength source facts (docs/specs/test-source-analysis-v0.md,
// TSA-V0-073..083). internal/testsource reads them from committed Jest, Vitest and Playwright
// source; this file holds the vocabulary and the one place they reach a projection. An invalid
// rule here uses its rule ID as the hygiene reason (TSA-V0-051).

// JS/TS example rule IDs.
const (
	// RuleJSSkipped: the example is skipped by an x-prefix, a skip or fixme member, an ancestor
	// describe, or an unconditional runtime skip (TSA-V0-074).
	RuleJSSkipped = "JS-SKIPPED"
	// RuleJSTodo: the example is a todo with no body (TSA-V0-074).
	RuleJSTodo = "JS-TODO"
	// RuleJSFocused: the example or an ancestor describe is focused (TSA-V0-075).
	RuleJSFocused = "JS-FOCUSED"
	// RuleJSEachRowBinding: the example runs once per table row, all bound to one line
	// (TSA-V0-076).
	RuleJSEachRowBinding = "JS-EACH-ROW-BINDING"
	// RuleJSNoAssertion: the example body asserts nothing the reader can see (TSA-V0-077).
	RuleJSNoAssertion = "JS-NO-ASSERTION"
	// RuleJSConstantExpectation: every expectation compares constants (TSA-V0-078).
	RuleJSConstantExpectation = "JS-CONSTANT-EXPECTATION"
	// RuleJSSubjectMocked: the module or class under test is mocked or spied on (TSA-V0-079).
	RuleJSSubjectMocked = "JS-SUBJECT-MOCKED"
	// RuleJSBoundaryMocked: an HTTP client, fetch or timer boundary is mocked (TSA-V0-079).
	RuleJSBoundaryMocked = "JS-BOUNDARY-MOCKED"
	// RuleJSAssertionNotAwaited: an assertion sits in a promise callback the example neither
	// awaits nor returns, so it can run after the example finished (TSA-V0-389).
	RuleJSAssertionNotAwaited = "JS-ASSERTION-NOT-AWAITED"
)

// Hygiene and strength reasons of the JS/TS example facts (TSA-V0-073, TSA-V0-080, TSA-V0-082).
const (
	HygieneReasonJSExampleRead      = "js-example-source-read"
	HygieneReasonJSShapeUnsupported = "js-example-shape-unsupported"
	HygieneReasonJSBodyNotLiteral   = "js-example-body-not-literal"
	HygieneReasonJSLineAmbiguous    = "js-example-line-ambiguous"
	HygieneReasonJSSourceUnbound    = "js-source-not-bound"
	HygieneReasonJSExampleUnplaced  = "js-example-not-located"
	ExampleSourceNotJSTestFile      = "not-a-js-test-file"
	StrengthReasonSubjectMocked     = "subject-mocked"
	StrengthReasonConstantOnly      = "constant-expectations-only"
	StrengthReasonBoundaryMocked    = "boundary-mocked"
)

// ApplyJSExampleSource returns p with hygiene and strength recomputed from one JS/TS example's
// source facts (TSA-V0-080). anchor is the example's `path:line`. It projects as
// ApplyExampleSource does, with js-example-source-read as the located reason. Strength stays
// NOT_MEASURED; its reason names the first stub, assertion or boundary fact that applies.
func ApplyJSExampleSource(p Projection, source ExampleSource, anchor string) Projection {
	if source.Read != ExampleSourceLocated {
		if p.Hygiene.State != HygieneIneligible {
			p.Hygiene = Axis{State: HygieneAbstained, Reason: source.Read, Anchors: []string{anchor}}
		}
		return p
	}
	if p.Hygiene.State != HygieneIneligible {
		p.Hygiene = Axis{State: HygieneEligible, Reason: HygieneReasonJSExampleRead, Anchors: []string{anchor}}
	}
	p = ApplySourceFindings(p, source.Findings)
	if p.Strength.State != StrengthNotMeasured {
		return p
	}
	p.Strength = JSExampleStrength(source, anchor)
	return p
}

// JSExampleStrength is the NOT_MEASURED strength axis of one located JS/TS example (TSA-V0-080).
func JSExampleStrength(source ExampleSource, anchor string) Axis {
	locations := func(rule string) []string {
		out := []string{}
		for _, f := range source.Findings {
			if f.Rule == rule {
				out = append(out, f.Location)
			}
		}
		return out
	}
	if subject := locations(RuleJSSubjectMocked); len(subject) > 0 {
		return Axis{State: StrengthNotMeasured, Reason: StrengthReasonSubjectMocked, Anchors: subject}
	}
	if source.Expectations == 0 {
		return Axis{State: StrengthNotMeasured, Reason: StrengthReasonNoBodyExpectation, Anchors: []string{anchor}}
	}
	if constant := locations(RuleJSConstantExpectation); len(constant) > 0 {
		return Axis{State: StrengthNotMeasured, Reason: StrengthReasonConstantOnly, Anchors: constant}
	}
	if boundary := locations(RuleJSBoundaryMocked); len(boundary) > 0 {
		return Axis{State: StrengthNotMeasured, Reason: StrengthReasonBoundaryMocked, Anchors: boundary}
	}
	return Axis{State: StrengthNotMeasured, Reason: StrengthReasonBodyExpectations, Anchors: append([]string{}, source.ExpectationAnchors...)}
}
