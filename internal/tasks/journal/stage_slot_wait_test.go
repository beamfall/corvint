//go:build darwin || linux

package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// stubStageSleep replaces the stage wait's sleep for one test, recording
// each pause and running then (if set) in place of sleeping.
func stubStageSleep(t *testing.T, then func(n int)) *[]time.Duration {
	t.Helper()
	pauses := &[]time.Duration{}
	saved := stageSleep
	stageSleep = func(d time.Duration) {
		*pauses = append(*pauses, d)
		if then != nil {
			then(len(*pauses))
		}
	}
	t.Cleanup(func() { stageSleep = saved })
	return pauses
}

// CTS-V0-008: a native writer starts its publish while an unlocked audit is
// reading. Its descriptor-less slot appears between the first attempt's two
// captures (SNAPSHOT_MOVED), stays put across the second attempt's captures
// as it does while the writer fsyncs or keeps a linked slot, and is removed by
// the writer's deferred RemoveStage. The audit waits for it and succeeds; it
// does not return the MALFORMED "unassigned stage slot" of V1-0825. The
// interleaving is driven by the capture and sleep hooks, not by timing.
func TestCTSV0008_AuditWaitsForWriterHeldStageSlot(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	slot := filepath.Join(repo.StateDir, "staging", "a00")
	captures := 0
	r.afterCapture = func() {
		if captures++; captures == 1 {
			fixture.Write(t, slot, fixture.Ticket("A").Encode())
		}
	}
	pauses := stubStageSleep(t, func(int) {
		if e := os.Remove(slot); e != nil {
			t.Fatal(e)
		}
	})
	if _, err := r.Audit(); err != nil {
		t.Fatalf("audit beside a writer-held slot: %v", err)
	}
	if len(*pauses) != 1 || (*pauses)[0] != snapshot.StageSlotPause {
		t.Fatalf("pauses %v; want one of %v", *pauses, snapshot.StageSlotPause)
	}
	if captures != 3 {
		t.Fatalf("attempts %d; want moved, waited, settled", captures)
	}
}

// CTS-V0-008: the wait is bounded. A killed writer's orphan slot, which only
// the next writer removes (CAL-V0-019), still earns the same MALFORMED at the
// same path once snapshot.StageSlotPatience is spent, as a plain *wire.Error.
func TestCTSV0008_OrphanStageSlotStillMalformedAfterBoundedWait(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	for _, s := range []string{"a03", "a00"} {
		fixture.Write(t, filepath.Join(repo.StateDir, "staging", s), []byte("x"))
	}
	pauses := stubStageSleep(t, nil)
	_, err := r.Audit()
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
	if err.Error() != wire.Errorf(wire.CodeMalformed, "staging/a00", "unassigned stage slot").Error() {
		t.Fatalf("refusal changed: %v", err)
	}
	var total time.Duration
	for i, d := range *pauses {
		if d <= 0 || d > snapshot.StageSlotPauseCeiling || (i > 0 && d > 2*(*pauses)[i-1]) {
			t.Fatalf("pause %d of %v outside the backoff", i, *pauses)
		}
		total += d
	}
	if total != snapshot.StageSlotPatience || len(*pauses) > 16 {
		t.Fatalf("waited %v in %d pauses; want exactly %v", total, len(*pauses), snapshot.StageSlotPatience)
	}
	_, err = r.Audit(ticketPath("A"))
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
}

// CTS-V0-008: only the descriptor-less shape is waited for. A slot beside a
// stage descriptor or a descriptor temp, which no native writer publishes,
// is refused at once.
func TestCTSV0008_UnassignedSlotBesideDescriptorRefusedAtOnce(t *testing.T) {
	for _, name := range []string{"active.json", "active.json.tmp"} {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
			dir := filepath.Join(repo.StateDir, "staging")
			fixture.Write(t, filepath.Join(dir, name), stageDescriptor(t, repo, false))
			fixture.Write(t, filepath.Join(dir, "a07"), []byte("x"))
			pauses := stubStageSleep(t, nil)
			_, err := r.Audit()
			requireRefusal(t, err, wire.CodeMalformed, "staging/a07")
			if len(*pauses) != 0 {
				t.Fatalf("waited %v", *pauses)
			}
		})
	}
}

// CTS-V0-008: an audit whose caller holds the writer lock (reconcile, barrier,
// policy, release, import, mutate and redo in the store) knows no writer can
// be publishing, so it refuses an orphan slot at once, as before the wait,
// rather than holding the lock for the whole budget. A moved first
// observation is still retried; the stable orphan then makes no pause.
func TestCTSV0008_WriterLockedAuditRefusesOrphanSlotWithoutPause(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	dir := filepath.Join(repo.StateDir, "staging")
	fixture.Write(t, filepath.Join(dir, "a00"), []byte("x"))
	r.WriterLocked = true
	pauses := stubStageSleep(t, nil)
	audits := map[string]func() error{
		"Audit":            func() error { _, err := r.Audit(); return err },
		"RequestIndex":     func() error { _, _, err := (&RequestIndex{Reader: r}).Lookup("request"); return err },
		"AuditForMutation": func() error { _, err := r.AuditForMutation("request"); return err },
	}
	for name, audit := range audits {
		t.Run(name, func(t *testing.T) {
			requireRefusal(t, audit(), wire.CodeMalformed, "staging/a00")
		})
	}
	captures := 0
	r.afterCapture = func() {
		if captures++; captures == 1 {
			fixture.Write(t, filepath.Join(dir, "a01"), []byte("x"))
		}
	}
	_, err := r.Audit()
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
	if captures != 2 {
		t.Fatalf("attempts %d; want moved then refused", captures)
	}
	if len(*pauses) != 0 {
		t.Fatalf("writer-locked audit waited %v", *pauses)
	}
}

// CTS-V0-008: the wait and the four SNAPSHOT_MOVED attempts are separate
// bounds that compose. A pause never spends an attempt, a move never resets
// or extends the wait, a move after the budget is spent costs no further
// pause, and four moves in all, before or after the budget is spent, still
// end in SNAPSHOT_MOVED.
func TestCTSV0008_MovedObservationsAndSpentWaitCompose(t *testing.T) {
	cases := map[string]struct {
		// move reports whether this capture's observation moves, given
		// the moves already made after the wait was spent.
		move       func(capture, after int, spent bool) bool
		code, path string
		after      int // moves made after the wait is spent
	}{
		"moves while waiting": {
			move: func(c, _ int, _ bool) bool { return c == 2 || c == 4 },
			code: wire.CodeMalformed, path: "staging/a00", after: 0,
		},
		"move after the budget is spent": {
			move: func(_, after int, spent bool) bool { return spent && after == 0 },
			code: wire.CodeMalformed, path: "staging/a00", after: 1,
		},
		"three moves waiting, one after": {
			move: func(c, _ int, spent bool) bool { return c == 2 || c == 3 || c == 4 || spent },
			code: wire.CodeSnapshotMoved, path: "/", after: 1,
		},
		"four moves after the budget is spent": {
			move: func(_, _ int, spent bool) bool { return spent },
			code: wire.CodeSnapshotMoved, path: "/", after: 4,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
			dir := filepath.Join(repo.StateDir, "staging")
			fixture.Write(t, filepath.Join(dir, "a00"), []byte("x"))
			pauses := stubStageSleep(t, nil)
			waited := func() (total time.Duration) {
				for _, d := range *pauses {
					total += d
				}
				return total
			}
			captures, moves, movedAfterSpent := 0, 0, 0
			r.afterCapture = func() {
				captures++
				spent := waited() == snapshot.StageSlotPatience
				if !tc.move(captures, movedAfterSpent, spent) {
					return
				}
				moves++
				if spent {
					movedAfterSpent++
				}
				fixture.Write(t, filepath.Join(dir, fmt.Sprintf("a%02d", moves)), []byte("x"))
			}
			_, err := r.Audit()
			requireRefusal(t, err, tc.code, tc.path)
			if waited() != snapshot.StageSlotPatience {
				t.Fatalf("waited %v in %v; want exactly %v", waited(), *pauses, snapshot.StageSlotPatience)
			}
			for i, d := range *pauses {
				if d <= 0 || d > snapshot.StageSlotPauseCeiling || (i > 0 && d > 2*(*pauses)[i-1]) {
					t.Fatalf("pause %d of %v outside the backoff", i, *pauses)
				}
			}
			if movedAfterSpent != tc.after {
				t.Fatalf("%d moves after the wait was spent; want %d", movedAfterSpent, tc.after)
			}
			want := len(*pauses) + moves
			if tc.code == wire.CodeMalformed {
				want++ // the final stable observation that returns the refusal
				if moves >= 4 {
					t.Fatalf("%d moves yet refused MALFORMED", moves)
				}
			} else if moves != 4 {
				t.Fatalf("SNAPSHOT_MOVED after %d moves; want 4", moves)
			}
			if captures != want {
				t.Fatalf("attempts %d; want %d (%d pauses, %d moves)", captures, want, len(*pauses), moves)
			}
		})
	}
}
