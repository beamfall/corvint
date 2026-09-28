package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: even a failed optimistic read waits for a live writer and
// rechecks its observation instead of reporting transient staging as damage.
func TestCALV0026_PreparationFailureWaitsForWriter(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(fmt.Sprint("recovery=", recovery), func(t *testing.T) { testPreparedWriter(t, recovery) })
	}
}

func testPreparedWriter(t *testing.T, recovery bool) {
	repo := cachedLeaseRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if _, err := session.Prepare(authority.Slot("a00"), authority.RoleReceipt, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	request := transaction.Request{Operation: transaction.Lease, QueueID: fixture.QueueID, RequestID: "prepared", Actor: mutation.Binding{ID: "tester", Role: "OWNER"}, Lease: &transaction.LeaseRequest{Verb: transaction.LeaseReap}}
	var p *preparedLease
	if recovery {
		p, err = prepareLeaseRecovery(repo)
	} else {
		p, err = prepareLease(ctx, repo, request, wire.Timestamp("2026-09-27T00:00:00Z"), nil)
	}
	if err != nil || p == nil || p.failure == nil {
		t.Fatalf("failed read was not retained for locked recheck: %+v %v", p, err)
	}
	defer p.guard.Close()
	done := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); done <- commitLease(ctx, repo, request, p, &Report{}, nil) }()
	defer func() { cancel(); <-joined }()
	select {
	case err := <-done:
		t.Fatalf("returned before writer released lock: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := session.RemoveStage(authority.Slot("a00")); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if wire.CodeOf(err) != wire.CodeSnapshotMoved {
			t.Fatalf("wanted retry after writer changed observation: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("prepared writer did not stop")
	}
}
