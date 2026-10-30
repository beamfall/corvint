//go:build unix

package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// claimDeclines counts the writer route's claim deferrals.
func claimDeclines(run writerRun) int {
	n := 0
	for _, s := range run.stages {
		if why, ok := strings.CutPrefix(s, "fast.declined: "); ok && strings.Contains(why, "claim Git observation deferred") {
			n++
		}
	}
	return n
}

// claimTargetStore is a writer-route store whose policy enforces no budget
// fields, a separate Git checkout for the claim to observe, and one ticket.
func claimTargetStore(t *testing.T) (*intent.Repository, string, string) {
	t.Helper()
	repo := writerStore(t, 70)
	// An external agent reports no enforced budget fields.
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", wire.String("2"))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	if rep, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "claim-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(policy)}, WallClock()); err != nil || rep.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", rep, err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	created := writerMutate(t, repo, "claim-target", nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	return repo, root, created.rep.Ticket
}

// V1-2138: the writer route never runs a claim's scope deriver or Git
// observation under the writer lock. With the deriver blocked mid-run, a
// concurrent writer commits. That write moved the audited snapshot but not
// the claim root's HEAD, and the base observation is bound to the ref files
// (PSR-V0-224), so the retried writer route serves the base and the
// derivation already made and commits the claim after one decline: the
// deriver ran exactly once.
func TestV12138_ClaimDerivationOutsideWriterLock(t *testing.T) {
	t.Parallel()
	repo, root, ticketID := claimTargetStore(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	derive := func(context.Context, string, string, string, string) ([]string, string, bool) {
		if runs.Add(1) == 1 {
			close(started)
		}
		<-release
		return []string{"src/"}, string(wire.Sum([]byte("src/"))), true
	}
	now := WallClock()
	done := make(chan writerRun, 1)
	go func() {
		done <- stagedWrite(func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "derived-claim", Root: root, Derive: derive,
				Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: ticketID, Holder: "agent-1", LeaseMinutes: "60"}}, now)
		}, nil)
	}()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	select {
	case <-started:
	case run := <-done:
		t.Fatalf("claim returned before the deriver ran: %+v %v %v", run.rep, run.err, run.stages)
	case <-time.After(2 * time.Minute):
		t.Fatal("deriver never started (hang guard)")
	}
	// The deriver is now blocked and cannot return before release.
	concurrent := make(chan writerRun, 1)
	go func() { concurrent <- writerMutate(t, repo, "claim-concurrent", nil) }()
	select {
	case run := <-concurrent:
		if !run.completed() {
			t.Fatalf("a concurrent writer did not commit while the deriver ran: %+v %v %v", run.rep, run.err, run.stages)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("a concurrent writer was blocked by the claim's deriver (hang guard)")
	}
	close(release)
	released = true
	var run writerRun
	select {
	case run = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("claim never returned after release (hang guard)")
	}
	if !run.completed() || !run.fast() {
		t.Fatalf("claim not committed on the writer route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if n := claimDeclines(run); n != 1 {
		t.Fatalf("writer route deferred the claim observation %d times, want 1: %v", n, run.stages)
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("deriver ran %d times, want once", n)
	}
	proof, err := readLeaseProof(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := lockedAttempt(proof, run.rep.AttemptID)
	if !ok || a.Scope == nil || a.Scope.Source != "DERIVED" {
		t.Fatalf("claim scope not the served derivation: %+v", a)
	}
}

// V1-2138: with no concurrent write, the claim's base and tree come from the
// unlocked prewarm, the deriver runs once outside the lock, and the retried
// writer route serves both and commits the claim.
func TestV12138_UnmovedClaimServedOnWriterRoute(t *testing.T) {
	t.Parallel()
	repo, root, ticketID := claimTargetStore(t)
	var runs atomic.Int32
	derive := func(context.Context, string, string, string, string) ([]string, string, bool) {
		runs.Add(1)
		return []string{"src/"}, string(wire.Sum([]byte("src/"))), true
	}
	run := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "unmoved-claim", Root: root, Derive: derive,
			Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: ticketID, Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
	}, nil)
	if !run.completed() || !run.fast() {
		t.Fatalf("claim not committed on the writer route: %+v %v %v", run.rep, run.err, run.stages)
	}
	if n := claimDeclines(run); n != 1 {
		t.Fatalf("writer route deferred the claim observation %d times, want 1: %v", n, run.stages)
	}
	if n := runs.Load(); n != 1 {
		t.Fatalf("deriver ran %d times, want once", n)
	}
}

// V1-2138: a mutable Git observation is bound to the audited snapshot. While
// the deriver is blocked, the claim's checkout gains a commit and a concurrent
// writer advances the journal. The retried writer route then refuses the base
// it observed before the advance instead of reusing it, declines once more,
// and the complete route binds the claim to the new base commit.
func TestV12138_ClaimRebindsMovedBaseAfterJournalAdvance(t *testing.T) {
	t.Parallel()
	repo, root, ticketID := claimTargetStore(t)
	started, release := make(chan struct{}), make(chan struct{})
	var runs atomic.Int32
	derive := func(context.Context, string, string, string, string) ([]string, string, bool) {
		if runs.Add(1) == 1 {
			close(started)
			<-release
		}
		return []string{"src/"}, string(wire.Sum([]byte("src/"))), true
	}
	now := WallClock()
	done := make(chan writerRun, 1)
	go func() {
		done <- stagedWrite(func(ctx context.Context) (*Report, error) {
			return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "moved-claim", Root: root, Derive: derive,
				Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: ticketID, Holder: "agent-1", LeaseMinutes: "60"}}, now)
		}, nil)
	}()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	select {
	case <-started:
	case run := <-done:
		t.Fatalf("claim returned before the deriver ran: %+v %v %v", run.rep, run.err, run.stages)
	case <-time.After(2 * time.Minute):
		t.Fatal("deriver never started (hang guard)")
	}
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "integrated")
	out, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	moved := strings.TrimSpace(string(out))
	if run := writerMutate(t, repo, "claim-advance", nil); !run.completed() {
		t.Fatalf("journal advance: %+v %v %v", run.rep, run.err, run.stages)
	}
	close(release)
	released = true
	var run writerRun
	select {
	case run = <-done:
	case <-time.After(2 * time.Minute):
		t.Fatal("claim never returned after release (hang guard)")
	}
	if !run.completed() {
		t.Fatalf("claim not committed: %+v %v %v", run.rep, run.err, run.stages)
	}
	if n := claimDeclines(run); n != 2 || run.fast() {
		t.Fatalf("writer route deferred the claim observation %d times (fast %v), want 2 then the complete route: %v", n, run.fast(), run.stages)
	}
	proof, err := readLeaseProof(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := lockedAttempt(proof, run.rep.AttemptID)
	if !ok || a.BaseCommit != moved {
		t.Fatalf("claim base %q, want the commit integrated before the journal advanced %q", a.BaseCommit, moved)
	}
}

// V1-2138: the locked claim observer runs no Git command. A root that is not
// a repository would fail any Git observation; the locked observer instead
// defers with the audited state recorded, and settle then observes outside
// the lock and fails, so the complete route decides the claim.
func TestV12138_LockedClaimObserverRunsNoGit(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	choice := LeaseChoice{QueueID: fixture.QueueID, Root: missing, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim}}
	p := newClaimProbe(context.Background(), nil, choice)
	if len(p.outputs) != 0 {
		t.Fatalf("a failed prewarm memoized %d outputs", len(p.outputs))
	}
	proof := &journal.Result{Records: map[string]journal.Record{}}
	_, err := p.observer(context.Background(), nil, choice)(proof, &transaction.Input{})
	if err != errClaimDeferred || p.pending == nil {
		t.Fatalf("locked observer: err %v pending %v, want the claim deferral", err, p.pending)
	}
	if p.settle(context.Background(), nil, choice) {
		t.Fatal("settle reported a failed observation as made")
	}
	if p.pending != nil || p.settle(context.Background(), nil, choice) {
		t.Fatal("settle reran without a new deferral")
	}
}
