package testacceptance

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestBBFV0016ControlApplicationSeparatesInertAndUnobserved(t *testing.T) {
	marker := &behaviorfalsify.ApplicationMarker{Kind: behaviorfalsify.MarkerAnnotation, Type: "control-applied"}
	plan := behaviorfalsify.Plan{Controls: []behaviorfalsify.PlannedControl{
		{ControlSpec: behaviorfalsify.ControlSpec{ID: "marked-inert", ApplicationMarker: marker}},
		{ControlSpec: behaviorfalsify.ControlSpec{ID: "marked-applied", ApplicationMarker: marker}},
		{ControlSpec: behaviorfalsify.ControlSpec{ID: "unmarked"}},
	}}
	receipt := behaviorfalsify.EvidenceReceipt{Report: behaviorfalsify.Report{InertControls: []string{"marked-inert"}}}
	notApplied, unobserved := controlApplication(plan, receipt)
	if !slices.Equal(notApplied, []string{"marked-inert"}) || !slices.Equal(unobserved, []string{"unmarked"}) {
		t.Fatalf("not applied %v, unobserved %v", notApplied, unobserved)
	}
}

func TestBBFV0016InertControlAbstainsOnStrength(t *testing.T) {
	life := Cleanup{OwnedGroup: true, Descendants: &procgroup.DescendantObservation{Absent: true}}
	for _, schema := range []string{RequestSchema, FreshRequestSchema} {
		t.Run("BBF-V0-016 "+schema, func(t *testing.T) {
			r := Request{Schema: schema, Repeat: 1, Tests: []Test{{ID: "checkout"}}}
			inert := Report{Controls: []Control{{ID: "checkout", Status: "blocked", Cleanup: life, NotApplied: []string{"control-inert"}}}}
			classify(&inert, r)
			a := inert.Assessments[0]
			if !slices.Contains(a.Reasons, "negative-control-not-applied") || a.Validity.Strength.State != testvalidity.StrengthNotMeasured || a.Validity.Strength.Reason != "negative-control-not-applied" || a.Verdict == "accepted" {
				t.Fatalf("inert control was credited: %+v", a)
			}
			// A control without a declared marker keeps its prior assessment; its
			// unobserved application is recorded on the control, not as a verdict reason.
			unmarked := Report{Controls: []Control{{ID: "checkout", Status: "blocked", Cleanup: life, ApplicationUnobserved: []string{"control-unmarked"}}}}
			classify(&unmarked, r)
			legacy := Report{Controls: []Control{{ID: "checkout", Status: "blocked", Cleanup: life}}}
			classify(&legacy, r)
			if !slices.Equal(unmarked.Assessments[0].Reasons, legacy.Assessments[0].Reasons) || !reflect.DeepEqual(unmarked.Assessments[0].Validity.Strength, legacy.Assessments[0].Validity.Strength) || slices.Contains(unmarked.Assessments[0].Reasons, "negative-control-not-applied") {
				t.Fatalf("unmarked control changed today's assessment: %+v vs %+v", unmarked.Assessments[0], legacy.Assessments[0])
			}
		})
	}
}
