//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"testing"
	"time"
)

// pressureCount is a sampler that counts its reads and records how many
// workers ran at each read.
type pressureCount struct {
	d       *Dispatcher
	sample  PressureSample
	reads   int
	running []int
}

func (p *pressureCount) read(context.Context, time.Time, pressureWant) PressureSample {
	p.reads++
	p.running = append(p.running, p.d.Running())
	return p.sample
}

// ATR-V0-181 (V1-0924): with pressure limits configured and nothing that
// could launch, consecutive ticks skip both the store observation and the
// pressure sample; once work is eligible the host is sampled before any
// launch, and a raised sample still holds it.
func TestCALV0351_IdlePressureConfigSkipsTheSample(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"go"}}
	p := issue497Config()
	p.LevelCaps = map[string]int{"1": 0, "2": 0}
	c.Pressure = &p
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	q := &witnessQueue{witness: "w1"}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Now = func() time.Time { return clock }
	d.goos = "linux"
	feed := &pressureCount{d: d, sample: issue497Sample(1, .1)}
	d.pressureSampler = feed.read
	tick := func() {
		t.Helper()
		clock = clock.Add(time.Second)
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// The first tick samples: the recorded state is UNKNOWN until then.
	tick()
	if feed.reads != 1 || d.pressureLabel() != AdmissionPressureCalm {
		t.Fatalf("first tick: %d samples, label %s", feed.reads, d.pressureLabel())
	}
	for i := 0; i < 3; i++ {
		tick()
	}
	observes, reads := q.observes, feed.reads
	for i := 0; i < 20; i++ {
		tick()
	}
	if q.observes != observes || feed.reads != reads {
		t.Fatalf("idle ticks with pressure configured: %d observations and %d samples, want 0 and 0", q.observes-observes, feed.reads-reads)
	}
	if feed.reads != 1 {
		t.Fatalf("an idle dispatcher sampled pressure %d times, want only the first tick's", feed.reads)
	}
	if d.idle == nil {
		t.Fatal("the idle gate is not armed")
	}

	// Eligible work: the tick reads the store and samples before launching.
	// The gap never advanced the dwell (TicksToChange 2), so a critical
	// first sample budgets at its pending level and holds the launch while
	// the recorded state keeps its hysteresis.
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1, "go")}
	q.witness = "w2"
	feed.sample = issue497Sample(9, .1)
	tick()
	if st := d.ledger.Pressure.State; q.observes == observes || feed.reads != 2 || d.Running() != 0 || st.Level != 0 || st.PendingLevel != 2 {
		t.Fatalf("eligible work under critical pressure: %d samples, %d running, state %+v", feed.reads, d.Running(), st)
	}
	if len(d.ledger.Pressure.Held) != 1 {
		t.Fatalf("held %+v", d.ledger.Pressure.Held)
	}
	// A raised level samples every tick; calm again, it launches after its
	// sample.
	feed.sample = issue497Sample(1, .1)
	tick()
	if d.Running() != 1 || feed.reads != 3 || feed.running[2] != 0 {
		t.Fatalf("calm launch: %d running, samples %d at running %v", d.Running(), feed.reads, feed.running)
	}
	if d.pressureGap || d.pressureHold {
		t.Fatalf("a resolved sample kept the gap %v or the hold %v", d.pressureGap, d.pressureHold)
	}
}

// idlePressureRig opens a dispatcher whose one ticket is not yet eligible,
// takes the first sample, and idles until the gate is armed.
func idlePressureRig(t *testing.T, p PressureConfig, sampler func(context.Context, time.Time, pressureWant) PressureSample) (*Dispatcher, *witnessQueue, func()) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"go"}}
	c.Pressure = &p
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	q := &witnessQueue{witness: "w1"}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1)}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	d.Now = func() time.Time { return clock }
	d.goos = "linux"
	d.pressureSampler = sampler
	tick := func() {
		t.Helper()
		clock = clock.Add(time.Second)
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		tick()
	}
	if d.idle == nil || d.pressureLabel() != AdmissionPressureCalm {
		t.Fatalf("idle gate %v, label %s", d.idle != nil, d.pressureLabel())
	}
	q.obs.Tickets = []Ticket{ticket("T-1", "P1", 1, "go")}
	q.witness = "w2"
	return d, q, tick
}

// ATR-V0-181: after a gap the raised pending level budgets every sample
// until the transition resolves, not only the first: with a dwell of 3,
// two critical samples after idle both hold the launch.
func TestCALV0351_PostIdleHoldLastsUntilThePendingLevelSettles(t *testing.T) {
	p := issue497Config()
	p.TicksToChange = 3
	p.LevelCaps = map[string]int{"1": 0, "2": 0}
	sample := issue497Sample(1, .1)
	d, _, tick := idlePressureRig(t, p, func(context.Context, time.Time, pressureWant) PressureSample { return sample })
	sample = issue497Sample(9, .1)
	for i := 1; i <= 3; i++ {
		tick()
		if st := d.ledger.Pressure.State; d.Running() != 0 {
			t.Fatalf("critical sample %d after idle launched; state %+v", i, st)
		}
	}
	if st := d.ledger.Pressure.State; st.Level != 2 {
		t.Fatalf("after the dwell: state %+v", st)
	}
	sample = issue497Sample(1, .1)
	for i := 0; i < 6 && d.Running() == 0; i++ {
		tick()
	}
	if d.Running() != 1 || d.pressureHold {
		t.Fatalf("calm again: %d running, hold %v", d.Running(), d.pressureHold)
	}
}

// ATR-V0-181: with the cpu signal selected, the first sample after a gap
// has no interval and is UNKNOWN; it admits nothing, and the next tick
// samples a fresh interval instead of deferring again.
func TestCALV0351_PostIdleCPUSampleWaitsForAFreshInterval(t *testing.T) {
	p := issue497Config()
	p.TicksToChange = 1
	p.LevelCaps = map[string]int{"1": 0, "2": 0}
	p.CalmCPU, p.CPUHigh, p.CPUCritical = .5, .8, .95
	p.Signals = map[string][]string{"linux": {PressureSignalCPU}}
	var busy, total uint64
	frac := uint64(10)
	var wants []pressureWant
	d, _, tick := idlePressureRig(t, p, func(_ context.Context, _ time.Time, w pressureWant) PressureSample {
		wants = append(wants, w)
		busy, total = busy+frac, total+100
		return PressureSample{Source: "linux-proc-host", CPUBusyTicks: busy, CPUTotalTicks: total, CPUTicksKnown: true}
	})
	if !wants[0].cpu || wants[0].load {
		t.Fatalf("the rig selected %+v, want cpu only", wants[0])
	}
	frac = 99
	reads := len(wants)
	tick()
	if d.Running() != 0 || len(wants) != reads+1 || d.ledger.Pressure.Sample.CPUUtilizationKnown || !d.pressureBaseline {
		t.Fatalf("first sample after idle: %d running, %d samples, sample %+v, baseline hold %v", d.Running(), len(wants)-reads, d.ledger.Pressure.Sample, d.pressureBaseline)
	}
	tick()
	if st := d.ledger.Pressure.State; d.Running() != 0 || len(wants) != reads+2 || !d.ledger.Pressure.Sample.CPUUtilizationKnown || st.Level != 2 {
		t.Fatalf("fresh interval on a saturated host: %d running, %d samples, state %+v", d.Running(), len(wants)-reads, st)
	}
	frac = 10
	tick()
	if d.Running() != 1 || d.pressureBaseline {
		t.Fatalf("calm interval: %d running, baseline hold %v", d.Running(), d.pressureBaseline)
	}
}

// ATR-V0-181: a deferred budget refuses its first candidate and records the
// consultation, so nothing launches before a sample.
func TestCALV0351_DeferredBudgetRefuses(t *testing.T) {
	b := &PressureBudget{deferred: true}
	if b.Accept(Assignment{Role: "r", Key: "k"}) || !b.consulted {
		t.Fatalf("deferred budget %+v", b)
	}
}
