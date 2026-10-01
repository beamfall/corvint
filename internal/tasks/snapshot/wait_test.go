package snapshot_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCTSV0006_ReadWaitsForInFlightWriter: a probe that lands inside a
// writer's receipt-to-head window (REDO_PENDING) pauses with backoff and
// probes again; when the writer finishes during the wait the read succeeds
// on the new head with the body run once, writing nothing.
func TestCTSV0006_ReadWaitsForInFlightWriter(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.PlantReceipt(t, r, 2)
	before := fixture.TreeSnapshot(t, r.StateDir)
	var pauses []time.Duration
	pending := uint64(2)
	rd := snapshot.Reader{StateDir: r.StateDir, Patience: time.Second, Sleep: func(d time.Duration) {
		pauses = append(pauses, d)
		if len(pauses) == 2 {
			fixture.ApplyReceipt(t, r, pending)
		}
	}}
	bodies := 0
	s, err := rd.Read(func(s *snapshot.Snapshot) error { bodies++; return nil })
	if err != nil || bodies != 1 || s.Head.LastSeq != "2" {
		t.Fatalf("read after writer finished: err=%v bodies=%d head=%+v", err, bodies, s.Head)
	}
	if len(pauses) != 2 || pauses[0] != 25*time.Millisecond || pauses[1] != 50*time.Millisecond {
		t.Errorf("pauses %v, want 25ms then 50ms", pauses)
	}
	after := fixture.TreeSnapshot(t, r.StateDir)
	// Only the head the simulated writer applied may differ.
	for _, e := range before {
		if e.Path == "/head.json" {
			continue
		}
		found := false
		for _, a := range after {
			if a == e {
				found = true
			}
		}
		if !found {
			t.Errorf("read changed %s", e.Path)
		}
	}
	// A pending redo seen by the re-probe (the writer committed during the
	// body) is waited out the same way.
	pauses = nil
	bodies = 0
	pending = 3
	s, err = rd.Read(func(s *snapshot.Snapshot) error {
		bodies++
		if bodies == 1 {
			fixture.PlantReceipt(t, r, 3)
		}
		return nil
	})
	if err != nil || bodies != 2 || s.Head.LastSeq != "3" || len(pauses) != 2 {
		t.Fatalf("pending re-probe: err=%v bodies=%d pauses=%v", err, bodies, pauses)
	}
}

// TestCTSV0006_ReadReportsPendingAfterPatience: a writer that never leaves
// the window (crashed, or slower than the budget) is still reported as
// REDO_PENDING once the patience is spent, naming the wait, with the pauses
// clipped to the budget; NoPatience reports it at once and verbatim.
func TestCTSV0006_ReadReportsPendingAfterPatience(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	fixture.PlantReceipt(t, r, 2)
	before := fixture.TreeSnapshot(t, r.StateDir)
	var total time.Duration
	pauses := 0
	rd := snapshot.Reader{StateDir: r.StateDir, Patience: 120 * time.Millisecond, Sleep: func(d time.Duration) {
		pauses++
		total += d
		time.Sleep(d)
	}}
	bodies := 0
	s, err := rd.Read(func(*snapshot.Snapshot) error { bodies++; return nil })
	if code(err) != wire.CodeRedoPending || bodies != 0 || s == nil || s.Head == nil || s.Head.LastSeq != "1" {
		t.Fatalf("pending after patience: err=%v bodies=%d snap=%+v", err, bodies, s)
	}
	if pauses == 0 || total > 120*time.Millisecond || !strings.Contains(err.Error(), "retryable") || !strings.Contains(err.Error(), "000000000002.json") {
		t.Errorf("pauses=%d total=%s err=%v", pauses, total, err)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.StateDir)) {
		t.Error("waiting read changed the store")
	}
	rd = snapshot.Reader{StateDir: r.StateDir, Patience: snapshot.NoPatience, Sleep: func(time.Duration) { t.Error("NoPatience must not sleep") }}
	_, err = rd.Read(func(*snapshot.Snapshot) error { bodies++; return nil })
	if code(err) != wire.CodeRedoPending || bodies != 0 || strings.Contains(err.Error(), "retryable") {
		t.Errorf("NoPatience: %v", err)
	}
}

// TestCTSV0006_MovedReadsPauseBetweenAttempts: a moved snapshot is paused
// on and re-read while the patience lasts; once it is spent the TM-V0-008
// retries run unpaused and the final SNAPSHOT_MOVED names the wait and the
// attempt count. A store that settles after one pause reads on the new
// snapshot.
func TestCTSV0006_MovedReadsPauseBetweenAttempts(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	var pauses []time.Duration
	rd := snapshot.Reader{StateDir: r.StateDir, Patience: 500 * time.Millisecond, Sleep: func(d time.Duration) {
		pauses = append(pauses, d)
		time.Sleep(d)
	}}
	bodies := 0
	_, err := rd.Read(func(*snapshot.Snapshot) error {
		bodies++
		fixture.Commit(t, r, "MUTATION")
		return nil
	})
	// At least one paused attempt inside the budget, then the four unpaused
	// TM-V0-008 attempts.
	if code(err) != wire.CodeSnapshotMoved || bodies < 5 || len(pauses) < 1 || pauses[0] != 25*time.Millisecond || !strings.Contains(err.Error(), "retryable") || !strings.Contains(err.Error(), "read attempts") {
		t.Errorf("err=%v bodies=%d pauses=%v", err, bodies, pauses)
	}
	// Every moved re-read pauses for the first backoff step (the last one
	// clipped to the remaining budget): a successful probe resets the stall,
	// so moved re-reads never escalate.
	for i, p := range pauses {
		if p > 25*time.Millisecond || (p < 25*time.Millisecond && i != len(pauses)-1) {
			t.Errorf("moved re-read pause %d is %s, want 25ms", i, p)
		}
	}
	// A moved read that settles after one pause succeeds on the new snapshot.
	// The budget is generous here: a slow `git commit` inside the body must
	// not spend it before the first pause.
	before, err := snapshot.Probe(r.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	rd.Patience = 10 * time.Second
	pauses = nil
	bodies = 0
	s, err := rd.Read(func(*snapshot.Snapshot) error {
		bodies++
		if bodies == 1 {
			fixture.Commit(t, r, "MUTATION")
		}
		return nil
	})
	if err != nil || bodies != 2 || len(pauses) != 1 || s.Head.LastSeq.Uint64() != before.Head.LastSeq.Uint64()+1 {
		t.Errorf("settled: err=%v bodies=%d pauses=%v", err, bodies, pauses)
	}
}
