package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// These tests replace the seal's parallel unlink and directory sync, which
// are package-wide, so none of them runs in parallel.

const faultHistory = 3*snapshot.SegmentReceipts + 30

func markerPresent(t *testing.T, repo *intent.Repository) bool {
	t.Helper()
	on, err := present(segmentPath(repo, snapshot.SealMarkerPath))
	if err != nil {
		t.Fatal(err)
	}
	return on
}

// sealedClean requires the store fully sealed through want with no marker,
// no sealed physical receipt left, and a complete audit that agrees.
func sealedClean(t *testing.T, repo *intent.Repository, want uint64, label string) {
	t.Helper()
	if markerPresent(t, repo) {
		t.Fatalf("%s: seal marker left behind", label)
	}
	if got := segmentThrough(t, repo); got != want {
		t.Fatalf("%s: sealed through %d, want %d", label, got, want)
	}
	for seq := uint64(1); seq <= want; seq++ {
		if receiptLinked(t, repo, seq) {
			t.Fatalf("%s: sealed receipt %d still linked", label, seq)
		}
	}
	res, err := segmentAudit(repo)
	if err != nil || res.StructuralConsistency != "CONSISTENT" || res.ProjectionAgreement != "AGREES" || res.LastSeq != wire.SizeOf(faultHistory) {
		t.Fatalf("%s: audit %v %v", label, res, err)
	}
}

// referenceThrough is how far a clean seal of a faultHistory store seals.
func referenceThrough(t *testing.T) uint64 {
	t.Helper()
	ref := historyStore(t, faultHistory)
	if _, err := SealJournal(context.Background(), ref); err != nil {
		t.Fatal(err)
	}
	return segmentThrough(t, ref)
}

func replaceSealIO(unlink func(string) error, sync func(*intent.Repository, string) error) (restore func()) {
	oldUnlink, oldSync := sealUnlink, sealSyncDir
	if unlink != nil {
		sealUnlink = unlink
	}
	if sync != nil {
		sealSyncDir = sync
	}
	return func() { sealUnlink, sealSyncDir = oldUnlink, oldSync }
}

// LGL-V0-814: when one worker's directory sync fails, the seal fails with
// that error, the published segment's marker stays for the next writer to
// settle (the immediate settle syncs through the same failing directory),
// and the next seal settles it and finishes as a clean seal would.
func TestLGLV0814_FailedDirectorySyncKeepsTheSealRecoverable(t *testing.T) {
	want := referenceThrough(t)
	repo := historyStore(t, faultHistory)
	var target atomic.Pointer[string]
	restore := replaceSealIO(nil, func(r *intent.Repository, dir string) error {
		if strings.HasPrefix(dir, "requests/") {
			target.CompareAndSwap(nil, &dir)
			if *target.Load() == dir {
				return &os.PathError{Op: "fsync", Path: dir, Err: syscall.EIO}
			}
		}
		return syncDir(r, dir)
	})
	_, err := SealJournal(context.Background(), repo)
	restore()
	if !errors.Is(err, syscall.EIO) {
		t.Fatalf("seal with a failing directory sync: %v", err)
	}
	if !markerPresent(t, repo) {
		t.Fatal("the failed seal removed its marker before every directory was synced")
	}
	if _, err := segmentAudit(repo); wire.CodeOf(err) != wire.CodeRedoPending {
		t.Fatalf("read of the unsettled seal: %v, want %s", err, wire.CodeRedoPending)
	}
	if _, err := SealJournal(context.Background(), repo); err != nil {
		t.Fatalf("the next seal did not settle and finish: %v", err)
	}
	sealedClean(t, repo, want, "after a failed directory sync")
}

// LGL-V0-814: when one worker's unlink fails while the others succeed, the
// seal fails with that error and leaves a partial drop under its marker; the
// next seal settles it (removing the sealed files still present only after
// checking their bytes) and finishes as a clean seal would.
func TestLGLV0814_PartialParallelUnlinkRecovers(t *testing.T) {
	want := referenceThrough(t)
	repo := historyStore(t, faultHistory)
	stuck := filepath.Join(repo.StateDir, receiptPath(100))
	var removed atomic.Int64
	restore := replaceSealIO(func(path string) error {
		if path == stuck {
			return &os.PathError{Op: "unlink", Path: path, Err: syscall.EIO}
		}
		removed.Add(1)
		return os.Remove(path)
	}, nil)
	_, err := SealJournal(context.Background(), repo)
	restore()
	if !errors.Is(err, syscall.EIO) {
		t.Fatalf("seal with a failing unlink: %v", err)
	}
	if !receiptLinked(t, repo, 100) || removed.Load() == 0 || receiptLinked(t, repo, 1) {
		t.Fatalf("want a partial drop: receipt 100 kept, receipt 1 removed (%d unlinks)", removed.Load())
	}
	if !markerPresent(t, repo) {
		t.Fatal("the failed seal removed its marker over a partial drop")
	}
	if _, err := SealJournal(context.Background(), repo); err != nil {
		t.Fatalf("the next seal did not settle the partial drop: %v", err)
	}
	sealedClean(t, repo, want, "after a partial unlink")
}

// LGL-V0-814: a seal removes its marker, and returns, only after every
// parallel unlink and directory sync has returned: a deliberately slow
// worker still sees the marker when it finishes.
func TestLGLV0814_MarkerRemovedOnlyAfterEveryWorker(t *testing.T) {
	want := referenceThrough(t)
	repo := historyStore(t, faultHistory)
	var mu sync.Mutex
	var violations []string
	var syncs, unlinks atomic.Int64
	slowDir, slowFile := "receipts", filepath.Join(repo.StateDir, receiptPath(7))
	check := func(what string) {
		if on, err := present(segmentPath(repo, snapshot.SealMarkerPath)); err != nil || !on {
			mu.Lock()
			violations = append(violations, what)
			mu.Unlock()
		}
	}
	restore := replaceSealIO(func(path string) error {
		if path == slowFile {
			time.Sleep(50 * time.Millisecond)
		}
		err := os.Remove(path)
		check("after unlink " + path)
		unlinks.Add(1)
		return err
	}, func(r *intent.Repository, dir string) error {
		if dir == slowDir {
			time.Sleep(50 * time.Millisecond)
		}
		err := syncDir(r, dir)
		check("after sync " + dir)
		syncs.Add(1)
		return err
	})
	_, err := SealJournal(context.Background(), repo)
	restore()
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Fatalf("a worker finished after the marker was removed: %v", violations[:min(3, len(violations))])
	}
	if syncs.Load() == 0 || unlinks.Load() == 0 {
		t.Fatalf("seams not exercised: %d syncs, %d unlinks", syncs.Load(), unlinks.Load())
	}
	sealedClean(t, repo, want, "with slow workers")
}
