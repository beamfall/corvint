package testvalidity

import (
	"reflect"
	"testing"
)

// TestApplyExampleSourceHygiene: a located example becomes ELIGIBLE and an invalid finding makes it
// INELIGIBLE with the rule ID as the reason; an unlocated one is ABSTAINED with the read reason;
// an INELIGIBLE axis is never relaxed; the association, freshness and execution axes are unchanged
// (TSA-V0-063..065, TSA-V0-068).
func TestApplyExampleSourceHygiene(t *testing.T) {
	p := eligibleProjection()
	p.Hygiene = Axis{State: HygieneUnsupported, Reason: "ruby-hygiene-not-produced", Anchors: []string{"h"}}
	clean := ApplyExampleSource(p, ExampleSource{Read: ExampleSourceLocated, Expectations: 1, Findings: []SourceFinding{}, ExpectationAnchors: []string{"s:3"}}, "s:2")
	if want := (Axis{State: HygieneEligible, Reason: "rspec-example-source-read", Anchors: []string{"s:2"}}); !reflect.DeepEqual(clean.Hygiene, want) {
		t.Errorf("clean hygiene %+v", clean.Hygiene)
	}
	if clean.Association.State != p.Association.State || clean.Freshness.State != p.Freshness.State || clean.Execution.State != p.Execution.State {
		t.Errorf("other axes changed: %+v", clean)
	}
	for _, rule := range []string{RuleRSpecSkipped, RuleRSpecFocused, RuleSharedExampleOnly} {
		got := ApplyExampleSource(p, ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{{Rule: rule, Severity: SeverityInvalid, Location: "s:2", Cause: "s:1"}}}, "s:2")
		if want := (Axis{State: HygieneIneligible, Reason: rule, Anchors: []string{"s:2", "s:1"}}); !reflect.DeepEqual(got.Hygiene, want) {
			t.Errorf("%s hygiene %+v", rule, got.Hygiene)
		}
	}
	unplaced := ApplyExampleSource(p, ExampleSource{Read: HygieneReasonRSpecSourceUnbound}, "s:2")
	if want := (Axis{State: HygieneAbstained, Reason: HygieneReasonRSpecSourceUnbound, Anchors: []string{"s:2"}}); !reflect.DeepEqual(unplaced.Hygiene, want) || !reflect.DeepEqual(unplaced.Strength, p.Strength) {
		t.Errorf("unplaced %+v", unplaced)
	}
	pending := p
	pending.Hygiene = Axis{State: HygieneIneligible, Reason: HygieneReasonRSpecPending, Anchors: []string{"s:2"}}
	for _, source := range []ExampleSource{{Read: ExampleSourceLocated}, {Read: HygieneReasonRSpecExampleUnplaced}} {
		if got := ApplyExampleSource(pending, source, "s:2"); !reflect.DeepEqual(got.Hygiene, pending.Hygiene) {
			t.Errorf("%q relaxed INELIGIBLE: %+v", source.Read, got.Hygiene)
		}
	}
}

// TestApplyExampleSourceStrength: strength stays NOT_MEASURED; its reason names a described_class
// stub, an example body with no expectation, or the body expectation lines; a strength a mutation
// run measured is kept (TSA-V0-066, TSA-V0-067, TSA-V0-069).
func TestApplyExampleSourceStrength(t *testing.T) {
	p := eligibleProjection()
	stub := SourceFinding{Rule: RuleDescribedClassStubbed, Severity: SeverityAdvisory, Location: "s:3", Cause: "s:2"}
	for _, c := range []struct {
		source ExampleSource
		want   Axis
	}{
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 2, Findings: []SourceFinding{stub}, ExpectationAnchors: []string{"s:4"}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonDescribedClassStub, Anchors: []string{"s:3"}}},
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 0},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonNoBodyExpectation, Anchors: []string{"s:2"}}},
		{ExampleSource{Read: ExampleSourceLocated, Expectations: 3, ExpectationAnchors: []string{"s:3", "s:4"}},
			Axis{State: StrengthNotMeasured, Reason: StrengthReasonBodyExpectations, Anchors: []string{"s:3", "s:4"}}},
	} {
		got := ApplyExampleSource(p, c.source, "s:2")
		if !reflect.DeepEqual(got.Strength, c.want) {
			t.Errorf("strength %+v, want %+v", got.Strength, c.want)
		}
		if c.source.Expectations == 2 && got.Hygiene.State != HygieneEligible {
			t.Errorf("advisory stub changed hygiene: %+v", got.Hygiene)
		}
	}
	killed := p
	killed.Strength = Axis{State: StrengthKilled, Reason: "mutant-killed", Anchors: []string{"m"}}
	if got := ApplyExampleSource(killed, ExampleSource{Read: ExampleSourceLocated, Findings: []SourceFinding{stub}}, "s:2"); !reflect.DeepEqual(got.Strength, killed.Strength) {
		t.Errorf("measured strength replaced: %+v", got.Strength)
	}
}
