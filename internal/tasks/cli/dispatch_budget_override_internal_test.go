package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ATR-V0-088: with ticketBudget overrides, dispatch status shows each
// ticket's effective limit, the selector that set it, whether the store
// supplied its labels and kind, and the hold reason. Without overrides the
// view carries no selector and reads no store.
func TestATRV0088_DispatchStatusShowsEffectiveLimitAndSelector(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	c := &dispatch.Config{Roles: []dispatch.Role{{Name: "impl"}}, TicketBudget: &dispatch.Budget{SessionsPerDay: 3}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Budget: &dispatch.SpendRecord{
			Sessions: []dispatch.SpendSession{
				{Worker: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Launched: now.Add(-time.Hour), Usage: dispatch.UsageUnknown},
				{Worker: "w2", Role: "impl", Ticket: "ticket:a:q:t2", Launched: now.Add(-time.Hour), Usage: dispatch.UsageUnknown},
			},
			Held: []dispatch.BudgetHold{{Scope: dispatch.ScopeTicket, Name: "ticket:a:q:t1", Limit: dispatch.LimitSessions, ResetsAt: now.Add(23 * time.Hour)}},
		}}
	tickets := []dispatch.Ticket{{ID: "ticket:a:q:t1", Local: "t1", Kind: "TASK", Labels: []string{"cheap"}}}
	scopes := func() []map[string]string {
		budget, _ := statusField(t, dispatchStatusValueFor(c, t.TempDir(), l, nil, now, tickets), "budget")
		arr, _ := statusField(t, budget, "scopes")
		var out []map[string]string
		for _, s := range arr.Arr {
			m := map[string]string{}
			for _, k := range []string{"scope", "name", "selector", "sessionsPerDay", "held"} {
				if v, ok := statusField(t, s, k); ok {
					m[k] = v.Str
				}
			}
			if v, ok := statusField(t, s, "selectorObserved"); ok {
				m["selectorObserved"] = map[bool]string{true: "true", false: "false"}[v.Bool]
			}
			out = append(out, m)
		}
		return out
	}
	for _, s := range scopes() {
		if _, ok := s["selector"]; ok {
			t.Fatalf("a selector without overrides: %v", s)
		}
	}
	if got := dispatchBudgetTickets(Env{Cwd: t.TempDir()}, c); got != nil {
		t.Fatalf("tickets read without overrides: %+v", got)
	}

	c.TicketBudget.Overrides = []dispatch.BudgetOverride{{Label: "cheap", SessionsPerDay: 1}}
	got := scopes()
	if len(got) != 3 {
		t.Fatalf("scopes %v", got)
	}
	want := []map[string]string{
		{"scope": "role", "name": "impl", "sessionsPerDay": "NONE", "held": "NONE"},
		{"scope": "ticket", "name": "ticket:a:q:t1", "selector": "label:cheap", "selectorObserved": "true", "sessionsPerDay": "1", "held": dispatch.LimitSessions},
		{"scope": "ticket", "name": "ticket:a:q:t2", "selector": dispatch.BudgetSelectorDefault, "selectorObserved": "false", "sessionsPerDay": "3", "held": "NONE"},
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Errorf("scope %d = %v, want %v", i, got[i], want[i])
		}
		for k, v := range want[i] {
			if got[i][k] != v {
				t.Errorf("scope %d %s = %q, want %q", i, k, got[i][k], v)
			}
		}
	}
	// A store that cannot be read leaves every ticket unobserved.
	if got := dispatchBudgetTickets(Env{Cwd: t.TempDir()}, c); got != nil {
		t.Fatalf("tickets from no store: %+v", got)
	}
}
