//go:build darwin || linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// resumeFixture is a writer store with the pool policy, a plain and a
// pooled-capable ticket, and a plain claim an acquire can extend.
type resumeFixture struct {
	repo  *intent.Repository
	root  string
	now   wire.Timestamp
	claim *Report
}

func newResumeFixture(t *testing.T) resumeFixture {
	t.Helper()
	return resumeFixtureOn(t, writerStore(t, 70))
}

// resumeFixtureOn is newResumeFixture over repo, a store whose writer
// checkpoint the route already serves.
func resumeFixtureOn(t *testing.T, repo *intent.Repository) resumeFixture {
	t.Helper()
	fixAttemptEntropy(t, repo, fixedEntropy(11))
	poolLeasePolicy(t, repo)
	if run := writerMutate(t, repo, "resume-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	f := resumeFixture{repo: repo, root: filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree"), now: WallClock()}
	holdGit(t, f.root, "init", "-q", "-b", "main")
	holdGit(t, f.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	created := stagedWrite(writerCreate(repo, "resume-ticket", f.now), nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	claim := stagedWrite(f.lease("resume-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-1", LeaseMinutes: "60", Scope: []string{"src/"}}), nil)
	if !claim.completed() {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	f.claim = claim.rep
	return f
}

func (f resumeFixture) lease(id string, l transaction.LeaseRequest) func(context.Context) (*Report, error) {
	return func(ctx context.Context) (*Report, error) {
		return Lease(ctx, f.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: f.root, Lease: l}, f.now)
	}
}

func (f resumeFixture) acquire(id string) func(context.Context) (*Report, error) {
	return f.lease(id, transaction.LeaseRequest{Verb: transaction.LeasePoolAcquire, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Pool: "db"})
}

// movingRun runs write while a dispatcher commits one write after each of
// its first `writes` complete preparation audits (every audit when writes
// is negative), the window in which a dispatcher committing every few
// seconds lands on a 45,000-receipt journal. after, when set, runs after
// each dispatcher write; tweak, when set, adjusts the command's hooks.
type movingRun struct {
	writerRun
	timing LeaseTiming
	audits int
	writes int
}

func moving(t *testing.T, f resumeFixture, write func(context.Context) (*Report, error), writes int, after func(n int), tweak ...func(*inventoryHooks)) movingRun {
	t.Helper()
	return movingStaged(t, f, write, writes, after, nil, tweak...)
}

// movingStaged is moving with stage, when set, as the command's stage hook.
func movingStaged(t *testing.T, f resumeFixture, write func(context.Context) (*Report, error), writes int, after func(n int), stage func(string), tweak ...func(*inventoryHooks)) movingRun {
	t.Helper()
	var run movingRun
	hooks := hooksForInventory(context.Background())
	for _, fn := range tweak {
		fn(&hooks)
	}
	audit := hooks.audit
	hooks.audit = func(r journal.Reader) (*journal.Result, journal.PhysicalObservation, error) {
		run.audits++
		res, obs, err := audit(r)
		if writes < 0 || run.writes < writes {
			run.writes++
			// The dispatcher shares the command's clock reading, so every
			// receipt is recorded in order whichever route commits.
			if d := stagedWrite(writerCreate(f.repo, "dispatcher-"+string(rune('a'+run.writes)), f.now), nil); !d.completed() {
				t.Errorf("dispatcher write: %+v %v", d.rep, d.err)
			}
			if after != nil {
				after(run.writes)
			}
		}
		return res, obs, err
	}
	run.writerRun = stagedWrite(func(ctx context.Context) (*Report, error) {
		ctx = context.WithValue(ctx, inventoryHooksKey{}, hooks)
		return write(WithLeaseTiming(ctx, &run.timing))
	}, stage)
	return run
}

// PSR-V0-050, PSR-V0-051 and PSR-V0-052 (proposed), V1-1560: a pool acquire
// whose every complete preparation audit is overtaken by a dispatcher write
// starves on the base, where each of the 16 rounds reruns the complete audit
// and moves. Here the first round's audit is the resume base and the second
// round resumes from it under the writer lock: one complete audit, one
// resumed write, the same bytes and report as the complete route on the
// same pre-state.
func TestPSRV0052_PoolAcquireResumesUnderWriter(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	parent := filepath.Dir(f.repo.PrimaryWorktree)
	pre := filepath.Join(t.TempDir(), "pre")
	run := moving(t, f, f.acquire("resume-acquire"), -1, func(int) {
		os.RemoveAll(pre)
		copyTree(t, parent, pre)
	})
	if !run.completed() || run.rep.PoolAllocation == nil {
		t.Fatalf("acquire under a moving journal: %+v %v (audits %d, writes %d, rounds %d)", run.rep, run.err, run.audits, run.writes, run.timing.Rounds)
	}
	if run.timing.Resumed != 1 || run.audits != 1 || !run.fast() {
		t.Fatalf("resumed %d, complete audits %d, stages %v", run.timing.Resumed, run.audits, run.stages)
	}
	wcPath := journal.WriterCheckpointPath(f.repo.StateDir)
	resumed := storeFiles(t, parent, wcPath)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, pre, parent)
	complete := stagedWrite(f.acquire("resume-acquire"), nil)
	if !complete.completed() || complete.fast() {
		t.Fatalf("complete route: %+v %v %v", complete.rep, complete.err, complete.stages)
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
	if !reflect.DeepEqual(*run.rep, *complete.rep) {
		t.Fatalf("reports differ:\nresumed  %+v\ncomplete %+v", *run.rep, *complete.rep)
	}
}

// PSR-V0-052 (proposed): widen, which stays off the retained-checkpoint
// route (CAL-V0-116), also resumes from its own audit when overtaken.
func TestPSRV0052_WidenResumesUnderWriter(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	run := moving(t, f, f.lease("resume-widen", transaction.LeaseRequest{Verb: transaction.LeaseWiden, AttemptID: f.claim.AttemptID, Generation: f.claim.Generation, Scope: []string{"docs/"}}), -1, nil)
	if !run.completed() || run.timing.Resumed != 1 || run.audits != 1 {
		t.Fatalf("widen under a moving journal: %+v %v resumed %d audits %d", run.rep, run.err, run.timing.Resumed, run.audits)
	}
}

// PSR-V0-053 (proposed): a prefix receipt rewritten in place, its
// modification time restored, or a stray request file, between the base
// audit and the resumed write, is never committed over: the resumed audit
// declines, the command keeps the complete audit, and the acquire ends with
// the complete route's own refusal for the held journal.
func TestPSRV0053_ResumeKeepsRewrittenPrefixDetection(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	second, _ := snapshot.ReceiptName(2)
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, state string)
	}{
		{"rewritten-receipt", func(t *testing.T, state string) {
			rewriteReceiptKeepingStamps(t, filepath.Join(state, "receipts", second))
		}},
		{"stray-request", func(t *testing.T, state string) {
			dirs, err := filepath.Glob(filepath.Join(state, "requests", "*"))
			if err != nil || len(dirs) == 0 {
				t.Fatalf("request shards: %v %v", dirs, err)
			}
			historyWrite(t, filepath.Join(dirs[0], "stray.json"), []byte("{}\n"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newResumeFixture(t)
			head := func() uint64 { return headSeqOf(t, f.repo) }
			var tampered uint64
			run := moving(t, f, f.acquire("tamper-acquire"), 1, func(int) {
				tc.change(t, f.repo.StateDir)
				tampered = head()
			})
			if run.completed() || run.fast() || run.timing.Resumed != 0 {
				t.Fatalf("acquire over a changed base: %+v %v resumed %d stages %v", run.rep, run.err, run.timing.Resumed, run.stages)
			}
			if wire.CodeOf(run.err) == wire.CodeSnapshotMoved {
				t.Fatalf("held journal answered as moved: %v", run.err)
			}
			if got := head(); got != tampered {
				t.Fatalf("head moved from %d to %d over a changed base", tampered, got)
			}
			oracle := stagedWrite(f.acquire("tamper-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// PSR-V0-056 (proposed): a base receipt rewritten after the resumed audit
// passed, while the resumed write observes or models under the writer lock,
// is seen by the recheck before commit. The resume declines, nothing is
// committed over the rewritten prefix, and the command keeps the complete
// route's own decision for that journal.
func TestPSRV0056_ResumeRechecksBaseBeforeCommit(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	second, _ := snapshot.ReceiptName(2)
	for _, stage := range []string{"fast.lease.observe", "fast.lease.model"} {
		t.Run(stage, func(t *testing.T) {
			f := newResumeFixture(t)
			var tampered uint64
			run := movingStaged(t, f, f.acquire("recheck-acquire"), 1, nil, at(stage, func() {
				rewriteReceiptKeepingStamps(t, filepath.Join(f.repo.StateDir, "receipts", second))
				tampered = headSeqOf(t, f.repo)
			}))
			if tampered == 0 {
				t.Fatalf("the resumed write never reached %s: %+v %v %v", stage, run.rep, run.err, run.stages)
			}
			if run.completed() || run.fast() || run.timing.Resumed != 0 {
				t.Fatalf("acquire over a prefix rewritten at %s: %+v %v resumed %d stages %v", stage, run.rep, run.err, run.timing.Resumed, run.stages)
			}
			if got := headSeqOf(t, f.repo); got != tampered {
				t.Fatalf("head moved from %d to %d over a rewritten prefix", tampered, got)
			}
			oracle := stagedWrite(f.acquire("recheck-oracle"), nil)
			sameDecision(t, run.writerRun, oracle)
		})
	}
}

// PSR-V0-054 (proposed): a change guard overtaken on every registration, as
// on a 45,000-receipt journal whose registration outlasts the gap between
// writes, serves no complete route, yet the round still audits for its resume
// base and the next round resumes under the writer lock: on the base every
// round ends at the guard and the acquire runs out of rounds.
func TestPSRV0054_OvertakenGuardStillResumes(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	f := newResumeFixture(t)
	watches := 0
	overtaken := func(h *inventoryHooks) {
		h.watch = func(*intent.Repository) (*authority.ChangeGuard, error) {
			watches++
			return nil, wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed during preparation")
		}
	}
	run := moving(t, f, f.acquire("overtaken-acquire"), -1, nil, overtaken)
	if !run.completed() || run.rep.PoolAllocation == nil || run.timing.Resumed != 1 || run.audits != 1 || watches != 1 {
		t.Fatalf("acquire under an overtaken guard: %+v %v resumed %d audits %d watches %d rounds %d", run.rep, run.err, run.timing.Resumed, run.audits, watches, run.timing.Rounds)
	}
}

// PSR-V0-053 and PSR-V0-054 (proposed): with the change guard overtaken on
// every registration, a rewritten prefix receipt still never commits: the
// resumed audit declines, the guardless rounds decide nothing, and the
// acquire ends with nothing written.
func TestPSRV0054_OvertakenGuardKeepsRewrittenPrefixDetection(t *testing.T) {
	t.Parallel()
	if !journal.ResumableAudits() {
		t.Skip("platform reports no change time")
	}
	second, _ := snapshot.ReceiptName(2)
	f := newResumeFixture(t)
	overtaken := func(h *inventoryHooks) {
		h.watch = func(*intent.Repository) (*authority.ChangeGuard, error) {
			return nil, wire.Errorf(wire.CodeSnapshotMoved, "change guard", "store changed during preparation")
		}
	}
	var tampered uint64
	run := moving(t, f, f.acquire("overtaken-tamper"), 1, func(int) {
		rewriteReceiptKeepingStamps(t, filepath.Join(f.repo.StateDir, "receipts", second))
		tampered = headSeqOf(t, f.repo)
	}, overtaken)
	if run.completed() || run.timing.Resumed != 0 {
		t.Fatalf("acquire over a rewritten prefix: %+v %v resumed %d", run.rep, run.err, run.timing.Resumed)
	}
	if got := headSeqOf(t, f.repo); got != tampered {
		t.Fatalf("head moved from %d to %d over a rewritten prefix", tampered, got)
	}
}

// rewriteReceiptKeepingStamps flips one byte inside the last string of
// receipt p and restores its modification time.
func rewriteReceiptKeepingStamps(t *testing.T, p string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.LastIndex(string(raw), "\"")
	raw[i-1] ^= 1
	historyWrite(t, p, raw)
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}
