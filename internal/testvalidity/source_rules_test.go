package testvalidity

import (
	"reflect"
	"testing"
)

func eligibleProjection() Projection {
	return Projection{
		Association: Axis{State: AssociationAssociated, Reason: "", Anchors: []string{"a"}},
		Hygiene:     Axis{State: HygieneEligible, Reason: "", Anchors: []string{"h"}},
		Freshness:   Axis{State: FreshnessCurrent, Reason: "", Anchors: []string{}},
		Execution:   Axis{State: ExecutionPassed, Reason: "", Anchors: []string{}},
		Strength:    Axis{State: StrengthNotMeasured, Reason: "no-mutation-run", Anchors: []string{}},
	}
}

// TestApplySourceFindingsInvalidMakesIneligible: an invalid finding makes hygiene INELIGIBLE and
// names the memoizing hook; the other axes are unchanged (TSA-V0-051).
func TestApplySourceFindingsInvalidMakesIneligible(t *testing.T) {
	p := eligibleProjection()
	got := ApplySourceFindings(p, []SourceFinding{
		{Rule: RuleIsolatedInSerial, Severity: SeverityAdvisory, Location: "e2e/a.spec.ts:5", Cause: "e2e/a.spec.ts:1"},
		{Rule: RuleMemoizedBlockExpectation, Severity: SeverityInvalid, Location: "spec/w_spec.rb:9", Cause: "spec/w_spec.rb:4"},
	})
	want := Axis{State: HygieneIneligible, Reason: HygieneReasonMemoizedBlockExpectation, Anchors: []string{"spec/w_spec.rb:9", "spec/w_spec.rb:4"}}
	if !reflect.DeepEqual(got.Hygiene, want) {
		t.Fatalf("hygiene %+v, want %+v", got.Hygiene, want)
	}
	got.Hygiene = p.Hygiene
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("other axes changed: %+v", got)
	}
	if !reflect.DeepEqual(p.Hygiene.Anchors, []string{"h"}) {
		t.Fatalf("input mutated: %+v", p.Hygiene)
	}
}

// TestApplySourceFindingsAdvisoryAndExisting: advisory findings change nothing (TSA-V0-047), and an
// already INELIGIBLE hygiene keeps its reason while gaining the anchors (TSA-V0-051).
func TestApplySourceFindingsAdvisoryAndExisting(t *testing.T) {
	p := eligibleProjection()
	if got := ApplySourceFindings(p, []SourceFinding{{Rule: RuleIsolatedInSerial, Severity: SeverityAdvisory}}); !reflect.DeepEqual(got, p) {
		t.Fatalf("advisory changed the projection: %+v", got)
	}
	if got := ApplySourceFindings(p, nil); !reflect.DeepEqual(got, p) {
		t.Fatalf("no findings changed the projection: %+v", got)
	}
	p.Hygiene = Axis{State: HygieneIneligible, Reason: "empty-body", Anchors: []string{"x"}}
	got := ApplySourceFindings(p, []SourceFinding{{Rule: RuleMemoizedBlockExpectation, Severity: SeverityInvalid, Location: "l", Cause: "c"}})
	if want := (Axis{State: HygieneIneligible, Reason: "empty-body", Anchors: []string{"x", "l", "c"}}); !reflect.DeepEqual(got.Hygiene, want) {
		t.Fatalf("hygiene %+v, want %+v", got.Hygiene, want)
	}
}
