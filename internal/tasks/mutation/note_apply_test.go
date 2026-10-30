package mutation_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0003_ApplyNoteRefusalMapping drives NOTE_SET through mutation.Apply
// and checks how Context.note maps each pure refusal onto the writer's
// outcome and code: a noted ticket whose prior event is unavailable is
// MISSING_EVIDENCE, revision capacity is VALIDATION_FAILED/LIMIT_EXCEEDED, an
// ARCHIVED ticket is BLOCKED/TICKET_STATE, and a non-null expectedRevision
// that does not match is REVISION_CONFLICT. No refusal carries an event.
func TestONV0003_ApplyNoteRefusalMapping(t *testing.T) {
	setPayload := obj("text", str("context"), "supersedes", wire.Null())
	base := fixture.Ticket("NT-01")
	first := apply(t, newCtx(t, owner, nil, base), envelope("n1", owner, "NT-01", "", mutation.OpNoteSet, setPayload))
	want(t, first, mutation.OutcomeCompleted, "")
	if first.DerivedEvent == nil || first.Post.OperatorNote == nil {
		t.Fatalf("first note: event %v reference %v", first.DerivedEvent != nil, first.Post.OperatorNote)
	}
	noted := first.Post
	// With the prior event supplied, the second note completes; without it
	// the writer cannot bind supersession and refuses.
	withEvent := newCtx(t, owner, nil, noted)
	withEvent.PriorNoteEvent = first.DerivedEvent
	want(t, apply(t, withEvent, envelope("n2", owner, "NT-01", "", mutation.OpNoteSet, setPayload)), mutation.OutcomeCompleted, "")

	full := fixture.Ticket("NT-01")
	prior := wire.Sum([]byte("prior record"))
	full.Revision, full.PreviousRecordSha256 = wire.CountOf(wire.MaxCountValue), &prior
	archived := fixture.Ticket("NT-01")
	from := ticket.StatusOpen
	archived.Status, archived.ArchivedFrom = ticket.StatusArchived, &from
	cases := []struct {
		name          string
		ctx           mutation.Context
		expected      string
		outcome, code string
	}{
		{"prior event missing", newCtx(t, owner, nil, noted), "", mutation.OutcomeValidationFailed, wire.CodeMissingEvidence},
		{"revision capacity", newCtx(t, owner, nil, full), "", mutation.OutcomeValidationFailed, wire.CodeLimitExceeded},
		{"archived ticket", newCtx(t, owner, nil, archived), "", mutation.OutcomeBlocked, wire.CodeTicketState},
		{"expected revision mismatch", newCtx(t, owner, nil, base), "9", mutation.OutcomeRevisionConflict, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := apply(t, c.ctx, envelope("n3", owner, "NT-01", c.expected, mutation.OpNoteSet, setPayload))
			want(t, plan, c.outcome, c.code)
			if plan.DerivedEvent != nil {
				t.Fatal("a refused note carried a derived event")
			}
		})
	}
}
