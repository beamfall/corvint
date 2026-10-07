//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func tickOK(t *testing.T, d *Dispatcher) {
	t.Helper()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCALV0155_SessionBudgetHoldsUntilTheWindowReleases(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Budget = &Budget{SessionsPerDay: 1}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return clock }

	tickOK(t, d)
	if n := len(eventsOf(t, d, "launched")); n != 1 {
		t.Fatalf("%d launches under a one-session budget", n)
	}
	held := eventsOf(t, d, "budget")
	if len(held) != 1 {
		t.Fatalf("budget events %+v", held)
	}
	want := map[string]string{"scope": ScopeRole, "name": "impl", "limit": LimitSessions, "resetsAt": "2026-10-08T12:00:00Z", "sessions": "1", "observedTokens": "0", "unknownSessions": "0"}
	for k, v := range want {
		if held[0].Detail[k] != v {
			t.Fatalf("budget detail %s = %q, want %q (%+v)", k, held[0].Detail[k], v, held[0].Detail)
		}
	}
	if held[0].Role != "impl" {
		t.Fatalf("budget event role %q", held[0].Role)
	}

	waitEnded(t, d)
	clock = clock.Add(time.Hour)
	tickOK(t, d)
	tickOK(t, d)
	if n := len(eventsOf(t, d, "budget")); n != 1 {
		t.Fatalf("a standing hold was reported %d times", n)
	}
	fin := eventsOf(t, d, "finished")
	if len(fin) != 1 || fin[0].Detail["usage"] != UsageUnknown || fin[0].Detail["tokens"] != UsageUnknown || fin[0].Detail["inputTokens"] != UsageUnknown {
		t.Fatalf("a worker without a usage format reported %+v", fin)
	}
	report := SpendReport(d.Config, d.ledger, clock)
	if len(report) != 1 || report[0].Sessions != 1 || report[0].Unknown != 1 || report[0].Hold == nil || !report[0].Hold.ResetsAt.Equal(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("spend report %+v", report)
	}

	clock = time.Date(2026, 10, 8, 12, 0, 1, 0, time.UTC)
	tickOK(t, d)
	if n := len(eventsOf(t, d, "launched")); n != 2 {
		t.Fatalf("the window did not release the hold: %d launches", n)
	}
	if s := d.ledger.Budget.Sessions; len(s) != 1 || !s[0].Launched.Equal(clock) {
		t.Fatalf("sessions outside the window were kept: %+v", s)
	}
	waitEnded(t, d)
}

func TestCALV0155_TokenBudgetCountsKnownTotals(t *testing.T) {
	c := testConfig(t, `echo "$2" > "$CORVINT_DISPATCH_WORKER.effort"; echo '{"type":"turn.completed","usage":{"input_tokens":10,"output_tokens":3}}'`)
	c.Hosts["sh"] = Host{Argv: append(c.Hosts["sh"].Argv, "{effort}")}
	c.Roles[0].Cap, c.Roles[0].Effort, c.Roles[0].UsageFormat = 1, "high", "codex"
	c.Roles[0].Budget = &Budget{TokensPerDay: 20}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return clock }
	for i := 0; i < 3; i++ {
		tickOK(t, d)
		waitEnded(t, d)
		clock = clock.Add(time.Minute)
	}
	tickOK(t, d)
	fin := eventsOf(t, d, "finished")
	if len(fin) != 2 {
		t.Fatalf("finished %+v", fin)
	}
	for k, v := range map[string]string{"usage": UsageKnown, "inputTokens": "10", "outputTokens": "3", "tokens": "13", "effort": "high"} {
		if fin[0].Detail[k] != v {
			t.Fatalf("finished detail %s = %q, want %q", k, fin[0].Detail[k], v)
		}
	}
	launched := eventsOf(t, d, "launched")
	if len(launched) != 2 || launched[0].Detail["effort"] != "high" {
		t.Fatalf("launched %+v", launched)
	}
	raw, err := os.ReadFile(filepath.Join(c.WorkRoot, launched[0].Worker+".effort"))
	if err != nil || strings.TrimSpace(string(raw)) != "high" {
		t.Fatalf("{effort} rendered %q: %v", raw, err)
	}
	held := eventsOf(t, d, "budget")
	if len(held) != 1 || held[0].Detail["limit"] != LimitTokens || held[0].Detail["observedTokens"] != "26" || held[0].Detail["resetsAt"] != "2026-10-08T12:00:00Z" {
		t.Fatalf("token hold %+v", held)
	}
}

func TestCALV0155_BudgetFollowsConfigReload(t *testing.T) {
	c := testConfig(t, "exit 0")
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2), ticket("t3", "P1", 3)}}}
	c.Roles[0].Cap = 1
	f := newReloadFixture(t, c, q)
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	f.d.Now = func() time.Time { return clock }
	f.tick()
	waitEnded(t, f.d)

	// A budget added by reload counts the launch made before it.
	budgeted := *c
	budgeted.Roles = []Role{c.Roles[0]}
	budgeted.Roles[0].Budget = &Budget{SessionsPerDay: 1}
	budgeted.TicketBudget = &Budget{SessionsPerDay: 5}
	f.write(&budgeted)
	clock = clock.Add(time.Minute)
	f.tick()
	if n := len(f.events("launched")); n != 1 {
		t.Fatalf("a reloaded budget did not count earlier launches: %d launches", n)
	}
	if h := f.events("budget"); len(h) != 1 || h[0].Detail["limit"] != LimitSessions {
		t.Fatalf("budget events %+v", h)
	}
	report := SpendReport(f.d.Config, f.d.ledger, clock)
	if len(report) != 2 || report[1].Scope != ScopeTicket || report[1].Name != "ticket:a:q:t1" || report[1].Sessions != 1 {
		t.Fatalf("spend report %+v", report)
	}

	// Removing it releases the hold at once.
	f.write(c)
	clock = clock.Add(time.Minute)
	f.tick()
	if n := len(f.events("launched")); n != 2 {
		t.Fatalf("removing the budget did not release it: %d launches", n)
	}
	if len(f.d.ledger.Budget.Held) != 0 {
		t.Fatalf("holds survived their budget: %+v", f.d.ledger.Budget.Held)
	}
	waitEnded(t, f.d)
}

func TestCALV0156_SpendHistoryIsBoundedAndHoldsWhenTruncated(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Budget = &Budget{SessionsPerDay: 1000}
	q := &fakeQueue{}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	a := Assignment{Role: "other", Ticket: "ticket:a:q:x"}
	for i := 0; i <= maxSpendSessions; i++ {
		d.recordSession(a, "w", UsageUnknown, now.Add(time.Duration(i)*time.Second))
	}
	r := d.ledger.Budget
	if len(r.Sessions) != maxSpendSessions || !r.Truncated.Equal(now) {
		t.Fatalf("history %d truncated %v", len(r.Sessions), r.Truncated)
	}
	g := d.spendGate(now.Add(time.Hour))
	if g.admits(Assignment{Role: "impl"}) {
		t.Fatal("a truncated history admitted a budgeted launch")
	}
	if h := g.holds[[2]string{ScopeRole, "impl"}]; h.Limit != LimitTruncated || !h.ResetsAt.Equal(now.Add(BudgetWindow)) {
		t.Fatalf("truncation hold %+v", h)
	}
	if !g.admits(Assignment{Role: "other"}) {
		t.Fatal("an unbudgeted role was held")
	}
}

func TestCALV0160_BudgetConfigIsClosed(t *testing.T) {
	ok := map[string]func(*Config){
		"role sessions": func(c *Config) { c.Roles[0].Budget = &Budget{SessionsPerDay: 3} },
		"role tokens":   func(c *Config) { c.Roles[0].UsageFormat, c.Roles[0].Budget = "codex", &Budget{TokensPerDay: 1000} },
		"ticket budget": func(c *Config) { c.TicketBudget = &Budget{SessionsPerDay: 2} },
		"ticket tokens": func(c *Config) { c.Roles[0].UsageFormat, c.TicketBudget = "claude-code", &Budget{TokensPerDay: 10} },
		"usage format":  func(c *Config) { c.Roles[0].UsageFormat = "opencode" },
		"rendered effort": func(c *Config) {
			c.Hosts["sh"] = Host{Argv: append(c.Hosts["sh"].Argv, "--effort={effort}")}
			c.Roles[0].Effort = "xhigh"
		},
	}
	bad := map[string]func(*Config){
		"empty budget":              func(c *Config) { c.Roles[0].Budget = &Budget{} },
		"sessions too high":         func(c *Config) { c.Roles[0].Budget = &Budget{SessionsPerDay: 1001} },
		"tokens without a format":   func(c *Config) { c.Roles[0].Budget = &Budget{TokensPerDay: 10} },
		"ticket tokens unobserved":  func(c *Config) { c.TicketBudget = &Budget{TokensPerDay: 10} },
		"unknown usage format":      func(c *Config) { c.Roles[0].UsageFormat = "gemini" },
		"effort not rendered":       func(c *Config) { c.Roles[0].Effort = "high" },
		"effort outside its format": func(c *Config) { c.Roles[0].Effort = "high effort" },
		"tokens too high": func(c *Config) {
			c.Roles[0].UsageFormat, c.Roles[0].Budget = "codex", &Budget{TokensPerDay: 1<<50 + 1}
		},
	}
	for name, f := range ok {
		c := testConfig(t, "exit 0")
		f(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err != nil {
			t.Errorf("%s: refused: %v", name, err)
		}
	}
	for name, f := range bad {
		c := testConfig(t, "exit 0")
		f(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A present limit of 0 or null is refused, not read as absent, and the
	// budget object stays closed.
	c := testConfig(t, "exit 0")
	c.Roles[0].UsageFormat, c.Roles[0].Budget = "codex", &Budget{SessionsPerDay: 3}
	raw, _ := json.Marshal(c)
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{`"tokensPerDay":0`, `"tokensPerDay":null`, `"tokensPerDay":-1`, `"sessionsPerDayX":1`, `"sessionsPerDay":3`, `"SessionsPerDay":1000`} {
		if _, err := DecodeConfig(bytes.Replace(raw, []byte(`"sessionsPerDay":3`), []byte(`"sessionsPerDay":3,`+member), 1)); err == nil {
			t.Errorf("budget with %s accepted", member)
		}
	}
	for _, member := range []string{`"sessionsPerDay":0`, `"sessionsPerDay":null`} {
		if _, err := DecodeConfig(bytes.Replace(raw, []byte(`"sessionsPerDay":3`), []byte(member+`,"tokensPerDay":5`), 1)); err == nil {
			t.Errorf("budget with %s accepted", member)
		}
	}
}

// CAL-V0-161: the /2 ledger round-trips the spend history, a worker's usage
// account and its declared effort, and refuses a malformed or unknown
// member of either without touching the worker.
func TestCALV0161_LedgerCarriesSpendAndUsage(t *testing.T) {
	c := testConfig(t, `echo '{"type":"turn.completed","usage":{"input_tokens":4,"output_tokens":1}}'; sleep 300`)
	c.Hosts["sh"] = Host{Argv: append(c.Hosts["sh"].Argv, "{effort}")}
	c.GlobalCap, c.Roles[0].Effort, c.Roles[0].UsageFormat = 1, "low", "codex"
	c.Roles[0].Budget = &Budget{SessionsPerDay: 1}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	tickOK(t, d)
	pid := d.ledger.Workers[0].PID
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	for i := 0; i < 100 && d.ledger.Workers[0].Usage.Records == 0; i++ {
		time.Sleep(20 * time.Millisecond)
		tickOK(t, d)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	dir := ProgramDir(c, "prog")
	path := filepath.Join(dir, "state.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(dir, "prog")
	if err != nil {
		t.Fatal(err)
	}
	w := l.Workers[0]
	if w.Effort != "low" || w.Usage == nil || w.Usage.Format != "codex" || w.Usage.Records != 1 || w.Usage.Input != 4 || w.Usage.Offset == 0 {
		t.Fatalf("worker round-trip %+v usage %+v", w, w.Usage)
	}
	if b := l.Budget; b == nil || len(b.Sessions) != 1 || b.Sessions[0].Worker != w.ID || b.Sessions[0].Usage != UsageRunning || b.Sessions[0].Input != 4 {
		t.Fatalf("spend round-trip %+v", l.Budget)
	}
	for name, edit := range map[string][2]string{
		"an unknown usage state":      {`"usage": "RUNNING"`, `"usage": "ZERO"`},
		"an unknown usage format":     {`"format": "codex"`, `"format": "gemini"`},
		"an unknown spend member":     {`"sessions": [`, `"spent": 1, "sessions": [`},
		"an aliased spend member":     {`"sessions": [`, `"SESSIONS": [], "sessions": [`},
		"a negative offset":           {`"offset": `, `"offset": -1, "x": `},
		"an unknown usage member":     {`"format": "codex"`, `"format": "codex", "tokens": 5`},
		"counters for unknown totals": {`"usage": "RUNNING"`, `"usage": "UNKNOWN"`},
	} {
		bad := bytes.Replace(good, []byte(edit[0]), []byte(edit[1]), 1)
		if bytes.Equal(bad, good) {
			t.Fatalf("%s: fixture not reached", name)
		}
		os.WriteFile(path, bad, 0o600)
		if _, err := LoadLedger(dir, "prog"); err == nil {
			t.Errorf("%s: loaded", name)
		}
		if syscall.Kill(pid, 0) != nil {
			t.Fatalf("%s: refusal touched the worker", name)
		}
	}
	os.WriteFile(path, good, 0o600)
}

// CAL-V0-155: with planSelected, a budget-held ticket leaves the selection
// window like a CAL-V0-105 work-state hold, so an unspent ticket behind it
// is selected and launches instead of starving until the hold ends.
func TestCALV0155_BudgetHeldTicketLeavesSelectionWindow(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{PlanSelected: true}
	c.TicketBudget = &Budget{SessionsPerDay: 1}
	ts := []Ticket{ticket("h1", "P0", 1), ticket("h2", "P0", 2), ticket("r1", "P1", 3)}
	plan := func(held map[string]bool) map[string]PlanView {
		out, used := map[string]PlanView{}, 0
		for _, t := range ts {
			switch {
			case held[t.ID]:
				out[t.ID] = PlanView{"DEFERRED", "BUDGET_HELD"}
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
	var got []map[string]bool
	q.obs.Replan = func(held, budgetHeld map[string]bool) map[string]PlanView {
		if len(held) != 0 {
			t.Errorf("work-state held %v", held)
		}
		got = append(got, budgetHeld)
		return plan(budgetHeld)
	}
	d, err := Open("budget-window", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	clock := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	d.Now = func() time.Time { return clock }
	d.ledger.Budget = &SpendRecord{Sessions: []SpendSession{
		{Worker: "w1", Role: c.Roles[0].Name, Ticket: ts[0].ID, Launched: clock.Add(-time.Hour), Usage: UsageKnown},
		{Worker: "w2", Role: c.Roles[0].Name, Ticket: ts[1].ID, Launched: clock.Add(-time.Hour), Usage: UsageUnknown},
	}}
	obs, err := d.observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0]) != 2 || !got[0][ts[0].ID] || !got[0][ts[1].ID] {
		t.Fatalf("budget-held replans %v", got)
	}
	if p := planOf(obs); p["r1"] != (PlanView{"SELECTED", "DEVELOPMENT_MODE"}) || p["h1"].Reason != "BUDGET_HELD" {
		t.Fatalf("plan %v", p)
	}
	if roster := Roster(c, obs, nil, nil); len(roster) != 1 || roster[0].Local != "r1" {
		t.Fatalf("roster %+v", roster)
	}
	// The roster never meets a deferred ticket, so the launch tick records
	// its hold from the deferral (CAL-V0-158).
	spend := d.spendGate(clock)
	spend.holdDeferred(obs.Tickets)
	d.recordBudget(spend)
	if h := d.ledger.Budget.Held; len(h) != 2 || h[0].Name != ts[0].ID || h[1].Name != ts[1].ID || h[0].Limit != LimitSessions {
		t.Fatalf("deferred holds %+v", h)
	}
	if n := len(eventsOf(t, d, "budget")); n != 2 {
		t.Fatalf("budget events %d", n)
	}

	// A role budget holds a ticket only when every role that would serve it
	// is exhausted; no budget holds nothing.
	c.TicketBudget, c.Roles[0].Budget = nil, &Budget{SessionsPerDay: 2}
	if h := d.budgetHeld(ts, clock); len(h) != 3 {
		t.Fatalf("role-exhausted holds %v", h)
	}
	c.Roles = append(c.Roles, Role{Name: "spare", Host: "sh", Cap: 1, Match: &Match{}, Prompt: "p", IdleSeconds: 30, WallSeconds: 60})
	if h := d.budgetHeld(ts, clock); len(h) != 0 {
		t.Fatalf("a ticket another role can serve is held: %v", h)
	}
	c.Roles, c.Roles[0].Budget = c.Roles[:1], nil
	if h := d.budgetHeld(ts, clock); h != nil {
		t.Fatalf("no budget holds %v", h)
	}
}
