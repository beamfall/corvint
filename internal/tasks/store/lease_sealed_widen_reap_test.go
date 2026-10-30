//go:build darwin || linux

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealedWidenReap is one widen or reap request on a sealed-base fixture.
// expire moves the fixture's clock past the claims' 60-minute leases, so a
// reap has expired attempts to fence; extra claims that many more tickets
// first. resumed is the command's exact resumed-write count and reaped the
// fences a bare reap reports.
type sealedWidenReap struct {
	name            string
	expire          bool
	extra           int
	resumed, reaped int
	lease           func(f resumeFixture) transaction.LeaseRequest
}

var sealedWidenReapCases = []sealedWidenReap{
	{"widen", false, 0, 1, 0, func(f resumeFixture) transaction.LeaseRequest {
		return transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Scope: []string{"docs/"}}
	}},
	{"reap", true, 0, 1, 0, func(f resumeFixture) transaction.LeaseRequest {
		return transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation}
	}},
	// The bare reap's survey and each of its two fences resume.
	{"reap-all", true, 1, 3, 2, func(resumeFixture) transaction.LeaseRequest {
		return transaction.LeaseRequest{Verb: transaction.LeaseReap}
	}},
}

func (c sealedWidenReap) fixture(t *testing.T) resumeFixture {
	t.Helper()
	f := newSealedBaseFixture(t)
	for i := 0; i < c.extra; i++ {
		id := fmt.Sprintf("sealed-extra-%d", i)
		created := stagedWrite(writerCreate(f.repo, id, f.now), nil)
		if !created.completed() {
			t.Fatalf("create %s: %+v %v", id, created.rep, created.err)
		}
		// A fresh attempt id: the fixture fixed its own claim's entropy.
		attemptEntropies.Store(f.repo.StateDir, fixedEntropy(byte(60+i)))
		claim := stagedWrite(f.lease(id+"-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-2", LeaseMinutes: "60", Scope: []string{"lib/"}}), nil)
		if !claim.completed() {
			t.Fatalf("claim %s: %+v %v %v", id, claim.rep, claim.err, claim.stages)
		}
	}
	if wc := readWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != segmentThrough(t, f.repo) {
		t.Fatalf("fixture lost the sealed base: %+v", wc)
	}
	if c.expire {
		at, err := time.Parse(time.RFC3339, string(f.now))
		if err != nil {
			t.Fatal(err)
		}
		f.now = wire.Timestamp(at.Add(61 * time.Minute).UTC().Format(time.RFC3339))
	}
	return f
}

// LGL-V0-012 (proposed), V1-2592: on a segmented store whose writer
// checkpoint holds the sealed base, a widen, a named reap and a bare reap
// each run no complete audit: each resumes from the sealed base under the
// writer lock, keeps FullSeq at S, and reports and writes the same bytes as
// the complete route on a copy of the same pre-state without the
// checkpoint. A bare reap over two expired leases commits each fence as its
// own write: its survey and both fences resume. Before the change each took
// the complete route (resumed 0, audits 1).
func TestLGLV0012_WidenAndReapResumeFromSealedBase(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, c := range sealedWidenReapCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := c.fixture(t)
			through := segmentThrough(t, f.repo)
			parent := filepath.Dir(f.repo.PrimaryWorktree)
			pre := filepath.Join(t.TempDir(), "pre")
			copyTree(t, parent, pre)
			id := "sealed-" + c.name
			resumed := sealedRun(t, f, f.lease(id, c.lease(f)), nil)
			if !resumed.completed() || resumed.timing.Resumed != c.resumed || resumed.audits != 0 || !resumed.fast() || len(resumed.rep.ReapReceipts) != c.reaped {
				t.Fatalf("%s: %+v %v resumed %d audits %d stages %v", c.name, resumed.rep, resumed.err, resumed.timing.Resumed, resumed.audits, resumed.stages)
			}
			if wc := readWriterCheckpoint(f.repo); wc == nil || wc.FullSeq != through {
				t.Fatalf("%s retained %+v, want FullSeq %d", c.name, wc, through)
			}
			wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
			resumedFiles := storeFiles(t, parent, wcPath)
			if err := os.RemoveAll(parent); err != nil {
				t.Fatal(err)
			}
			copyTree(t, pre, parent)
			for _, p := range []string{wcPath, wcPath + ".base"} {
				if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			complete := sealedRun(t, f, f.lease(id, c.lease(f)), nil)
			if !complete.completed() || complete.fast() || complete.audits == 0 {
				t.Fatalf("complete route: %+v %v audits %d stages %v", complete.rep, complete.err, complete.audits, complete.stages)
			}
			completeFiles := storeFiles(t, parent, wcPath)
			for p, d := range completeFiles {
				if resumedFiles[p] != d {
					t.Errorf("%s: resumed %q, complete %q", p, resumedFiles[p], d)
				}
			}
			for p := range resumedFiles {
				if _, ok := completeFiles[p]; !ok {
					t.Errorf("%s: only the resumed route wrote it", p)
				}
			}
			if !reflect.DeepEqual(*resumed.rep, *complete.rep) {
				t.Fatalf("reports differ:\nresumed  %+v\ncomplete %+v", *resumed.rep, *complete.rep)
			}
			mustSegmentAudit(t, f.repo, headSeqOf(t, f.repo))
		})
	}
}

// LGL-V0-012 (proposed), V1-2592: the sealed base a resumed widen or reap
// reuses is the one the preparation gate re-validates before it commits. A
// sealed segment or the manifest rewritten in place with its stamps kept
// after the resume precheck (outside the gate) or while the resumed write
// observes under the writer lock, or a tail receipt rewritten after the
// precheck, declines the resume: nothing is committed over the change and
// the command reports the complete route's own decision.
func TestLGLV0012_ResumedWidenAndReapRevalidateBaseUnderGate(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, c := range sealedWidenReapCases[:2] {
		for _, stage := range []string{"fast.resume.precheck", "fast.lease.observe"} {
			for _, target := range []string{"segment", "manifest", "tail-receipt"} {
				if target == "tail-receipt" && stage != "fast.resume.precheck" {
					continue
				}
				t.Run(c.name+"/"+stage+"/"+target, func(t *testing.T) {
					t.Parallel()
					f := c.fixture(t)
					var tampered uint64
					change := func() {
						switch target {
						case "segment":
							rewriteSegmentKeepingStamps(t, filepath.Join(f.repo.StateDir, filepath.FromSlash(snapshot.SegmentName(1, snapshot.SegmentReceipts))))
						case "manifest":
							rewriteSegmentKeepingStamps(t, filepath.Join(f.repo.StateDir, filepath.FromSlash(snapshot.SegmentManifestName(segmentThrough(t, f.repo)))))
						default:
							name, _ := snapshot.ReceiptName(segmentThrough(t, f.repo) + 2)
							rewriteReceiptKeepingStamps(t, filepath.Join(f.repo.StateDir, "receipts", name))
						}
						tampered = headSeqOf(t, f.repo)
					}
					run := sealedRun(t, f, f.lease("tampered-"+c.name, c.lease(f)), at(stage, change))
					if tampered == 0 {
						t.Fatalf("the change never ran: %+v %v %v", run.rep, run.err, run.stages)
					}
					if run.completed() || run.fast() || run.timing.Resumed != 0 || run.timing.ResumeDeclines != 1 {
						t.Fatalf("%s over a rewritten %s: %+v %v resumed %d declines %d stages %v", c.name, target, run.rep, run.err, run.timing.Resumed, run.timing.ResumeDeclines, run.stages)
					}
					if got := headSeqOf(t, f.repo); got != tampered {
						t.Fatalf("head moved from %d to %d over a rewritten %s", tampered, got, target)
					}
					oracle := stagedWrite(f.lease("tampered-oracle-"+c.name, c.lease(f)), nil)
					sameDecision(t, run.writerRun, oracle)
				})
			}
		}
	}
}

// LGL-V0-012 (proposed), V1-2592: a widen or reap whose request the resumed
// route finds already committed reports the replay only after the base check
// under the writer lock. A sealed segment rewritten in place with its stamps
// kept while the replay observes declines the resume, so the command reports
// the complete route's decision over the rewritten store, never the replay.
func TestLGLV0012_ResumedReplayRevalidatesBaseUnderGate(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	for _, c := range sealedWidenReapCases[:2] {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := c.fixture(t)
			id := "replayed-" + c.name
			first := sealedRun(t, f, f.lease(id, c.lease(f)), nil)
			if !first.completed() || first.timing.Resumed != 1 {
				t.Fatalf("first %s: %+v %v resumed %d stages %v", c.name, first.rep, first.err, first.timing.Resumed, first.stages)
			}
			rewritten := false
			run := sealedRun(t, f, f.lease(id, c.lease(f)), at("fast.lease.observe", func() {
				rewriteSegmentKeepingStamps(t, filepath.Join(f.repo.StateDir, filepath.FromSlash(snapshot.SegmentName(1, snapshot.SegmentReceipts))))
				rewritten = true
			}))
			if !rewritten {
				t.Fatalf("the replay never observed: %+v %v %v", run.rep, run.err, run.stages)
			}
			for _, s := range run.stages {
				if strings.HasPrefix(s, "fast.lease.replay") {
					t.Fatalf("%s replayed over a rewritten segment: %v", c.name, run.stages)
				}
			}
			if run.timing.Resumed != 0 || run.timing.ResumeDeclines != 1 {
				t.Fatalf("%s replay: resumed %d declines %d stages %v", c.name, run.timing.Resumed, run.timing.ResumeDeclines, run.stages)
			}
			sameDecision(t, run.writerRun, stagedWrite(f.lease(id, c.lease(f)), nil))
		})
	}
}
