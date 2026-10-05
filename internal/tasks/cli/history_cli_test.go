package cli_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-079: claim results always carry poolAllocation, a retry's prior
// generation records the ended stage and member, and attempt show writes nothing.
func TestCALV0079_ClaimAllocationAndPriorHistory(t *testing.T) {
	t.Run("CAL-V0-079 ClaimAllocationAndPriorHistory", func(t *testing.T) {
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		runOK := func(args ...string) run {
			t.Helper()
			r := handoffCLI(t, root, args...)
			if r.res.Outcome != wire.OutcomeOK {
				t.Fatalf("%v: %s", args, r.stdout)
			}
			return r
		}
		repo, err := intent.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		v, err := wire.Parse(repo.Policy.Raw)
		if err != nil {
			t.Fatal(err)
		}
		v.Obj.Set("policyVersion", wire.String("2"))
		v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a", "b"})))))
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		policyFile := filepath.Join(dir, "policy.json")
		if err := os.WriteFile(policyFile, wire.EncodeFile(v), 0600); err != nil {
			t.Fatal(err)
		}
		runOK("policy", "update", "--request-id", "pool-policy", "--expected-policy-version", "1", "--file", policyFile)

		plain := planTicket(t, root, "plain-history", "P1", `["src/plain/"]`)
		c := runOK("claim", plain, "--holder", "builder", "--request-id", "plain-1")
		if alloc, ok := c.res.Items[0].Obj.Get("poolAllocation"); !ok || alloc.Kind != wire.KindNull {
			t.Fatalf("unpooled claim result lacks poolAllocation null: %s", c.stdout)
		}
		a := field(c.res.Items[0], "attemptId").Str
		runOK("release", "--attempt", a, "--generation", field(c.res.Items[0], "generation").Str, "--request-id", "plain-release")
		runOK("claim", plain, "--holder", "builder", "--request-id", "plain-2")

		pooled := planTicket(t, root, "pooled-history", "P1", `["src/pooled/"]`)
		p := runOK("claim", pooled, "--holder", "builder", "--stage", "implement", "--pool", "lanes", "--request-id", "pooled-1")
		member := field(field(p.res.Items[0], "poolAllocation"), "memberId").Str
		if member == "" {
			t.Fatalf("pooled claim result: %s", p.stdout)
		}
		pa := field(p.res.Items[0], "attemptId").Str
		runOK("release", "--attempt", pa, "--generation", field(p.res.Items[0], "generation").Str, "--request-id", "pooled-release")
		runOK("claim", pooled, "--holder", "builder", "--stage", "implement", "--pool", "lanes", "--request-id", "pooled-2")

		resolved, err := intent.Resolve(root)
		if err != nil {
			t.Fatal(err)
		}
		before := fixture.TreeSnapshot(t, resolved.StateDir)
		for _, tc := range []struct{ attempt, stage, pool, member string }{{a, "null", "null", "null"}, {pa, "implement", "lanes", member}} {
			show := runOK("attempt", "show", tc.attempt)
			prior := field(show.res.Items[0], "priorGenerations")
			if len(prior.Arr) != 1 {
				t.Fatalf("prior: %s", show.stdout)
			}
			g := prior.Arr[0]
			if field(g, "history").Str != "RECORDED" {
				t.Fatalf("history: %s", show.stdout)
			}
			for key, want := range map[string]string{"stage": tc.stage, "poolId": tc.pool, "memberId": tc.member} {
				got := field(g, key)
				if (want == "null") != (got.Kind == wire.KindNull) || (want != "null" && got.Str != want) {
					t.Fatalf("%s = %s want %s: %s", key, wire.Encode(got), want, show.stdout)
				}
			}
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, resolved.StateDir)) {
			t.Fatal("attempt show wrote state")
		}
		runOK("receipt", "audit")
	})
}
