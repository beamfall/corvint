package transaction

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502Question is one escalation reference entry; a closed question
// carries its second event.
func issue502Question(id, kind, state, acceptance string) ticket.EscalationRef {
	r := ticket.EscalationRef{RequestID: id, OriginSha256: wire.Sum([]byte(id + "-origin")), HeadSha256: wire.Sum([]byte(id + "-" + state)), Revision: "1", AcceptanceRevision: wire.Count(acceptance), Kind: kind, State: state}
	if state != "OPEN" {
		r.Revision = "2"
	}
	return r
}

// issue502Asking is a ticket at acceptance revision 2 carrying entries, one
// typed transaction per event; the codec must admit it.
func issue502Asking(t *testing.T, local string, order int64, entries ...ticket.EscalationRef) *ticket.Record {
	t.Helper()
	rec := fixture.Ticket(local)
	rec.Order = wire.CountOf(order)
	rec.Effects.TouchPaths = []string{"src/" + local + "/"}
	rec.AcceptanceRevision, rec.Revision = "2", "2"
	d := wire.Sum([]byte("revision 1"))
	rec.PreviousRecordSha256 = &d
	if len(entries) > 0 {
		var events int64
		for _, e := range entries {
			events += e.Revision.Int()
		}
		rec.Revision = wire.CountOf(2 + events)
		rec.Escalations = &ticket.EscalationRefs{Revision: wire.CountOf(events), LastControlTicketRevision: rec.Revision, WorkRevision: "2", Entries: entries}
	}
	back, err := ticket.Decode(rec.Encode())
	if err != nil {
		t.Fatalf("%s: %v", local, err)
	}
	return back
}

func issue502ClaimContext(t *testing.T, verb, ticketID string, recs ...*ticket.Record) leaseContext {
	t.Helper()
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	p, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	p.RequireEnforcedFields = nil
	tickets, err := ticket.NewInventory(q.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := NewInventory(nil, nil)
	l := &LeaseRequest{Verb: verb, TicketID: ticketID, Holder: "builder", LeaseMinutes: "60"}
	c := leaseContext{r: admin(Lease, "claim"), l: l, seq: "2", in: Input{Inventory: inv, RecordedAt: timestamp, LeaseFacts: LeaseFacts{AttemptID: "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BaseCommit: strings.Repeat("a", 40)}}, st: inputState{queue: q, policy: p, tickets: tickets, head: &snapshot.Head{Generation: "0", LastSeq: "1"}, pools: &snapshot.PoolState{QueueID: q.QueueID}, reservations: &snapshot.ReservationSet{QueueID: q.QueueID}, attempts: map[string]*snapshot.Attempt{}}}
	c.r.Lease = l
	return c
}

func issue502Refused(out leaseOutcome) (code, detail string) {
	if out.result == nil || out.result.Kind != "Refused" || out.result.Outcome.Outcome != mutation.OutcomeBlocked || len(out.result.Outcome.Codes) != 1 {
		return "", ""
	}
	return out.result.Outcome.Codes[0], out.result.Detail
}

// TestIssue502_ClaimHonoursEscalationHold proves ESC-V0-006 on every native
// admission path: eligibility, direct claim, claim-next and the recorded
// claimability all refuse a ticket with a current OPEN decision, scope or
// blocked question as ESCALATION_PENDING naming its sorted request IDs,
// while stale, infrastructure, answered and superseded questions admit. The
// hold is derived: refusing writes nothing.
func TestIssue502_ClaimHonoursEscalationHold(t *testing.T) {
	q := issue502Question
	held := []struct {
		name    string
		entries []ticket.EscalationRef
		ids     string
	}{
		{"decision", []ticket.EscalationRef{q("q-a", "decision", "OPEN", "2")}, "q-a"},
		{"scope", []ticket.EscalationRef{q("q-a", "scope", "OPEN", "2")}, "q-a"},
		{"blocked", []ticket.EscalationRef{q("q-a", "blocked", "OPEN", "2")}, "q-a"},
		{"sorted subset", []ticket.EscalationRef{q("q-a", "blocked", "OPEN", "2"), q("q-b", "infrastructure", "OPEN", "2"), q("q-c", "decision", "ANSWERED", "2"), q("q-d", "decision", "OPEN", "1"), q("q-e", "scope", "OPEN", "2")}, "q-a,q-e"},
	}
	for _, tc := range held {
		t.Run("held "+tc.name, func(t *testing.T) {
			rec := issue502Asking(t, "AT-01", 0, tc.entries...)
			id := rec.TicketID.Raw
			want := "escalation questions pending: " + tc.ids
			c := issue502ClaimContext(t, LeaseClaim, id, rec)
			if out := c.eligibility(id); out == nil {
				t.Fatal("eligibility admitted a held ticket")
			} else if code, detail := issue502Refused(*out); code != wire.CodeEscalationPending || detail != want {
				t.Fatalf("eligibility %s %q", code, detail)
			}
			if code, detail := issue502Refused(planClaim(c)); code != wire.CodeEscalationPending || detail != want {
				t.Fatalf("claim %s %q", code, detail)
			}
			c = issue502ClaimContext(t, LeaseClaimNext, "", rec)
			code, detail := issue502Refused(planClaimNext(c))
			if code != wire.CodeEscalationPending || !strings.HasSuffix(detail, "is BLOCKED ESCALATION_PENDING on "+tc.ids) {
				t.Fatalf("claim-next %s %q", code, detail)
			}
			in := PlanInput{Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
			if got, reason := RecordedClaimability(in, rec); got.Kind != wire.KindBool || got.Bool || reason != wire.CodeEscalationPending {
				t.Fatalf("claimability %s %s", wire.Encode(got), reason)
			}
			if e := PriorityFirst(in).Entries[0]; e.State != PlanBlocked || e.Reason != wire.CodeEscalationPending {
				t.Fatalf("plan %s %s", e.State, e.Reason)
			}
			if len(c.st.reservations.Entries) != 0 || rec.Status != ticket.StatusOpen || len(rec.Holds) != 0 {
				t.Fatal("the derived hold wrote state")
			}
		})
	}
	admitted := []struct {
		name    string
		entries []ticket.EscalationRef
	}{
		{"none", nil},
		{"infrastructure", []ticket.EscalationRef{q("q-a", "infrastructure", "OPEN", "2")}},
		{"stale", []ticket.EscalationRef{q("q-a", "decision", "OPEN", "1")}},
		{"answered", []ticket.EscalationRef{q("q-a", "scope", "ANSWERED", "2")}},
		{"superseded", []ticket.EscalationRef{q("q-a", "blocked", "SUPERSEDED", "2")}},
	}
	for _, tc := range admitted {
		t.Run("admitted "+tc.name, func(t *testing.T) {
			rec := issue502Asking(t, "AT-01", 0, tc.entries...)
			id := rec.TicketID.Raw
			c := issue502ClaimContext(t, LeaseClaim, id, rec)
			if out := c.eligibility(id); out != nil {
				t.Fatalf("eligibility refused %+v", out.result)
			}
			if out := planClaim(c); out.result != nil || out.effect == nil || len(out.posts) == 0 {
				t.Fatalf("claim not admitted %+v", out.result)
			}
			if out := planClaimNext(issue502ClaimContext(t, LeaseClaimNext, "", rec)); out.result != nil || out.effect == nil || len(out.posts) == 0 {
				t.Fatalf("claim-next not admitted %+v", out.result)
			}
			in := PlanInput{Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
			if got, reason := RecordedClaimability(in, rec); got.Kind != wire.KindBool || !got.Bool || reason != PlanSelected {
				t.Fatalf("claimability %s %s", wire.Encode(got), reason)
			}
		})
	}
	t.Run("claim-next passes over a held ticket", func(t *testing.T) {
		first := issue502Asking(t, "AT-01", 0, q("q-a", "decision", "OPEN", "2"))
		second := issue502Asking(t, "AT-02", 1, q("q-b", "infrastructure", "OPEN", "2"))
		c := issue502ClaimContext(t, LeaseClaimNext, "", first, second)
		in := PlanInput{Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
		plan := PriorityFirst(in)
		if s := plan.Selected(); s == nil || s.Ticket.TicketID.Raw != second.TicketID.Raw {
			t.Fatalf("plan %+v", plan.Entries)
		}
		if out := planClaimNext(c); out.result != nil || out.effect == nil || len(out.posts) == 0 {
			t.Fatalf("claim-next not admitted %+v", out.result)
		}
	})
	t.Run("sixteen current questions", func(t *testing.T) {
		var entries []ticket.EscalationRef
		var ids []string
		for i := 0; i < wire.EscalationMaxCurrentOpen; i++ {
			id := "q-" + string(rune('a'+i))
			entries = append(entries, q(id, "decision", "OPEN", "2"))
			ids = append(ids, id)
		}
		entries = append(entries, q("q-z", "scope", "OPEN", "1"))
		rec := issue502Asking(t, "AT-01", 0, entries...)
		if !slices.Equal(rec.EscalationPending(), ids) {
			t.Fatalf("holds %v", rec.EscalationPending())
		}
		c := issue502ClaimContext(t, LeaseClaim, rec.TicketID.Raw, rec)
		if code, detail := issue502Refused(planClaim(c)); code != wire.CodeEscalationPending || detail != "escalation questions pending: "+strings.Join(ids, ",") {
			t.Fatalf("claim %s %q", code, detail)
		}
	})
}
