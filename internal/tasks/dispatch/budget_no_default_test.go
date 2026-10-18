//go:build darwin || linux

package dispatch

import (
	"fmt"
	"testing"
	"time"
)

// ATR-V0-085 (V1-1835, issue 1149): a ticketBudget of overrides alone
// validates, and a ticket no override selects has no ticket limit.
func TestATRV0085_OverridesWithoutDefaultValidate(t *testing.T) {
	c, err := DecodeConfig(overrideConfig(t, `{"overrides":[{"label":"cheap","sessionsPerDay":1},{"ticket":"t9","tokensPerDay":40}]}`))
	if err != nil {
		t.Fatalf("overrides alone refused: %v", err)
	}
	if b, sel := c.TicketLimit(ticket("t2", "P1", 2)); b != nil || sel != "" {
		t.Fatalf("unmatched ticket resolved %+v %q", b, sel)
	}
	if b, sel := c.TicketLimit(ticket("t1", "P1", 1, "cheap")); b == nil || b.SessionsPerDay != 1 || b.TokensPerDay != 0 || sel != "label:cheap" {
		t.Fatalf("label override resolved %+v %q", b, sel)
	}
	if b, sel := ticketLimit(c, nil, "t9"); b == nil || b.TokensPerDay != 40 || b.SessionsPerDay != 0 || sel != "ticket:t9" {
		t.Fatalf("ticket override resolved %+v %q", b, sel)
	}
}

// ATR-V0-085: with no default limit, a ticket matching no override is never
// held by the ticket budget however much it spent, while an overridden
// ticket is held by its own limit; the spend report lists only override
// scopes.
func TestATRV0085_UnmatchedTicketsNeverHeld(t *testing.T) {
	d, clock, ts := overrideDispatcher(t, []BudgetOverride{{Label: "cheap", SessionsPerDay: 1}})
	d.Config.TicketBudget.SessionsPerDay = 0
	for i := range 40 {
		d.ledger.Budget.Sessions = append(d.ledger.Budget.Sessions, SpendSession{Worker: fmt.Sprintf("x%d", i), Role: "impl", Ticket: ts[1].ID, Launched: clock.Add(-3 * time.Hour), Usage: UsageUnknown})
	}
	if held := d.budgetHeld(ts, clock); !held[ts[0].ID] || held[ts[1].ID] {
		t.Fatalf("budget-held tickets %v", held)
	}
	tickOK(t, d)
	launched := eventsOf(t, d, "launched")
	if len(launched) != 1 || launched[0].Ticket != ts[1].ID {
		t.Fatalf("launches %+v", launched)
	}
	held := eventsOf(t, d, "budget")
	if len(held) != 1 || held[0].Ticket != ts[0].ID || held[0].Detail["selector"] != "label:cheap" {
		t.Fatalf("budget events %+v", held)
	}
	waitEnded(t, d)
	report := SpendReportFor(d.Config, d.ledger, clock, ts)
	if len(report) != 2 || report[0].Scope != ScopeRole || report[1].Name != ts[0].ID || report[1].Selector != "label:cheap" {
		t.Fatalf("spend report %+v", report)
	}
}
