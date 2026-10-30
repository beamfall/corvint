package cli_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-105: the native dispatcher observation replans its own snapshot
// with a held set. Two held tickets that fill a two-slot window then defer
// as WORK_STATE_HELD and the actionable ticket is selected; replanning is a
// pure in-memory read that leaves the store untouched.
func TestCALV0105_DispatchObservationReplansHeldTickets(t *testing.T) {
	r := fixture.TempRepo(t)
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("2")).Set("maxWorkersTotal", wire.String("2")).Set("classes", wire.Array())))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(v))
	git(t, r.Root, "init", "-b", "main")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-m", "base")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatal(x.res)
	}
	h1 := planTicket(t, r.Root, "h1", "P0", `["h1"]`)
	h2 := planTicket(t, r.Root, "h2", "P0", `["h2"]`)
	a1 := planTicket(t, r.Root, "a1", "P1", `["a1"]`)

	obs, err := cli.ObserveDispatch(r.Root)
	if err != nil {
		t.Fatal(err)
	}
	plans := map[string]dispatch.PlanView{}
	for _, tk := range obs.Tickets {
		plans[tk.ID] = dispatch.PlanView{State: tk.Plan, Reason: tk.PlanReason}
	}
	if plans[h1].State != "SELECTED" || plans[h2].State != "SELECTED" || plans[a1] != (dispatch.PlanView{State: "DEFERRED", Reason: wire.CodeLimitExceeded}) {
		t.Fatalf("starvation not reproduced: %v", plans)
	}
	if obs.Replan == nil {
		t.Fatal("the native observation offers no replan")
	}
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	got := obs.Replan(map[string]bool{h1: true, h2: true}, nil)
	if got[h1] != (dispatch.PlanView{State: "DEFERRED", Reason: "WORK_STATE_HELD"}) || got[h2] != got[h1] || got[a1].State != "SELECTED" {
		t.Fatalf("replan %v", got)
	}
	if again := obs.Replan(map[string]bool{h1: true, h2: true}, nil); again[a1] != got[a1] || again[h1] != got[h1] {
		t.Fatalf("replan is not deterministic: %v vs %v", again, got)
	}
	if !reflect.DeepEqual(state, fixture.TreeSnapshot(t, r.StateDir)) || !reflect.DeepEqual(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("replanning changed the store")
	}
}
