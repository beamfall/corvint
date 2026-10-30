package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func statusStrings(t *testing.T, v wire.Value, want map[string]string) {
	t.Helper()
	for k, s := range want {
		if got, ok := statusField(t, v, k); !ok || got.Str != s {
			t.Errorf("%s = %+v, want %q", k, got, s)
		}
	}
}

// CAL-V0-158 and CAL-V0-159: dispatch status shows each worker's declared
// effort and running token account, UNKNOWN and never 0 when unobserved, and
// each scope's spend in the window with its limits, hold and reset time.
func TestCALV0158_DispatchStatusShowsBudgetsAndUsage(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	c := &dispatch.Config{Roles: []dispatch.Role{
		{Name: "impl", Effort: "high", UsageFormat: "codex", Budget: &dispatch.Budget{SessionsPerDay: 2, TokensPerDay: 100}},
		{Name: "plain"},
	}, TicketBudget: &dispatch.Budget{SessionsPerDay: 3}}
	resets := now.Add(20 * time.Hour)
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Workers: []*dispatch.Worker{
			{ID: "w1", Role: "impl", Key: "ticket:a:q:t1", Effort: "high", Usage: &dispatch.WorkerUsage{Format: "codex", Records: 1, Input: 7, Output: 2}},
			{ID: "w2", Role: "plain", Key: "ticket:a:q:t2"},
		},
		Budget: &dispatch.SpendRecord{
			Sessions: []dispatch.SpendSession{
				{Worker: "old", Role: "impl", Ticket: "ticket:a:q:t1", Launched: now.Add(-25 * time.Hour), Usage: dispatch.UsageKnown, Input: 500},
				{Worker: "w0", Role: "impl", Ticket: "ticket:a:q:t1", Launched: now.Add(-4 * time.Hour), Usage: dispatch.UsageKnown, Input: 90, Output: 10},
				{Worker: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Launched: now.Add(-time.Hour), Usage: dispatch.UsageRunning, Input: 7, Output: 2},
				{Worker: "w2", Role: "plain", Ticket: "ticket:a:q:t2", Launched: now.Add(-time.Minute), Usage: dispatch.UsageUnknown},
			},
			HistoryFrom: now.Add(-30 * time.Hour),
			Held:        []dispatch.BudgetHold{{Scope: dispatch.ScopeRole, Name: "impl", Limit: dispatch.LimitTokens, ResetsAt: resets}},
		}}
	v := dispatchStatusValue(c, t.TempDir(), l, nil, now)
	workers, _ := statusField(t, v, "workers")
	statusStrings(t, workers.Arr[0], map[string]string{"effort": "high"})
	u0, _ := statusField(t, workers.Arr[0], "usage")
	statusStrings(t, u0, map[string]string{"format": "codex", "state": dispatch.UsageRunning, "inputTokens": "7", "outputTokens": "2", "tokens": "9"})
	if _, ok := statusField(t, workers.Arr[1], "effort"); ok {
		t.Error("an undeclared effort is shown")
	}
	u1, _ := statusField(t, workers.Arr[1], "usage")
	statusStrings(t, u1, map[string]string{"format": "NONE", "state": dispatch.UsageUnknown, "tokens": dispatch.UsageUnknown})

	budget, ok := statusField(t, v, "budget")
	if !ok {
		t.Fatal("no budget in status")
	}
	statusStrings(t, budget, map[string]string{"windowSeconds": "86400", "historyTruncatedThrough": "NONE", "historyFrom": "NONE"})
	scopes, _ := statusField(t, budget, "scopes")
	if len(scopes.Arr) != 4 {
		t.Fatalf("scopes %+v", scopes)
	}
	statusStrings(t, scopes.Arr[0], map[string]string{"scope": "role", "name": "impl", "sessions": "2", "observedTokens": "109", "runningSessions": "1", "unknownSessions": "0", "sessionsPerDay": "2", "tokensPerDay": "100", "held": dispatch.LimitTokens, "resetsAt": "2026-10-08T08:00:00Z"})
	statusStrings(t, scopes.Arr[1], map[string]string{"scope": "role", "name": "plain", "sessions": "1", "observedTokens": "0", "unknownSessions": "1", "sessionsPerDay": "NONE", "tokensPerDay": "NONE", "held": "NONE", "resetsAt": "NONE"})
	statusStrings(t, scopes.Arr[2], map[string]string{"scope": "ticket", "name": "ticket:a:q:t1", "sessions": "2", "sessionsPerDay": "3"})
	statusStrings(t, scopes.Arr[3], map[string]string{"scope": "ticket", "name": "ticket:a:q:t2", "unknownSessions": "1"})
}
