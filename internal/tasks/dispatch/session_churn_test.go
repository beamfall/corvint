//go:build darwin || linux

package dispatch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// churnFixture runs one ticket's sessions under a watched configuration and
// a test clock; each session lasts the given seconds of that clock.
type churnFixture struct {
	*reloadFixture
	c     *Config
	clock time.Time
}

func newChurnFixture(t *testing.T, s *ShortSessions) *churnFixture {
	return newChurnFixtureScript(t, s, "exit 0")
}

func newChurnFixtureScript(t *testing.T, s *ShortSessions, script string) *churnFixture {
	c := testConfig(t, script)
	c.Roles[0].Cap = 1
	c.Backoff.ParkAfter = 100
	c.ShortSessions = s
	f := &churnFixture{reloadFixture: newReloadFixture(t, c, &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}), c: c}
	f.clock = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	f.d.Now = func() time.Time { return f.clock }
	return f
}

// session ends the running worker after secs and ticks once.
func (f *churnFixture) session(secs int) {
	f.t.Helper()
	waitEnded(f.t, f.d)
	f.clock = f.clock.Add(time.Duration(secs) * time.Second)
	f.tick()
}

func (f *churnFixture) holds() []Event {
	var out []Event
	for _, e := range f.events("cooldown") {
		if e.Detail["reason"] == ChurnReason {
			out = append(out, e)
		}
	}
	return out
}

// ATR-V0-060, ATR-V0-061: three consecutive short sessions hold the next
// launch for holdSeconds, the hold is an event and a record, and one long
// session resets the streak.
func TestATRV0060_ShortSessionsHoldThenLongSessionResets(t *testing.T) {
	f := newChurnFixture(t, &ShortSessions{Seconds: 60, Count: 3, HoldSeconds: 600})
	f.tick()
	first := f.clock.Add(10 * time.Second).Format(time.RFC3339)
	f.session(10)
	f.session(20)
	if n := len(f.events("launched")); n != 3 || len(f.holds()) != 0 {
		t.Fatalf("two short sessions: %d launches, %d holds", n, len(f.holds()))
	}
	f.session(15)
	h := f.holds()
	if n := len(f.events("launched")); n != 3 || len(h) != 1 {
		t.Fatalf("third short session: %d launches, %d holds", n, len(h))
	}
	until := f.clock.Add(600 * time.Second).Format(time.RFC3339)
	want := map[string]string{"reason": ChurnReason, "sessions": "3", "medianSeconds": "15", "firstSeen": first, "holdUntil": until}
	for k, v := range want {
		if h[0].Detail[k] != v {
			t.Fatalf("hold detail %s = %q, want %q (%+v)", k, h[0].Detail[k], v, h[0].Detail)
		}
	}
	rec, err := ReadSessionChurn(SessionChurnPath(f.d.dir))
	if err != nil || rec.Tickets["ticket:a:q:t1"] == nil || rec.Tickets["ticket:a:q:t1"].HoldUntil != until {
		t.Fatalf("record %+v %v", rec, err)
	}
	f.clock = f.clock.Add(599 * time.Second)
	f.tick()
	if n := len(f.events("launched")); n != 3 {
		t.Fatalf("launched during the hold: %d", n)
	}
	f.clock = f.clock.Add(2 * time.Second)
	f.tick()
	if n := len(f.events("launched")); n != 4 {
		t.Fatalf("the hold did not end: %d launches", n)
	}
	f.session(120)
	if _, err := os.Stat(SessionChurnPath(f.d.dir)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a long session did not reset the streak: %v", err)
	}
	if n := len(f.events("launched")); n != 5 {
		t.Fatalf("after the reset: %d launches", n)
	}
	waitEnded(t, f.d)
}

// ATR-V0-060: without the setting, short sessions hold nothing and leave no
// record; a role's own setting replaces the global one.
func TestATRV0060_AbsentSettingIsUnchangedAndRoleOverrides(t *testing.T) {
	f := newChurnFixture(t, nil)
	f.tick()
	for range 4 {
		f.session(1)
	}
	if n := len(f.events("launched")); n != 5 || len(f.holds()) != 0 {
		t.Fatalf("no setting: %d launches, %d holds", n, len(f.holds()))
	}
	if _, err := os.Stat(SessionChurnPath(f.d.dir)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record without the setting: %v", err)
	}
	waitEnded(t, f.d)

	g := newChurnFixture(t, &ShortSessions{Seconds: 60, Count: 2, HoldSeconds: 600})
	g.c.Roles[0].ShortSessions = &ShortSessions{Seconds: 5, Count: 2, HoldSeconds: 600}
	g.write(g.c)
	g.tick()
	g.session(10)
	g.session(10)
	if len(g.holds()) != 0 {
		t.Fatal("the global setting applied over the role's own")
	}
	waitEnded(t, g.d)
}

// ATR-V0-060: removing the setting by a configuration reload lifts the hold
// on the next tick; ATR-V0-061: a worker of another ticket keeps running.
func TestATRV0060_ReloadLiftsHoldAndRunningWorkersContinue(t *testing.T) {
	stop := `case "$CORVINT_DISPATCH_TICKET" in *t2) while [ ! -f "$(dirname "$0")/stop" ]; do sleep 0.05; done;; esac`
	f := newChurnFixtureScript(t, &ShortSessions{Seconds: 60, Count: 2, HoldSeconds: 3600}, stop)
	f.tick()
	f.session(5)
	f.session(5)
	if len(f.holds()) != 1 {
		t.Fatalf("%d holds", len(f.holds()))
	}
	f.c.Roles[0].Cap = 2
	f.write(f.c)
	f.q.obs.Tickets = append(f.q.obs.Tickets, ticket("t2", "P1", 2))
	f.tick()
	if f.d.Running() != 1 || f.d.ledger.Workers[0].Key != "ticket:a:q:t2" {
		t.Fatalf("held ticket launched or t2 missing: %+v", f.d.ledger.Workers)
	}
	pid := f.d.ledger.Workers[0].PID
	f.c.ShortSessions = nil
	f.write(f.c)
	f.tick()
	if _, err := os.Stat(SessionChurnPath(f.d.dir)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record kept after the setting was removed: %v", err)
	}
	kept := false
	for _, w := range f.d.ledger.Workers {
		kept = kept || w.Key == "ticket:a:q:t2" && w.PID == pid
	}
	if f.d.Running() != 2 || !kept {
		t.Fatalf("hold not lifted or the running worker changed: %+v", f.d.ledger.Workers)
	}
	if err := os.WriteFile(filepath.Join(f.c.WorkRoot, "stop"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitEnded(t, f.d)
}

// ATR-V0-061: a newer-version record is kept and disables tracking; a
// malformed one is replaced.
func TestATRV0061_RecordVersions(t *testing.T) {
	f := newChurnFixture(t, &ShortSessions{Seconds: 60, Count: 2, HoldSeconds: 600})
	path := SessionChurnPath(f.d.dir)
	newer := []byte(`{"profile":"taskman-dispatch-session-churn/1"}`)
	if err := os.WriteFile(path, newer, 0o600); err != nil {
		t.Fatal(err)
	}
	f.tick()
	f.session(1)
	f.session(1)
	if raw, _ := os.ReadFile(path); string(raw) != string(newer) || len(f.holds()) != 0 {
		t.Fatalf("newer record replaced or tracked: %q", raw)
	}
	waitEnded(t, f.d)

	g := newChurnFixture(t, &ShortSessions{Seconds: 60, Count: 2, HoldSeconds: 600})
	if err := os.WriteFile(SessionChurnPath(g.d.dir), []byte(`{"profile":"taskman-dispatch-session-churn/0","program":"prog","tickets":{"x":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	g.tick()
	g.session(1)
	g.session(1)
	if len(g.holds()) != 1 {
		t.Fatalf("a malformed record was not replaced: %d holds", len(g.holds()))
	}
	waitEnded(t, g.d)
}

func TestATRV0060_ShortSessionsValidation(t *testing.T) {
	for _, s := range []ShortSessions{{0, 3, 1}, {60, 1, 1}, {60, 3, 0}, {86401, 3, 1}, {60, 101, 1}} {
		c := testConfig(t, "exit 0")
		c.ShortSessions = &s
		if c.validate() == nil {
			t.Fatalf("accepted %+v", s)
		}
		c.ShortSessions, c.Roles[0].ShortSessions = nil, &s
		if c.validate() == nil {
			t.Fatalf("role accepted %+v", s)
		}
	}
}
