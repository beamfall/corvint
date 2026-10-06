package store_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0110_HandoffReleaseReplaysAfterLockTimeout: an evidence HANDOFF
// release refused LOCK_TIMEOUT under a caller wait wrote nothing. The same
// request ID then commits exactly once, and every later submission of it
// replays that receipt sequence without a write. The next claim of the ticket
// carries no retry charge.
func TestCALV0110_HandoffReleaseReplaysAfterLockTimeout(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "one")
	claim := claimOf(id, "src/")
	claim.Stage = "integrate"
	a := s.lease(t, "claim-1", claim, 0, nil)
	if a.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("claim: %+v", a)
	}
	handoff := releaseOf(a)
	handoff.Reason, handoff.Evidence = "HANDOFF", "local:review-result"
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "release-1", Root: s.root, Lease: handoff}

	before, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
	held, err := authority.AcquirePreparation(context.Background(), s.repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := store.WithLeaseLockWait(context.Background(), 200*time.Millisecond)
	start := time.Now()
	_, err = store.Lease(ctx, s.repo, operator(), choice, s.at(t, 1))
	elapsed := time.Since(start)
	if closeErr := held.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if wire.CodeOf(err) != wire.CodeLockTimeout {
		t.Fatalf("want LOCK_TIMEOUT, got %v", err)
	}
	if elapsed >= authority.DefaultLockWait {
		t.Fatalf("LOCK_TIMEOUT after %v ignored the caller wait", elapsed)
	}
	if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) || headDigest(t, s) != digest {
		t.Fatal("a timed-out HANDOFF release changed the store")
	}

	first, err := store.Lease(ctx, s.repo, operator(), choice, s.at(t, 2))
	if err != nil || first.Outcome.Outcome != mutation.OutcomeCompleted || replayed(first.Kind) || first.Outcome.ReceiptSeq == nil {
		t.Fatalf("same-request retry: %+v %v", first, err)
	}
	seq := *first.Outcome.ReceiptSeq
	committed, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
	for i, c := range []context.Context{context.Background(), ctx} {
		r, err := store.Lease(c, s.repo, operator(), choice, s.at(t, 3+i))
		if err != nil || r.Kind != "Replay" || r.Outcome.ReceiptSeq == nil || *r.Outcome.ReceiptSeq != seq || r.AttemptID != a.AttemptID || r.Generation != a.Generation {
			t.Fatalf("replay %d: %+v %v", i, r, err)
		}
		if headDigest(t, s) != digest || !reflect.DeepEqual(committed, fixture.TreeSnapshot(t, s.repo.StateDir)) {
			t.Fatalf("replay %d changed the store", i)
		}
	}
	if got := s.attempt(t, a.AttemptID).HandoffEvidence; got != "local:review-result" {
		t.Fatalf("handoff evidence %q", got)
	}
	next := claimOf(id, "src/")
	next.Stage = "review"
	b := s.lease(t, "claim-2", next, 5, nil)
	if b.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("reclaim: %+v", b)
	}
	if got := s.attempt(t, b.AttemptID).RetryCount.Int(); got != 0 {
		t.Fatalf("the replayed HANDOFF charged a retry: %d", got)
	}
}
