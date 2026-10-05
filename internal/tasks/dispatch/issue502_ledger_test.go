//go:build darwin || linux

package dispatch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502Reload loads the saved ledger and requires the recorded hold.
func issue502Reload(t *testing.T, d *Dispatcher, id string, want []string) {
	t.Helper()
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil {
		t.Fatal("saved ledger refused by its own reader:", err)
	}
	if got := l.Seen.Escalations[id]; !reflect.DeepEqual(got, want) {
		t.Fatalf("seen.escalations[%s] = %v, want %v", id, got, want)
	}
}

// issue502Restart closes d and reopens it on the same state directory, as a
// dispatcher restart does, then ticks once.
func issue502Restart(t *testing.T, d *Dispatcher) *Dispatcher {
	t.Helper()
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := Open(d.Program, d.Config, d.Queue, io.Discard)
	if err != nil {
		t.Fatal("restart refused its own ledger:", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	return r
}

// ESC-V0-006: a held ticket's seen.escalations survives save, load and
// restart beside progress history, which switches LoadLedger to its strict
// member allowlist.
func TestIssue502_LedgerKeepsEscalationsBesideProgress(t *testing.T) {
	d, q, source := progressDispatcher(t)
	writeProgress(t, source, "A")
	q.obs.Tickets[0].EscalationPending = []string{"q-a", "q-b"}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil || !strings.Contains(string(raw), `"progress"`) || !strings.Contains(string(raw), `"escalations"`) {
		t.Fatalf("fixture did not persist progress and escalations: %v\n%s", err, raw)
	}
	issue502Reload(t, d, "ticket:a:q:t", []string{"q-a", "q-b"})
	r := issue502Restart(t, d)
	issue502Reload(t, r, "ticket:a:q:t", []string{"q-a", "q-b"})
	q.obs.Tickets[0].EscalationPending = nil
	if err := r.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(r.dir, r.Program)
	if err != nil || len(l.Seen.Escalations) != 0 || l.Progress["ticket:a:q:t"] == nil {
		t.Fatalf("answered hold not dropped or progress lost: %+v %v", l, err)
	}
}

// The same with a recorded pool-sweep operation in the ledger.
func TestIssue502_LedgerKeepsEscalationsBesidePoolSweeps(t *testing.T) {
	d, q := psrDispatch(t, func(context.Context, PoolSweepRequest) (PoolSweepResult, error) {
		return PoolSweepResult{Outcome: "COMPLETED"}, nil
	})
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	psrCall(t, q)
	psrCollect(t, d)
	held := ticket("h", "P1", 1)
	held.EscalationPending = []string{"q-a"}
	q.obs.Tickets = []Ticket{held}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.PoolSweeps) == 0 {
		t.Fatal("fixture has no pool sweep record")
	}
	issue502Reload(t, d, "ticket:a:q:h", []string{"q-a"})
	r := issue502Restart(t, d)
	issue502Reload(t, r, "ticket:a:q:h", []string{"q-a"})
	if len(r.ledger.Workers) != 0 {
		t.Fatal("held ticket launched after restart")
	}
}

// The strict reader admits only the canonical member and the shape diff
// writes; it refuses aliases, duplicates and malformed holds without
// rewriting the file.
func TestIssue502_LedgerRefusesMalformedEscalations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	a := progressDigest("A")
	l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
		Progress: map[string]*ProgressHistory{"ticket:a:q:t": {Current: a, Seen: []string{a}}},
		Seen:     &Seen{Tickets: map[string]string{}, Claims: map[string]string{}, Lanes: map[string]string{}, Escalations: map[string][]string{"ticket:a:q:t": {"q-a", "q-b"}}}}
	good, err := ledgerBytes(l)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(good)
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(dir, "prog"); err != nil {
		t.Fatal("valid ledger refused:", err)
	}
	for name, bad := range map[string]string{
		"alias":     strings.Replace(valid, `"escalations":`, `"Escalations":`, 1),
		"duplicate": strings.Replace(valid, `"escalations": {`, `"escalations": {}, "escalations": {`, 1),
		"unsorted":  strings.Replace(valid, `"q-a",`, `"q-c",`, 1),
		"repeated":  strings.Replace(valid, `"q-b"`, `"q-a"`, 1),
		"empty":     strings.Replace(strings.Replace(valid, `"q-a",`, ``, 1), `"q-b"`, ``, 1),
		"bad-id":    strings.Replace(valid, `"q-b"`, `"`+strings.Repeat("x", wire.MaxIdentifierBytes+1)+`"`, 1),
		"bad-key":   strings.Replace(valid, `"ticket:a:q:t": [`, `"t": [`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bad == valid {
				t.Fatal("malformed fixture not reached")
			}
			if err := os.WriteFile(path, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLedger(dir, "prog"); err == nil {
				t.Fatal("malformed escalations admitted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != bad {
				t.Fatal("refusal rewrote the ledger")
			}
		})
	}
}
