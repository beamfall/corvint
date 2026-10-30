package mutation_test

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// poolPolicy is the fixture policy with pools "db" and "gpu", so a REFINE
// that names either passes the unknown-pool check.
func poolPolicy(t *testing.T) []byte {
	t.Helper()
	v := fixture.PolicyValue()
	pool := func(id string) wire.Value {
		return wire.ObjectValue(wire.NewObject().Set("id", wire.String(id)).Set("members", wire.Strings([]string{id + "-a"})))
	}
	v.Obj.Set("pools", wire.Array(pool("db"), pool("gpu")))
	raw := wire.EncodeFile(v)
	if _, err := intent.DecodePolicy(raw); err != nil {
		t.Fatalf("pool policy: %v", err)
	}
	return raw
}

// TestCTSV0011_AdoptComposesPoolAndRoles: an intent file that adds, changes
// or removes requiresPool or requiredRoles is adopted through one composed
// REFINE whose post record carries exactly the file's value (V1-0758). The
// difference is computed over the union of keys, so an added key is never
// dropped while the file is reported adopted.
func TestCTSV0011_AdoptComposesPoolAndRoles(t *testing.T) {
	rolesA := map[string][]string{"implement": {"BUILDER"}, "review": {"REVIEWER"}, "integrate": {"VERIFIER"}}
	rolesB := map[string][]string{"implement": {"BUILDER", "REPAIR"}, "review": {"REVIEWER"}, "integrate": {"VERIFIER"}}
	set := func(pool string, roles map[string][]string) func(*ticket.Record) {
		return func(r *ticket.Record) {
			r.RequiresPool = pool
			r.RequiredRoles = roles
		}
	}
	cases := []struct {
		name        string
		canon, file func(*ticket.Record)
	}{
		{"add pool", set("", nil), set("gpu", nil)},
		{"change pool", set("db", nil), set("gpu", nil)},
		{"remove pool", set("db", nil), set("", nil)},
		{"add roles", set("", nil), set("", rolesA)},
		{"change roles", set("", rolesA), set("", rolesB)},
		{"remove roles", set("", rolesA), set("", nil)},
		{"change both", set("db", rolesA), set("gpu", rolesB)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canonical, err := ticket.Decode(fileOf(t, fixture.Ticket("AT-01"), tc.canon))
			if err != nil {
				t.Fatalf("canonical: %v", err)
			}
			file := fileOf(t, canonical, tc.file)
			expect, err := ticket.Decode(file)
			if err != nil {
				t.Fatalf("file: %v", err)
			}
			plan := adopt(t, ctxWithPolicy(t, owner, nil, poolPolicy(t), canonical), canonical, file)
			want(t, plan, mutation.OutcomeCompleted, "")
			if len(plan.Composed) != 1 || plan.Composed[0] != mutation.OpRefine {
				t.Fatalf("composed %v; want [REFINE]", plan.Composed)
			}
			if plan.Post.RequiresPool != expect.RequiresPool {
				t.Fatalf("post requiresPool %q; want the file's %q", plan.Post.RequiresPool, expect.RequiresPool)
			}
			if !reflect.DeepEqual(plan.Post.RequiredRoles, expect.RequiredRoles) {
				t.Fatalf("post requiredRoles %v; want the file's %v", plan.Post.RequiredRoles, expect.RequiredRoles)
			}
			// Both keys are acceptance-relevant (CAL-V0-028, requiredRoles):
			// the one adoption revision also bumps acceptanceRevision once.
			if plan.Post.Revision != "2" || plan.Post.AcceptanceRevision != "2" {
				t.Fatalf("post revision %s acceptanceRevision %s; want 2 and 2", plan.Post.Revision, plan.Post.AcceptanceRevision)
			}
		})
	}
}

// TestCTSV0011_AdoptUnknownPoolRefused: the composed REFINE is validated by
// the same step rules as a direct one, so a file naming a pool the policy
// does not declare is refused and the ticket stays diverged.
func TestCTSV0011_AdoptUnknownPoolRefused(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	file := fileOf(t, canonical, func(r *ticket.Record) { r.RequiresPool = "nope" })
	plan := adopt(t, ctxWithPolicy(t, owner, nil, poolPolicy(t), canonical), canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	stillDiverged(t, plan, canonical, before)
}
