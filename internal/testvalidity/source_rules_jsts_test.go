package testvalidity

import (
	"reflect"
	"testing"
)

// TestApplyJSExampleSource: a located JS/TS example is ELIGIBLE js-example-source-read, an invalid
// JS-SKIPPED, JS-TODO or JS-FOCUSED finding makes it INELIGIBLE, an unlocated one is ABSTAINED and
// INELIGIBLE is never relaxed; strength stays NOT_MEASURED with the first applicable reason and a
// measured strength is kept (TSA-V0-074, TSA-V0-075, TSA-V0-080).
func TestApplyJSExampleSource(t *testing.T) {
	p := eligibleProjection()
	clean := ApplyJSExampleSource(p, ExampleSource{Read: ExampleSourceLocated, Expectations: 1, Findings: []SourceFinding{}, ExpectationAnchors: []string{"s:3"}}, "s:2")
	if want := (Axis{State: HygieneEligible, Reason: HygieneReasonJSExampleRead, Anchors: []string{"s:2"}}); !reflect.DeepEqual(clean.Hygiene, want) {
		t.Errorf("clean hygiene %+v", clean.Hygiene)
	}
	if clean.Association.State != p.Association.State || clean.Freshness.State != p.Freshness.State || clean.Execution.State != p.Execution.State {
		t.Errorf("other axes changed: %+v", clean)
	}
	for _, rule := range []string{RuleJSSkipped, RuleJSTodo, RuleJSFocused} {
		got := ApplyJSExampleSource(p, ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{{Rule: rule, Severity: SeverityInvalid, Location: "s:2", Cause: "s:1"}}}, "s:2")
		if want := (Axis{State: HygieneIneligible, Reason: rule, Anchors: []string{"s:2", "s:1"}}); !reflect.DeepEqual(got.Hygiene, want) {
			t.Errorf("%s hygiene %+v", rule, got.Hygiene)
		}
	}
	unplaced := ApplyJSExampleSource(p, ExampleSource{Read: HygieneReasonJSSourceUnbound}, "s:2")
	if want := (Axis{State: HygieneAbstained, Reason: HygieneReasonJSSourceUnbound, Anchors: []string{"s:2"}}); !reflect.DeepEqual(unplaced.Hygiene, want) || !reflect.DeepEqual(unplaced.Strength, p.Strength) {
		t.Errorf("unplaced %+v", unplaced)
	}
	refused := p
	refused.Hygiene = Axis{State: HygieneIneligible, Reason: "isolated-in-serial", Anchors: []string{"s:2"}}
	for _, source := range []ExampleSource{{Read: ExampleSourceLocated}, {Read: HygieneReasonJSExampleUnplaced}} {
		if got := ApplyJSExampleSource(refused, source, "s:2"); !reflect.DeepEqual(got.Hygiene, refused.Hygiene) {
			t.Errorf("%q relaxed INELIGIBLE: %+v", source.Read, got.Hygiene)
		}
	}

	finding := func(rule, location string) SourceFinding {
		return SourceFinding{Rule: rule, Severity: SeverityAdvisory, Location: location, Cause: "s:2"}
	}
	subject, boundary, constant := finding(RuleJSSubjectMocked, "s:1"), finding(RuleJSBoundaryMocked, "s:5"), finding(RuleJSConstantExpectation, "s:4")
	for _, c := range []struct {
		source ExampleSource
		want   Axis
	}{
		{ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{subject, boundary}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonSubjectMocked, Anchors: []string{"s:1"}}},
		{ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{boundary}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonNoBodyExpectation, Anchors: []string{"s:2"}}},
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 1, Findings: []SourceFinding{constant, boundary}, ExpectationAnchors: []string{"s:4"}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonConstantOnly, Anchors: []string{"s:4"}}},
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 1, Findings: []SourceFinding{boundary}, ExpectationAnchors: []string{"s:4"}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonBoundaryMocked, Anchors: []string{"s:5"}}},
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 2, ExpectationAnchors: []string{"s:3", "s:4"}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonBodyExpectations, Anchors: []string{"s:3", "s:4"}}},
	} {
		got := ApplyJSExampleSource(p, c.source, "s:2")
		if !reflect.DeepEqual(got.Strength, c.want) {
			t.Errorf("strength %+v, want %+v", got.Strength, c.want)
		}
		if got.Hygiene.State != HygieneEligible {
			t.Errorf("advisory finding changed hygiene: %+v", got.Hygiene)
		}
	}
	killed := p
	killed.Strength = Axis{State: StrengthKilled, Reason: "mutant-killed", Anchors: []string{"m"}}
	if got := ApplyJSExampleSource(killed, ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{subject}}, "s:2"); !reflect.DeepEqual(got.Strength, killed.Strength) {
		t.Errorf("measured strength replaced: %+v", got.Strength)
	}
}
