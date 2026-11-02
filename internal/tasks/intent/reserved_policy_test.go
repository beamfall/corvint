package intent_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0802_ReservedPolicyFields: each parsed-but-unenforced policy field
// is named when set to a non-default value and only then; a policy that
// sets none of them has no warning (V1-3948).
func TestTCLV0802_ReservedPolicyFields(t *testing.T) {
	decode := func(edit func(*wire.Object)) *intent.Policy {
		t.Helper()
		v := fixture.PolicyValue()
		v.Obj.Set("reviewLane", wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(false))))
		v.Obj.Set("docsLane", wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(false))))
		v.Obj.Set("cemRequired", wire.Bool(false))
		v.Obj.Set("ocmRequired", wire.Bool(false))
		capacity, _ := v.Obj.Get("capacity")
		capacity.Obj.Set("classes", wire.Array())
		if edit != nil {
			edit(v.Obj)
		}
		p, err := intent.DecodePolicy(wire.EncodeFile(v))
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := decode(nil); len(intent.ReservedPolicyFields(p)) != 0 || intent.ReservedPolicyWarning(p) != "" {
		t.Fatalf("default policy: %v", intent.ReservedPolicyFields(p))
	}
	required := func(key string) func(*wire.Object) {
		return func(o *wire.Object) { o.Set(key, wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(true)))) }
	}
	for field, edit := range map[string]func(*wire.Object){
		"reviewLane.required": required("reviewLane"),
		"docsLane.required":   required("docsLane"),
		"cemRequired":         func(o *wire.Object) { o.Set("cemRequired", wire.Bool(true)) },
		"ocmRequired":         func(o *wire.Object) { o.Set("ocmRequired", wire.Bool(true)) },
		"capacity.classes": func(o *wire.Object) {
			capacity, _ := o.Get("capacity")
			capacity.Obj.Set("classes", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("gpu")).Set("units", wire.String("1")))))
		},
	} {
		p := decode(edit)
		if got := intent.ReservedPolicyFields(p); !slices.Equal(got, []string{field}) {
			t.Fatalf("%s: reserved %v", field, got)
		}
		if w := intent.ReservedPolicyWarning(p); !strings.Contains(w, field) || !strings.Contains(w, "TCL-V0-802") {
			t.Fatalf("%s: warning %q", field, w)
		}
	}
}
