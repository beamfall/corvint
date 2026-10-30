package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func statusField(t *testing.T, v wire.Value, key string) (wire.Value, bool) {
	t.Helper()
	if v.Kind != wire.KindObject {
		t.Fatalf("not an object: %+v", v)
	}
	return v.Obj.Get(key)
}

// CAL-V0-057: dispatch status shows each running worker's tier and model and,
// per ticket, the no-progress streak and the tier each laddered role launches
// next. A configuration without a ladder keeps the previous status shape.
func TestCALV0057_DispatchStatusShowsTierAndStreak(t *testing.T) {
	no := false
	c := &dispatch.Config{Roles: []dispatch.Role{
		{Name: "impl", Model: "A", Escalate: []dispatch.Tier{{After: 2, Model: "B"}, {After: 4, Model: "C"}}},
		{Name: "sticky", Model: "A", Escalate: []dispatch.Tier{{After: 1, Model: "B"}}, DeescalateOnProgress: &no},
		{Name: "plain"},
	}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Workers: []*dispatch.Worker{{ID: "w", Role: "impl", Key: "ticket:a:q:t1", Tier: 1, Model: "B"}, {ID: "p", Role: "plain", Key: "ticket:a:q:t2"}},
		Escalation: map[string]*dispatch.EscalationState{
			"ticket:a:q:t1": {Streak: 3, Tiers: map[string]int{"impl": 1}},
			"ticket:a:q:t3": {Streak: 0, Tiers: map[string]int{"sticky": 1}},
		}}
	now := time.Now()
	v := dispatchStatusValue(c, t.TempDir(), l, nil, now)
	workers, _ := statusField(t, v, "workers")
	if m, _ := statusField(t, workers.Arr[0], "model"); m.Str != "B" {
		t.Fatalf("worker model = %+v", m)
	}
	if tier, _ := statusField(t, workers.Arr[0], "tier"); tier.Str != "1" {
		t.Fatalf("worker tier = %+v", tier)
	}
	if _, ok := statusField(t, workers.Arr[1], "model"); ok {
		t.Fatal("model-less worker shows a model")
	}
	esc, ok := statusField(t, v, "escalation")
	if !ok || len(esc.Arr) != 2 {
		t.Fatalf("escalation = %+v", esc)
	}
	want := []struct {
		key, streak string
		roles       [][3]string
	}{
		{"ticket:a:q:t1", "3", [][3]string{{"impl", "1", "B"}, {"sticky", "1", "B"}}},
		{"ticket:a:q:t3", "0", [][3]string{{"impl", "0", "A"}, {"sticky", "1", "B"}}},
	}
	for i, w := range want {
		key, _ := statusField(t, esc.Arr[i], "key")
		streak, _ := statusField(t, esc.Arr[i], "streak")
		roles, _ := statusField(t, esc.Arr[i], "roles")
		if key.Str != w.key || streak.Str != w.streak || len(roles.Arr) != len(w.roles) {
			t.Fatalf("escalation[%d] = %s %s %d roles", i, key.Str, streak.Str, len(roles.Arr))
		}
		for j, r := range w.roles {
			name, _ := statusField(t, roles.Arr[j], "role")
			tier, _ := statusField(t, roles.Arr[j], "tier")
			model, _ := statusField(t, roles.Arr[j], "model")
			if got := [3]string{name.Str, tier.Str, model.Str}; got != r {
				t.Fatalf("escalation[%d].roles[%d] = %v, want %v", i, j, got, r)
			}
		}
	}
	c.Roles = c.Roles[2:]
	if _, ok := statusField(t, dispatchStatusValue(c, t.TempDir(), l, nil, now), "escalation"); ok {
		t.Fatal("ladder-free configuration shows escalation")
	}
}
