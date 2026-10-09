package transaction

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestONV0006_DerivedEventSlotClosedToDeclaringOperations checks that the
// Model posts a derived event only for an operation that declares one: the
// stage contract sees every mutation as MUTATE, so REFINE (or any other
// non-note verb) carrying an event is refused here and stages nothing.
func TestONV0006_DerivedEventSlotClosedToDeclaringOperations(t *testing.T) {
	event := []byte(`{"kind":"event"}`)
	for _, op := range intent.Operations {
		posts := map[string][]byte{}
		e := derivedEventPost(op, event, posts)
		// ERG-V0-009 review operations declare the slot too; their receipt
		// audit binding is ExternalReviewReceiptAudit (slot-reuse rule).
		// TOL-V0-002 obligation operations bind ObligationReceiptAudit.
		declares := mutation.IsNoteOperation(op) || mutation.IsReviewOperation(op) || ticket.IsObligationOperation(op)
		if mutation.DeclaresDerivedEvent(op) != declares {
			t.Fatalf("%s: only note, review and obligation operations declare a derived event", op)
		}
		if declares {
			if e != nil || len(posts) != 1 || string(posts["evidence/"+string(wire.Sum(event))]) != string(event) {
				t.Fatalf("%s: declared event not posted: %v %v", op, e, posts)
			}
			continue
		}
		if e == nil || wire.CodeOf(e) != wire.CodeUnsupported || len(posts) != 0 {
			t.Fatalf("%s carried a derived event: %v %v", op, e, posts)
		}
		if r := failed("req", e); r.Kind == "Transaction" || r.Outcome.Outcome != mutation.OutcomeUnsupported {
			t.Fatalf("%s: refusal outcome %+v", op, r.Outcome)
		}
	}
	posts := map[string][]byte{}
	if e := derivedEventPost(mutation.OpRefine, nil, posts); e != nil || len(posts) != 0 {
		t.Fatalf("REFINE without an event: %v %v", e, posts)
	}
}
