package mutation

import (
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// External review operations (ERG-V0-009). Their envelope may carry a null
// expectedRevision; their payload is {request}, the closed
// taskman-external-review-request/0 object, which the transaction layer
// decodes and binds (this package cannot import snapshot).
const (
	OpReviewRecord   = "REVIEW_RECORD"
	OpReviewResubmit = "REVIEW_RESUBMIT"
)

// IsReviewOperation reports whether op is REVIEW_RECORD or REVIEW_RESUBMIT.
func IsReviewOperation(op string) bool { return op == OpReviewRecord || op == OpReviewResubmit }

// ReviewPayload carries the review request value unchanged; its canonical
// file bytes are what the event retains and digests.
type ReviewPayload struct {
	Op      string
	Request wire.Value
}

func (p *ReviewPayload) operation() string { return p.Op }

func readReview(op string, r *wire.Reader) *ReviewPayload {
	r.Closed(PayloadKeys[op]...)
	q := r.Field("request")
	if r.Err() == nil && q.Value().Kind != wire.KindObject {
		q.Fail(wire.CodeMalformed, "request must be an object")
	}
	return &ReviewPayload{Op: op, Request: q.Value()}
}

// ExternalReviewPost is the transaction layer's audited result for a review
// operation: either the per-gate reference and canonical event to post, or a
// refusal that the reducer or an observation produced. Authority (recorder
// roles, leases, CAS, subject and candidate binding) is decided there, from
// the trusted binding and the audited inventory; this package only applies it.
type ExternalReviewPost struct {
	GateID string
	Ref    ticket.ExternalReviewRef
	Event  []byte
	// Outcome is non-empty for a refusal, with its optional wire code.
	Outcome, Code, Detail string
}

// review applies REVIEW_RECORD/REVIEW_RESUBMIT: exactly one gate reference
// changes, the ticket revision advances by one, acceptanceRevision is
// preserved (a verdict is routing evidence, not acceptance), and the event
// rides in Plan.DerivedEvent.
func (c Context) review(plan *Plan, env *Envelope) *Plan {
	pre, ok := c.Inventory.Get(env.TargetID.Raw)
	if !ok {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "target %s does not exist in the queue", env.TargetID.Raw))
	}
	plan.Pre = pre
	if pre.Status == ticket.StatusArchived {
		return plan.refused(refuse(OutcomeBlocked, wire.CodeTicketState, "ticket is ARCHIVED; a review requires an unarchived ticket"))
	}
	if pre.Source.Kind != "NATIVE" || pre.ShadowOverlay {
		return plan.refused(refuse(OutcomeUnauthorized, "", "a review requires a native ticket"))
	}
	if env.ExpectedRevision != nil && *env.ExpectedRevision != pre.Revision {
		return plan.refused(refuse(OutcomeRevisionConflict, "", "expectedRevision %s but the canonical record is at revision %s", *env.ExpectedRevision, pre.Revision))
	}
	x := c.ExternalReview
	if x == nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "no audited external review observation"))
	}
	if x.Outcome != "" {
		return plan.refused(refuse(x.Outcome, x.Code, "%s", x.Detail))
	}
	if x.Event == nil || wire.Sum(x.Event) != x.Ref.Head {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "external review event does not match its reference"))
	}
	work, err := clone(pre)
	if err != nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeOf(err), "canonical record is not valid: %v", err))
	}
	if work.ExternalReviews == nil {
		work.ExternalReviews = map[string]ticket.ExternalReviewRef{}
	}
	if _, exists := work.ExternalReviews[x.GateID]; !exists && len(work.ExternalReviews) >= ticket.MaxExternalReviewGates {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeLimitExceeded, "ticket already carries %d external review gates", ticket.MaxExternalReviewGates))
	}
	work.ExternalReviews[x.GateID] = x.Ref
	if r := c.finalize(pre, work, false); r != nil {
		return plan.refused(r)
	}
	if work.Revision.Int() != pre.Revision.Int()+1 || work.AcceptanceRevision != pre.AcceptanceRevision {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "review transition must advance only the ticket revision"))
	}
	plan.DerivedEvent = append([]byte(nil), x.Event...)
	return plan.completed(work)
}
