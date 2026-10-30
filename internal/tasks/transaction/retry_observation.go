package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func emptyRetryReasons() map[string]wire.Count {
	out := map[string]wire.Count{}
	for _, k := range snapshot.RetryReasonNames {
		out[k] = "0"
	}
	return out
}
func retryReasons(a *snapshot.Attempt) map[string]wire.Count {
	out := emptyRetryReasons()
	if a.RetryReasons == nil {
		out["UNKNOWN"] = a.RetryCount
		return out
	}
	for _, k := range snapshot.RetryReasonNames {
		out[k] = a.RetryReasons[k]
	}
	return out
}

// Only terminal state identifies a charge's reason. Failure-accounting flags
// establish debt eligibility, not an authenticated historical cause.
func chargedReason(a *snapshot.Attempt) string {
	if a.Phase == "FAILED" && a.Cause != nil && *a.Cause == snapshot.CauseLeaseExpired {
		return "EXPIRED"
	}
	if a.Phase == "FAILED" {
		return "FAILED"
	}
	if a.Phase == "CANCELLED" {
		return "RELEASED"
	}
	return "UNKNOWN"
}

// RetryObservation uses exactly the admission predicate for the current
// acceptance revision; remaining is retry capacity, not initial-admission count.
func RetryObservation(attempts map[string]*snapshot.Attempt, rec *ticket.Record, limit int64) wire.Value {
	a := lastAttemptOf(attempts, rec.TicketID.Raw)
	charged := int64(0)
	reasons := emptyRetryReasons()
	if a != nil && a.TicketRevision == rec.AcceptanceRevision {
		charged = a.RetryCount.Int()
		reasons = retryReasons(a)
	}
	remaining := limit - charged
	if remaining < 0 {
		remaining = 0
	}
	history := "COMPLETE"
	if reasons["UNKNOWN"].Int() > 0 {
		history = "INCOMPLETE"
	}
	reason := "RETRY_AVAILABLE"
	switch {
	case a == nil:
		reason = "INITIAL_ADMISSION"
	case a.TicketRevision != rec.AcceptanceRevision:
		reason = "NEW_ACCEPTANCE"
	case a.Phase == "COMPLETED":
		reason = "COMPLETED_ATTEMPT"
	case cleanHandoff(a):
		reason = "VERIFIED_HANDOFF"
	case retryExhausted(attempts, rec, limit):
		reason = "RETRY_EXHAUSTED"
	}
	return wire.ObjectValue(wire.NewObject().Set("remainingMeaning", wire.String("RETRY_CAPACITY")).Set("retryAdmissionReason", wire.String(reason)).Set("charged", wire.String(string(wire.CountOf(charged)))).Set("limit", wire.String(string(wire.CountOf(limit)))).Set("remaining", wire.String(string(wire.CountOf(remaining)))).Set("exhausted", wire.Bool(retryExhausted(attempts, rec, limit))).Set("byReason", snapshot.RetryReasonsValue(reasons)).Set("reasonHistory", wire.String(history)))
}

// NextStageStale is the CAL-V0-084 value of a recorded next stage whose
// acceptance revision has since changed.
const NextStageStale = "STALE"

// NextStage derives CAL-V0-084 `nextStage` from the ticket's latest attempt
// generation: its recorded next stage when that generation is terminal and
// bound to the current acceptance revision, STALE when it records one at an
// earlier revision, and null otherwise. It is advisory and never gates a
// claim.
func NextStage(attempts map[string]*snapshot.Attempt, rec *ticket.Record) wire.Value {
	a := lastAttemptOf(attempts, rec.TicketID.Raw)
	if a == nil {
		return wire.Null()
	}
	stage := a.NextStage()
	switch {
	case stage == "":
		return wire.Null()
	case a.TicketRevision != rec.AcceptanceRevision:
		return wire.String(NextStageStale)
	}
	return wire.String(stage)
}
