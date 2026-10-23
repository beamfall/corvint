package store_test

import (
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-372: leaseWrite's unlocked pre-round guard read can see head.json
// replaced between its stat and open by a writer committing under the lock.
// That torn observation spends the round; the next round reads the new head
// and the verb commits. Any other guard failure still ends the write at once.
func TestCALV0372_TornGuardReadSpendsOneRound(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "a")
	var reads, torn atomic.Int32
	restore := store.SetLeaseGuardFaultForTest(func(stateDir string) error {
		if stateDir != s.repo.StateDir {
			return nil
		}
		if reads.Add(1) == 1 {
			torn.Add(1)
			return wire.Errorf(wire.CodeSnapshotMoved, "head.json", "file replaced between stat and open")
		}
		return nil
	})
	defer restore()
	report, err := s.try("claim-a", claimOf(id, "docs/"), s.at(t, 1))
	if err != nil {
		t.Fatalf("claim after a torn guard read: %v", err)
	}
	if report.Kind != "Transaction" || report.Outcome.Outcome != mutation.OutcomeCompleted || report.AttemptID == "" {
		t.Fatalf("claim after a torn guard read: %+v", report)
	}
	if torn.Load() != 1 || reads.Load() < 2 {
		t.Fatalf("guard reads %d, torn %d: the torn round was not retried", reads.Load(), torn.Load())
	}
	restore()
	restore = store.SetLeaseGuardFaultForTest(func(stateDir string) error {
		if stateDir != s.repo.StateDir {
			return nil
		}
		return wire.Errorf(wire.CodeJournalForked, "head.json", "injected guard failure")
	})
	defer restore()
	if _, err := s.try("claim-a-forked", claimOf(id, "web/"), s.at(t, 2)); wire.CodeOf(err) != wire.CodeJournalForked {
		t.Fatalf("non-retryable guard failure: %v", err)
	}
}
