package ticket_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func prereq(local, obligation string, gate *string, stages ...string) ticket.Prerequisite {
	return ticket.Prerequisite{TicketID: fixture.Ticket(local).TicketID, Obligation: obligation, GateID: gate, Stages: stages}
}

// TestCALV0099_RecordKeyRoundTrip: the optional key is omitted when absent
// (legacy bytes unchanged), round-trips byte-identically when present, and is
// emitted in canonical-byte order whatever the in-memory order.
func TestCALV0099_RecordKeyRoundTrip(t *testing.T) {
	legacy := fixture.Ticket("A")
	raw := legacy.Encode()
	if bytes.Contains(raw, []byte("executionPrerequisites")) {
		t.Fatalf("legacy record gained the key: %s", raw)
	}
	back, err := ticket.Decode(raw)
	if err != nil || back.ExecutionPrerequisites != nil || !bytes.Equal(back.Encode(), raw) {
		t.Fatalf("legacy round trip: %v", err)
	}
	rec := fixture.Ticket("A")
	rec.ExecutionPrerequisites = []ticket.Prerequisite{prereq("C", "COMPLETED", nil, "review", "integrate"), prereq("B", "GATE_PASSED", strPtr("verify"), "implement")}
	raw = rec.Encode()
	back, err = ticket.Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(back.Encode(), raw) {
		t.Fatalf("round trip differs:\n%s\n%s", raw, back.Encode())
	}
	if !bytes.Contains(raw, []byte(`"executionPrerequisites":[{"gateId":"verify","obligation":"GATE_PASSED","stages":["implement"],"ticketId":"ticket:acme:main:B"},{"gateId":null,"obligation":"COMPLETED","stages":["integrate","review"],"ticketId":"ticket:acme:main:C"}]`)) {
		t.Fatalf("not canonical: %s", raw)
	}
	// Not acceptance-relevant (CAL-V0-099): the key is outside the list.
	for _, f := range ticket.AcceptanceRelevantFields {
		if f == "executionPrerequisites" {
			t.Fatal("executionPrerequisites must not be acceptance-relevant")
		}
	}
}

// TestCALV0099_RecordKeyRefusals: the key is bounded and validated like
// `dependencies`, and an absent set is never spelled as an empty array.
func TestCALV0099_RecordKeyRefusals(t *testing.T) {
	ok := fixture.Ticket("A")
	ok.ExecutionPrerequisites = []ticket.Prerequisite{prereq("B", "COMPLETED", nil, "integrate")}
	edit := func(f func(string) string) []byte { return []byte(f(string(ok.Encode()))) }
	entry := `{"gateId":null,"obligation":"COMPLETED","stages":["integrate"],"ticketId":"ticket:acme:main:B"}`
	swap := func(with string) []byte {
		return edit(func(s string) string { return strings.Replace(s, entry, with, 1) })
	}
	many := make([]string, wire.MaxDependencies+1)
	for i := range many {
		many[i] = `{"gateId":null,"obligation":"COMPLETED","stages":["integrate"],"ticketId":"ticket:acme:main:B` + strings.Repeat("x", i) + `"}`
	}
	cases := []struct {
		name, code string
		raw        []byte
	}{
		{"empty", wire.CodeMalformed, swap(``)},
		{"unsorted", wire.CodeMalformed, swap(entry[:len(entry)-3] + `C"},` + entry)},
		{"duplicate", wire.CodeMalformed, swap(entry + "," + entry)},
		{"duplicate edge, other stages", wire.CodeDuplicateID, swap(entry + `,` + strings.Replace(entry, `["integrate"]`, `["review"]`, 1))},
		{"over bound", wire.CodeLimitExceeded, swap(strings.Join(many, ","))},
		{"gate on COMPLETED", wire.CodeMalformed, swap(strings.Replace(entry, `"gateId":null`, `"gateId":"verify"`, 1))},
		{"no gate on GATE_PASSED", wire.CodeMalformed, swap(strings.Replace(entry, `"COMPLETED"`, `"GATE_PASSED"`, 1))},
		{"self", wire.CodeMalformed, swap(strings.Replace(entry, "main:B", "main:A", 1))},
		{"other queue", wire.CodeDependencyMissing, swap(strings.Replace(entry, "acme:main:B", "acme:other:B", 1))},
		{"no stages", wire.CodeMalformed, swap(strings.Replace(entry, `["integrate"]`, `[]`, 1))},
		{"unknown stage", wire.CodeMalformed, swap(strings.Replace(entry, `["integrate"]`, `["deploy"]`, 1))},
		{"unsorted stages", wire.CodeMalformed, swap(strings.Replace(entry, `["integrate"]`, `["review","integrate"]`, 1))},
		{"open entry", wire.CodeMalformed, swap(strings.Replace(entry, `{"gateId"`, `{"extra":1,"gateId"`, 1))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ticket.Decode(c.raw); code(err) != c.code {
				t.Fatalf("decoded with %v; want %s", err, c.code)
			}
		})
	}
}

// TestCALV0099_StageScopedView: a prerequisite blocks only the stages it
// lists (a stageless read applies all of them), names the prerequisite, is
// never a structural problem or cycle, and an unobservable GATE_PASSED
// prerequisite is an unknown, not a blocker.
func TestCALV0099_StageScopedView(t *testing.T) {
	ctx := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK"}
	open := fixture.Ticket("P")
	a := fixture.Ticket("A")
	a.ExecutionPrerequisites = []ticket.Prerequisite{prereq("P", "COMPLETED", nil, "integrate")}
	// A prerequisite pointing back at A is no dependency cycle.
	open.ExecutionPrerequisites = []ticket.Prerequisite{prereq("A", "COMPLETED", nil, "review")}
	inv := inventory(t, open, a)
	if probs := inv.Problems(fixture.TicketID("A")); len(probs) != 0 {
		t.Fatalf("prerequisites reached structural problems: %+v", probs)
	}
	view := func(stage string, inv *ticket.Inventory) ticket.View {
		c := ctx
		c.Stage = stage
		v, _ := inv.View(fixture.TicketID("A"), c)
		return v
	}
	for _, stage := range []string{"integrate", ""} {
		v := view(stage, inv)
		if len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodePrerequisiteUnsatisfied || v.Blockers[0].TicketID != fixture.TicketID("P") || v.NextAction != "wait-dependency" {
			t.Fatalf("stage %q: %+v", stage, v.Blockers)
		}
		if !strings.Contains(v.Blockers[0].Detail, "integrate") {
			t.Fatalf("detail does not explain the stage scope: %s", v.Blockers[0].Detail)
		}
	}
	for _, stage := range []string{"implement", "review"} {
		if v := view(stage, inv); len(v.Blockers) != 0 {
			t.Fatalf("unlisted stage %q blocked: %+v", stage, v.Blockers)
		}
	}
	// Completing the prerequisite satisfies it; nothing else changes.
	done := fixture.Ticket("P")
	done.Status = "COMPLETED"
	done.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
	if v := view("integrate", inventory(t, done, a)); len(v.Blockers) != 0 {
		t.Fatalf("completed prerequisite still blocks: %+v", v.Blockers)
	}
	// A missing prerequisite (read-time) blocks the listed stage.
	if v := view("integrate", inventory(t, a)); len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodePrerequisiteUnsatisfied {
		t.Fatalf("missing prerequisite: %+v", v.Blockers)
	}
	// GATE_PASSED: NOT_OBSERVED is an unknown; observed results decide.
	g := fixture.Ticket("A")
	g.ExecutionPrerequisites = []ticket.Prerequisite{prereq("P", "GATE_PASSED", strPtr("verify"), "integrate")}
	ginv := inventory(t, fixture.Ticket("P"), g)
	v := view("integrate", ginv)
	if len(v.Blockers) != 0 || v.Eligibility != ticket.EligibilityUnknown {
		t.Fatalf("unobserved gate prerequisite became a blocker: %+v", v.Blockers)
	}
	found := false
	for _, u := range v.Unknowns {
		if u.Code == wire.CodePrerequisiteUnsatisfied && u.TicketID == fixture.TicketID("P") && strings.Contains(u.Detail, "NOT_OBSERVED") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no NOT_OBSERVED prerequisite unknown: %+v", v.Unknowns)
	}
	if v := view("review", ginv); len(v.Unknowns) != 1 {
		t.Fatalf("unlisted stage carries the gate unknown: %+v", v.Unknowns)
	}
	obs := ctx
	obs.Stage = "integrate"
	obs.Gates = gateOracle(ticket.Unsatisfied)
	if v, _ := ginv.View(fixture.TicketID("A"), obs); len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodePrerequisiteUnsatisfied {
		t.Fatalf("observed unsatisfied gate prerequisite: %+v", v.Blockers)
	}
	obs.Gates = gateOracle(ticket.Satisfied)
	if v, _ := ginv.View(fixture.TicketID("A"), obs); len(v.Blockers) != 0 || len(v.Unknowns) != 1 {
		t.Fatalf("observed satisfied gate prerequisite: %+v %+v", v.Blockers, v.Unknowns)
	}
	// The view exposes the set so `ticket blockers` explains the block.
	vv := view("", inv).Value(false)
	if _, ok := vv.Obj.Get("executionPrerequisites"); !ok {
		t.Fatal("view omits executionPrerequisites")
	}
	plain, _ := inventory(t, fixture.Ticket("P")).View(fixture.TicketID("P"), ctx)
	if _, ok := plain.Value(false).Obj.Get("executionPrerequisites"); ok {
		t.Fatal("view of a ticket without prerequisites gained the key")
	}
}
