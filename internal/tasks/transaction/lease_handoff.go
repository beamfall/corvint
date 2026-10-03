package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
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
	if a.ConfigSha256 != a.PolicySha256 || (a.PolicySha256 != wire.Sum(c.st.policy.Raw) && !c.handoffPolicyCompatible(a)) {
		return refuse(wire.CodeStalePolicy, "handoff policy changed")
	}
	return c.verifyHandoffWork(a)
}

func (c leaseContext) verifyHandoffWork(a *snapshot.Attempt) *leaseOutcome {
	refuse := func(code, detail string) *leaseOutcome {
		out := c.refuse(mutation.OutcomeBlocked, code, detail)
		return &out
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

// HandoffPolicyCandidate runs only after the ordinary reducer refused stale
// policy. Invalid or already replayed requests retain their ordinary ordering
// without paying for a second audit. The reducer rechecks all eligibility later.
func HandoffPolicyCandidate(r Request, in Input, result Result) *snapshot.Attempt {
	if r.Operation != Lease || r.Lease == nil || r.Lease.Verb != LeaseRelease || (r.Lease.Reason != wire.CodeHandoff && r.Lease.Reason != wire.CodeReviewReturned) || result.Kind != "Refused" || len(result.Outcome.Codes) != 1 || result.Outcome.Codes[0] != wire.CodeStalePolicy || in.Replay.State != "ABSENT" {
		return nil
	}
	st, err := validateInput(r, in)
	if err != nil {
		return nil
	}
	c := leaseContext{r: r, l: r.Lease, in: in, st: st}
	a := st.attempts[r.Lease.AttemptID]
	if a == nil || a.Supervision != nil || c.fenced(a) != "" || a.ConfigSha256 != a.PolicySha256 || a.PolicySha256 == wire.Sum(st.policy.Raw) || c.verifyHandoffWork(a) != nil {
		return nil
	}
	return a
}

func (c leaseContext) handoffPolicyCompatible(a *snapshot.Attempt) bool {
	h := c.in.HandoffPolicy
	if h == nil || !h.Compatible || h.AttemptID != a.AttemptID || h.Generation != a.Generation || h.HeadSha256 != wire.Sum(c.in.Head) || h.LastSeq != c.st.head.LastSeq || h.LastReceiptSha256 != wire.Sum(c.in.HeadReceipt) || h.FinalPolicySha256 != wire.Sum(c.st.policy.Raw) {
		return false
	}
	for _, seq := range []wire.Size{h.OriginalSeq, h.FirstAttemptSeq} {
		if _, err := wire.ParseSize("handoff history sequence", string(seq)); err != nil {
			return false
		}
	}
	if h.OriginalPath != "intent/policy.json" || h.OriginalSeq.Uint64() == 0 || h.OriginalSeq.Uint64() >= h.FirstAttemptSeq.Uint64() || h.FirstAttemptSeq.Uint64() > h.LastSeq.Uint64() || h.FirstAttemptPath != attemptPath(a.AttemptID) || h.OriginalSha256 != a.PolicySha256 || wire.Sum(h.OriginalRaw) != a.PolicySha256 || h.FirstPolicySha256 != a.PolicySha256 || h.FirstConfigSha256 != a.ConfigSha256 || h.FirstCapabilitySha256 != a.CapabilityProfileSha256 {
		return false
	}
	for _, digest := range []wire.Digest{h.OriginalReceiptSha256, h.FirstAttemptReceiptSha256, h.FirstAttemptSha256} {
		if _, err := wire.ParseDigest("handoff history", string(digest)); err != nil {
			return false
		}
	}
	if canonical(h.OriginalRaw) != nil {
		return false
	}
	allocation := wire.Null()
	pool, member := "", ""
	if a.PoolAllocation != nil {
		allocation = snapshot.PoolAllocationValue(a.PoolAllocation)
		pool, member = a.PoolAllocation.PoolID, a.PoolAllocation.MemberID
	}
	if h.PoolID != pool || h.MemberID != member || h.FirstAllocationSha256 != wire.Sum(wire.EncodeFile(allocation)) {
		return false
	}
	compatible, err := intent.HandoffPolicyCompatible(h.OriginalRaw, c.st.policy.Raw, pool, member)
	return err == nil && compatible
}
