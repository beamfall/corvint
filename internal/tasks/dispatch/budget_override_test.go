//go:build darwin || linux

package dispatch

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// overrideConfig is a valid configuration whose ticketBudget is raw JSON.
func overrideConfig(t *testing.T, ticketBudget string) []byte {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat = "claude-code"
	c.TicketBudget = &Budget{SessionsPerDay: 9}
	raw, _ := json.Marshal(c)
	out := bytes.Replace(raw, []byte(`"ticketBudget":{"sessionsPerDay":9}`), []byte(`"ticketBudget":`+ticketBudget), 1)
	if bytes.Equal(out, raw) {
		t.Fatal("fixture did not place the ticketBudget")
	}
	return out
}

func TestATRV0085_TicketBudgetOverridesDecode(t *testing.T) {
	c, err := DecodeConfig(overrideConfig(t, `{"sessionsPerDay":5,"overrides":[{"ticket":"V1-1","sessionsPerDay":20},{"label":"cheap","tokensPerDay":100},{"kind":"BUG","sessionsPerDay":2,"tokensPerDay":50}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []BudgetOverride{{Ticket: "V1-1", SessionsPerDay: 20}, {Label: "cheap", TokensPerDay: 100}, {Kind: "BUG", SessionsPerDay: 2, TokensPerDay: 50}}
	if got := c.TicketBudget.Overrides; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("overrides %+v", got)
	}
	// Without overrides the configuration and its encoding are unchanged.
	plain, err := DecodeConfig(overrideConfig(t, `{"sessionsPerDay":5}`))
	if err != nil || plain.TicketBudget.Overrides != nil {
		t.Fatalf("plain ticketBudget %+v %v", plain.TicketBudget, err)
	}
	if raw, _ := json.Marshal(plain.TicketBudget); string(raw) != `{"sessionsPerDay":5}` {
		t.Fatalf("plain ticketBudget encodes %s", raw)
	}
}

func TestATRV0086_MalformedOrAmbiguousOverridesRefuse(t *testing.T) {
	malformed := []string{
		`{"sessionsPerDay":5,"overrides":{}}`,
		`{"sessionsPerDay":5,"overrides":[1]}`,
		`{"sessionsPerDay":5,"overrides":[{"sessionsPerDay":1}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","label":"b","sessionsPerDay":1}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a"}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"","sessionsPerDay":1}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","sessionsPerDay":0}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","sessionsPerDay":1001}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","tokensPerDay":null}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","sessionsPerDay":1,"sessionsPerDay":2}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","sessionsPerDay":1,"priority":"P0"}]}`,
		`{"sessionsPerDay":5,"overrides":[{"Ticket":"a","sessionsPerDay":1}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":7,"sessionsPerDay":1}]}`,
		`{"sessionsPerDay":5,"overrides":[` + strings.TrimSuffix(strings.Repeat(`{"ticket":"a","sessionsPerDay":1},`, 257), ",") + `]}`,
	}
	for _, tb := range malformed {
		if _, err := DecodeConfig(overrideConfig(t, tb)); err == nil || !strings.Contains(err.Error(), BudgetOverrideMalformed) {
			t.Errorf("%.90s: %v", tb, err)
		}
	}
	for _, tb := range []string{
		`{"sessionsPerDay":5,"overrides":[{"label":"x","sessionsPerDay":1},{"label":"x","tokensPerDay":3}]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"V1-1","sessionsPerDay":1},{"kind":"BUG","sessionsPerDay":1},{"ticket":"V1-1","sessionsPerDay":2}]}`,
	} {
		if _, err := DecodeConfig(overrideConfig(t, tb)); err == nil || !strings.Contains(err.Error(), BudgetOverrideAmbiguous) {
			t.Errorf("%s: %v", tb, err)
		}
	}
	// A ticketBudget with neither a default limit nor an override refuses
	// (ATR-V0-085), and a role budget takes no overrides.
	for _, tb := range []string{`{}`, `{"overrides":[]}`} {
		if _, err := DecodeConfig(overrideConfig(t, tb)); err == nil {
			t.Errorf("%s: a ticketBudget with no limit was accepted", tb)
		}
	}
	c := testConfig(t, "exit 0")
	c.Roles[0].Budget = &Budget{SessionsPerDay: 3, Overrides: []BudgetOverride{{Ticket: "a", SessionsPerDay: 1}}}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), BudgetOverrideMalformed) {
		t.Errorf("role budget overrides: %v", err)
	}
	// An override's tokensPerDay needs observed usage like the default's.
	c = testConfig(t, "exit 0")
	c.TicketBudget = &Budget{SessionsPerDay: 3, Overrides: []BudgetOverride{{Label: "x", TokensPerDay: 10}}}
	raw, _ = json.Marshal(c)
	if _, err := DecodeConfig(raw); err == nil || !strings.Contains(err.Error(), "usageFormat") {
		t.Errorf("override tokens without a usage format: %v", err)
	}
}

func TestATRV0087_EffectiveLimitPrecedence(t *testing.T) {
	c := &Config{TicketBudget: &Budget{SessionsPerDay: 5, TokensPerDay: 1000, Overrides: []BudgetOverride{
		{Kind: "BUG", SessionsPerDay: 2},
		{Label: "slow", SessionsPerDay: 1},
		{Label: "cheap", TokensPerDay: 10},
		{Ticket: "V1-7", SessionsPerDay: 30},
		{Ticket: "ticket:a:q:raw", TokensPerDay: 7},
	}}}
	cases := []struct {
		name string
		t    Ticket
		want Budget
		sel  string
	}{
		{"default", Ticket{ID: "ticket:a:q:x", Local: "x", Kind: "TASK"}, Budget{SessionsPerDay: 5, TokensPerDay: 1000}, BudgetSelectorDefault},
		{"kind", Ticket{ID: "ticket:a:q:x", Kind: "BUG"}, Budget{SessionsPerDay: 2, TokensPerDay: 1000}, "kind:BUG"},
		{"label over kind", Ticket{ID: "ticket:a:q:x", Kind: "BUG", Labels: []string{"cheap"}}, Budget{SessionsPerDay: 5, TokensPerDay: 10}, "label:cheap"},
		{"label tie: first in configuration order", Ticket{ID: "ticket:a:q:x", Labels: []string{"cheap", "slow"}}, Budget{SessionsPerDay: 1, TokensPerDay: 1000}, "label:slow"},
		{"ticket by local key over label and kind", Ticket{ID: "ticket:a:q:V1-7", Local: "V1-7", Kind: "BUG", Labels: []string{"slow"}}, Budget{SessionsPerDay: 30, TokensPerDay: 1000}, "ticket:V1-7"},
		{"ticket by ID", Ticket{ID: "ticket:a:q:raw", Local: "raw"}, Budget{SessionsPerDay: 5, TokensPerDay: 7}, "ticket:ticket:a:q:raw"},
	}
	for _, tc := range cases {
		b, sel := c.TicketLimit(tc.t)
		if b == nil || b.SessionsPerDay != tc.want.SessionsPerDay || b.TokensPerDay != tc.want.TokensPerDay || b.Overrides != nil || sel != tc.sel {
			t.Errorf("%s: %+v %q, want %+v %q", tc.name, b, sel, tc.want, tc.sel)
		}
	}
	if b, sel := (&Config{}).TicketLimit(Ticket{ID: "x"}); b != nil || sel != "" {
		t.Errorf("no ticketBudget resolved %+v %q", b, sel)
	}
	// A ticket key the observation did not supply resolves by key alone.
	if b, sel := ticketLimit(c, nil, "ticket:a:q:raw"); b.TokensPerDay != 7 || sel != "ticket:ticket:a:q:raw" {
		t.Errorf("unobserved ticket override %+v %q", b, sel)
	}
	if _, sel := ticketLimit(c, nil, "ticket:a:q:bug"); sel != BudgetSelectorDefault {
		t.Errorf("unobserved ticket guessed %q", sel)
	}
}

// overrideDispatcher has t1 (label cheap, override 1 session) and t2 (the
// default 2 sessions) each with one session already in the window.
func overrideDispatcher(t *testing.T, overrides []BudgetOverride) (*Dispatcher, time.Time, []Ticket) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.TicketBudget = &Budget{SessionsPerDay: 2, Overrides: overrides}
	ts := []Ticket{ticket("t1", "P1", 1, "cheap"), ticket("t2", "P1", 2)}
	d, err := Open("prog", c, &fakeQueue{obs: Observation{Tickets: ts}}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return clock }
	d.ledger.Budget = &SpendRecord{Sessions: []SpendSession{
		{Worker: "w1", Role: "impl", Ticket: ts[0].ID, Launched: clock.Add(-time.Hour), Usage: UsageUnknown},
		{Worker: "w2", Role: "impl", Ticket: ts[1].ID, Launched: clock.Add(-2 * time.Hour), Usage: UsageUnknown},
	}}
	return d, clock, ts
}

func TestATRV0088_HoldsAndReportNameTheSelector(t *testing.T) {
	d, clock, ts := overrideDispatcher(t, []BudgetOverride{{Label: "cheap", SessionsPerDay: 1}})
	tickOK(t, d)
	launched := eventsOf(t, d, "launched")
	if len(launched) != 1 || launched[0].Ticket != "ticket:a:q:t2" {
		t.Fatalf("launches %+v", launched)
	}
	held := eventsOf(t, d, "budget")
	if len(held) != 1 {
		t.Fatalf("budget events %+v", held)
	}
	want := map[string]string{"scope": ScopeTicket, "name": "ticket:a:q:t1", "limit": LimitSessions, "selector": "label:cheap", "sessionsPerDay": "1", "tokensPerDay": "0", "resetsAt": "2026-10-08T11:00:00Z"}
	for k, v := range want {
		if held[0].Detail[k] != v {
			t.Fatalf("budget detail %s = %q, want %q (%+v)", k, held[0].Detail[k], v, held[0].Detail)
		}
	}
	if !strings.Contains(held[0].Message, "label:cheap") {
		t.Fatalf("budget message %q", held[0].Message)
	}
	waitEnded(t, d)
	report := SpendReportFor(d.Config, d.ledger, clock, ts)
	if len(report) != 3 {
		t.Fatalf("spend report %+v", report)
	}
	t1, t2 := report[1], report[2]
	if t1.Name != "ticket:a:q:t1" || t1.Selector != "label:cheap" || t1.Budget.SessionsPerDay != 1 || t1.Hold == nil || t1.Hold.Limit != LimitSessions {
		t.Fatalf("t1 report %+v", t1)
	}
	if t2.Selector != BudgetSelectorDefault || t2.Budget.SessionsPerDay != 2 || t2.Sessions != 2 {
		t.Fatalf("t2 report %+v", t2)
	}
}

func TestATRV0089_RollingWindowPerEffectiveLimit(t *testing.T) {
	// Without overrides both tickets share the default and the event has
	// no selector: behaviour and output are unchanged.
	d, clock, _ := overrideDispatcher(t, nil)
	tickOK(t, d)
	if n := len(eventsOf(t, d, "launched")); n != 2 {
		t.Fatalf("%d launches under the default budget", n)
	}
	waitEnded(t, d)
	if r := SpendReport(d.Config, d.ledger, clock); r[1].Selector != BudgetSelectorDefault || r[1].Budget.SessionsPerDay != 2 {
		t.Fatalf("default report %+v", r[1])
	}

	// With the override, t1's own window releases it 24 hours after its
	// one session; t2 is judged against the default.
	d, clock, ts := overrideDispatcher(t, []BudgetOverride{{Label: "cheap", SessionsPerDay: 1}})
	g := d.spendGate(clock)
	g.observe(ts)
	h, held := g.exhausted([2]string{ScopeTicket, "ticket:a:q:t1"})
	if !held || h.Limit != LimitSessions || !h.ResetsAt.Equal(clock.Add(BudgetWindow-time.Hour)) {
		t.Fatalf("override hold %+v %v", h, held)
	}
	if _, held := g.exhausted([2]string{ScopeTicket, "ticket:a:q:t2"}); held {
		t.Fatal("the default ticket was held")
	}
	g = d.spendGate(clock.Add(BudgetWindow - time.Hour + time.Second))
	g.observe(ts)
	if _, held := g.exhausted([2]string{ScopeTicket, "ticket:a:q:t1"}); held {
		t.Fatal("the window did not release the override hold")
	}
	// Token limits follow the same rule per effective limit.
	d.Config.Roles[0].UsageFormat = "claude-code"
	d.Config.TicketBudget.Overrides = []BudgetOverride{{Label: "cheap", TokensPerDay: 10}}
	d.ledger.Budget.Sessions[0].Usage, d.ledger.Budget.Sessions[0].Input = UsageKnown, 10
	g = d.spendGate(clock)
	g.observe(ts)
	if h, held := g.exhausted([2]string{ScopeTicket, "ticket:a:q:t1"}); !held || h.Limit != LimitTokens {
		t.Fatalf("override token hold %+v %v", h, held)
	}
}

// ATR-V0-086: a JSON null `overrides`, or a null entry in it, is not an
// array of override objects and refuses; it never falls back to the
// default limits as if no overrides were configured.
func TestATRV0086_NullOverridesRefuse(t *testing.T) {
	for _, tb := range []string{
		`{"sessionsPerDay":5,"overrides":null}`,
		`{"sessionsPerDay":5,"overrides": null }`,
		`{"sessionsPerDay":5,"overrides":[null]}`,
		`{"sessionsPerDay":5,"overrides":[{"ticket":"a","sessionsPerDay":1},null]}`,
	} {
		if _, err := DecodeConfig(overrideConfig(t, tb)); err == nil || !strings.Contains(err.Error(), BudgetOverrideMalformed) {
			t.Errorf("%s: %v", tb, err)
		}
	}
	// An absent `overrides` and an empty array remain the default alone.
	for _, tb := range []string{`{"sessionsPerDay":5}`, `{"sessionsPerDay":5,"overrides":[]}`} {
		if _, err := DecodeConfig(overrideConfig(t, tb)); err != nil {
			t.Errorf("%s: %v", tb, err)
		}
	}
}
