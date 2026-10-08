package store_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func fencedReapOf(a *store.Report, expires wire.Timestamp) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: a.AttemptID, Generation: a.Generation, LeaseExpiresAt: expires}
}

// TestCALV0191_ReapIsFencedOnTheObservedLeaseExpiry: a reap that names the
// lease expiry its caller observed refuses, without a change, once a renewal
// moved that expiry, even after the renewed lease has itself expired; with
// the current expiry it reaps and its retry replays the receipt. A reap of
// an attempt released since the observation changes nothing and writes no
// receipt, which is how the dispatcher tells it apart from an actual reap.
func TestCALV0191_ReapIsFencedOnTheObservedLeaseExpiry(t *testing.T) {
	t.Run("CAL-V0-191 renewal between observation and reap", func(t *testing.T) {
		s := newLeaseStore(t)
		l := claimOf(s.ticket(t, "one"), "src")
		l.LeaseMinutes = "5"
		claim := s.lease(t, "claim-1", l, 0, nil)
		observed := s.attempt(t, claim.AttemptID).Lease.ExpiresAt
		if r := s.lease(t, "renew-1", renewOf(claim), 4, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("renew: %+v", r)
		}
		renewed := s.attempt(t, claim.AttemptID).Lease.ExpiresAt
		if renewed == observed {
			t.Fatalf("renewal kept expiry %s", observed)
		}
		// 70 minutes: past the renewed expiry too, so only the fence refuses.
		stale := s.lease(t, "reap-stale", fencedReapOf(claim, observed), 70, nil)
		refusedWith(t, stale, mutation.OutcomeRevisionConflict, wire.CodeFenced)
		if a := s.attempt(t, claim.AttemptID); !a.Live() || a.Lease.ExpiresAt != renewed {
			t.Fatalf("fenced reap changed the attempt: %+v", a)
		}
		reaped := s.lease(t, "reap-current", fencedReapOf(claim, renewed), 70, nil)
		if reaped.Outcome.Outcome != mutation.OutcomeCompleted || reaped.Kind == "NoChange" || reaped.Receipt == "" {
			t.Fatalf("reap at the current expiry: %+v", reaped)
		}
		if a := s.attempt(t, claim.AttemptID); a.Phase != "FAILED" || a.Cause == nil || *a.Cause != snapshot.CauseLeaseExpired {
			t.Fatalf("reaped attempt: %+v", a)
		}
		replay := s.lease(t, "reap-current", fencedReapOf(claim, renewed), 71, nil)
		if replay.Outcome.Outcome != mutation.OutcomeCompleted || !replay.Outcome.Replayed || replay.Outcome.ReceiptSeq == nil {
			t.Fatalf("replayed reap: %+v", replay)
		}
		auditOK(t, s.repo)
	})
	t.Run("CAL-V0-191 release between observation and reap", func(t *testing.T) {
		s := newLeaseStore(t)
		l := claimOf(s.ticket(t, "one"), "src")
		l.LeaseMinutes = "5"
		claim := s.lease(t, "claim-1", l, 0, nil)
		observed := s.attempt(t, claim.AttemptID).Lease.ExpiresAt
		if r := s.lease(t, "release-1", releaseOf(claim), 1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("release: %+v", r)
		}
		late := s.lease(t, "reap-late", fencedReapOf(claim, observed), 30, nil)
		if late.Outcome.Outcome != mutation.OutcomeCompleted || late.Kind != "NoChange" || late.Receipt != "" {
			t.Fatalf("reap after release: %+v", late)
		}
		if a := s.attempt(t, claim.AttemptID); a.Phase != "CANCELLED" {
			t.Fatalf("released attempt changed: %+v", a)
		}
		auditOK(t, s.repo)
	})
	t.Run("CAL-V0-191 expiry only with an attempt", func(t *testing.T) {
		s := newLeaseStore(t)
		malformed := func(id string, l transaction.LeaseRequest) {
			t.Helper()
			choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, Lease: l}
			if _, err := store.Lease(context.Background(), s.repo, operator(), choice, s.at(t, 1)); wire.CodeOf(err) != wire.CodeMalformed {
				t.Fatalf("%s: want MALFORMED, got %v", id, err)
			}
		}
		malformed("reap-bare", transaction.LeaseRequest{Verb: transaction.LeaseReap, LeaseExpiresAt: s.t0})
		claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src")
		renew := renewOf(claim)
		renew.LeaseExpiresAt = s.t0
		malformed("renew-expiry", renew)
		reap := fencedReapOf(claim, "2026-13-01T00:00:00Z")
		malformed("reap-bad-expiry", reap)
	})
}
