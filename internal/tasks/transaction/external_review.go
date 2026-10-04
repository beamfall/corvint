package transaction

import (
	"bytes"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"reflect"
)

// These supplied facts are a pure model, not native observation or authority.
// The future locked writer must produce and audit them; none are CLI flags.
type ExternalReviewBinding struct {
	GateID                         string
	TicketID                       string
	AcceptanceRevision             wire.Count
	DefinitionSha256, PolicySha256 wire.Digest
	Subject                        snapshot.ExternalReviewSubject
	Candidate                      snapshot.ExternalReviewCandidate
}
type ExternalReviewPolicy struct {
	RecorderRoles, ReviewStages, AuthorStages []string
	RequireReviewerLease                      bool
}
type ExternalReviewLeaseObservation struct {
	State, TicketID, Stage string
	Lease                  snapshot.ExternalReviewLease
}
type ExternalReviewContext struct {
	QueueID, TicketID, RequestID string
	RequestSha256                wire.Digest
	Pre                          *snapshot.ExternalReviewRef
	// Post is an independently supplied audited post-reference for recovery.
	Post *snapshot.ExternalReviewRef
}
type ExternalReviewObservations struct {
	Actor                                                        mutation.Binding
	PolicyState, SubjectState, CandidateLinkState, EvidenceState string
	Binding                                                      ExternalReviewBinding
	Policy                                                       ExternalReviewPolicy
	Reviewer, Author                                             ExternalReviewLeaseObservation
	Current                                                      *snapshot.ExternalReviewRef
	CurrentEvent                                                 []byte
	Context                                                      ExternalReviewContext
	ReplayState                                                  string
	ReplayEvent                                                  []byte
	RecordedAt                                                   wire.Timestamp
	ReceiptSeq                                                   wire.Size
}
type ExternalReviewTransition struct {
	Kind  string
	Event []byte
	Ref   *snapshot.ExternalReviewRef
}
type ExternalReviewView struct {
	Verdict                                        *string
	Generation, Revision                           wire.Count
	Resubmitted                                    bool
	Status                                         string
	Candidate                                      *snapshot.ExternalReviewCandidate
	Subject                                        *snapshot.ExternalReviewSubject
	ActorAuthentication, Independence, TrustSource string
	EvidenceSha256                                 *wire.Digest
}

func externalHas(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func externalRefEqual(a, b *snapshot.ExternalReviewRef) bool { return reflect.DeepEqual(a, b) }
func externalRefEvent(ref *snapshot.ExternalReviewRef, raw []byte) (*snapshot.ExternalReviewEvent, error) {
	if ref == nil {
		return nil, fmt.Errorf("reference missing")
	}
	if _, err := ref.Encode(); err != nil {
		return nil, err
	}
	e, err := snapshot.CanonicalExternalReviewEvent(raw)
	if err != nil {
		return nil, err
	}
	if wire.Sum(raw) != ref.Head || e.ReviewGeneration != ref.Generation || e.EventRevision != ref.Revision {
		return nil, fmt.Errorf("head binding differs")
	}
	return e, nil
}

// ExternalReviewCurrent returns actionable state only from a bound canonical
// head and explicit current acceptance/definition/subject snapshot. Policy and
// unrelated content revisions are historical, not implicit staleness rules.
func ExternalReviewCurrent(ref *snapshot.ExternalReviewRef, raw []byte, current *ExternalReviewBinding) ExternalReviewView {
	v := ExternalReviewView{Generation: "0", Revision: "0", Status: "UNKNOWN"}
	if ref == nil {
		if len(raw) == 0 {
			v.Status = "NONE"
		}
		return v
	}
	e, err := externalRefEvent(ref, raw)
	if err != nil || current == nil {
		return v
	}
	q := e.Request
	if q.TicketID != current.TicketID || q.GateID != current.GateID {
		return v
	}
	v = ExternalReviewView{Verdict: q.Verdict, Generation: ref.Generation, Revision: ref.Revision, Resubmitted: q.Action == "RESUBMIT", Status: "CURRENT", Candidate: &q.Candidate, Subject: &q.Subject, ActorAuthentication: "NOT_OBSERVED", Independence: "NOT_OBSERVED", TrustSource: e.TrustSource, EvidenceSha256: &ref.Head}
	if q.AcceptanceRevision != current.AcceptanceRevision || q.DefinitionSha256 != current.DefinitionSha256 || q.Subject != current.Subject || q.Candidate != current.Candidate {
		v.Status = "STALE"
	}
	return v
}
func externalLeaseMatches(q *snapshot.ExternalReviewLease, o ExternalReviewLeaseObservation, actor mutation.Binding, ticket string, stages []string) bool {
	return q != nil && o.State == "LIVE" && o.TicketID == ticket && o.Lease == *q && q.Holder == actor.ID && externalHas(stages, o.Stage)
}

// ApplyExternalReview validates supplied observations and produces a canonical
// hypothetical event. It writes nothing and cannot satisfy executable gates.
func ApplyExternalReview(q snapshot.ExternalReviewRequest, o ExternalReviewObservations) (ExternalReviewTransition, error) {
	fail := func(reason string) (ExternalReviewTransition, error) {
		return ExternalReviewTransition{Kind: "REFUSED"}, fmt.Errorf("external review: %s", reason)
	}
	if _, err := wire.ParseLabel("/binding/id", o.Actor.ID); err != nil || !externalHas([]string{"OWNER", "OPERATOR", "REVIEWER", "WORKER"}, o.Actor.Role) {
		return fail("trusted actor binding")
	}
	raw, err := q.Encode()
	if err != nil {
		return fail("request shape: " + err.Error())
	}
	// Replay follows actor validation and precedes fresh policy/CAS eligibility.
	switch o.ReplayState {
	case "FOUND":
		e, err := snapshot.CanonicalExternalReviewEvent(o.ReplayEvent)
		if err != nil {
			return fail("replay evidence")
		}
		old, err := e.Request.Encode()
		if err != nil || !bytes.Equal(old, raw) || e.ActorID != o.Actor.ID || e.ActorRole != o.Actor.Role {
			return fail("request ID conflict")
		}
		return ExternalReviewTransition{Kind: "REPLAY", Event: append([]byte(nil), o.ReplayEvent...), Ref: &snapshot.ExternalReviewRef{Generation: e.ReviewGeneration, Revision: e.EventRevision, Head: wire.Sum(o.ReplayEvent)}}, nil
	case "ABSENT":
	default:
		return fail("replay observation unknown")
	}
	if o.PolicyState != "VERIFIED" || o.SubjectState != "VERIFIED" || o.CandidateLinkState != "VERIFIED" || o.EvidenceState != "VERIFIED" {
		return fail("required observation unknown")
	}
	b := o.Binding
	if b.TicketID != q.TicketID || b.GateID != q.GateID || b.AcceptanceRevision != q.AcceptanceRevision || b.DefinitionSha256 != q.DefinitionSha256 || b.PolicySha256 != q.PolicySha256 {
		return fail("current acceptance/definition/policy")
	}
	if b.Subject != q.Subject || b.Candidate != q.Candidate {
		return fail("subject receipt POST/candidate")
	}
	id, err := wire.ParseTicketID("/ticketId", q.TicketID)
	if err != nil {
		return fail("ticket identity")
	}
	ctx := o.Context
	if ctx.QueueID != id.QueueID() || ctx.TicketID != q.TicketID || ctx.RequestID != q.RequestID || ctx.RequestSha256 != wire.Sum(raw) {
		return fail("descriptor/receipt context")
	}
	if !externalRefEqual(ctx.Pre, o.Current) {
		return fail("audited pre-reference")
	}
	var old *snapshot.ExternalReviewEvent
	generation, revision := wire.Count("0"), wire.Count("0")
	if o.Current != nil {
		old, err = externalRefEvent(o.Current, o.CurrentEvent)
		if err != nil || old.Request.TicketID != q.TicketID || old.Request.GateID != q.GateID {
			return fail("current head binding")
		}
		generation, revision = o.Current.Generation, o.Current.Revision
	} else if len(o.CurrentEvent) != 0 {
		return fail("orphan current event")
	}
	if q.ExpectedGeneration != generation || q.ExpectedRevision != revision {
		return fail("generation/revision CAS")
	}
	if revision.Int() >= snapshot.MaxExternalReviewEvents {
		return fail("event history capacity")
	}
	previous := (*wire.Digest)(nil)
	if o.Current != nil {
		head := o.Current.Head
		previous = &head
	}
	nextGeneration := generation
	source := "LEASE_BOUND"
	if q.Action == "RESUBMIT" {
		if old == nil || old.Request.Verdict == nil || *old.Request.Verdict != "RETURN" || q.PriorReturn == nil || *q.PriorReturn != o.Current.Head {
			return fail("prior RETURN binding")
		}
		if !externalLeaseMatches(q.AuthorLease, o.Author, o.Actor, q.TicketID, o.Policy.AuthorStages) {
			return fail("author lease")
		}
		nextGeneration = wire.CountOf(generation.Int() + 1)
	} else {
		if o.Actor.Role == "WORKER" || !externalHas(o.Policy.RecorderRoles, o.Actor.Role) {
			return fail("recorder role")
		}
		if q.ReviewerLease == nil {
			if o.Policy.RequireReviewerLease || !externalHas([]string{"OWNER", "OPERATOR"}, o.Actor.Role) {
				return fail("reviewer lease required")
			}
			source = "OPERATOR_ATTESTED"
		} else if !externalLeaseMatches(q.ReviewerLease, o.Reviewer, o.Actor, q.TicketID, o.Policy.ReviewStages) {
			return fail("reviewer lease")
		}
		if old == nil {
			nextGeneration = "1"
		} else {
			view := ExternalReviewCurrent(o.Current, o.CurrentEvent, &b)
			if view.Status == "UNKNOWN" {
				return fail("current state unknown")
			}
			if view.Status == "STALE" {
				// A resubmission can become stale before its reviewer responds.
				// Fresh authorized evidence opens a new cycle without reviving
				// old approval or requiring an impossible second RETURN head.
				stalePass := old.Request.Verdict != nil && *old.Request.Verdict == "PASS"
				if !stalePass && old.Request.Action != "RESUBMIT" {
					return fail("stale RETURN requires explicit resubmit")
				}
				nextGeneration = wire.CountOf(generation.Int() + 1)
			}
		}
	}
	event := snapshot.ExternalReviewEvent{Request: q, ReviewGeneration: nextGeneration, EventRevision: wire.CountOf(revision.Int() + 1), ActorID: o.Actor.ID, ActorRole: o.Actor.Role, TrustSource: source, Previous: previous, RecordedAt: o.RecordedAt, ReceiptSeq: o.ReceiptSeq}
	out, err := event.Encode()
	if err != nil {
		return fail("event: " + err.Error())
	}
	ref := &snapshot.ExternalReviewRef{Generation: nextGeneration, Revision: event.EventRevision, Head: wire.Sum(out)}
	if ctx.Post != nil && !externalRefEqual(ctx.Post, ref) {
		return fail("audited post-reference")
	}
	return ExternalReviewTransition{Kind: "EVENT", Event: out, Ref: ref}, nil
}

// ValidateExternalReviewRecovery checks the actual retained event and post
// against the same audited material transition. A valid digest alone is not a
// fulfilled precondition; the future native recovery wrapper must supply facts.
func ValidateExternalReviewRecovery(raw []byte, post snapshot.ExternalReviewRef, o ExternalReviewObservations) error {
	e, err := snapshot.CanonicalExternalReviewEvent(raw)
	if err != nil {
		return err
	}
	o.Context.Post = &post
	expected, err := ApplyExternalReview(e.Request, o)
	if err != nil {
		return err
	}
	if expected.Kind != "EVENT" || !bytes.Equal(expected.Event, raw) || !externalRefEqual(expected.Ref, &post) {
		return fmt.Errorf("external review: recovery produced event differs")
	}
	return nil
}
