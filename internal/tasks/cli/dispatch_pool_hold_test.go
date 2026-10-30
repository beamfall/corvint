package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ATR-V0-120, ATR-V0-122, ATR-V0-124 (V1-1833, issue 1147): the
// dispatcher's pre-launch claim runs the native `claim --pool`, so a ticket
// waiting on the pool (PSR-V0-132: grace, then priority, then arrival) keeps
// its place and the claim of another ticket is refused naming it; the
// waiter's claim allocates the member, and `pool release` returns that
// exact allocation while the attempt stays live.
func TestATRV0124_PoolClaimKeepsWaiterOrderCLI(t *testing.T) {
	r := exclusionCLIRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("db")).Set("members", wire.Strings([]string{"a"})).Set("maxWaitSeconds", wire.String("600")))))
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init %s", x.stdout)
	}
	one := planTicket(t, r.Root, "one", "P2", `[]`)
	two := planTicket(t, r.Root, "two", "P2", `[]`)
	if x := atm(t, r.Root, nil, "pool", "wait", "--ticket", two, "--holder", "hold.impl.2", "--pool", "db", "--ttl-seconds", "300", "--request-id", "w2"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket wait %s", x.stdout)
	}
	_, err := cli.PoolClaimDispatch(r.Root, dispatch.Assignment{Ticket: one, Pool: "db", NextStage: "implement"}, "hold.impl.1", "claim-one")
	if err == nil || !strings.Contains(err.Error(), "RESOURCE_COLLISION") || !strings.Contains(err.Error(), two) {
		t.Fatalf("claim ahead of the waiter: %v", err)
	}
	h, err := cli.PoolClaimDispatch(r.Root, dispatch.Assignment{Ticket: two, Pool: "db", NextStage: "implement"}, "hold.impl.2", "claim-two")
	if err != nil || h.Attempt == "" || h.Generation == "" || h.Member != "a" || h.Allocation == "" {
		t.Fatalf("waiter claim %+v %v", h, err)
	}
	if got := identityPools(t, r).Waiters; len(got) != 0 {
		t.Fatalf("waiter entry kept: %+v", got)
	}
	if err := cli.PoolReleaseDispatch(r.Root, dispatch.Attempt{ID: h.Attempt, Generation: h.Generation}, h.Allocation, "release-two"); err != nil {
		t.Fatalf("pool release %v", err)
	}
	obs, err := cli.ObserveDispatch(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range obs.Attempts {
		if a.ID == h.Attempt && (!a.Live || a.Member != "") {
			t.Fatalf("attempt after the return %+v", a)
		}
	}
}
