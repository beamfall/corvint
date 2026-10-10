package store_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// loopHandoffs claims id at stage implement n times and releases each
// generation as a clean no-tree hand-off, returning the last claim.
func loopHandoffs(t *testing.T, s *leaseStore, id, prefix string, n, minute int) *store.Report {
	t.Helper()
	var a *store.Report
	for i := 0; i < n; i++ {
		a = handoffClaim(t, s, id, "implement", fmt.Sprintf("%s-claim-%d", prefix, i), minute)
		l := releaseOf(a)
		l.Reason, l.Evidence = wire.CodeHandoff, "local:no-change"
		if r := s.lease(t, fmt.Sprintf("%s-handoff-%d", prefix, i), l, minute, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("handoff %d: %+v", i, r)
		}
	}
	return a
}

// TestCALV0102_StoreLoopHoldAndOwnerReopen proves CAL-V0-102 and CAL-V0-103
// through the store: under an opted-in policy each retry records its ended
// generation's loopEvidence, the third consecutive no-progress hand-off holds
// the ticket LOOP_DETECTED on claim without rewriting its status, the owner
// reopen acknowledges it with a new acceptance revision and a fresh attempt
// admits. Without the policy the same history records no loopEvidence key
// and keeps admitting (D8).
func TestCALV0102_StoreLoopHoldAndOwnerReopen(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-102 CAL-V0-103 StoreLoopHoldAndOwnerReopen", func(t *testing.T) {
		s := newLeaseStore(t)
		handoffPolicyUpdate(t, s, func(v wire.Value) {
			v.Obj.Set("loopDetection", obj("maxNoProgressGenerations", str("2"), "maxAlternatingReturns", str("2")))
		})
		id := s.ticket(t, "looping")
		last := loopHandoffs(t, s, id, "loop", 3, 1)
		a := s.attempt(t, last.AttemptID)
		if len(a.PriorGenerations) != 2 {
			t.Fatalf("prior %+v", a.PriorGenerations)
		}
		for _, p := range a.PriorGenerations {
			if p.History == nil || p.History.Loop == nil || p.History.Loop.Disposition != wire.CodeHandoff || p.History.Loop.CandidateTreeOid != nil {
				t.Fatalf("prior evidence %+v", p.History)
			}
		}
		before := s.record(t, id)
		refusedWith(t, s.lease(t, "held", claimOf(id, "src/"), 2, nil), mutation.OutcomeBlocked, wire.CodeLoopDetected)
		next := claimOf("", "src/")
		next.Verb = transaction.LeaseClaimNext
		refusedWith(t, s.lease(t, "held-next", next, 2, nil), mutation.OutcomeBlocked, wire.CodeLoopDetected)
		if after := s.record(t, id); after.Status != before.Status || after.Revision != before.Revision {
			t.Fatalf("hold rewrote the ticket: %+v", after)
		}

		req := envelope("ack-loop", mutation.OpReopen, id, string(before.Revision), obj("reason", str("owner acknowledges the no-progress loop")))
		r, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 3))
		if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted || *r.Outcome.ResultingAcceptanceRevision == before.AcceptanceRevision {
			t.Fatalf("owner reopen: %+v %v", r, err)
		}
		fresh := s.claim(t, "fresh", id, 4, "src/")
		if fresh.AttemptID == last.AttemptID || s.attempt(t, fresh.AttemptID).TicketRevision == before.AcceptanceRevision {
			t.Fatalf("fresh attempt %+v", fresh)
		}
		auditOK(t, s.repo)

		plain := newLeaseStore(t)
		other := plain.ticket(t, "looping")
		end := loopHandoffs(t, plain, other, "plain", 4, 1)
		raw, err := os.ReadFile(filepath.Join(plain.repo.StateDir, "attempts", end.AttemptID+".json"))
		if err != nil || bytes.Contains(raw, []byte("loopEvidence")) {
			t.Fatalf("policy-absent attempt gained loopEvidence: %v", err)
		}
		auditOK(t, plain.repo)
	})
}

// TestCALV0103_ReopenRefusesUnheldTicket: under the opted-in policy an
// owner reopen of a ticket that is neither retry-exhausted nor loop-held
// still refuses with the unchanged recovery reason.
func TestCALV0103_ReopenRefusesUnheldTicket(t *testing.T) {
	t.Parallel()
	t.Run("CAL-V0-103 ReopenRefusesUnheldTicket", func(t *testing.T) {
		s := newLeaseStore(t)
		handoffPolicyUpdate(t, s, func(v wire.Value) {
			v.Obj.Set("loopDetection", obj("maxNoProgressGenerations", str("2"), "maxAlternatingReturns", str("2")))
		})
		id := s.ticket(t, "progressing")
		loopHandoffs(t, s, id, "loop", 2, 1)
		rec := s.record(t, id)
		req := envelope("ack-loop", mutation.OpReopen, id, string(rec.Revision), obj("reason", str("nothing to acknowledge")))
		r, err := store.Mutate(context.Background(), s.repo, operator(), req, s.at(t, 3))
		if err != nil || r.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("reopen admitted an unheld ticket: %+v %v", r, err)
		}
		s.claim(t, "still-claimable", id, 4, "src/")
	})
}
