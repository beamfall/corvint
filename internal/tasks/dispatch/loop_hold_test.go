//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CAL-V0-102/103: the roster never launches a LOOP_DETECTED ticket, and the
// dispatcher raises one typed blocked needs-owner escalation per episode: on
// its first observation, not again on later ticks or after a restart, again
// for a new episode (a newer counted generation) and after the hold clears
// and recurs.
func TestCALV0102_DispatcherSkipsAndEscalatesLoopOnce(t *testing.T) {
	t.Run("CAL-V0-102 CAL-V0-103 DispatcherSkipsAndEscalatesLoopOnce", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		held, free := ticket("h", "P0", 1), ticket("f", "P1", 2)
		held.Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"1", "2", "3"}}
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{held, free}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		tick := func(d *Dispatcher) {
			t.Helper()
			if err := d.Tick(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		count := func(d *Dispatcher, want int) {
			t.Helper()
			var got []Event
			for _, e := range eventsOf(t, d, "needs-owner") {
				if e.Detail["code"] == "LOOP_DETECTED" {
					got = append(got, e)
				}
			}
			if len(got) != want {
				t.Fatalf("loop escalations %d, want %d: %+v", len(got), want, got)
			}
			if e := got[len(got)-1]; e.Ticket != held.ID || e.Detail["kind"] != "blocked" || e.Detail["signal"] == "" || e.Detail["generations"] == "" || !strings.Contains(e.Message, "corvint-tasks ticket reopen") {
				t.Fatalf("escalation %+v", e)
			}
		}
		tick(d)
		for _, w := range d.ledger.Workers {
			if w.Ticket == held.ID {
				t.Fatal("held ticket launched")
			}
		}
		if d.Running() != 1 {
			t.Fatalf("running %d", d.Running())
		}
		waitEnded(t, d)
		count(d, 1)
		tick(d)
		count(d, 1)
		l, err := LoadLedger(d.dir, d.Program)
		if err != nil || l.Seen.Loops[held.ID].Signal != "NO_PROGRESS" || len(l.Seen.Loops[held.ID].Generations) != 3 {
			t.Fatalf("seen.loops not persisted: %+v %v", l, err)
		}
		d = issue502Restart(t, d)
		count(d, 1)

		q.obs.Tickets[0].Loop = &LoopHold{Signal: "ALTERNATING_RETURNS", AcceptanceRevision: "0", Generations: []string{"4", "5", "6", "7", "8", "9"}}
		tick(d)
		count(d, 2)
		if e := eventsOf(t, d, "needs-owner"); e[len(e)-1].Detail["generations"] != "4,5,6,7,8,9" || e[len(e)-1].Detail["signal"] != "ALTERNATING_RETURNS" {
			t.Fatalf("second episode %+v", e[len(e)-1])
		}
		q.obs.Tickets[0].Loop = nil
		tick(d)
		if len(d.ledger.Seen.Loops) != 0 {
			t.Fatalf("cleared hold kept: %+v", d.ledger.Seen.Loops)
		}
		q.obs.Tickets[0].Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"10", "11", "12"}}
		tick(d)
		count(d, 3)
	})
}

// D8: a dispatcher that observes no hold writes no loops member, and the
// strict reader refuses a malformed recorded hold without rewriting it.
func TestCALV0102_LedgerLoopsAreClosedAndOptional(t *testing.T) {
	t.Run("CAL-V0-102 LedgerLoopsAreClosedAndOptional", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("f", "P1", 1)}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		waitEnded(t, d)
		d.Close()
		raw, _ := os.ReadFile(filepath.Join(d.dir, "state.json"))
		if strings.Contains(string(raw), `"loops"`) {
			t.Fatalf("hold-free ledger carries loops:\n%s", raw)
		}

		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")
		a := progressDigest("A")
		l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
			Progress: map[string]*ProgressHistory{"ticket:a:q:t": {Current: a, Seen: []string{a}}},
			Seen:     &Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}, Loops: map[string]LoopHold{"ticket:a:q:t": {Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"7", "8", "9"}}}}}
		good, err := ledgerBytes(l)
		if err != nil {
			t.Fatal(err)
		}
		valid := string(good)
		if err := os.WriteFile(path, good, 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := LoadLedger(dir, "prog"); err != nil || got.Seen.Loops["ticket:a:q:t"].Generations[2] != "9" {
			t.Fatal("valid ledger refused:", err)
		}
		for name, bad := range map[string]string{
			"alias":          strings.Replace(valid, `"loops":`, `"Loops":`, 1),
			"unknown member": strings.Replace(valid, `"signal":`, `"extra": 1, "signal":`, 1),
			"bad signal":     strings.Replace(valid, `"NO_PROGRESS"`, `"STUCK"`, 1),
			"bad generation": strings.Replace(valid, `"8"`, `"x"`, 1),
			"bad revision":   strings.Replace(valid, `"acceptanceRevision": "0"`, `"acceptanceRevision": "x"`, 1),
			"no revision":    strings.Replace(valid, `"acceptanceRevision": "0",`, ``, 1),
			"empty":          strings.Replace(strings.Replace(strings.Replace(valid, `"7",`, ``, 1), `"8",`, ``, 1), `"9"`, ``, 1),
			"bad key":        strings.Replace(valid, `"ticket:a:q:t"`, `"t"`, 1), // seen precedes progress,
		} {
			t.Run(name, func(t *testing.T) {
				if bad == valid {
					t.Fatal("malformed fixture not reached")
				}
				if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := LoadLedger(dir, "prog"); err == nil {
					t.Fatal("malformed loops admitted")
				}
				after, _ := os.ReadFile(path)
				if string(after) != bad {
					t.Fatal("refusal rewrote the ledger")
				}
			})
		}
	})
}

// CAL-V0-103: a ledger whose only token-era member is seen.loops is still
// read strictly, so an alias, a duplicate member, a null map or hold, and a
// pending marker other than true refuse without progress or poolSweeps.
func TestCALV0103_LedgerLoopsStrictWithoutProgress(t *testing.T) {
	t.Run("CAL-V0-103 LedgerLoopsStrictWithoutProgress", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")
		l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
			Seen: &Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}, Loops: map[string]LoopHold{"ticket:a:q:t": {Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"7", "8", "9"}}}}}
		good, err := ledgerBytes(l)
		if err != nil {
			t.Fatal(err)
		}
		valid := string(good)
		if strings.Contains(valid, `"progress"`) || strings.Contains(valid, `"poolSweeps"`) {
			t.Fatalf("fixture carries progress:\n%s", valid)
		}
		pending := strings.Replace(valid, `"signal":`, `"pending":true,"signal":`, 1)
		for name, text := range map[string]string{"valid": valid, "pending": pending} {
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := LoadLedger(dir, "prog")
			if err != nil || got.Seen.Loops["ticket:a:q:t"].Pending != (name == "pending") {
				t.Fatalf("%s ledger refused or misread: %v", name, err)
			}
		}
		for name, bad := range map[string]string{
			"alias":            strings.Replace(valid, `"loops":`, `"Loops":`, 1),
			"seen alias":       strings.Replace(valid, `"seen":`, `"Seen":`, 1),
			"duplicate loops":  strings.Replace(valid, `"loops":`, `"loops":{},"loops":`, 1),
			"duplicate hold":   strings.Replace(valid, `"ticket:a:q:t":`, `"ticket:a:q:t":{"signal":"NO_PROGRESS","acceptanceRevision":"0","generations":["1"]},"ticket:a:q:t":`, 1),
			"duplicate signal": strings.Replace(valid, `"signal":`, `"signal":"NO_PROGRESS","signal":`, 1),
			"duplicate seen":   strings.Replace(valid, `"seen":`, `"seen":{"loops":{}},"seen":`, 1),
			"null loops":       `{"profile":"` + StateProfile + `","program":"prog","workers":[],"backoff":{},"seen":{"tickets":{},"claims":{},"lanes":{},"loops":null}}`,
			"null hold":        `{"profile":"` + StateProfile + `","program":"prog","workers":[],"backoff":{},"seen":{"tickets":{},"claims":{},"lanes":{},"loops":{"ticket:a:q:t":null}}}`,
			"pending false":    strings.Replace(valid, `"signal":`, `"pending":false,"signal":`, 1),
			"pending null":     strings.Replace(valid, `"signal":`, `"pending":null,"signal":`, 1),
		} {
			t.Run(name, func(t *testing.T) {
				if bad == valid {
					t.Fatal("malformed fixture not reached")
				}
				if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := LoadLedger(dir, "prog"); err == nil {
					t.Fatalf("malformed loops admitted:\n%s", bad)
				}
			})
		}
	})
}

// CAL-V0-103: when the event log cannot take the needs-owner event, the
// episode stays pending in the writable ledger, and a restart raises it
// exactly once after the log recovers.
func TestCALV0103_LoopEscalationSurvivesEventAppendFailure(t *testing.T) {
	t.Run("CAL-V0-103 LoopEscalationSurvivesEventAppendFailure", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		held := ticket("h", "P0", 1)
		held.Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"1", "2", "3"}}
		q := &fakeQueue{obs: Observation{Tickets: []Ticket{held}}}
		d, err := Open("prog", c, q, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		events := filepath.Join(d.dir, "events.jsonl")
		_ = os.Remove(events)
		if err := os.Mkdir(events, 0o700); err != nil { // appends now fail; state.json stays writable
			t.Fatal(err)
		}
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		l, err := LoadLedger(d.dir, d.Program)
		if err != nil || !l.Seen.Loops[held.ID].Pending {
			t.Fatalf("unrecorded episode not pending: %+v %v", l, err)
		}
		if err := os.Remove(events); err != nil {
			t.Fatal(err)
		}
		loops := func(d *Dispatcher) int {
			t.Helper()
			n := 0
			for _, e := range eventsOf(t, d, "needs-owner") {
				if e.Detail["code"] == "LOOP_DETECTED" && e.Ticket == held.ID {
					n++
				}
			}
			return n
		}
		r := issue502Restart(t, d)
		if n := loops(r); n != 1 {
			t.Fatalf("loop escalations after recovery %d, want 1", n)
		}
		if r.ledger.Seen.Loops[held.ID].Pending {
			t.Fatal("recorded episode still pending")
		}
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		r = issue502Restart(t, r)
		if n := loops(r); n != 1 {
			t.Fatalf("recorded episode raised again: %d", n)
		}
	})
}

// loopFaultDispatcher opens a dispatcher over one NO_PROGRESS hold and
// counts its readable LOOP_DETECTED events.
func loopFaultDispatcher(t *testing.T) (*Dispatcher, *fakeQueue, func(*Dispatcher) int) {
	t.Helper()
	held := ticket("h", "P0", 1)
	held.Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "0", Generations: []string{"1", "2", "3"}}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{held}}}
	d, err := Open("prog", testConfig(t, "exit 0"), q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { appendEvent = appendEventLog })
	return d, q, func(d *Dispatcher) int {
		t.Helper()
		n := 0
		for _, e := range eventsOf(t, d, "needs-owner") {
			if e.Detail["code"] == "LOOP_DETECTED" && e.Ticket == held.ID {
				n++
			}
		}
		return n
	}
}

// loopPending reloads the closed dispatcher's ledger and reports whether
// the hold's episode is still pending.
func loopPending(t *testing.T, d *Dispatcher) bool {
	t.Helper()
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil {
		t.Fatal(err)
	}
	h, ok := l.Seen.Loops[ticket("h", "P0", 1).ID]
	if !ok {
		t.Fatal("hold not recorded")
	}
	return h.Pending
}

// CAL-V0-103: an append that fails part-way leaves an unterminated
// fragment; the next append ends it first, so the lines after it and the
// retried escalation parse, the escalation is readable exactly once, and a
// well-formed log receives exactly each line.
func TestCALV0103_LoopEscalationSurvivesPartialEventWrite(t *testing.T) {
	t.Run("CAL-V0-103 LoopEscalationSurvivesPartialEventWrite", func(t *testing.T) {
		d, _, loops := loopFaultDispatcher(t)
		events := filepath.Join(d.dir, "events.jsonl")
		appendEvent = func(dir string, e Event) error {
			if e.Detail["code"] != "LOOP_DETECTED" {
				return appendEventLog(dir, e)
			}
			raw, err := json.Marshal(e)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(events, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
			if err != nil {
				return err
			}
			_, _ = f.Write(raw[:len(raw)/2])
			_ = f.Close()
			return errors.New("short write")
		}
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := loops(d); n != 0 {
			t.Fatalf("fragment read as %d events", n)
		}
		if !loopPending(t, d) {
			t.Fatal("partially written episode not pending")
		}
		torn, err := os.ReadFile(events)
		if err != nil || !bytes.Contains(torn, []byte("over generations 1,2,\n")) {
			t.Fatalf("fault left no fragment: %q %v", torn, err)
		}
		if len(eventsOf(t, d, "stopped")) != 1 {
			t.Fatal("the line after the fragment does not parse")
		}
		appendEvent = appendEventLog
		r := issue502Restart(t, d)
		if n := loops(r); n != 1 {
			t.Fatalf("loop escalations after the retry %d, want 1", n)
		}
		if r.ledger.Seen.Loops[ticket("h", "P0", 1).ID].Pending {
			t.Fatal("retried episode still pending")
		}
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		r = issue502Restart(t, r)
		if n := loops(r); n != 1 {
			t.Fatalf("retried episode raised again: %d", n)
		}

		dir := t.TempDir()
		line := func(n int) []byte {
			raw, err := json.Marshal(Event{Profile: EventProfile, Seq: uint64(n), Kind: "started"})
			if err != nil {
				t.Fatal(err)
			}
			return append(raw, '\n')
		}
		for n := 1; n <= 2; n++ {
			if err := appendEventLog(dir, Event{Profile: EventProfile, Seq: uint64(n), Kind: "started"}); err != nil {
				t.Fatal(err)
			}
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "events.jsonl")); !bytes.Equal(got, append(line(1), line(2)...)) {
			t.Fatalf("well-formed log bytes changed: %q", got)
		}
	})
}

// CAL-V0-103: an append whose line landed but whose close failed leaves the
// episode pending; the retry finds the readable event and clears pending
// without a duplicate.
func TestCALV0103_LoopEscalationNotDuplicatedAfterCloseFailure(t *testing.T) {
	t.Run("CAL-V0-103 LoopEscalationNotDuplicatedAfterCloseFailure", func(t *testing.T) {
		d, _, loops := loopFaultDispatcher(t)
		appendEvent = func(dir string, e Event) error {
			if err := appendEventLog(dir, e); err != nil || e.Detail["code"] != "LOOP_DETECTED" {
				return err
			}
			return errors.New("close failed")
		}
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := loops(d); n != 1 {
			t.Fatalf("landed escalations %d, want 1", n)
		}
		if !loopPending(t, d) {
			t.Fatal("episode with a failed close not pending")
		}
		appendEvent = appendEventLog
		r := issue502Restart(t, d)
		if n := loops(r); n != 1 {
			t.Fatalf("retry duplicated the readable escalation: %d", n)
		}
		if r.ledger.Seen.Loops[ticket("h", "P0", 1).ID].Pending {
			t.Fatal("recorded episode still pending")
		}
	})
}

// CAL-V0-103: a crash between the event append and the ledger save loses
// the recorded episode; the restart finds the readable event and raises no
// duplicate, while a new acceptance revision is a new episode.
func TestCALV0103_LoopEscalationNotDuplicatedAfterUnsavedLedger(t *testing.T) {
	t.Run("CAL-V0-103 LoopEscalationNotDuplicatedAfterUnsavedLedger", func(t *testing.T) {
		d, q, loops := loopFaultDispatcher(t)
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if loopPending(t, d) {
			t.Fatal("recorded episode pending")
		}
		l, err := LoadLedger(d.dir, d.Program)
		if err != nil {
			t.Fatal(err)
		}
		delete(l.Seen.Loops, ticket("h", "P0", 1).ID) // the save the crash lost
		if err := l.save(d.dir); err != nil {
			t.Fatal(err)
		}
		r := issue502Restart(t, d)
		if n := loops(r); n != 1 {
			t.Fatalf("unsaved episode raised again: %d", n)
		}
		if _, ok := r.ledger.Seen.Loops[ticket("h", "P0", 1).ID]; !ok {
			t.Fatal("restart did not record the episode")
		}
		q.obs.Tickets[0].Loop = &LoopHold{Signal: "NO_PROGRESS", AcceptanceRevision: "1", Generations: []string{"1", "2", "3"}}
		if err := r.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if n := loops(r); n != 2 {
			t.Fatalf("new acceptance revision raised %d escalations, want 2", n)
		}
	})
}
