//go:build darwin || linux

package dispatch

import (
	"reflect"
	"testing"
	"time"
)

// TestATRV0150_HeldRecordNamesEveryFence is the V1-1938 coverage. A run-only
// ticket outside the plan window, whose role bypasses focus while its pool
// has a free member, meets each fence alone. GLOBAL_CAP, ROLE_CAP, TIER_CAP,
// BUDGET and PRESSURE are each recorded under that fence; the PSR-V0-140 pool
// room and a skipped key keep it out without a record. Each case runs twice:
// as a first launch and as a pending next-stage relaunch preferring a second
// bypassing role, where the hold is recorded under that role and the bypass
// is kept (never PLAN_WINDOW).
func TestATRV0150_HeldRecordNamesEveryFence(t *testing.T) {
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	type input struct {
		c      *Config
		obs    *Observation
		busy   []Busy
		skip   map[string]bool
		tierOf func(role, key string) int
		budget *PressureBudget
		spend  *spendGate
		room   *poolRoom
		calm   bool
	}
	for _, tc := range []struct {
		fence string // "" expects no record
		set   func(in *input, role string)
	}{
		{FenceGlobalCap, func(in *input, role string) {
			in.c.GlobalCap = 1
			in.busy = []Busy{{Role: "impl", Key: "ticket:a:q:other"}}
		}},
		{FenceRoleCap, func(in *input, role string) {
			in.c.roleNamed(role).Cap = 1
			in.busy = []Busy{{Role: role, Key: "ticket:a:q:other"}}
		}},
		{FenceTierCap, func(in *input, role string) {
			in.c.roleNamed(role).Escalate = []Tier{{After: 1, Model: "B", Cap: 1}}
			in.tierOf = func(string, string) int { return 1 }
			in.busy = []Busy{{Role: role, Key: "ticket:a:q:other", Slot: 1, Tier: 1}}
		}},
		{FenceBudget, func(in *input, role string) {
			in.c.roleNamed(role).Budget = &Budget{SessionsPerDay: 1}
			in.spend = &spendGate{c: in.c, now: now, charged: map[[2]string]int{}, holds: map[[2]string]BudgetHold{}, r: &SpendRecord{Sessions: []SpendSession{
				{Worker: "w1", Role: role, Launched: now.Add(-time.Hour), Usage: UsageKnown, Input: 1},
			}}}
		}},
		{FencePressure, func(in *input, role string) {
			in.budget, in.calm = &PressureBudget{enabled: true}, false
		}},
		{"", func(in *input, role string) { // PSR-V0-140 pool room
			in.room = newPoolRoom(in.obs, []*Worker{{Role: role, Key: "ticket:a:q:other", Ticket: "ticket:a:q:other", Pool: "lanes", State: "RUNNING"}})
		}},
		{"", func(in *input, role string) { // a skipped key
			in.skip = map[string]bool{"ticket:a:q:r1": true}
		}},
	} {
		for _, relaunch := range []bool{false, true} {
			name := tc.fence
			if name == "" && tc.set != nil {
				name = "unrecorded"
			}
			// The relaunch case adds a second bypassing role that the
			// pending relaunch prefers.
			config := func() *Config {
				c := runOnlyConfig(t, true)
				if relaunch {
					second := c.Roles[0]
					second.Name = "runner2"
					c.Roles = append(c.Roles, second)
				}
				return c
			}
			c := config()
			role, prefer := "runner", map[string]string(nil)
			if relaunch {
				role, prefer = "runner2", map[string]string{"ticket:a:q:r1": "runner2"}
			}
			r1 := unselectedRun("r1", 1)
			r1.NextStage = "review"
			other := ticket("other", "P1", 2)
			other.Plan = "SELECTED"
			in := &input{c: c, obs: &Observation{Tickets: []Ticket{r1, other}, PoolFree: map[string]int{"lanes": 1}}, calm: true}
			tc.set(in, role)
			adm := &admission{calm: in.calm}
			out, _ := roster(in.c, in.obs, in.busy, in.skip, in.tierOf, in.budget, prefer, in.spend, in.room, adm)
			for _, a := range out {
				if a.Ticket == "ticket:a:q:r1" {
					t.Fatalf("%s relaunch=%v: r1 launched past its fence (%v)", name, relaunch, keysOf(out))
				}
			}
			want := map[string]string{}
			if tc.fence != "" {
				want[role+"/ticket:a:q:r1"] = tc.fence
			}
			if got := holdFences(adm); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s relaunch=%v: holds %v, want %v", name, relaunch, got, want)
			}
			// Without the fence the same input launches r1 under role, so
			// the hold is the fence's alone.
			if out, _ := roster(config(), in.obs, nil, nil, nil, nil, prefer, nil, nil, &admission{calm: true}); len(out) == 0 || out[0].Ticket != "ticket:a:q:r1" || out[0].Role != role {
				t.Fatalf("%s relaunch=%v: unfenced roster %v", name, relaunch, keysOf(out))
			}
		}
	}
}
