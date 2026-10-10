package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func headDigest(t *testing.T, s *leaseStore) wire.Digest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	return wire.Sum(raw)
}

// TestGH494_TimedOutClaimReplaysExactly: a caller that gave up on a claim
// recovers by retrying the exact request ID. A claim that never committed
// wrote nothing and charges nothing; a committed or receipt-pending claim
// replays its original attempt, generation and lease with the head digest and
// every journal file's SHA-256 unchanged.
func TestGH494_TimedOutClaimReplaysExactly(t *testing.T) {
	t.Run("cancelled in admission", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "one")
		before, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
		held, err := authority.AcquirePreparation(context.Background(), s.repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-1", Root: s.root, Lease: claimOf(id, "src/")}
		_, err = store.Lease(ctx, s.repo, operator(), choice, s.at(t, 0))
		cancel()
		if closeErr := held.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err == nil {
			t.Fatal("claim admitted while preparation was held")
		}
		if !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) || headDigest(t, s) != digest {
			t.Fatal("an unadmitted claim changed the store")
		}
		r, err := s.try("claim-1", claimOf(id, "src/"), s.at(t, 1))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted || replayed(r.Kind) || r.Redone || r.Generation != "1" {
			t.Fatalf("retry after timeout: %+v %v", r, err)
		}
		a := s.attempt(t, r.AttemptID)
		for _, reason := range snapshot.RetryReasonNames {
			if a.RetryReasons[reason].Int() != 0 {
				t.Fatalf("attempt charged %s without a prior attempt: %+v", reason, a)
			}
		}
		if a.RetryCount.Int() != 0 {
			t.Fatalf("attempt charged without a prior attempt: %+v", a)
		}
	})

	t.Run("committed after the caller gave up", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "one")
		first := s.claim(t, "claim-1", id, 0, "src/")
		attempt := s.attempt(t, first.AttemptID)
		before, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
		for _, minutes := range []int{1, 30, 90} {
			r, err := s.try("claim-1", claimOf(id, "src/"), s.at(t, minutes))
			if err != nil || r.Kind != "Replay" || r.AttemptID != first.AttemptID || r.Generation != first.Generation || r.Redone {
				t.Fatalf("replay at +%dm: %+v %v", minutes, r, err)
			}
			if !reflect.DeepEqual(attempt, s.attempt(t, r.AttemptID)) {
				t.Fatalf("replay at +%dm reports a different attempt or lease", minutes)
			}
			if headDigest(t, s) != digest || !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) {
				t.Fatalf("replay at +%dm changed the store", minutes)
			}
		}
	})

	t.Run("cancelled after a charged attempt", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "one")
		for i, request := range []string{"claim-1", "claim-2"} {
			a := s.claim(t, request, id, 0, "src/")
			if got := s.attempt(t, a.AttemptID).RetryCount.Int(); got != int64(i) {
				t.Fatalf("%s retry count %d", request, got)
			}
			if r := s.lease(t, "release-"+request, releaseOf(a), 0, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
				t.Fatalf("release %s: %+v", request, r)
			}
		}
		before, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
		held, err := authority.AcquirePreparation(context.Background(), s.repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-3", Root: s.root, Lease: claimOf(id, "src/")}
		_, err = store.Lease(ctx, s.repo, operator(), choice, s.at(t, 1))
		cancel()
		if closeErr := held.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err == nil || !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) || headDigest(t, s) != digest {
			t.Fatalf("cancelled claim wrote: %v", err)
		}
		r, err := s.try("claim-3", claimOf(id, "src/"), s.at(t, 2))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted || replayed(r.Kind) {
			t.Fatalf("retry after cancellation: %+v %v", r, err)
		}
		a := s.attempt(t, r.AttemptID)
		var charged int64
		for _, reason := range snapshot.RetryReasonNames {
			charged += a.RetryReasons[reason].Int()
		}
		if a.RetryCount.Int() != 2 || charged != 2 {
			t.Fatalf("cancelled claim added a charge: %+v", a)
		}
	})

	t.Run("receipt pending when the caller gave up", func(t *testing.T) {
		s := newLeaseStore(t)
		id := s.ticket(t, "one")
		seen := false
		restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
			if seen {
				return errInjected
			}
			seen = a.Role == "RECEIPT"
			return nil
		})
		_, err := s.try("claim-1", claimOf(id, "src/"), s.at(t, 0))
		restore()
		if !errors.Is(err, errInjected) {
			t.Fatalf("fault after receipt: %v", err)
		}
		redo, err := s.try("claim-1", claimOf(id, "src/"), s.at(t, 1))
		if err != nil || redo.Outcome.Outcome != mutation.OutcomeCompleted || !redo.Redone || !replayed(redo.Kind) || redo.AttemptID == "" || redo.Generation != "1" {
			t.Fatalf("redo: %+v %v", redo, err)
		}
		if a := s.attempt(t, redo.AttemptID); a.Phase != "RUNNING" || a.Generation != redo.Generation {
			t.Fatalf("redone attempt: %+v", a)
		}
		before, digest := fixture.TreeSnapshot(t, s.repo.StateDir), headDigest(t, s)
		again, err := s.try("claim-1", claimOf(id, "src/"), s.at(t, 2))
		if err != nil || again.Kind != "Replay" || again.Redone || again.AttemptID != redo.AttemptID || again.Generation != redo.Generation {
			t.Fatalf("replay after redo: %+v %v", again, err)
		}
		if headDigest(t, s) != digest || !reflect.DeepEqual(before, fixture.TreeSnapshot(t, s.repo.StateDir)) {
			t.Fatal("replay after redo changed the store")
		}
		s.consistent(t)
	})
}

// TestGH494_LeaseTimingSumsEveryTransaction: the `--timing` collector covers
// each lease transaction a command runs, including a claim's reaps, and
// never changes what the command writes.
func TestGH494_LeaseTimingSumsEveryTransaction(t *testing.T) {
	t.Parallel()
	s := newLeaseStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	l := claimOf(one, "src/")
	l.LeaseMinutes = "5"
	old := s.lease(t, "claim-1", l, 0, nil)
	var timing store.LeaseTiming
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "claim-2", Root: s.root, Lease: claimOf(two, "src/a.go")}
	next, err := store.Lease(store.WithLeaseTiming(context.Background(), &timing), s.repo, operator(), choice, s.at(t, 10))
	if err != nil || next.Outcome.Outcome != mutation.OutcomeCompleted || len(next.Reaped) != 1 || next.Reaped[0].AttemptID != old.AttemptID {
		t.Fatalf("timed claim after expiry: %+v %v", next, err)
	}
	if timing.Transactions < 2 || timing.Rounds < timing.Transactions || timing.SnapshotRead <= 0 || timing.JournalWrite <= 0 || timing.Fsync <= 0 || timing.LockHold < timing.JournalWrite+timing.Fsync {
		t.Fatalf("timing: %+v", timing)
	}
	s.consistent(t)
}
