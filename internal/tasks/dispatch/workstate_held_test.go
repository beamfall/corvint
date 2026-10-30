//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// windowQueue plans a two-slot selection window over its tickets in order:
// held tickets defer as WORK_STATE_HELD and take no slot (the planner's
// CAL-V0-105 rule, which internal/tasks/transaction tests directly).
func windowQueue(ts []Ticket, replans *[]map[string]bool) *fakeQueue {
	plan := func(held map[string]bool) map[string]PlanView {
		out, used := map[string]PlanView{}, 0
		for _, t := range ts {
			switch {
			case held[t.ID]:
				out[t.ID] = PlanView{"DEFERRED", "WORK_STATE_HELD"}
			case used < 2:
				used++
				out[t.ID] = PlanView{"SELECTED", "DEVELOPMENT_MODE"}
			default:
				out[t.ID] = PlanView{"DEFERRED", "LIMIT_EXCEEDED"}
			}
		}
		return out
	}
	base := plan(nil)
	for i := range ts {
		ts[i].Plan, ts[i].PlanReason = base[ts[i].ID].State, base[ts[i].ID].Reason
	}
	q := &fakeQueue{obs: Observation{Tickets: ts}}
	q.obs.Replan = func(held, _ map[string]bool) map[string]PlanView {
		*replans = append(*replans, held)
		return plan(held)
	}
	return q
}

func heldRig(t *testing.T, states map[string]string) (*Config, []Ticket) {
	t.Helper()
	c := testConfig(t, "exit 0")
	dir := t.TempDir()
	for local, state := range states {
		if err := os.WriteFile(filepath.Join(dir, local+".md"), []byte("state: "+state+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c.WorkState = &WorkState{Kind: "status-line", Path: filepath.Join(dir, "{ticketLocal}.md"), Key: "state"}
	c.Roles[0].Match = &Match{ExcludeStates: []string{"hold-lane", "hold-dep"}, PlanSelected: true}
	return c, []Ticket{ticket("h1", "P0", 1), ticket("h2", "P0", 2), ticket("r1", "P1", 3)}
}

func planOf(obs *Observation) map[string]PlanView {
	out := map[string]PlanView{}
	for _, t := range obs.Tickets {
		out[t.Local] = PlanView{t.Plan, t.PlanReason}
	}
	return out
}

// TestCALV0105_HeldWorkStateLeavesDispatcherWindow reproduces issue 624: two
// tickets the program's work state holds fill the window as SELECTED, and the
// actionable ticket starves DEFERRED LIMIT_EXCEEDED until the dispatcher
// replans with the held set.
func TestCALV0105_HeldWorkStateLeavesDispatcherWindow(t *testing.T) {
	c, ts := heldRig(t, map[string]string{"h1": "hold-lane", "h2": "hold-dep", "r1": "ready"})
	var replans []map[string]bool
	q := windowQueue(ts, &replans)
	if starved := planOf(&q.obs); starved["r1"] != (PlanView{"DEFERRED", "LIMIT_EXCEEDED"}) {
		t.Fatalf("fixture does not reproduce the starvation: %v", starved)
	}
	d, err := Open("held", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	obs, err := d.observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := planOf(obs)
	want := map[string]PlanView{"h1": {"DEFERRED", "WORK_STATE_HELD"}, "h2": {"DEFERRED", "WORK_STATE_HELD"}, "r1": {"SELECTED", "DEVELOPMENT_MODE"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	if len(replans) != 1 || !reflect.DeepEqual(replans[0], map[string]bool{"ticket:a:q:h1": true, "ticket:a:q:h2": true}) {
		t.Fatalf("replans %v", replans)
	}
	if roster := Roster(c, obs, nil, nil); len(roster) != 1 || roster[0].Local != "r1" {
		t.Fatalf("roster %+v", roster)
	}
}

// TestCALV0105_UnknownOrUnreadStateKeepsWindow: UNKNOWN, NONE, no reader and
// a state some role admits are never held, so the plan is today's.
func TestCALV0105_UnknownOrUnreadStateKeepsWindow(t *testing.T) {
	for name, setup := range map[string]func(*Config){
		"unknown": func(c *Config) {}, // h1 and h2 carry control bytes below
		"no reader": func(c *Config) {
			c.WorkState = nil
			c.Roles[0].Match.ExcludeStates = nil
		},
		"admitted": func(c *Config) {
			c.Roles = append(c.Roles, Role{Name: "lander", Host: "sh", Cap: 1, Match: &Match{States: []string{"hold-lane", "hold-dep"}}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60})
		},
	} {
		t.Run(name, func(t *testing.T) {
			states := map[string]string{"h1": "hold-lane", "h2": "hold-dep", "r1": "ready"}
			if name == "unknown" {
				states = map[string]string{"h1": "bad\x01", "h2": "bad\x01", "r1": "ready"}
			}
			c, ts := heldRig(t, states)
			setup(c)
			var replans []map[string]bool
			q := windowQueue(ts, &replans)
			d, err := Open("held", c, q, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			obs, err := d.observe(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if got := planOf(obs); got["r1"] != (PlanView{"DEFERRED", "LIMIT_EXCEEDED"}) || got["h1"].State != "SELECTED" || len(replans) != 0 {
				t.Fatalf("plan %v replans %v", got, replans)
			}
		})
	}
}

func TestCALV0105_StateMatchesIsTheRolePredicate(t *testing.T) {
	m := &Match{States: []string{"ready"}, ExcludeStates: []string{"hold"}}
	for state, want := range map[string]bool{"ready": true, "hold": false, StateUnknown: false, "other": false} {
		if stateMatches(m, state) != want {
			t.Errorf("stateMatches(%q) = %v", state, !want)
		}
	}
	if !stateMatches(&Match{}, StateUnknown) {
		t.Error("a role naming no states refused UNKNOWN")
	}
}
