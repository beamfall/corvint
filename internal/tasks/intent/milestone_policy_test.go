package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0195_PolicyMilestonesOptIn pins the optional milestones policy
// key: omission keeps the canonical bytes and an optional milestone, a closed
// boolean required sets the opt-in, and anything else refuses MALFORMED.
func TestCALV0195_PolicyMilestonesOptIn(t *testing.T) {
	raw := fixture.PolicyBytes()
	p, err := intent.DecodePolicy(raw)
	if err != nil || p.Milestones != nil || p.MilestoneRequired() || bytes.Contains(raw, []byte("milestones")) {
		t.Fatalf("default policy: %v %+v", err, p)
	}
	if (*intent.Policy)(nil).MilestoneRequired() {
		t.Fatal("nil policy requires a milestone")
	}
	decode := func(def wire.Value) (*intent.Policy, []byte, error) {
		v := fixture.PolicyValue()
		v.Obj.Set("milestones", def)
		b := wire.EncodeFile(v)
		p, err := intent.DecodePolicy(b)
		return p, b, err
	}
	for _, on := range []bool{true, false} {
		p, b, err := decode(wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(on))))
		if err != nil || p.Milestones == nil || p.MilestoneRequired() != on || !bytes.Equal(p.Raw, b) {
			t.Fatalf("required %v: %v %+v", on, err, p)
		}
	}
	for name, def := range map[string]wire.Value{
		"unknown member": wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(true)).Set("onRefine", wire.Bool(true))),
		"not a boolean":  wire.ObjectValue(wire.NewObject().Set("required", wire.String("true"))),
		"empty":          wire.ObjectValue(wire.NewObject()),
		"null":           wire.Null(),
		"not an object":  wire.Bool(true),
	} {
		if _, _, err := decode(def); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: got %v, want MALFORMED", name, err)
		}
	}
}
