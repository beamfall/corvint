//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCALV0218_ParseCPUTime(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"0:00.03":     30 * time.Millisecond,
		"514:42.63":   514*time.Minute + 42630*time.Millisecond, // Darwin keeps counting minutes
		"00:00:07":    7 * time.Second,                          // Linux HH:MM:SS
		"01:02:03":    time.Hour + 2*time.Minute + 3*time.Second,
		"2-01:00:00":  49 * time.Hour,
		"12:34":       12*time.Minute + 34*time.Second,
		"3-0:00:00.5": 72*time.Hour + 500*time.Millisecond,
		"0:02.01":     2010 * time.Millisecond, // exact, no float rounding
	} {
		if got, ok := parseCPUTime(in); !ok || got != want {
			t.Errorf("parseCPUTime(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "7", "x:00", "1:60", "1:2:3:4", "-1:00", "a-00:00:01", "1:-1:00", "0:-1", "0:+1", "0:1e1", "0:"} {
		if _, ok := parseCPUTime(bad); ok {
			t.Errorf("parseCPUTime(%q) parsed", bad)
		}
	}
}

// A non-ignored child is busy only when it is new since the baseline, its
// CPU time is unreadable or went backwards, or it used at least 1% of the
// elapsed wall time; a sleeping helper that merely exists is not (issue 793).
func TestCALV0218_BusyChildNeedsWorkNotPresence(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	w := &Worker{PID: 10, Members: []Proc{{PID: 10, Identity: "leader"}, {PID: 11, Identity: "mcp"}}}
	base := childCPU{at: t0, cpu: map[string]time.Duration{"11/mcp": 2 * time.Second}}
	at := func(cpu time.Duration, known bool, comm string) map[int]proc {
		return map[int]proc{
			10: {pid: 10, comm: "agent", cpu: time.Hour, cpuKnown: true},
			11: {pid: 11, ppid: 10, comm: comm, cpu: cpu, cpuKnown: known},
		}
	}
	later := t0.Add(900 * time.Second)
	cases := []struct {
		name   string
		procs  map[int]proc
		ignore []string
		base   childCPU
		want   bool
	}{
		{"sleeping helper below 1%", at(2*time.Second+2700*time.Millisecond, true, "node"), nil, base, false},
		{"no CPU at all", at(2*time.Second, true, "node"), nil, base, false},
		{"child at 1% or more", at(2*time.Second+9*time.Second, true, "node"), nil, base, true},
		{"unreadable CPU time", at(0, false, "node"), nil, base, true},
		{"CPU time below the baseline", at(time.Second, true, "node"), nil, base, true},
		{"child absent from the baseline", at(2*time.Second, true, "node"), nil, childCPU{at: t0, cpu: map[string]time.Duration{}}, true},
		{"ignored command", at(time.Hour, true, "node"), []string{"node"}, base, false},
	}
	for _, c := range cases {
		if got := busyChild(w, c.procs, c.ignore, c.base, later); got != c.want {
			t.Errorf("%s: busyChild = %v, want %v", c.name, got, c.want)
		}
	}
	// Linux start identities omit the PID: two children started in the same
	// tick keep separate baselines, so neither looks busy while both sleep.
	twins := &Worker{PID: 10, Members: []Proc{{PID: 10, Identity: "leader"}, {PID: 11, Identity: "tick"}, {PID: 12, Identity: "tick"}}}
	procs := map[int]proc{
		10: {pid: 10, comm: "agent", cpuKnown: true},
		11: {pid: 11, comm: "node", cpu: 2 * time.Second, cpuKnown: true},
		12: {pid: 12, comm: "node", cpuKnown: true},
	}
	if b := childSample(twins, procs, nil, t0); busyChild(twins, procs, nil, b, later) {
		t.Error("two sleeping children sharing a start identity counted busy")
	}
	// The leader's own CPU never counts, however much it used.
	solo := &Worker{PID: 10, Members: []Proc{{PID: 10, Identity: "leader"}}}
	if busyChild(solo, at(0, true, "node"), nil, childCPU{at: t0}, later) {
		t.Error("the leader counted as a busy child")
	}
}

// The first observation (a launch or an adoption after a restart) takes a
// baseline and counts each non-ignored child busy, since none is in a
// baseline yet; later activity retakes it, so the CPU share is measured from
// the last activity.
func TestCALV0218_ActiveBaselineFollowsActivity(t *testing.T) {
	defer func(f func(map[int]proc, []int)) { refineChildCPU = f }(refineChildCPU)
	refineChildCPU = func(map[int]proc, []int) {} // keep the synthetic CPU times on Linux
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	w := &Worker{ID: "w", PID: 10, Members: []Proc{{PID: 10, Identity: "leader"}, {PID: 11, Identity: "mcp"}}}
	dir := d.workerDir(w.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	procs := func(cpu time.Duration) map[int]proc {
		return map[int]proc{10: {pid: 10, comm: "agent", cpuKnown: true}, 11: {pid: 11, comm: "node", cpu: cpu, cpuKnown: true}}
	}
	t0 := time.Unix(1_000_000, 0)
	if !d.active(w, procs(time.Second), Host{}, t0) {
		t.Fatal("the first observation did not count its child busy")
	}
	leader := &Worker{ID: "solo", PID: 10, Members: []Proc{{PID: 10, Identity: "leader"}}}
	if d.active(leader, procs(0), Host{}, t0) {
		t.Fatal("the first observation of a lone leader counted as activity")
	}
	if d.active(w, procs(time.Second+100*time.Millisecond), Host{}, t0.Add(60*time.Second)) {
		t.Fatal("0.17% child CPU counted as busy")
	}
	if err := os.WriteFile(filepath.Join(dir, "stdout.log"), []byte("event\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !d.active(w, procs(time.Second+200*time.Millisecond), Host{}, t0.Add(120*time.Second)) {
		t.Fatal("log growth did not count as activity")
	}
	// The baseline moved to t0+120s at 1.2s: 0.5s over 30s is busy, while
	// measured from t0 (0.7s over 150s) it would not be.
	if !d.active(w, procs(1700*time.Millisecond), Host{}, t0.Add(150*time.Second)) {
		t.Fatal("the baseline did not follow the last activity")
	}
}

// Issue 793: a worker whose only child is a long-lived sleeping process is
// stopped IDLE once the idle timeout passes with no log growth.
func TestCALV0218_SleepingChildDoesNotBlockIdleStop(t *testing.T) {
	c := testConfig(t, `sleep 300 & wait`)
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if d.Running() != 1 {
		t.Fatalf("running %d, want 1", d.Running())
	}
	w := d.ledger.Workers[0]
	leader := w.PID
	t.Cleanup(func() { syscall.Kill(-leader, syscall.SIGKILL) }) // Close keeps workers running
	for i := 0; i < 100 && len(w.Members) < 2; i++ {
		time.Sleep(20 * time.Millisecond)
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.Members) < 2 {
		t.Fatalf("tree has %d members, want the leader and its sleeping child", len(w.Members))
	}
	// One more tick so the child is in the baseline.
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	d.Now = func() time.Time { return time.Now().Add(45 * time.Second) } // idle 30s < 45s < wall 60s
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !gone(leader) {
		t.Fatal("the idle worker's leader survived")
	}
	events, _ := ReadEvents(d.dir, 1000)
	idle := false
	for _, e := range events {
		idle = idle || (e.Kind == "killing" && e.Detail["reason"] == "IDLE")
	}
	if !idle {
		t.Fatalf("no IDLE stop: events %v", kinds(t, d))
	}
}

func TestCALV0218_ParseProcStatCPU(t *testing.T) {
	line := "4242 (node (mcp) x) S 1 4242 4242 0 -1 4194560 100 0 0 0 237 13 0 0 20 0 11 0 99 1 2 3\n"
	if got, ok := parseProcStatCPU(line); !ok || got != 2500*time.Millisecond {
		t.Fatalf("parseProcStatCPU = %v, %v; want 2.5s", got, ok)
	}
	for _, bad := range []string{"", "4242 node S 1", "4242 (sh) S 1 2 3", "4242 (sh) S 1 4242 4242 0 -1 0 0 0 0 0 x 13 0"} {
		if _, ok := parseProcStatCPU(bad); ok {
			t.Errorf("parseProcStatCPU(%q) parsed", bad)
		}
	}
}

// Review round 4: every worker's activity is observed before any stop, so a
// stop that blocks for its grace cannot age a later worker's CPU sample.
func TestCALV0218_ActivityObservedBeforeBlockingStops(t *testing.T) {
	c := testConfig(t, `trap '' TERM; sleep 300 & wait`)
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1), ticket("t2", "P1", 2)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if d.Running() != 2 {
		t.Fatalf("running %d, want 2", d.Running())
	}
	for _, w := range d.ledger.Workers {
		pid := w.PID
		t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	}
	// The trap is set before the sleep starts, so a sleeping child proves it.
	for i := 0; i < 100 && (len(d.ledger.Workers[0].Members) < 2 || len(d.ledger.Workers[1].Members) < 2); i++ {
		time.Sleep(20 * time.Millisecond)
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for _, w := range d.ledger.Workers {
		if len(w.Members) < 2 {
			t.Fatalf("worker %s has %d members, want the shell and its sleeping child", w.ID, len(w.Members))
		}
	}
	first := d.ledger.Workers[0]
	first.State, first.KillReason = "KILLING", "WALL"
	defer func(f func(map[int]proc, []int)) { refineChildCPU = f }(refineChildCPU)
	var calls []time.Time
	refineChildCPU = func(procs map[int]proc, pids []int) {
		calls = append(calls, time.Now())
		preciseCPU(procs, pids)
	}
	start := time.Now()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	if took := end.Sub(start); took < 900*time.Millisecond {
		t.Fatalf("the stop took %v; the TERM-ignoring tree should hold it for the 1s grace", took)
	}
	if len(calls) != 2 {
		t.Fatalf("activity observed %d times, want once per worker", len(calls))
	}
	// Measured back from the tick's end, so load before the stop only adds margin.
	for i, at := range calls {
		if end.Sub(at) < 500*time.Millisecond {
			t.Fatalf("worker %d's activity was observed %v before the tick ended, after the blocking stop", i, end.Sub(at))
		}
	}
}
