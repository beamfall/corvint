//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// outageRig is a health rig with two OPEN tickets, no roles and a clock the
// test advances, so the CAL-V0-276 window is exact.
func outageRig(t *testing.T, ws func(*WorkState)) (*Dispatcher, *fakeQueue, string, func(time.Duration)) {
	t.Helper()
	d, q, source := healthRig(t, "exit 0", ws)
	d.Config.Roles = nil
	q.obs.Tickets = []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}
	clock := time.Now()
	d.Now = func() time.Time { return clock }
	return d, q, source, func(step time.Duration) { clock = clock.Add(step) }
}

func tickOnce(t *testing.T, d *Dispatcher) {
	t.Helper()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestCALV0274_TimedOutReadIsNamedAndUnavailableOnce is the #984 timeout
// fixture: a command that overruns workState.timeoutSeconds is reported as
// a timeout, never as the kill signal, and an outage carries exactly one
// provider-level UNAVAILABLE event beside its CAL-V0-053 per-read alerts.
func TestCALV0274_TimedOutReadIsNamedAndUnavailableOnce(t *testing.T) {
	d, _, _ := healthRig(t, "exit 0", func(w *WorkState) {
		w.Argv, w.TimeoutSeconds = []string{"/bin/sleep", "30"}, 1
	})
	d.Config.Roles = nil
	started := time.Now()
	for i := 0; i < 2; i++ {
		tickOnce(t, d)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Fatalf("timeoutSeconds 1 not applied: %v", elapsed)
	}
	alerts := eventsOf(t, d, "alert")
	if len(alerts) != 2 {
		t.Fatalf("alerts %+v", alerts)
	}
	for _, a := range alerts {
		if !strings.Contains(a.Message, "work state command: timed out after 1s") || strings.Contains(a.Message, "signal") {
			t.Fatalf("timeout not named: %q", a.Message)
		}
	}
	if !strings.Contains(d.ledger.WorkState.LastError, "timed out after 1s") {
		t.Fatalf("health %+v", d.ledger.WorkState)
	}
	if u := workStateEvents(t, d, "alert", "UNAVAILABLE"); len(u) != 1 || u[0].Detail["since"] == "" || !strings.Contains(u[0].Message, "work state unavailable since") {
		t.Fatalf("unavailable events %+v", u)
	}
	if (&WorkState{}).Timeout() != stateTimeout || stateTimeout != 60*time.Second {
		t.Fatal("the default timeout is not CAL-V0-053's 60 s")
	}
}

// TestCALV0276_OneFailedReadEmitsNoPerTicketStateEvents is the #984 churn
// fixture: a failed read inside the window (default one tick) emits zero
// per-ticket state events, keeps the last known states in the baseline,
// and recovers silently; an outage that outlasts the window takes UNKNOWN
// once. The observation itself is UNKNOWN throughout (CAL-V0-053).
func TestCALV0276_OneFailedReadEmitsNoPerTicketStateEvents(t *testing.T) {
	t.Run("default window", func(t *testing.T) {
		d, _, source, advance := outageRig(t, nil)
		writeStates(t, source)
		tickOnce(t, d)
		advance(time.Second)
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		obs, err := d.observe(context.Background())
		if err != nil || obs.Tickets[0].State != StateUnknown {
			t.Fatalf("a failed read must stay UNKNOWN for decisions: %v %+v", err, obs)
		}
		tickOnce(t, d)
		if n := len(eventsOf(t, d, "state")); n != 0 {
			t.Fatalf("one failed read emitted %d state events", n)
		}
		if !strings.Contains(d.ledger.Seen.Tickets["ticket:a:q:t1"], "|work|") || !WorkStateStale(d.Config, d.ledger.WorkState, d.Now()) {
			t.Fatalf("last known state not kept stale: %+v", d.ledger.Seen.Tickets)
		}
		writeStates(t, source)
		advance(time.Second)
		tickOnce(t, d)
		if n := len(eventsOf(t, d, "state")); n != 0 || len(workStateEvents(t, d, "alert", "RECOVERED")) != 1 {
			t.Fatalf("recovery emitted %d state events", n)
		}
		// An outage beyond the window takes UNKNOWN once and recovers once.
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 4; i++ {
			advance(time.Second)
			tickOnce(t, d)
		}
		if n := len(eventsOf(t, d, "state")); n != 2 || WorkStateStale(d.Config, d.ledger.WorkState, d.Now()) {
			t.Fatalf("outage past the window: %d state events", n)
		}
		writeStates(t, source)
		advance(time.Second)
		tickOnce(t, d)
		if n := len(eventsOf(t, d, "state")); n != 4 {
			t.Fatalf("recovery after the window: %d state events", n)
		}
	})
	t.Run("staleAfterSeconds", func(t *testing.T) {
		d, _, source, advance := outageRig(t, func(w *WorkState) { w.StaleAfterSeconds = 10 })
		writeStates(t, source)
		tickOnce(t, d)
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			advance(time.Second)
			tickOnce(t, d)
		}
		if n := len(eventsOf(t, d, "state")); n != 0 {
			t.Fatalf("outage inside staleAfterSeconds emitted %d state events", n)
		}
		advance(10 * time.Second)
		tickOnce(t, d)
		if n := len(eventsOf(t, d, "state")); n != 2 {
			t.Fatalf("outage past staleAfterSeconds: %d state events", n)
		}
	})
}

// TestCALV0276_FailedReadKeepsHeldPlansInBaseline: a failed read also
// drops the CAL-V0-105 held set, so the replanned window changes; inside
// the window the baseline keeps each UNKNOWN ticket's last plan as well,
// so the outage and its recovery still emit no state event.
func TestCALV0276_FailedReadKeepsHeldPlansInBaseline(t *testing.T) {
	c, ts := heldRig(t, map[string]string{})
	source := filepath.Join(c.WorkRoot, "states.json")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
	var replans []map[string]bool
	d, err := Open("held", c, windowQueue(ts, &replans), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	clock := time.Now()
	d.Now = func() time.Time { return clock }
	states := []byte(`{"h1":"hold-lane","h2":"hold-dep","r1":"ready"}`)
	if err := os.WriteFile(source, states, 0o600); err != nil {
		t.Fatal(err)
	}
	tickOnce(t, d)
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	obs, err := d.observe(context.Background())
	if err != nil || planOf(obs)["h1"] == (PlanView{"DEFERRED", "WORK_STATE_HELD"}) {
		t.Fatalf("the fixture does not change the plan on a failed read: %v %v", err, planOf(obs))
	}
	tickOnce(t, d)
	if err := os.WriteFile(source, states, 0o600); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	tickOnce(t, d)
	if s := eventsOf(t, d, "state"); len(s) != 0 {
		t.Fatalf("a failed read inside the window emitted state events %+v", s)
	}
}

// TestCALV0280_TerminalTicketWorkStateEmitsNoStateEvent: a work-state
// change on a COMPLETED or ARCHIVED ticket emits no state event, while an
// open ticket's change and a terminal ticket's status change still do.
func TestCALV0280_TerminalTicketWorkStateEmitsNoStateEvent(t *testing.T) {
	d, q, source, advance := outageRig(t, nil)
	q.obs.Tickets[1].Status = "COMPLETED"
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"t1":"work","t2":"done"}`)
	tickOnce(t, d)
	write(`{"t1":"review","t2":"other"}`)
	advance(time.Second)
	tickOnce(t, d)
	s := eventsOf(t, d, "state")
	if len(s) != 1 || s[0].Ticket != "ticket:a:q:t1" {
		t.Fatalf("state events %+v", s)
	}
	q.obs.Tickets[1].Status = "ARCHIVED"
	write(`{"t1":"review","t2":"again"}`)
	advance(time.Second)
	tickOnce(t, d)
	if s = eventsOf(t, d, "state"); len(s) != 2 || s[1].Ticket != "ticket:a:q:t2" || !strings.Contains(s[1].Message, "COMPLETED→ARCHIVED") {
		t.Fatalf("terminal status change %+v", s)
	}
}
