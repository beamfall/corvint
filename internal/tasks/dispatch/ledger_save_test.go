//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// readOnlyStateDir makes the program's state directory read/execute-only, so
// the ledger's temporary file cannot be created, and returns the restore. It
// proves the fixture by failing CreateTemp itself.
func readOnlyStateDir(t *testing.T, dir string) func() {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind root")
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	done := false
	restore := func() {
		if !done {
			done = true
			if err := os.Chmod(dir, 0o700); err != nil {
				t.Error(err)
			}
		}
	}
	t.Cleanup(restore)
	if f, err := os.CreateTemp(dir, ".probe-*"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal("fixture: CreateTemp succeeded in a read-only state directory")
	}
	return restore
}

func stateBytes(t *testing.T, d *Dispatcher) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// CAL-V0-194: a tick whose final ledger save fails reports ErrLedgerUnsaved
// with the original cause, while its state stays in memory and the event log;
// the next tick saves the current ledger, never an older snapshot.
func TestCALV0194_FailedTickSaveIsReportedAndRetried(t *testing.T) {
	c := testConfig(t, "exit 0")
	c.Roles[0].Match = &Match{Labels: []string{"never"}}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	before := stateBytes(t, d)
	restore := readOnlyStateDir(t, d.dir)
	q.obs.Tickets[0].Status = "IN_PROGRESS"
	err = d.Tick(ctx)
	if !errors.Is(err, ErrLedgerUnsaved) {
		t.Fatalf("tick with an unwritable ledger returned %v, want ErrLedgerUnsaved", err)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("original cause lost: %v", err)
	}
	if d.tickSaved {
		t.Fatal("failed save reported as saved")
	}
	// The difference the failure leaves: emitted in-memory state, unchanged ledger.
	if got := d.ledger.Seen.Tickets["ticket:a:q:t1"]; !strings.HasPrefix(got, "IN_PROGRESS|") {
		t.Fatalf("in-memory state did not advance: %q", got)
	}
	if !bytes.Equal(stateBytes(t, d), before) {
		t.Fatal("state.json changed although its save failed")
	}
	if l, err := LoadLedger(d.dir, d.Program); err != nil || strings.HasPrefix(l.Seen.Tickets["ticket:a:q:t1"], "IN_PROGRESS|") {
		t.Fatalf("persisted ledger claims the unsaved state: %v", err)
	}
	if !has(kinds(t, d), "state") {
		t.Fatalf("state change not published: %v", kinds(t, d))
	}
	// A bounded run surfaces the same failure and records it as an alert.
	q.obs.Tickets[0].Status = "OPEN"
	if err := d.Run(ctx, 1); !errors.Is(err, ErrLedgerUnsaved) {
		t.Fatalf("run returned %v, want ErrLedgerUnsaved", err)
	}
	events, err := ReadEvents(d.dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if last := events[len(events)-1]; last.Kind != "alert" || !strings.Contains(last.Message, ErrLedgerUnsaved.Error()) {
		t.Fatalf("last event %+v, want the unsaved-ledger alert", last)
	}
	if !bytes.Equal(stateBytes(t, d), before) {
		t.Fatal("state.json changed although its save failed")
	}
	// Once the directory is writable again the next tick saves the current ledger.
	restore()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !d.tickSaved {
		t.Fatal("recovered save not reported")
	}
	want, err := ledgerBytes(d.ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stateBytes(t, d), want) {
		t.Fatal("recovered save did not write the current ledger")
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || l.EventSeq != d.ledger.EventSeq || !strings.HasPrefix(l.Seen.Tickets["ticket:a:q:t1"], "OPEN|") {
		t.Fatalf("saved ledger is not current: %v", err)
	}
}

// CAL-V0-194: an unsaved tick keeps supervising the workers it holds; once a
// save succeeds the ledger records them and a restart adopts them.
func TestCALV0194_UnsavedTickKeepsRunningWorkers(t *testing.T) {
	c := testConfig(t, "sleep 300")
	c.GlobalCap = 1
	c.Backoff.CooldownSeconds = 3600
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t1", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	if err := d.Tick(ctx); err != nil || d.Running() != 1 {
		t.Fatalf("launch: %v running %d", err, d.Running())
	}
	w := *d.ledger.Workers[0]
	t.Cleanup(func() { syscall.Kill(-w.PID, syscall.SIGKILL); syscall.Kill(w.PID, syscall.SIGKILL) })
	restore := readOnlyStateDir(t, d.dir)
	// An unchanged ledger is not rewritten (CAL-V0-139), so each tick
	// observes a change that must be saved.
	for i, status := range []string{"IN_PROGRESS", "OPEN"} {
		q.obs.Tickets[0].Status = status
		if err := d.Tick(ctx); !errors.Is(err, ErrLedgerUnsaved) {
			t.Fatalf("tick %d returned %v, want ErrLedgerUnsaved", i, err)
		}
		if d.Running() != 1 || d.ledger.Workers[0].ID != w.ID || d.ledger.Workers[0].State != "RUNNING" {
			t.Fatalf("worker lost or stopped after a failed save: %+v", d.ledger.Workers)
		}
		if id, _ := identityOf(w.PID); id != w.LeaderIdentity {
			t.Fatal("worker process stopped after a failed save")
		}
	}
	if has(kinds(t, d), "killing") {
		t.Fatal("a failed save stopped the worker")
	}
	restore()
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || len(l.Workers) != 1 || l.Workers[0].ID != w.ID {
		t.Fatalf("saved ledger lost the worker: %v %+v", err, l)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if r.Running() != 1 || r.ledger.Workers[0].ID != w.ID {
		t.Fatalf("restart did not adopt the worker: running %d", r.Running())
	}
}
