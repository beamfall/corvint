package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ATR-V0-085 (V1-1835, issue 1149): with a ticketBudget of overrides alone,
// dispatch status lists only the tickets an override selects as ticket
// scopes, never an unmatched ticket under an invented default.
func TestATRV0085_DispatchStatusListsOnlyOverrideScopes(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	c := &dispatch.Config{Roles: []dispatch.Role{{Name: "impl"}}, TicketBudget: &dispatch.Budget{Overrides: []dispatch.BudgetOverride{{Label: "cheap", SessionsPerDay: 1}}}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Budget: &dispatch.SpendRecord{Sessions: []dispatch.SpendSession{
			{Worker: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Launched: now.Add(-time.Hour), Usage: dispatch.UsageUnknown},
			{Worker: "w2", Role: "impl", Ticket: "ticket:a:q:t2", Launched: now.Add(-time.Hour), Usage: dispatch.UsageUnknown},
		}}}
	tickets := []dispatch.Ticket{{ID: "ticket:a:q:t1", Local: "t1", Kind: "TASK", Labels: []string{"cheap"}}, {ID: "ticket:a:q:t2", Local: "t2", Kind: "TASK"}}
	budget, _ := statusField(t, dispatchStatusValueFor(c, t.TempDir(), l, nil, now, tickets), "budget")
	arr, _ := statusField(t, budget, "scopes")
	var names []string
	for _, s := range arr.Arr {
		scope, _ := statusField(t, s, "scope")
		name, _ := statusField(t, s, "name")
		names = append(names, scope.Str+"/"+name.Str)
		if scope.Str == dispatch.ScopeTicket {
			if sel, _ := statusField(t, s, "selector"); sel.Str != "label:cheap" {
				t.Errorf("ticket scope %s selector %q", name.Str, sel.Str)
			}
			if lim, _ := statusField(t, s, "sessionsPerDay"); lim.Str != "1" {
				t.Errorf("ticket scope %s sessionsPerDay %q", name.Str, lim.Str)
			}
		}
	}
	if len(names) != 2 || names[0] != "role/impl" || names[1] != "ticket/ticket:a:q:t1" {
		t.Fatalf("scopes %v, want the role and the override ticket only", names)
	}
}
