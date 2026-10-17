//go:build darwin || linux

package dispatch

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// drainingControl is a fakeControl that also reports a drain (ATR-V0-058).
type drainingControl struct {
	*fakeControl
	draining bool
	err      error
}

func (c *drainingControl) Draining() (bool, error) { return c.draining, c.err }

// ATR-V0-058: a worker launched before the drain reads the drain-pending
// signal through CORVINT_DISPATCH_DRAIN_FILE; the signal is written once,
// kept through a failed observation, and removed when the drain lifts.
func TestATRV0058_DrainSignalReachesLiveWorkerAndClears(t *testing.T) {
	c := testConfig(t, `printf '%s' "$CORVINT_DISPATCH_DRAIN_FILE" > "$CORVINT_DISPATCH_WORKER.path"
i=0; while [ ! -f "$CORVINT_DISPATCH_DRAIN_FILE" ] && [ $i -lt 400 ]; do sleep 0.05; i=$((i+1)); done
cat "$CORVINT_DISPATCH_DRAIN_FILE" > "$CORVINT_DISPATCH_WORKER.seen"`)
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	f := &drainingControl{fakeControl: &fakeControl{admit: true}}
	d, err := OpenControlled("prog", c, q, io.Discard, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("launch: %v running %d", err, d.Running())
	}
	signal := DrainSignalPath(d.dir)
	if _, err := os.Stat(signal); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("signal before the drain: %v", err)
	}
	worker := d.ledger.Workers[0].ID
	path := filepath.Join(c.WorkRoot, worker+".path")
	for i := 0; i < 200; i++ {
		if raw, _ := os.ReadFile(path); len(raw) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if raw, _ := os.ReadFile(path); string(raw) != signal {
		t.Fatalf("worker env names %q, want %q", raw, signal)
	}

	f.draining, f.admit = true, false
	for range 2 {
		if err := d.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, err := ReadDrainSignal(signal)
	if err != nil || s.Program != "prog" || s.Notice != DrainNotice {
		t.Fatalf("signal %+v %v", s, err)
	}
	if n := len(eventsWith(t, d, "alert", "drain", "PENDING")); n != 1 {
		t.Fatalf("%d PENDING events", n)
	}
	waitEnded(t, d)
	seen, err := os.ReadFile(filepath.Join(c.WorkRoot, worker+".seen"))
	if err != nil || !strings.Contains(string(seen), DrainSignalProfile) {
		t.Fatalf("the live worker did not read the signal: %q %v", seen, err)
	}

	before, _ := os.ReadFile(signal)
	f.draining, f.err = false, errors.New("control unreadable")
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(signal); string(after) != string(before) {
		t.Fatal("a failed observation changed the signal")
	}
	f.err = nil
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(signal); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("signal after the drain lifted: %v", err)
	}
	if n := len(eventsWith(t, d, "alert", "drain", "LIFTED")); n != 1 {
		t.Fatalf("%d LIFTED events", n)
	}
}

// ATR-V0-058: an uncontrolled dispatcher never drains, so a leftover
// signal is removed on its first tick.
func TestATRV0058_UncontrolledDispatcherClearsLeftoverSignal(t *testing.T) {
	c := testConfig(t, "exit 0")
	d, err := Open("prog", c, &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	signal := DrainSignalPath(d.dir)
	if err := os.WriteFile(signal, []byte(`{"profile":"taskman-dispatch-drain-pending/0","program":"prog","since":"2026-10-17T00:00:00Z","notice":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(signal); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("leftover signal kept: %v", err)
	}
}
