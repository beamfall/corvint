//go:build darwin || linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// newSealedResumeFixture is newResumeFixture over a segmented store: a
// segmentHistory-receipt history with receipts 1..512 sealed, then one
// complete-route write that retained the writer checkpoint.
func newSealedResumeFixture(t *testing.T) resumeFixture {
	t.Helper()
	repo := historyStore(t, segmentHistory)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if segmentThrough(t, repo) == 0 {
		t.Fatal("history was not sealed")
	}
	lowerWriterCheckpoint(t, repo)
	if run := writerMutate(t, repo, "sealed-writer-seed", nil); !run.completed() || run.fast() {
		t.Fatalf("seed: %+v %v %v", run.rep, run.err, run.stages)
	}
	return resumeFixtureOn(t, repo)
}

// JSG-V0-011, V1-2202: on a segmented store a widen or reap whose every
// complete preparation audit is overtaken by a dispatcher write resumes
// from its first audit under the writer lock, as on an unsegmented store
// (PSR-V0-052), instead of rerunning the complete audit in every round and
// ending SNAPSHOT_MOVED.
func TestJSGV0011_SealedStoreWidenAndReapResume(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	t.Run("widen", func(t *testing.T) {
		f := newSealedResumeFixture(t)
		start := time.Now()
		run := moving(t, f, f.lease("sealed-widen", transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Scope: []string{"docs/"}}), -1, nil)
		t.Logf("widen: %v, %d audits, %d resumed", time.Since(start), run.audits, run.timing.Resumed)
		if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 {
			t.Fatalf("widen under a moving sealed journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
		}
	})
	t.Run("reap", func(t *testing.T) {
		f := newSealedResumeFixture(t)
		at, err := time.Parse(time.RFC3339, string(f.now))
		if err != nil {
			t.Fatal(err)
		}
		f.now = wire.Timestamp(at.Add(61 * time.Minute).UTC().Format(time.RFC3339))
		start := time.Now()
		run := moving(t, f, f.lease("sealed-reap", transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation}), -1, nil)
		t.Logf("reap: %v, %d audits, %d resumed", time.Since(start), run.audits, run.timing.Resumed)
		if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 || run.rep.AttemptID != f.claim.AttemptID {
			t.Fatalf("reap under a moving sealed journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
		}
	})
}

// JSG-V0-011: a sealed segment rewritten in place, its modification time
// restored, after the base audit, after the resume precheck re-hashed it
// before the gate (PSR-V0-225), or while the resumed write observes under
// the writer lock, is never committed over. The resume declines and the
// command keeps the complete route's own decision for that journal.
func TestJSGV0011_ResumeDeclinesRewrittenSegment(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	first := snapshot.SegmentName(1, snapshot.SegmentReceipts)
	for _, stage := range []string{"base", "fast.resume.precheck", "fast.lease.observe"} {
		t.Run(stage, func(t *testing.T) {
			f := newSealedResumeFixture(t)
			var tampered uint64
			change := func() {
				rewriteSegmentKeepingStamps(t, filepath.Join(f.repo.StateDir, filepath.FromSlash(first)))
				tampered = headSeqOf(t, f.repo)
			}
			var run movingRun
			if stage == "base" {
				run = moving(t, f, f.acquire("segment-acquire"), 1, func(int) { change() })
			} else {
				run = movingStaged(t, f, f.acquire("segment-acquire"), 1, nil, at(stage, change))
			}
			if tampered == 0 {
				t.Fatalf("the change never ran: %+v %v %v", run.rep, run.err, run.stages)
			}
			if run.completed() || run.fast() || run.timing.Resumed != 0 {
				t.Fatalf("acquire over a rewritten segment: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
			}
			if got := headSeqOf(t, f.repo); got != tampered {
				t.Fatalf("head moved from %d to %d over a rewritten segment", tampered, got)
			}
			oracle := stagedWrite(f.acquire("segment-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// rewriteSegmentKeepingStamps flips one byte of segment p in place, keeping
// its file identity, size and modification time.
func rewriteSegmentKeepingStamps(t *testing.T, p string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	fh, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 1)
	at := info.Size() / 2
	if _, err := fh.ReadAt(b, at); err != nil {
		t.Fatal(err)
	}
	b[0] ^= 1
	if _, err := fh.WriteAt(b, at); err != nil {
		t.Fatal(err)
	}
	if err := fh.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}
