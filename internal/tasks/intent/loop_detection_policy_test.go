package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func loopDetectionDef(noProgress, returns string) *wire.Object {
	return wire.NewObject().Set("maxNoProgressGenerations", wire.String(noProgress)).Set("maxAlternatingReturns", wire.String(returns))
}

// TestCALV0102_PolicyLoopDetectionOptIn: a policy without loopDetection
// decodes with no loop policy and keeps its exact bytes as its identity (D8);
// a closed, bounded definition decodes; an unknown member, a missing member,
// zero, a value above MaxLoopDetectionBound or a non-object refuses.
func TestCALV0102_PolicyLoopDetectionOptIn(t *testing.T) {
	t.Run("CAL-V0-102 PolicyLoopDetectionOptIn", func(t *testing.T) {
		raw := fixture.PolicyBytes()
		p, err := intent.DecodePolicy(raw)
		if err != nil || p.LoopDetection != nil || !bytes.Equal(p.Raw, raw) || bytes.Contains(raw, []byte("loopDetection")) {
			t.Fatalf("default policy: %v %+v", err, p)
		}
		decode := func(def wire.Value) (*intent.Policy, []byte, error) {
			v := fixture.PolicyValue()
			v.Obj.Set("loopDetection", def)
			b := wire.EncodeFile(v)
			p, err := intent.DecodePolicy(b)
			return p, b, err
		}
		p, b, err := decode(wire.ObjectValue(loopDetectionDef("2", "3")))
		if err != nil || p.LoopDetection == nil || p.LoopDetection.MaxNoProgressGenerations != "2" || p.LoopDetection.MaxAlternatingReturns != "3" || !bytes.Equal(p.Raw, b) {
			t.Fatalf("definition: %v %+v", err, p)
		}
		if _, _, err := decode(wire.ObjectValue(loopDetectionDef("1", "256"))); err != nil {
			t.Fatalf("bounds: %v", err)
		}
		for name, bad := range map[string]wire.Value{
			"unknown member": wire.ObjectValue(loopDetectionDef("2", "2").Set("window", wire.String("1"))),
			"missing member": wire.ObjectValue(wire.NewObject().Set("maxNoProgressGenerations", wire.String("2"))),
			"zero":           wire.ObjectValue(loopDetectionDef("0", "2")),
			"above bound":    wire.ObjectValue(loopDetectionDef("2", "257")),
			"not a count":    wire.ObjectValue(loopDetectionDef("two", "2")),
			"null":           wire.Null(),
			"empty":          wire.ObjectValue(wire.NewObject()),
		} {
			if _, _, err := decode(bad); err == nil {
				t.Errorf("%s accepted", name)
			}
		}
		if intent.MaxLoopDetectionBound != 256 {
			t.Fatal("bound drifted from the spec")
		}
	})
}
