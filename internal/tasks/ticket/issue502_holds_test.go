package ticket_test

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502HoldCasesFixture is shared with Core's planner test
// (internal/taskman TestIssue502_HoldAgreement): both readers decode the
// records this codec writes and must name the same ESCALATION_PENDING holds.
const issue502HoldCasesFixture = "testdata/issue502-escalation-hold-cases.json"

type issue502HoldCase struct {
	name    string
	entries []ticket.EscalationRef
	held    []string
}

// issue502HoldRef is one question at the given kind, lifecycle state and
// source acceptance revision; a closed question carries its second event.
func issue502HoldRef(id, kind, state, acceptance string) ticket.EscalationRef {
	r := issue502Ref(id, state, acceptance)
	r.Kind = kind
	if state != "OPEN" {
		r.Revision = "2"
	}
	return r
}

func issue502HoldCases() []issue502HoldCase {
	ref := issue502HoldRef
	sixteen := []ticket.EscalationRef{}
	held := []string{}
	for i := 0; i < wire.EscalationMaxCurrentOpen; i++ {
		id := fmt.Sprintf("q-%02d", i)
		sixteen = append(sixteen, ref(id, "decision", "OPEN", "2"))
		held = append(held, id)
	}
	sixteen = append(sixteen, ref("q-99", "scope", "OPEN", "1"))
	return []issue502HoldCase{
		{"none", nil, nil},
		{"open-decision", []ticket.EscalationRef{ref("q-a", "decision", "OPEN", "2")}, []string{"q-a"}},
		{"open-scope", []ticket.EscalationRef{ref("q-a", "scope", "OPEN", "2")}, []string{"q-a"}},
		{"open-blocked", []ticket.EscalationRef{ref("q-a", "blocked", "OPEN", "2")}, []string{"q-a"}},
		{"open-infrastructure", []ticket.EscalationRef{ref("q-a", "infrastructure", "OPEN", "2")}, nil},
		{"stale-decision", []ticket.EscalationRef{ref("q-a", "decision", "OPEN", "1")}, nil},
		{"answered-decision", []ticket.EscalationRef{ref("q-a", "decision", "ANSWERED", "2")}, nil},
		{"superseded-scope", []ticket.EscalationRef{ref("q-a", "scope", "SUPERSEDED", "2")}, nil},
		{"mixed", []ticket.EscalationRef{ref("q-a", "blocked", "OPEN", "2"), ref("q-b", "infrastructure", "OPEN", "2"), ref("q-c", "decision", "ANSWERED", "2"), ref("q-d", "decision", "OPEN", "1"), ref("q-e", "scope", "OPEN", "2"), ref("q-f", "decision", "OPEN", "2")}, []string{"q-a", "q-e", "q-f"}},
		{"sixteen-current-open", sixteen, held},
	}
}

// issue502HoldRecord is AT-01 at acceptance revision 2 carrying entries, one
// typed transaction per event after work revision 2.
func issue502HoldRecord(entries []ticket.EscalationRef) *ticket.Record {
	rec := fixture.Ticket("AT-01")
	rec.AcceptanceRevision, rec.Revision, rec.PreviousRecordSha256 = "2", "2", digest("revision 1")
	if len(entries) == 0 {
		return rec
	}
	var events int64
	for _, e := range entries {
		events += e.Revision.Int()
	}
	rec.Revision = wire.CountOf(2 + events)
	rec.Escalations = &ticket.EscalationRefs{Revision: wire.CountOf(events), LastControlTicketRevision: rec.Revision, WorkRevision: "2", Entries: entries}
	return rec
}

// TestIssue502_EscalationPendingView proves the ESC-V0-006 derived hold in
// native eligibility: current OPEN decision, scope and blocked questions
// block as ESCALATION_PENDING naming the sorted request IDs; stale,
// infrastructure, answered and superseded questions do not. The shared
// fixture Core reads is exactly this codec's encoding of the same cases.
func TestIssue502_EscalationPendingView(t *testing.T) {
	var shared []wire.Value
	for _, tc := range issue502HoldCases() {
		rec := issue502HoldRecord(tc.entries)
		raw := rec.Encode()
		back, err := ticket.Decode(raw)
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if got := back.EscalationPending(); !slices.Equal(got, tc.held) {
			t.Fatalf("%s: record holds %v, want %v", tc.name, got, tc.held)
		}
		v, _ := inventory(t, back).View(back.TicketID.Raw, ticket.Context{})
		var pending []ticket.Blocker
		for _, b := range v.Blockers {
			if b.Code == wire.CodeEscalationPending {
				pending = append(pending, b)
			}
		}
		if len(tc.held) == 0 {
			if len(pending) != 0 {
				t.Fatalf("%s: unexpected hold %+v", tc.name, pending)
			}
		} else if len(pending) != 1 || pending[0].Detail != "escalation questions pending: "+strings.Join(tc.held, ",") || v.NextAction != "answer" {
			t.Fatalf("%s: hold %+v next %s", tc.name, pending, v.NextAction)
		}
		if back.Status != ticket.StatusOpen || len(back.Holds) != 0 || !bytes.Equal(back.Encode(), raw) {
			t.Fatalf("%s: the derived hold changed the record", tc.name)
		}
		record, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		shared = append(shared, wire.ObjectValue(wire.NewObject().Set("held", wire.Strings(append([]string{}, tc.held...))).Set("name", wire.String(tc.name)).Set("record", record)))
	}
	want := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("cases", wire.Array(shared...))))
	file, err := os.ReadFile(issue502HoldCasesFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(file, want) {
		if os.Getenv("CORVINT_ISSUE502_WRITE_HOLD_CASES") == "1" {
			if err := os.WriteFile(issue502HoldCasesFixture, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Fatalf("%s is not this codec's encoding of the hold cases", issue502HoldCasesFixture)
	}
}
