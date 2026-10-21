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
// appends the expectation and the memoizing hook to the existing anchors; the other axes are
// unchanged (TSA-V0-051).
func TestApplySourceFindingsInvalidMakesIneligible(t *testing.T) {
	p := eligibleProjection()
	got := ApplySourceFindings(p, []SourceFinding{
		{Rule: RuleIsolatedInSerial, Severity: SeverityAdvisory, Location: "e2e/a.spec.ts:5", Cause: "e2e/a.spec.ts:1"},
		{Rule: RuleMemoizedBlockExpectation, Severity: SeverityInvalid, Location: "spec/w_spec.rb:9", Cause: "spec/w_spec.rb:4"},
	})
	want := Axis{State: HygieneIneligible, Reason: HygieneReasonMemoizedBlockExpectation, Anchors: []string{"h", "spec/w_spec.rb:9", "spec/w_spec.rb:4"}}
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
	short := ApplySourceFindings(p, []SourceFinding{{Rule: RuleMemoizedBlockExpectation, Severity: SeverityInvalid, Location: "l", Cause: "c"}})
	if !reflect.DeepEqual(short.Hygiene.Anchors, []string{"h", "l", "c"}) {
		t.Fatalf("anchors %+v, want [h l c]", short.Hygiene.Anchors)
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

// TestApplySourceFindingsBlockNotCalled: an invalid BLOCK-NOT-CALLED finding makes hygiene
// INELIGIBLE with its own reason and appends the call and the helper to the anchors (TSA-V0-056).
func TestApplySourceFindingsBlockNotCalled(t *testing.T) {
	p := eligibleProjection()
	got := ApplySourceFindings(p, []SourceFinding{{Rule: RuleBlockNotCalled, Severity: SeverityInvalid, Location: "spec/n_spec.rb:11", Cause: "spec/n_spec.rb:2"}})
	want := Axis{State: HygieneIneligible, Reason: HygieneReasonBlockNotCalled, Anchors: []string{"h", "spec/n_spec.rb:11", "spec/n_spec.rb:2"}}
	if !reflect.DeepEqual(got.Hygiene, want) || HygieneReasonBlockNotCalled != "block-not-called" {
		t.Fatalf("hygiene %+v, want %+v", got.Hygiene, want)
	}
	got.Hygiene = p.Hygiene
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("other axes changed: %+v", got)
	}
}

// TestApplySourceFindingsAnchorsDeduplicated: an anchor that is both a location and a cause, or
// that repeats across findings or the existing anchors, is listed once at its first position, and
// no anchor is dropped (TSA-V0-051, V1-1865).
func TestApplySourceFindingsAnchorsDeduplicated(t *testing.T) {
	p := eligibleProjection()
	p.Hygiene.Anchors = []string{"spec/a_spec.rb:6"}
	got := ApplySourceFindings(p, []SourceFinding{
		{Rule: RuleBlockNotCalled, Severity: SeverityInvalid, Location: "spec/a_spec.rb:6", Cause: "spec/a_spec.rb:6"},
		{Rule: RuleMemoizedBlockExpectation, Severity: SeverityInvalid, Location: "spec/a_spec.rb:9", Cause: "spec/a_spec.rb:6"},
	})
	if want := []string{"spec/a_spec.rb:6", "spec/a_spec.rb:9"}; !reflect.DeepEqual(got.Hygiene.Anchors, want) {
		t.Fatalf("anchors %+v, want %+v", got.Hygiene.Anchors, want)
	}
	abst := ApplySourceFindings(eligibleProjection(), []SourceFinding{{Rule: "R", Severity: SeverityAbstained, Location: "h", Cause: "h"}})
	if want := []string{"h"}; !reflect.DeepEqual(abst.Hygiene.Anchors, want) {
		t.Fatalf("abstained anchors %+v, want %+v", abst.Hygiene.Anchors, want)
	}
}
