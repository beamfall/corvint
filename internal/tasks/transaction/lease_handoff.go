package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// cleanHandoff reads writer-produced journal state, never a caller's reason
// alone. Legacy and supervised attempts retain ordinary retry accounting.
func cleanHandoff(a *snapshot.Attempt) bool {
	if a == nil || a.RuntimeID != snapshot.RuntimeExternalAgent || a.Phase != "CANCELLED" || a.RetryAccounting == nil {
		return false
	}
	x := a.RetryAccounting
	return !x.FailedOrUnknown && (x.Disposition == wire.CodeHandoff || x.Disposition == wire.CodeReviewReturned)
}

func exhaustedAttempt(a *snapshot.Attempt, limit int64) bool {
	return a != nil && a.Phase != "COMPLETED" && a.RetryCount.Int() >= limit && !cleanHandoff(a)
}

// verifyHandoff checks recorded eligibility, not physical quiescence, actor
// authentication, independent review, or the truth of external-agent feedback.
// The ordinary release path already fenced the generation and lease expiry.
func (c leaseContext) verifyHandoff(a *snapshot.Attempt) *leaseOutcome {
	refuse := func(code, detail string) *leaseOutcome {
		out := c.refuse(mutation.OutcomeBlocked, code, detail)
		return &out
	}
	rec, _ := c.st.tickets.Get(a.TicketID.Raw)
	if rec == nil || rec.AcceptanceRevision != a.TicketRevision {
		return refuse(wire.CodeStaleTicket, "handoff acceptance revision changed")
	}
	if a.PolicySha256 != wire.Sum(c.st.policy.Raw) || a.ConfigSha256 != a.PolicySha256 {
		return refuse(wire.CodeStalePolicy, "handoff policy changed")
	}
	if a.RuntimeID != snapshot.RuntimeExternalAgent || (a.Stage != "implement" && a.Stage != "review" && a.Stage != "integrate") || (c.l.Reason == wire.CodeReviewReturned && a.Stage != "review") {
		return refuse(wire.CodeTicketState, "handoff requires implement/review/integrate; REVIEW_RETURNED requires review")
	}
	if a.RetryAccounting == nil || a.RetryAccounting.FailedOrUnknown || a.RetryAccounting.Disposition != "NONE" {
		return refuse(wire.CodeMissingEvidence, "generation has no clean prospective accounting evidence")
	}
	if c.l.Evidence != "" {
		if a.Phase != "RUNNING" || a.CandidateTreeOid != nil || a.ScopeCheck != "UNKNOWN" || len(a.GateResults) != 0 || len(a.PendingEffects) != 0 {
			return refuse(wire.CodeMissingEvidence, "external evidence handoff requires RUNNING, no candidate, no gates and no pending effects")
		}
		return nil
	}
	if (a.Phase != "BUILT" && a.Phase != "CHECKING") || a.CandidateTreeOid == nil || a.ScopeCheck != "WITHIN" || len(a.PendingEffects) != 0 {
		return refuse(wire.CodeMissingEvidence, "handoff needs a scope-checked submitted candidate and no pending effects")
	}
	return nil
}
