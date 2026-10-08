package intent_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0008_PolicyKnowHowWorkerAddOptIn pins the optional knowHow policy
// key: omission keeps the canonical bytes and no WORKER grant, a closed
// boolean workerAdd sets the opt-in, and anything else refuses. The key never
// lets a roles row name KNOWHOW_ADD for WORKER.
func TestKHNV0008_PolicyKnowHowWorkerAddOptIn(t *testing.T) {
	raw := fixture.PolicyBytes()
	p, err := intent.DecodePolicy(raw)
	if err != nil || p.KnowHow != nil || p.WorkerKnowHowAdd() || bytes.Contains(raw, []byte("knowHow")) {
		t.Fatalf("default policy: %v %+v", err, p)
	}
	decode := func(def wire.Value) (*intent.Policy, []byte, error) {
		v := fixture.PolicyValue()
		v.Obj.Set("knowHow", def)
		b := wire.EncodeFile(v)
		p, err := intent.DecodePolicy(b)
		return p, b, err
	}
	for _, on := range []bool{true, false} {
		p, b, err := decode(wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.Bool(on))))
		if err != nil || p.KnowHow == nil || p.WorkerKnowHowAdd() != on || !bytes.Equal(p.Raw, b) {
			t.Fatalf("workerAdd %v: %v %+v", on, err, p)
		}
	}
	for name, def := range map[string]wire.Value{
		"unknown member": wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.Bool(true)).Set("prefixes", wire.Strings([]string{"docs/"}))),
		"not a boolean":  wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.String("true"))),
		"empty":          wire.ObjectValue(wire.NewObject()),
		"null":           wire.Null(),
		"not an object":  wire.Bool(true),
	} {
		if _, _, err := decode(def); wire.CodeOf(err) != wire.CodeMalformed {
			t.Fatalf("%s: got %v, want MALFORMED", name, err)
		}
	}
	v := fixture.PolicyValue()
	v.Obj.Set("knowHow", wire.ObjectValue(wire.NewObject().Set("workerAdd", wire.Bool(true))))
	v.Obj.Set("roles", wire.ObjectValue(wire.NewObject().Set("WORKER", wire.Strings([]string{"KNOWHOW_ADD"}))))
	if _, err := intent.DecodePolicy(wire.EncodeFile(v)); err == nil {
		t.Fatal("a WORKER row naming KNOWHOW_ADD was admitted beside knowHow.workerAdd")
	}
}
