package intent_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func externalReviewDef(gate string) *wire.Object {
	return wire.NewObject().Set("authorStages", wire.Strings([]string{"implement"})).Set("gateId", wire.String(gate)).
		Set("purpose", wire.String("ROUTING_ONLY")).Set("recorderRoles", wire.Strings([]string{"OWNER"})).
		Set("requireReviewerLease", wire.Bool(true)).Set("reviewStages", wire.Strings([]string{"review"}))
}

// TestERGV0009_PolicyExternalReviewsGrantNothingByDefault: a policy without
// externalReviews declares no review gate; a valid closed definition decodes
// with its canonical-entry digest; a duplicate, an executable-gate collision,
// a non-routing purpose, an unknown member or an empty recorder list refuse.
func TestERGV0009_PolicyExternalReviewsGrantNothingByDefault(t *testing.T) {
	decode := func(defs ...*wire.Object) (*intent.Policy, error) {
		v := fixture.PolicyValue()
		if defs != nil {
			items := make([]wire.Value, 0, len(defs))
			for _, d := range defs {
				items = append(items, wire.ObjectValue(d))
			}
			v.Obj.Set("externalReviews", wire.Array(items...))
		}
		return intent.DecodePolicy(wire.EncodeFile(v))
	}
	p, err := decode()
	if err != nil || p.ExternalReview("g1") != nil || len(p.ExternalReviews) != 0 {
		t.Fatalf("default policy: %v %+v", err, p)
	}
	def := externalReviewDef("g1")
	p, err = decode(def)
	if err != nil {
		t.Fatal(err)
	}
	got := p.ExternalReview("g1")
	if got == nil || !got.RequireReviewerLease || got.Sha256 != wire.Sum(wire.EncodeFile(wire.ObjectValue(def))) {
		t.Fatalf("definition: %+v", got)
	}
	for name, bad := range map[string][]*wire.Object{
		"duplicate":       {externalReviewDef("g1"), externalReviewDef("g1")},
		"executable gate": {externalReviewDef("verify")},
		"purpose":         {externalReviewDef("g1").Set("purpose", wire.String("ACCEPTANCE"))},
		"unknown member":  {externalReviewDef("g1").Set("zz", wire.Null())},
		"no recorders":    {externalReviewDef("g1").Set("recorderRoles", wire.Strings(nil))},
		"worker recorder": {externalReviewDef("g1").Set("recorderRoles", wire.Strings([]string{"WORKER"}))},
		"empty":           {},
	} {
		if _, err := decode(bad...); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
