//go:build darwin || linux

package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
)

func pressureRosterConfig(t *testing.T) *Config {
	c := testConfig(t, "exit 0")
	c.GlobalCap = 5
	c.Roles[0].Cap, c.Roles[0].Priority = 4, 10
	c.Roles = append(c.Roles, Role{Name: "review", Host: "sh", Cap: 2, Priority: 20, Match: &Match{Labels: []string{"review"}}, Prompt: "review {ticket}", IdleSeconds: 30, WallSeconds: 60})
	c.Pinned = []string{"t4"}
	p := issue497Config()
	p.LevelCaps = map[string]int{"1": 2, "2": 1}
	p.ExemptRoles, p.ExemptTickets = []string{"review"}, []string{"t3"}
	c.Pressure = &p
	return c
}

func pressureObs() *Observation {
	return &Observation{Tickets: []Ticket{ticket("t1", "P0", 1), ticket("t2", "P1", 1), ticket("t3", "P2", 1), ticket("r1", "P3", 1, "review"), ticket("t4", "P3", 2)}}
}

func keysOf(as []Assignment) []string {
	var out []string
	for _, a := range as {
		out = append(out, a.Role+"/"+a.Local+"/"+string(rune('0'+a.Slot)))
	}
	return out
}

// CAL-V0-068: the pressure budget applies after the static fences and before
// a candidate consumes a slot or key; pins, exempt tickets and exempt roles
// bypass it, a held key that a later exempt role launches is not held, and a
// held candidate reserves nothing.
func TestCALV0068_RosterPressurePrecedence(t *testing.T) {
	c, obs := pressureRosterConfig(t), pressureObs()
	budget := func(level int, running ...Assignment) *PressureBudget {
		b, err := NewPressureBudget(c.Pressure, PressureState{Level: level}, running, c.Pinned)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	out, held := RosterWithPressure(c, obs, nil, nil, budget(2))
	if got, want := keysOf(out), []string{"impl/t4/1", "impl/t1/2", "impl/t3/3", "review/r1/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("launches %v want %v", got, want)
	}
	if got, want := keysOf(held), []string{"impl/t2/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("held %v want %v", got, want)
	}
	again, heldAgain := RosterWithPressure(c, obs, nil, nil, budget(2))
	if !reflect.DeepEqual(again, out) || !reflect.DeepEqual(heldAgain, held) {
		t.Fatal("pressure roster is not deterministic")
	}
	// A running non-exempt worker consumes the level cap; it is not stopped.
	out, held = RosterWithPressure(c, obs, []Busy{{Role: "impl", Key: "ticket:a:q:t9", Slot: 1}}, nil, budget(2, Assignment{Role: "impl", Key: "ticket:a:q:t9", Ticket: "ticket:a:q:t9", Local: "t9"}))
	if got, want := keysOf(out), []string{"impl/t4/2", "impl/t3/3", "review/r1/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("busy launches %v want %v", got, want)
	}
	if got, want := keysOf(held), []string{"impl/t1/0", "impl/t2/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("busy held %v want %v", got, want)
	}
	// Static fences take precedence: the global cap stops the roster before
	// the budget is consulted, and skipped keys are never reported as held.
	c.GlobalCap = 2
	out, held = RosterWithPressure(c, obs, nil, map[string]bool{"ticket:a:q:t2": true}, budget(2))
	if got := keysOf(out); !reflect.DeepEqual(got, []string{"impl/t4/1", "impl/t1/2"}) || len(held) != 0 {
		t.Fatalf("global cap: launches %v held %v", got, keysOf(held))
	}
	c.GlobalCap = 5
	// Level 0, an absent budget and Roster agree.
	calm, held := RosterWithPressure(c, obs, nil, nil, budget(0))
	if !reflect.DeepEqual(calm, Roster(c, obs, nil, nil)) || len(held) != 0 {
		t.Fatal("calm pressure changed the roster")
	}
	if plain, held := RosterWithPressure(c, obs, nil, nil, nil); !reflect.DeepEqual(plain, calm) || held != nil {
		t.Fatal("nil budget changed the roster")
	}
}

// CAL-V0-068: held reports only candidates the static role and global caps
// would still have admitted; candidates those caps block are not held, and
// admission is unchanged.
func TestCALV0068_HeldRespectsStaticCaps(t *testing.T) {
	c, obs := pressureRosterConfig(t), pressureObs()
	c.Pinned, c.Pressure.ExemptTickets = nil, nil
	c.Pressure.LevelCaps = map[string]int{"1": 1, "2": 0}
	b, err := NewPressureBudget(c.Pressure, PressureState{Level: 2}, nil, c.Pinned)
	if err != nil {
		t.Fatal(err)
	}
	c.Roles[0].Cap = 1
	out, held := RosterWithPressure(c, obs, nil, nil, b)
	if got, want := keysOf(out), []string{"review/r1/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("role cap launches %v want %v", got, want)
	}
	if got, want := keysOf(held), []string{"impl/t1/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("role cap held %v want %v", got, want)
	}
	c.Roles[0].Cap, c.GlobalCap = 4, 2
	out, held = RosterWithPressure(c, obs, nil, nil, b)
	if got, want := keysOf(out), []string{"review/r1/1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("global cap launches %v want %v", got, want)
	}
	if got, want := keysOf(held), []string{"impl/t1/0", "impl/t2/0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("global cap held %v want %v", got, want)
	}
}

func TestCALV0068_ConfigValidation(t *testing.T) {
	for name, f := range map[string]func(*Config){
		"unknown exempt role": func(c *Config) { c.Pressure.ExemptRoles = []string{"ghost"} },
		"thresholds":          func(c *Config) { c.Pressure.SwapCritical = 2 },
		"caps":                func(c *Config) { c.Pressure.LevelCaps = map[string]int{"1": 1} },
	} {
		c := pressureRosterConfig(t)
		f(c)
		raw, _ := json.Marshal(c)
		if _, err := DecodeConfig(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	raw, _ := json.Marshal(pressureRosterConfig(t))
	if _, err := DecodeConfig(raw); err != nil {
		t.Fatalf("valid pressure config refused: %v", err)
	}
	raw = []byte(strings.Replace(string(raw), `"ticksToChange":2`, `"ticksToChange":2,"extra":1`, 1))
	if _, err := DecodeConfig(raw); err == nil {
		t.Fatal("unknown pressure field accepted")
	}
}

type pressureFeed struct{ sample PressureSample }

func (f *pressureFeed) read(context.Context, time.Time, pressureWant) PressureSample {
	return f.sample
}

func throttled(t *testing.T, d *Dispatcher) []Event {
	t.Helper()
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []Event
	for _, e := range events {
		if e.Kind == "throttled" {
			out = append(out, e)
		}
	}
	return out
}

// CAL-V0-068: pressure dwells before it changes level, holds only new
// non-exempt launches, never stops a running worker, keeps its level on an
// UNKNOWN sample, reports one throttled event per change and releases after
// calm dwell.
func TestCALV0068_DispatcherThrottlesNewLaunchesOnly(t *testing.T) {
	c := testConfig(t, "exec sleep 60")
	c.Roles[0].Cap = 3
	p := issue497Config()
	p.LevelCaps = map[string]int{"1": 1, "2": 0}
	c.Pressure = &p
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	t.Cleanup(func() {
		for _, w := range d.ledger.Workers {
			syscall.Kill(w.PID, syscall.SIGKILL)
			if !gone(w.PID) {
				t.Errorf("worker %d survived cleanup", w.PID)
			}
		}
	})
	feed := &pressureFeed{sample: issue497Sample(1, .1)}
	d.pressureSampler = feed.read
	ctx := context.Background()
	tick := func() {
		t.Helper()
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	tick()
	if d.Running() != 1 || len(throttled(t, d)) != 0 {
		t.Fatalf("calm tick: running %d throttled %v", d.Running(), throttled(t, d))
	}
	first := d.ledger.Workers[0].ID
	feed.sample = issue497Sample(9, .1)
	tick()
	if st := d.ledger.Pressure.State; st.Level != 0 || st.PendingLevel != 2 || st.PendingTicks != 1 {
		t.Fatalf("dwell: %+v", st)
	}
	q.obs.Tickets = append(q.obs.Tickets, ticket("t2", "P1", 2), ticket("t3", "P1", 3))
	tick()
	tick()
	if st := d.ledger.Pressure.State; st.Level != 2 || d.Running() != 1 || d.ledger.Workers[0].ID != first || d.ledger.Workers[0].State != "RUNNING" {
		t.Fatalf("critical: state %+v running %d", st, d.Running())
	}
	if held := d.ledger.Pressure.Held; len(held) != 2 || held[0].Key != "ticket:a:q:t2" || held[1].Key != "ticket:a:q:t3" {
		t.Fatalf("held %+v", held)
	}
	ev := throttled(t, d)
	if len(ev) != 1 || ev[0].Detail["level"] != "2" || ev[0].Detail["held"] != "2" || ev[0].Detail["sample"] != "OBSERVED" || ev[0].Detail["loadPerCpu"] != "9.000" || ev[0].Detail["swapFraction"] != "0.100" || !strings.Contains(ev[0].Message, "t2, t3") {
		t.Fatalf("throttled events %+v", ev)
	}
	if has(kinds(t, d), "killing") {
		t.Fatal("pressure stopped a running worker")
	}
	// UNKNOWN keeps the level and cancels dwell; it is reported once.
	feed.sample = PressureSample{Source: "test", Problems: []string{strings.Repeat("x", 500)}}
	tick()
	tick()
	if st := d.ledger.Pressure.State; st.Level != 2 || !st.Unknown || d.Running() != 1 || len(throttled(t, d)) != 2 || throttled(t, d)[1].Detail["sample"] != StateUnknown {
		t.Fatalf("unknown: state %+v throttled %+v", st, throttled(t, d))
	}
	if p := d.ledger.Pressure.Sample.Problems; len(p) != 1 || len(p[0]) != maxPressureProblem {
		t.Fatalf("sample problems not bounded: %d", len(p[0]))
	}
	// Bounding keeps valid UTF-8 and cuts at a rune boundary.
	bounded := boundPressureSample(PressureSample{Source: "\xff" + strings.Repeat("é", 200), Problems: []string{strings.Repeat("é", 150)}})
	if !utf8.ValidString(bounded.Source) || len(bounded.Source) > 256 || !utf8.ValidString(bounded.Problems[0]) || len(bounded.Problems[0]) != maxPressureProblem {
		t.Fatalf("bounded sample %q %q", bounded.Source, bounded.Problems)
	}
	// The record round-trips through the ledger.
	if err := d.ledger.save(d.dir); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(d.dir, "prog")
	if err != nil || l.Pressure == nil || !reflect.DeepEqual(l.Pressure.Held, d.ledger.Pressure.Held) {
		t.Fatalf("ledger round trip: %v %+v", err, l)
	}
	// Calm must persist for ticksToChange ticks before release; the first
	// calm tick reports the sample known again while the level holds.
	feed.sample = issue497Sample(1, .1)
	tick()
	if d.Running() != 1 || len(throttled(t, d)) != 3 || throttled(t, d)[2].Detail["level"] != "2" {
		t.Fatalf("released before calm dwell: running %d", d.Running())
	}
	tick()
	if st := d.ledger.Pressure.State; st.Level != 0 || d.Running() != 3 || len(d.ledger.Pressure.Held) != 0 {
		t.Fatalf("release: state %+v running %d", st, d.Running())
	}
	if ev := throttled(t, d); len(ev) != 4 || ev[3].Detail["level"] != "0" || ev[3].Detail["held"] != "0" || !strings.Contains(ev[3].Message, "none") {
		t.Fatalf("release event %+v", ev)
	}
}

// CAL-V0-068: a restart keeps the recorded level but cancels dwell until the
// first sample; removing the configuration clears the derived record.
func TestCALV0068_RestartKeepsLevelAndDisableClears(t *testing.T) {
	c := testConfig(t, "exit 0")
	p := issue497Config()
	p.TicksToChange = 1
	c.Pressure = &p
	q := &fakeQueue{}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if st := d.ledger.Pressure.State; st.Level != 0 || !st.Unknown {
		t.Fatalf("fresh pressure is not UNKNOWN warmup: %+v", st)
	}
	d.pressureSampler = (&pressureFeed{sample: issue497Sample(9, .95)}).read
	if err := d.Tick(context.Background()); err != nil || d.ledger.Pressure.State.Level != 2 {
		t.Fatalf("tick: %v %+v", err, d.ledger.Pressure.State)
	}
	d.ledger.Pressure.State.PendingLevel, d.ledger.Pressure.State.PendingTicks = 0, 0
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if st := d.ledger.Pressure.State; st.Level != 2 || !st.Unknown || st.PendingLevel != 2 || st.PendingTicks != 0 {
		t.Fatalf("restart state %+v", st)
	}
	if s := d.ledger.Pressure.Sample; !s.SampledAt.IsZero() || s.LoadKnown || s.Source != "" {
		t.Fatalf("restart kept the previous run's sample: %+v", s)
	}
	d.Close()
	c.Pressure = nil
	d, err = Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()
	if l, err := LoadLedger(d.dir, "prog"); err != nil || l.Pressure != nil {
		t.Fatalf("disabled pressure kept its record: %v", err)
	}
}

// CAL-V0-068: the ledger refuses an invalid or unbounded pressure record and
// accepts a valid one beside checked progress history.
func TestCALV0068_LedgerPressureRecordValidated(t *testing.T) {
	dir := t.TempDir()
	write := func(pressure string) error {
		raw := `{"profile":"taskman-dispatch-state/0","program":"prog","launchSeq":0,"eventSeq":0,"workers":[],"backoff":{},"progress":{},"pressure":` + pressure + `}`
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadLedger(dir, "prog")
		return err
	}
	valid := `{"state":{"level":2,"pendingLevel":2,"pendingTicks":0,"unknown":false},"sample":{"sampledAt":"2026-10-04T00:00:00Z","source":"test","loadAverage":1,"cpus":1,"swapTotalBytes":0,"swapUsedBytes":0,"loadKnown":true,"cpuKnown":true,"swapKnown":true},"held":[{"role":"impl","key":"ticket:a:q:t1","ticket":"ticket:a:q:t1"}]}`
	if err := write(valid); err != nil {
		t.Fatalf("valid record refused: %v", err)
	}
	for name, bad := range map[string]string{
		"level":      strings.Replace(valid, `"level":2`, `"level":3`, 1),
		"pending":    strings.Replace(valid, `"pendingTicks":0`, `"pendingTicks":-1`, 1),
		"duplicate":  strings.Replace(valid, `}]}`, `},{"role":"impl","key":"ticket:a:q:t1"}]}`, 1),
		"bad role":   strings.Replace(valid, `"role":"impl"`, `"role":"Impl"`, 1),
		"unknown":    strings.Replace(valid, `"held":`, `"extra":1,"held":`, 1),
		"duplicated": strings.Replace(valid, `"held":`, `"state":{},"held":`, 1),
		"problems":   strings.Replace(valid, `"swapKnown":true`, `"swapKnown":true,"problems":["`+strings.Repeat("x", 201)+`"]`, 1),
	} {
		if err := write(bad); err == nil {
			t.Errorf("%s: invalid pressure record accepted", name)
		}
	}
}
