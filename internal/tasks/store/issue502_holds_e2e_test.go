package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestIssue502_WriterReferenceHoldsAndReleases is ESC-V0-006 end to end over
// a reference the native writer produced: after the holder raises a decision
// question and releases its attempt, direct claim and claim-next refuse as
// ESCALATION_PENDING naming the question and write nothing; the operator's
// answer lifts the derived hold with no further write, and the ticket claims.
func TestIssue502_WriterReferenceHoldsAndReleases(t *testing.T) {
	s, id, src := escalationClaim(t)
	committed(t, escalate(t, s, holder, openRequest(t, "q-1", src, "", ""), 1), "OPEN")
	released := s.lease(t, "release-1", transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: src.AttemptID, Generation: src.Generation}, 2, nil)
	if released.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v", released)
	}
	before := storeDigest(t, s.repo)
	direct := s.lease(t, "claim-2", claimOf(id), 3, nil)
	refusedWith(t, direct, mutation.OutcomeBlocked, wire.CodeEscalationPending)
	if direct.Detail != "escalation questions pending: q-1" {
		t.Fatalf("direct claim detail %q", direct.Detail)
	}
	next := s.lease(t, "next-2", claimNext, 3, nil)
	refusedWith(t, next, mutation.OutcomeBlocked, wire.CodeEscalationPending)
	if next.AttemptID != "" || next.Detail != "no ticket is SELECTED; the first of 1 planned tickets, "+id+", is BLOCKED ESCALATION_PENDING on q-1" {
		t.Fatalf("claim-next took or misreported a held ticket: %+v", next)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("a refused claim wrote")
	}
	committed(t, answer(t, s, operator(), answerRequest(t, "a-1", id, operator(), "", ""), 4), "ANSWER")
	if c := s.claim(t, "claim-3", id, 5); c.Ticket != id {
		t.Fatalf("claim after answer: %+v", c)
	}
	auditOK(t, s.repo)
}
