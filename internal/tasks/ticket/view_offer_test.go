package ticket_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0011_OfferReplacesOnlyAnUnblockedAdmit: the read-only completion
// offer turns only an OPEN, unblocked admit with nothing NOT_OBSERVED into
// complete-manual with suggested evidence; every other view, and a view given
// no evidence, renders byte-identically to one never offered.
func TestERGV0011_OfferReplacesOnlyAnUnblockedAdmit(t *testing.T) {
	heads := []wire.Digest{wire.Digest(bytes.Repeat([]byte("a"), 64)), wire.Digest(bytes.Repeat([]byte("b"), 64))}
	idle := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK", Attempts: attemptOracle(ticket.Unsatisfied)}
	live := idle
	live.Attempts = attemptOracle(ticket.Satisfied)
	unobserved := idle
	unobserved.Attempts = nil
	held := fixture.Ticket("H")
	held.Status = ticket.StatusHeld
	held.Holds = []ticket.Hold{{HoldID: "h1", Actor: "o", Reason: "x", PlacedAt: fixture.Timestamp}}
	cases := []struct {
		name string
		rec  *ticket.Record
		ctx  ticket.Context
		ev   []wire.Digest
		want string
	}{
		{"unblocked admit", fixture.Ticket("A"), idle, heads, ticket.NextActionCompleteManual},
		{"no evidence", fixture.Ticket("A"), idle, nil, "admit"},
		{"empty evidence", fixture.Ticket("A"), idle, []wire.Digest{}, "admit"},
		{"live attempt", fixture.Ticket("A"), live, heads, "wait-attempt"},
		{"attempt liveness NOT_OBSERVED", fixture.Ticket("A"), unobserved, heads, "admit"},
		{"held", held, idle, heads, "release-hold"},
	}
	for _, c := range cases {
		id := c.rec.TicketID.Raw
		plain, _ := inventory(t, c.rec).View(id, c.ctx)
		v, _ := inventory(t, c.rec).View(id, c.ctx)
		offered := v.OfferCompletion(c.ev)
		if v.NextAction != c.want || offered != (c.want == ticket.NextActionCompleteManual) {
			t.Fatalf("%s: nextAction %q offered %t", c.name, v.NextAction, offered)
		}
		got := wire.Encode(v.Value(true))
		if !offered {
			if want := wire.Encode(plain.Value(true)); !bytes.Equal(got, want) {
				t.Fatalf("%s: a view without the offer changed:\n%s\n%s", c.name, want, got)
			}
			continue
		}
		ev, ok := v.Value(false).Obj.Get("suggestedEvidence")
		if !ok || len(ev.Arr) != 2 || ev.Arr[0].Str != string(heads[0]) || ev.Arr[1].Str != string(heads[1]) {
			t.Fatalf("%s: suggested evidence %s", c.name, got)
		}
		if v.Record.Status != ticket.StatusOpen || v.Record.Completion != nil {
			t.Fatalf("%s: the offer changed the record", c.name)
		}
	}
}
