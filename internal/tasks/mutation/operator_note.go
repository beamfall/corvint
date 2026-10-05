package mutation

// ProposeOperatorNote is the pure transition; Context.note dispatches
// NOTE_SET/NOTE_CLEAR through it after Apply's replay and role checks.
import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// OperatorNoteContext is audited input, not data copied from a note payload.
// Allowed is a required explicit policy observation, including the OPERATOR row
// requirement. This helper cannot authenticate its caller or establish replay.
type OperatorNoteContext struct {
	Record        *ticket.Record
	Prior         *ticket.OperatorNoteReference
	PriorEvent    []byte
	Binding       Binding
	Allowed       bool
	QueueID       wire.QueueID
	RequestID     string
	RequestSha256 wire.Digest
	RecordedAt    wire.Timestamp
}

// OperatorNoteProposal is not a receipt or ticket post. Integration must commit
// its reference/blob and the ordinary finalized ticket as one MUTATE transaction.
type OperatorNoteProposal struct {
	Reference ticket.OperatorNoteReference
	Event     []byte
}

// OperatorNoteRefusal carries an existing outcome without inventing a wire
// detail code. No receipt or mutation result is published by this pure helper.
type OperatorNoteRefusal struct{ Outcome, Where, Detail string }

func (e *OperatorNoteRefusal) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Outcome, e.Where, e.Detail)
}

func ProposeOperatorNote(c OperatorNoteContext, raw []byte) (*OperatorNoteProposal, error) {
	q, err := ticket.DecodeOperatorNoteRequest(raw)
	if err != nil {
		return nil, err
	}
	if c.Binding.ID == "" || q.ActorID != c.Binding.ID || q.ActorRole != c.Binding.Role || !c.Allowed {
		return nil, &OperatorNoteRefusal{OutcomeUnauthorized, "/actor", "note requires matching trusted binding and explicit policy authorization"}
	}
	if q.QueueID.Raw != c.QueueID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/queueId", "audited queue mismatch")
	}
	if q.RequestID != c.RequestID {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/requestId", "audited request mismatch")
	}
	if wire.Sum(raw) != c.RequestSha256 {
		return nil, wire.Errorf(wire.CodeMalformed, "/requestSha256", "audited request digest mismatch")
	}
	if _, err = wire.ParseTimestamp("/recordedAt", string(c.RecordedAt)); err != nil {
		return nil, err
	}
	if c.Record == nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/ticketId", "missing audited ticket")
	}
	// Revalidate the explicit pre-record; never mutate the caller's record.
	pre, err := ticket.Decode(c.Record.Encode())
	if err != nil {
		return nil, err
	}
	if q.TicketID.Raw != pre.TicketID.Raw {
		return nil, wire.Errorf(wire.CodeMalformed, "/request/targetId", "audited ticket mismatch")
	}
	// Archived state refuses like the existing writer's tombstone rule; import or
	// shadow ownership is an authority refusal.
	if pre.Status == ticket.StatusArchived {
		return nil, &OperatorNoteRefusal{OutcomeBlocked, "/ticketId", "ticket is ARCHIVED; note requires an unarchived ticket"}
	}
	if pre.Source.Kind != "NATIVE" || pre.ShadowOverlay {
		return nil, &OperatorNoteRefusal{OutcomeUnauthorized, "/ticketId", "note requires a native ticket"}
	}
	if q.ExpectedRevision != nil && *q.ExpectedRevision != pre.Revision {
		return nil, &OperatorNoteRefusal{OutcomeRevisionConflict, "/request/expectedRevision", fmt.Sprintf("expected pre-ticket revision %s", pre.Revision)}
	}
	priorRevision := wire.Count("0")
	var previous *wire.Digest
	if c.Prior != nil {
		prior, err := ticket.ResolveOperatorNote(pre.TicketID, *c.Prior, c.PriorEvent)
		if err != nil {
			return nil, err
		}
		if prior.TicketRevision.Int() > pre.Revision.Int() || prior.AcceptanceRevision.Int() > pre.AcceptanceRevision.Int() {
			return nil, wire.Errorf(wire.CodeMalformed, "/previous", "prior note cannot postdate audited ticket revisions")
		}
		priorRevision = c.Prior.Revision
		head := c.Prior.Head
		previous = &head
	} else if len(c.PriorEvent) != 0 {
		return nil, wire.Errorf(wire.CodeMalformed, "/previous", "unexpected prior event without reference")
	}
	if q.Supersedes != nil && *q.Supersedes != priorRevision {
		return nil, &OperatorNoteRefusal{OutcomeRevisionConflict, "/request/payload/supersedes", fmt.Sprintf("expected prior note revision %s", priorRevision)}
	}
	if priorRevision.Int() >= ticket.MaxOperatorNoteEvents || pre.Revision.Int() >= wire.MaxCountValue {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/revision", "note or ticket revision capacity")
	}
	n := ticket.OperatorNoteEvent{TicketID: pre.TicketID, NoteRevision: wire.CountOf(priorRevision.Int() + 1), TicketRevision: wire.CountOf(pre.Revision.Int() + 1), AcceptanceRevision: pre.AcceptanceRevision, Operation: q.Operation[len("NOTE_"):], Previous: previous, ActorID: c.Binding.ID, ActorRole: c.Binding.Role, RecordedAt: c.RecordedAt, RequestSha256: c.RequestSha256, Request: q.Raw}
	blob, err := n.Encode()
	if err != nil {
		return nil, err
	}
	head := wire.Sum(blob)
	ref := ticket.OperatorNoteReference{Revision: n.NoteRevision, Head: head}
	if n.Operation == "SET" {
		current := head
		ref.Current = &current
	}
	return &OperatorNoteProposal{Reference: ref, Event: blob}, nil
}

// ValidateOperatorNoteMaterial applies the same semantic pre-state bindings to
// supplied event material. Matching digests alone do not establish either CAS.
// A future stage/redo adapter must call this with independently audited context.
func ValidateOperatorNoteMaterial(c OperatorNoteContext, ref ticket.OperatorNoteReference, raw []byte) error {
	n, err := ticket.DecodeOperatorNoteEvent(raw)
	if err != nil {
		return err
	}
	p, err := ProposeOperatorNote(c, n.Request)
	if err != nil {
		return err
	}
	if !bytes.Equal(p.Event, raw) || !bytes.Equal(wire.EncodeFile(p.Reference.Value()), wire.EncodeFile(ref.Value())) {
		return wire.Errorf(wire.CodeMalformed, "/operatorNote", "event/reference differs from audited transition")
	}
	return nil
}

// note applies NOTE_SET/NOTE_CLEAR (ON-V0-003/004): the reference changes, the
// ticket revision advances by one, acceptanceRevision is preserved, and the
// derived note event rides in Plan.DerivedEvent. A capacity failure maps to
// VALIDATION_FAILED/LIMIT_EXCEEDED like the ticket-revision overflow; that is
// the recorded conservative default (ON-V0 open decision 1).
func (c Context) note(plan *Plan, env *Envelope) *Plan {
	pre, ok := c.Inventory.Get(env.TargetID.Raw)
	if !ok {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "target %s does not exist in the queue", env.TargetID.Raw))
	}
	plan.Pre = pre
	if pre.OperatorNote != nil && len(c.PriorNoteEvent) == 0 {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMissingEvidence, "operator note head %s is unavailable", pre.OperatorNote.Head))
	}
	prop, err := ProposeOperatorNote(OperatorNoteContext{
		Record: pre, Prior: pre.OperatorNote, PriorEvent: c.PriorNoteEvent, Binding: c.Binding,
		Allowed: true, QueueID: c.Queue.QueueID, RequestID: env.RequestID,
		RequestSha256: env.Sha256(), RecordedAt: c.Now,
	}, env.Raw)
	if err != nil {
		var nr *OperatorNoteRefusal
		if errors.As(err, &nr) {
			code := ""
			if nr.Outcome == OutcomeBlocked {
				code = wire.CodeTicketState
			}
			return plan.refused(refuse(nr.Outcome, code, "%s: %s", nr.Where, nr.Detail))
		}
		return plan.refused(refuseErr(err))
	}
	work, err := clone(pre)
	if err != nil {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeOf(err), "canonical record is not valid: %v", err))
	}
	ref := prop.Reference
	work.OperatorNote = &ref
	if r := c.finalize(pre, work, false); r != nil {
		return plan.refused(r)
	}
	if work.Revision.Int() != pre.Revision.Int()+1 || work.AcceptanceRevision != pre.AcceptanceRevision {
		return plan.refused(refuse(OutcomeValidationFailed, wire.CodeMalformed, "note transition must advance only the ticket revision"))
	}
	plan.DerivedEvent = prop.Event
	return plan.completed(work)
}
