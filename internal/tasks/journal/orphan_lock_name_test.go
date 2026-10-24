//go:build darwin || linux

package journal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CTS-V0-026: the orphan proof probes the very lock native writers hold. A
// slot held under a real authority.AcquireLock stays MALFORMED after the
// CTS-V0-008 wait; once that writer is gone (its lock released, its slot
// left behind as a SIGKILL leaves it) the same store reads, naming the orphan.
// This pins the journal's lock name to authority.LockFileName.
func TestCTSV0026_ProbeNamesTheWriterLock(t *testing.T) {
	repo := fixture.TempRepo(t)
	fixture.WriteIntent(t, repo)
	fixture.WriteState(t, repo)
	fixture.Write(t, filepath.Join(repo.StateDir, "staging", "a00"), []byte("x"))
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	r := journal.Reader{Source: journal.Native{StateDir: repo.StateDir, PrimaryWorktree: repo.Root}, QueueID: q, PrimaryWorktree: repo.Root}
	resolved, err := intent.Resolve(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.LockPath != filepath.Join(repo.CommonDir, authority.LockFileName) {
		t.Fatalf("lock path %s", resolved.LockPath)
	}
	lock, err := authority.AcquireLock(context.Background(), resolved, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Audit()
	if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeMalformed || e.Where != "staging/a00" {
		t.Fatalf("audit under a held writer lock: %v; want MALFORMED at staging/a00", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	res, err := r.Audit()
	if err != nil {
		t.Fatalf("audit after the writer is gone: %v", err)
	}
	if !res.StagingPresent || len(res.OrphanStageSlots) != 1 || res.OrphanStageSlots[0] != "staging/a00" {
		t.Fatalf("staging %v orphans %v", res.StagingPresent, res.OrphanStageSlots)
	}
}
