package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PSR-V0-130, PSR-V0-134, PSR-V0-135: `pool wait` registers an attempt or a
// ticket on the waitlist, replays by request id, shows in queue status and
// plan preview, and the waiting attempt's acquire consumes its entry, after
// which neither view carries the key. PSR-V0-140: the dispatcher's
// observation counts the pool's free members before and after the acquire.
func TestPSRV0135_PoolWaitCLI(t *testing.T) {
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
	a, g := identityClaim(t, r, "one")
	two := planTicket(t, r.Root, "two", "P2", `["two"]`)
	wait := []string{"pool", "wait", "--attempt", a, "--generation", g, "--pool", "db", "--ttl-seconds", "300", "--request-id", "w1"}
	for i := 0; i < 2; i++ {
		if x := atm(t, r.Root, nil, wait...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("pool wait %d %s", i, x.stdout)
		}
	}
	if x := atm(t, r.Root, nil, "pool", "wait", "--ticket", two, "--holder", "builder", "--pool", "db", "--ttl-seconds", "300", "--request-id", "w2"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket wait %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "pool", "wait", "--ticket", two, "--holder", "builder", "--pool", "db", "--ttl-seconds", "601", "--request-id", "w3"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("601 s admitted %s", x.stdout)
	}
	if got := identityPools(t, r).Waiters; len(got) != 2 {
		t.Fatalf("waiters %+v", got)
	}
	for _, args := range [][]string{{"queue", "status"}, {"plan", "preview"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK || !bytes.Contains(x.stdout, []byte(`"poolWaitlist":[`)) || !bytes.Contains(x.stdout, []byte(`"attemptId":"`+a+`"`)) {
			t.Fatalf("%v %s", args, x.stdout)
		}
	}
	// PSR-V0-140: the dispatcher observes each declared pool's free members.
	poolFree := func() int {
		t.Helper()
		obs, err := cli.ObserveDispatch(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		return obs.PoolFree["db"]
	}
	if n := poolFree(); n != 1 {
		t.Fatalf("free before acquire %d", n)
	}
	if x := atm(t, r.Root, nil, "pool", "acquire", "--attempt", a, "--generation", g, "--pool", "db", "--request-id", "acq"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("waiter acquire %s", x.stdout)
	}
	if got := identityPools(t, r).Waiters; len(got) != 1 || got[0].TicketID != two {
		t.Fatalf("consumed entry kept: %+v", got)
	}
	if n := poolFree(); n != 0 {
		t.Fatalf("free after acquire %d", n)
	}
}
