package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func slotDef(id, capacity string, reserved *wire.Object) wire.Value {
	o := wire.NewObject().Set("id", wire.String(id)).Set("capacity", wire.String(capacity))
	if reserved != nil {
		o.Set("reserved", wire.ObjectValue(reserved))
	}
	return wire.ObjectValue(o)
}

// TestPSRV0030_PolicyDeclaresSlots pins the optional policy `slots` key:
// omission keeps the canonical bytes, a closed declaration decodes with its
// sorted reserved classes, and every out-of-bound or malformed declaration
// refuses.
func TestPSRV0030_PolicyDeclaresSlots(t *testing.T) {
	raw := fixture.PolicyBytes()
	p, err := intent.DecodePolicy(raw)
	if err != nil || len(p.Slots) != 0 || p.SlotResource("gpu") != nil || bytes.Contains(raw, []byte(`"slots"`)) {
		t.Fatalf("default policy: %v %+v", err, p)
	}
	decode := func(slots wire.Value) (*intent.Policy, []byte, error) {
		v := fixture.PolicyValue()
		v.Obj.Set("slots", slots)
		b := wire.EncodeFile(v)
		p, err := intent.DecodePolicy(b)
		return p, b, err
	}
	p, b, err := decode(wire.Array(slotDef("gpu", "4", wire.NewObject().Set("nightly", wire.String("1")).Set("ci", wire.String("2"))), slotDef("api", "64", nil)))
	if err != nil || !bytes.Equal(p.Raw, b) || len(p.Slots) != 2 {
		t.Fatalf("declared: %v %+v", err, p)
	}
	gpu := p.SlotResource("gpu")
	if gpu == nil || gpu.Capacity.Int() != 4 || gpu.ReservedFor("ci") != 2 || gpu.ReservedFor("nightly") != 1 || gpu.ReservedFor("other") != 0 {
		t.Fatalf("gpu %+v", gpu)
	}
	if c := gpu.Classes(); len(c) != 2 || c[0] != "ci" || c[1] != "nightly" {
		t.Fatalf("classes %v", c)
	}
	if api := p.SlotResource("api"); api == nil || len(api.Classes()) != 0 {
		t.Fatalf("api %+v", api)
	}
	many := []wire.Value{}
	for i := 0; i <= intent.MaxSlotResources; i++ {
		many = append(many, slotDef("r"+string(rune('a'+i)), "1", nil))
	}
	classes := wire.NewObject()
	for i := 0; i <= intent.MaxSlotClasses; i++ {
		classes.Set("c"+string(rune('a'+i)), wire.String("1"))
	}
	for name, tc := range map[string]struct {
		slots wire.Value
		code  string
	}{
		"empty":             {wire.Array(), wire.CodeMalformed},
		"not an array":      {wire.ObjectValue(wire.NewObject()), wire.CodeMalformed},
		"too many":          {wire.Array(many...), wire.CodeLimitExceeded},
		"zero capacity":     {wire.Array(slotDef("gpu", "0", nil)), wire.CodeLimitExceeded},
		"capacity over 64":  {wire.Array(slotDef("gpu", "65", nil)), wire.CodeLimitExceeded},
		"duplicate":         {wire.Array(slotDef("gpu", "1", nil), slotDef("gpu", "2", nil)), wire.CodeDuplicateID},
		"bad id":            {wire.Array(slotDef("bad\nid", "1", nil)), wire.CodeMalformed},
		"unknown key":       {wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("gpu")).Set("capacity", wire.String("1")).Set("ttl", wire.String("1")))), wire.CodeMalformed},
		"empty reserved":    {wire.Array(slotDef("gpu", "2", wire.NewObject())), wire.CodeMalformed},
		"reserved zero":     {wire.Array(slotDef("gpu", "2", wire.NewObject().Set("ci", wire.String("0")))), wire.CodeLimitExceeded},
		"reserved overflow": {wire.Array(slotDef("gpu", "2", wire.NewObject().Set("ci", wire.String("2")).Set("nightly", wire.String("1")))), wire.CodeLimitExceeded},
		"too many classes":  {wire.Array(slotDef("gpu", "64", classes)), wire.CodeLimitExceeded},
		"bad class":         {wire.Array(slotDef("gpu", "2", wire.NewObject().Set("bad\tclass", wire.String("1")))), wire.CodeMalformed},
	} {
		if _, _, err := decode(tc.slots); wire.CodeOf(err) != tc.code {
			t.Fatalf("%s: got %v, want %s", name, err, tc.code)
		}
	}
}
