//go:build darwin || linux

package journal

import (
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// orphanSetup is setup with one committed ticket and the descriptor-less
// slots a writer killed mid-publish leaves (V1-2506), beside an existing,
// unheld writer lock file.
func orphanSetup(t *testing.T, slots ...string) (*fixture.Repo, Reader) {
	t.Helper()
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): fixture.Ticket("A").Encode()}, "", true, true, false)
	for _, s := range slots {
		fixture.Write(t, filepath.Join(repo.StateDir, "staging", s), []byte("x"))
	}
	if err := os.WriteFile(filepath.Join(repo.CommonDir, writerLockFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, r
}

func spent(t *testing.T, pauses []time.Duration) {
	t.Helper()
	var total time.Duration
	for _, d := range pauses {
		total += d
	}
	if total != snapshot.StageSlotPatience {
		t.Fatalf("waited %v in %v; want the whole %v first", total, pauses, snapshot.StageSlotPatience)
	}
}

// CTS-V0-026: after the CTS-V0-008 wait is spent under an unchanged head, a
// free writer lock proves the descriptor-less slots a killed writer's
// orphans. The audit sets exactly them aside and reads the store: staging
// is reported present, the orphans are named, a ticket read succeeds, and
// nothing in the store or the lock file changes.
func TestCTSV0026_FreeWriterLockSetsAsideKilledWritersOrphans(t *testing.T) {
	repo, r := orphanSetup(t, "a03", "a00")
	lock := filepath.Join(repo.CommonDir, writerLockFileName)
	before, err := os.Lstat(lock)
	if err != nil {
		t.Fatal(err)
	}
	pauses := stubStageSleep(t, nil)
	res, err := r.Audit()
	if err != nil {
		t.Fatalf("audit beside a killed writer's orphans: %v", err)
	}
	spent(t, *pauses)
	if !res.StagingPresent || !slices.Equal(res.OrphanStageSlots, []string{"staging/a00", "staging/a03"}) {
		t.Fatalf("staging %v orphans %v; want present and both named", res.StagingPresent, res.OrphanStageSlots)
	}
	if _, err := r.Audit(ticketPath("A")); err != nil {
		t.Fatalf("ticket read beside orphans: %v", err)
	}
	if _, err := r.AuditForMutation("request"); err != nil && wire.CodeOf(err) == wire.CodeMalformed {
		t.Fatalf("mutation audit beside orphans: %v", err)
	}
	for _, s := range []string{"a00", "a03"} {
		if _, err := os.Lstat(filepath.Join(repo.StateDir, "staging", s)); err != nil {
			t.Fatalf("read removed orphan %s: %v", s, err)
		}
	}
	after, err := os.Lstat(lock)
	if err != nil || !os.SameFile(before, after) || after.Size() != 0 || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the probe changed the lock file: %v", err)
	}
}

// CTS-V0-026: the grant needs proof. A held writer lock (a live writer), an
// absent lock file, or a lock name that is a symlink proves nothing, so the
// spent wait keeps the CTS-V0-008 MALFORMED.
func TestCTSV0026_UnprovenOrphansStayMalformed(t *testing.T) {
	cases := map[string]func(t *testing.T, lock string){
		"held": func(t *testing.T, lock string) {
			f, err := os.Open(lock)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { f.Close() })
			if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				t.Fatal(err)
			}
		},
		"absent": func(t *testing.T, lock string) {
			if err := os.Remove(lock); err != nil {
				t.Fatal(err)
			}
		},
		"symlink": func(t *testing.T, lock string) {
			target := filepath.Join(t.TempDir(), "elsewhere.lock")
			if err := os.WriteFile(target, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(lock); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, lock); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			repo, r := orphanSetup(t, "a00")
			arrange(t, filepath.Join(repo.CommonDir, writerLockFileName))
			pauses := stubStageSleep(t, nil)
			_, err := r.Audit()
			requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
			spent(t, *pauses)
		})
	}
}

// CTS-V0-026: the grant covers exactly the slots the refused observation
// listed. A writer that stages a new slot after the probe, or replaces a
// granted slot, is live again: the audit refuses the retryable
// SNAPSHOT_MOVED at "staging", never a read that skips its slot.
func TestCTSV0026_SlotsStagedAfterTheProofAreRetryable(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"new slot": func(t *testing.T, dir string) {
			fixture.Write(t, filepath.Join(dir, "a05"), []byte("x"))
		},
		"replaced orphan": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "a00")); err != nil {
				t.Fatal(err)
			}
			fixture.Write(t, filepath.Join(dir, "a00"), []byte("live writer bytes"))
		},
	}
	for name, after := range cases {
		t.Run(name, func(t *testing.T) {
			repo, r := orphanSetup(t, "a00")
			dir := filepath.Join(repo.StateDir, "staging")
			saved := probeWriterLock
			t.Cleanup(func() { probeWriterLock = saved })
			probes := 0
			probeWriterLock = func(common string) bool {
				probes++
				free := saved(common)
				after(t, dir)
				return free
			}
			stubStageSleep(t, nil)
			_, err := r.Audit()
			requireRefusal(t, err, wire.CodeSnapshotMoved, "staging")
			if !wire.RetryOf(wire.CodeOf(err)).Retryable || probes != 1 {
				t.Fatalf("refusal %v after %d probes; want one probe and a retryable refusal", err, probes)
			}
		})
	}
}

// CTS-V0-026: a writer-locked audit keeps refusing an orphan at once (its
// caller removes orphans under the lock), and a source other than a native
// `<common>/taskman` state directory is never probed.
func TestCTSV0026_WriterLockedAndNonNativeAuditsNeverProbe(t *testing.T) {
	saved := probeWriterLock
	t.Cleanup(func() { probeWriterLock = saved })
	probes := 0
	probeWriterLock = func(string) bool { probes++; return true }
	repo, r := orphanSetup(t, "a00")
	r.WriterLocked = true
	pauses := stubStageSleep(t, nil)
	_, err := r.Audit()
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
	if len(*pauses) != 0 || probes != 0 {
		t.Fatalf("writer-locked audit waited %v and probed %d times", *pauses, probes)
	}
	moved := filepath.Join(repo.CommonDir, "elsewhere")
	if err := os.Rename(repo.StateDir, moved); err != nil {
		t.Fatal(err)
	}
	r.WriterLocked = false
	r.Source = Native{StateDir: moved, PrimaryWorktree: repo.Root}
	_, err = r.Audit()
	requireRefusal(t, err, wire.CodeMalformed, "staging/a00")
	if probes != 0 {
		t.Fatalf("a state directory not named taskman was probed %d times", probes)
	}
}
