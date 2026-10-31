//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// newSealedBaseFixture is newSealedResumeFixture run through validated
// seals until the retained writer checkpoint's FullSeq is the sealed
// sequence S: the state a segmented store reaches once its writes take the
// writer route (JSG-V0-010), and the one SealedResumeBase resumes from.
func newSealedBaseFixture(t *testing.T) resumeFixture {
	t.Helper()
	f := newSealedResumeFixture(t)
	fast := func(run writerRun) {
		t.Helper()
		if !run.completed() || !run.fast() {
			t.Fatalf("write: %+v %v %v", run.rep, run.err, run.stages)
		}
	}
	for i := 0; i < 3; i++ {
		if wc := readWriterCheckpoint(f.repo); wc != nil && wc.FullSeq == segmentThrough(t, f.repo) {
			// The padding recorded receipts after the fixture's clock.
			f.now = WallClock()
			return f
		}
		padToSeal(t, f.repo, fast)
		fast(writerMutate(t, f.repo, fmt.Sprintf("sealed-base-%d", i), nil))
	}
	t.Fatalf("FullSeq %+v never reached the sealed sequence %d", readWriterCheckpoint(f.repo), segmentThrough(t, f.repo))
	return f
}

func (f resumeFixture) release(id string, allocation string) func(context.Context) (*Report, error) {
	return f.lease(id, transaction.LeaseRequest{Verb: transaction.LeasePoolRelease, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Allocation: allocation})
}

// sealedRun runs write with the moving hooks but no dispatcher, so it
// counts the command's complete audits.
func sealedRun(t *testing.T, f resumeFixture, write func(context.Context) (*Report, error), stage func(string)) movingRun {
	t.Helper()
	return movingStaged(t, f, write, 0, nil, stage)
}

// LGL-V0-009 and LGL-V0-010 (proposed), V1-1702: on a segmented store
// whose writer checkpoint holds the sealed base, a pool acquire and a pool
// release each run no complete audit: each resumes from the sealed base
// under the writer lock, with the same report and the same store bytes as
// the complete route on the same pre-state, and keeps FullSeq at S.
func TestLGLV0009_PoolVerbsResumeFromSealedBase(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newSealedBaseFixture(t)
	through := segmentThrough(t, f.repo)
	acquired := sealedRun(t, f, f.acquire("sealed-acquire"), nil)
	if !acquired.completed() || acquired.rep.PoolAllocation == nil || acquired.timing.Resumed != 1 || acquired.audits != 0 || !acquired.fast() {
		t.Fatalf("acquire: %+v %v resumed %d audits %d stages %v", acquired.rep, acquired.err, acquired.timing.Resumed, acquired.audits, acquired.stages)
	}
	if wc := readWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != through {
		t.Fatalf("acquire retained %+v, want FullSeq %d", wc, through)
	}
	parent := filepath.Dir(f.repo.PrimaryWorktree)
	pre := filepath.Join(t.TempDir(), "pre")
	copyTree(t, parent, pre)
	allocation := string(acquired.rep.PoolAllocation.AllocationID)
	released := sealedRun(t, f, f.release("sealed-release", allocation), nil)
	if !released.completed() || released.rep.ReleasedPoolAllocation == nil || released.timing.Resumed != 1 || released.audits != 0 || !released.fast() {
		t.Fatalf("release: %+v %v resumed %d audits %d stages %v", released.rep, released.err, released.timing.Resumed, released.audits, released.stages)
	}
	if wc := readWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != through {
		t.Fatalf("release retained %+v, want FullSeq %d", wc, through)
	}
	wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
	resumed := storeFiles(t, parent, wcPath)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, pre, parent)
	// With no writer checkpoint there is no sealed base: the oracle is the
	// complete route.
	for _, p := range []string{wcPath, wcPath + ".base"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	complete := sealedRun(t, f, f.release("sealed-release", allocation), nil)
	if !complete.completed() || complete.fast() || complete.audits == 0 {
		t.Fatalf("complete route: %+v %v audits %d stages %v", complete.rep, complete.err, complete.audits, complete.stages)
	}
	completeFiles := storeFiles(t, parent, wcPath)
	for p, d := range completeFiles {
		if resumed[p] != d {
			t.Errorf("%s: resumed %q, complete %q", p, resumed[p], d)
		}
	}
	for p := range resumed {
		if _, ok := completeFiles[p]; !ok {
			t.Errorf("%s: only the resumed route wrote it", p)
		}
	}
	if !reflect.DeepEqual(*released.rep, *complete.rep) {
		t.Fatalf("reports differ:\nresumed  %+v\ncomplete %+v", *released.rep, *complete.rep)
	}
	mustSegmentAudit(t, f.repo, headSeqOf(t, f.repo))
}

// LGL-V0-009 (proposed): a sealed segment or the manifest rewritten in
// place with its modification time restored, before the command derives the
// sealed base, after the resume precheck, or while the resumed write
// observes under the writer lock, is never committed over: the resume
// declines and the command keeps the complete route's own decision, which for
// the manifest is a non-retryable JOURNAL_FORKED (LGL-V0-900).
func TestLGLV0009_SealedBaseDeclinesRewrittenSegment(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, stage := range []string{"before", "fast.resume.precheck", "fast.lease.observe"} {
		for _, target := range []string{"segment", "manifest"} {
			t.Run(stage+"/"+target, func(t *testing.T) {
				t.Parallel()
				f := newSealedBaseFixture(t)
				p := snapshot.SegmentName(1, snapshot.SegmentReceipts)
				if target == "manifest" {
					p = snapshot.SegmentManifestName(segmentThrough(t, f.repo))
				}
				var tampered uint64
				change := func() {
					rewriteSegmentKeepingStamps(t, filepath.Join(f.repo.StateDir, filepath.FromSlash(p)))
					tampered = headSeqOf(t, f.repo)
				}
				var run movingRun
				if stage == "before" {
					change()
					run = sealedRun(t, f, f.acquire("segment-acquire"), nil)
				} else {
					run = sealedRun(t, f, f.acquire("segment-acquire"), at(stage, change))
				}
				if tampered == 0 {
					t.Fatalf("the change never ran: %+v %v %v", run.rep, run.err, run.stages)
				}
				// A manifest rewritten before the command fails the head
				// guards, which run before any resume is tried.
				declines := 1
				if stage == "before" && target == "manifest" {
					declines = 0
				}
				if run.completed() || run.fast() || run.timing.Resumed != 0 || run.timing.ResumeDeclines != declines {
					t.Fatalf("acquire over a rewritten %s: %+v %v resumed %d declines %d stages %v", target, run.rep, run.err, run.timing.Resumed, run.timing.ResumeDeclines, run.stages)
				}
				// LGL-V0-900 (V1-2623): a manifest rewritten at a stable head
				// is corrupt, not moving: the command refuses JOURNAL_FORKED,
				// not SNAPSHOT_MOVED after every round, and once before it.
				t.Logf("refusal %v after %d rounds", run.err, run.timing.Rounds)
				if target == "manifest" && (wire.CodeOf(run.err) != wire.CodeJournalForked || stage == "before" && run.timing.Rounds != 1) {
					t.Fatalf("acquire over a rewritten manifest: %v after %d rounds", run.err, run.timing.Rounds)
				}
				if got := headSeqOf(t, f.repo); got != tampered {
					t.Fatalf("head moved from %d to %d over a rewritten %s", tampered, got, target)
				}
				oracle := stagedWrite(f.acquire("segment-oracle"), nil)
				sameDecision(t, run.writerRun, oracle)
			})
		}
	}
}

// LGL-V0-009 (proposed): a stray request file, a tail receipt rewritten in
// place with its stamps kept, or a writer invalidation between the sealed
// base and the resumed write declines the resume; the command keeps the
// complete route, whose decision it reports.
func TestLGLV0009_SealedBaseDeclinesChangedState(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, f resumeFixture)
		refuse bool
	}{
		{"stray-request", func(t *testing.T, f resumeFixture) {
			dirs, err := filepath.Glob(filepath.Join(f.repo.StateDir, "requests", "*"))
			if err != nil || len(dirs) == 0 {
				t.Fatalf("request shards: %v %v", dirs, err)
			}
			historyWrite(t, filepath.Join(dirs[0], "stray.json"), []byte("{}\n"))
		}, true},
		{"rewritten-tail-receipt", func(t *testing.T, f resumeFixture) {
			name, _ := snapshot.ReceiptName(segmentThrough(t, f.repo) + 2)
			rewriteReceiptKeepingStamps(t, filepath.Join(f.repo.StateDir, "receipts", name))
		}, true},
		{"invalidated", func(t *testing.T, f resumeFixture) {
			invalidateWriterCheckpoint(context.Background(), f.repo)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newSealedBaseFixture(t)
			run := sealedRun(t, f, f.acquire("changed-acquire"), at("fast.resume.precheck", func() { tc.change(t, f) }))
			if run.timing.Resumed != 0 || run.timing.ResumeDeclines != 1 || run.fast() {
				t.Fatalf("acquire over a changed base resumed: %+v %v resumed %d declines %d stages %v", run.rep, run.err, run.timing.Resumed, run.timing.ResumeDeclines, run.stages)
			}
			if tc.refuse && run.completed() {
				t.Fatalf("acquire over a changed base completed: %+v", run.rep)
			}
			if !tc.refuse {
				if !run.completed() || run.audits == 0 {
					t.Fatalf("acquire after an invalidation: %+v %v audits %d", run.rep, run.err, run.audits)
				}
				return
			}
			oracle := stagedWrite(f.acquire("changed-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// LGL-V0-010 and LGL-V0-012 (proposed): the sealed base is tried only for
// the pool verbs, widen and reap, and only when the writer checkpoint's
// FullSeq is the sealed sequence. Every other lease verb keeps its route;
// an acquire on a store whose checkpoint a complete-route write rebased to
// its head finds no sealed base.
func TestLGLV0010_SealedBaseGate(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	t.Run("verbs", func(t *testing.T) {
		sealed := map[string]bool{transaction.LeasePoolAcquire: true, transaction.LeasePoolRelease: true, transaction.LeaseWiden: true, transaction.LeaseReap: true}
		for _, verb := range []string{transaction.LeaseClaim, transaction.LeaseClaimNext, transaction.LeaseRenew, transaction.LeaseHeartbeat, transaction.LeaseRelease, transaction.LeasePoolSafe, transaction.LeasePoolAcquire, transaction.LeasePoolRelease, transaction.LeasePoolWait, transaction.LeasePoolSweep, transaction.LeaseWiden, transaction.LeaseReap, transaction.LeaseSupervisor} {
			got := sealedResumeVerb(transaction.Request{Operation: transaction.Lease, Lease: &transaction.LeaseRequest{Verb: verb}})
			if got != sealed[verb] {
				t.Errorf("sealedResumeVerb(%s) = %v, want %v", verb, got, sealed[verb])
			}
		}
		if sealedResumeVerb(transaction.Request{Operation: transaction.Lease}) {
			t.Error("a lease request with no lease body resumes")
		}
	})
	t.Run("full-seq-past-the-seal", func(t *testing.T) {
		f := newSealedResumeFixture(t)
		wc := readWriterCheckpoint(f.repo)
		if wc == nil || wc.FullSeq == segmentThrough(t, f.repo) {
			t.Fatalf("fixture checkpoint %+v is at the sealed sequence", wc)
		}
		run := sealedRun(t, f, f.acquire("gate-acquire"), nil)
		if !run.completed() || run.timing.Resumed != 0 || run.timing.ResumeDeclines != 0 || run.audits == 0 {
			t.Fatalf("acquire: %+v %v resumed %d declines %d audits %d", run.rep, run.err, run.timing.Resumed, run.timing.ResumeDeclines, run.audits)
		}
	})
}
