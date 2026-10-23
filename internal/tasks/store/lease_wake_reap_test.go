package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-361, V1-1634: the store judges expiry on the lease timestamp
// against the clock of the writing call and holds no record of a host
// suspend. A lease that ran out while its holder's host slept is reaped by
// the first colliding claim or reap after the wake, and the holder's
// liveness writes are then fenced. This is the accepted limit the
// requirement states; the dispatcher's own grace (ATR-V0-064) does not
// reach these store paths.
func TestCALV0361_StoreReapAfterWakeIsNotGapAware(t *testing.T) {
	t.Parallel()
	const asleep = 180 // minutes: a three-hour suspend of a 60-minute lease
	for _, tc := range []struct {
		name string
		reap func(t *testing.T, s *leaseStore, old *store.Report, two string) *store.Report
	}{
		{"colliding-claim", func(t *testing.T, s *leaseStore, old *store.Report, two string) *store.Report {
			return s.lease(t, "claim-2", claimOf(two, "src/a.go"), asleep, nil)
		}},
		{"reap", func(t *testing.T, s *leaseStore, old *store.Report, two string) *store.Report {
			return s.lease(t, "reap-1", transaction.LeaseRequest{Verb: transaction.LeaseReap}, asleep, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newLeaseStore(t)
			one, two := s.ticket(t, "one"), s.ticket(t, "two")
			old := s.claim(t, "claim-1", one, 0, "src/")
			woke := tc.reap(t, s, old, two)
			if woke.Outcome.Outcome != mutation.OutcomeCompleted || len(woke.Reaped) != 1 || woke.Reaped[0].AttemptID != old.AttemptID {
				t.Fatalf("first writer after the wake: %+v", woke)
			}
			a := s.attempt(t, old.AttemptID)
			if a.Phase != "FAILED" || a.Cause == nil || *a.Cause != snapshot.CauseLeaseExpired || a.Quiescence != "FENCED" {
				t.Fatalf("attempt reaped at the wake: %+v", a)
			}
			beat := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: old.AttemptID, Generation: old.Generation}
			refusedWith(t, s.lease(t, "beat-1", beat, asleep+1, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
			refusedWith(t, s.lease(t, "renew-1", renewOf(old), asleep+1, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
			auditOK(t, s.repo)
		})
	}
}
