package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// orphanRouteStore is a writer-route store with one claimed ticket, whose
// claim the next liveness writes extend.
func orphanRouteStore(t *testing.T) (*intent.Repository, string, *Report) {
	t.Helper()
	repo := writerStore(t, 70)
	holdLeasePolicy(t, repo)
	created := writerMutate(t, repo, "orphan-create", nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	claim := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "orphan-claim", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: created.rep.Ticket, Holder: "agent-1", LeaseMinutes: "60"}}, WallClock())
	}, nil)
	if !claim.completed() {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	return repo, root, claim.rep
}

// LGL-V0-001 (proposed): a ready writer route takes no orphan-cleanup lock
// before the preparation gate. With a staging entry present while another
// process holds the writer lock, a heartbeat spends none of that hold in its
// guards; it waits for the lock inside the route, which then finds the entry
// under the lock, declines "staging", and the complete route clears the
// orphan under a lock and commits the heartbeat exactly once.
func TestLGLV0001_ReadyRouteTakesNoCleanupLockOutsideGate(t *testing.T) {
	repo, root, claim := orphanRouteStore(t)
	slot := filepath.Join(repo.StateDir, "staging", "a00")
	fixture.Write(t, slot, []byte("orphan"))
	held, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const hold = 600 * time.Millisecond
	released := make(chan error, 1)
	go func() {
		time.Sleep(hold)
		released <- held.Close()
	}()
	var timing LeaseTiming
	run := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(WithLeaseTiming(ctx, &timing), repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "orphan-heartbeat", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
	}, nil)
	if err := <-released; err != nil {
		t.Fatal(err)
	}
	if !run.completed() || run.fast() || run.declined() != "lease HEARTBEAT: staging" {
		t.Fatalf("heartbeat: %+v %v %v", run.rep, run.err, run.stages)
	}
	if timing.Guards >= hold/2 {
		t.Fatalf("guards spent %v of a %v peer hold outside the gate", timing.Guards, hold)
	}
	if timing.LockWait < hold/2 {
		t.Fatalf("the route waited %v for a lock held %v", timing.LockWait, hold)
	}
	if _, err := os.Stat(slot); !os.IsNotExist(err) {
		t.Fatalf("orphan not cleared: %v", err)
	}
	// The next heartbeat is served by the route again: nothing was left
	// behind, and the committed heartbeat replays rather than repeating.
	again := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "orphan-heartbeat", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
	}, nil)
	if again.err != nil || again.rep == nil || again.rep.Kind != "Replay" {
		t.Fatalf("same-request heartbeat: %+v %v %v", again.rep, again.err, again.stages)
	}
	next := stagedWrite(func(ctx context.Context) (*Report, error) {
		return Lease(ctx, repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: "orphan-heartbeat-2", Root: root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: claim.AttemptID, Generation: claim.Generation}}, WallClock())
	}, nil)
	if !next.completed() || !next.fast() {
		t.Fatalf("next heartbeat: %+v %v %v", next.rep, next.err, next.stages)
	}
}
