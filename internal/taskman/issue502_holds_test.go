package taskman

import (
	"os"
	"slices"
	"testing"
)

// TestIssue502_HoldAgreement: Core's read-only planner and the native Tasks
// reader name the same ESCALATION_PENDING holds (ESC-V0-006) for the records
// the Tasks codec writes (internal/tasks/ticket
// TestIssue502_EscalationPendingView), through the shared wire predicate
// (decision 0397). Stale, infrastructure, answered and superseded questions
// never hold; a held ticket blocks as ESCALATION_PENDING, not TICKET_STATE.
func TestIssue502_HoldAgreement(t *testing.T) {
	raw, e := os.ReadFile("../tasks/ticket/testdata/issue502-escalation-hold-cases.json")
	if e != nil {
		t.Fatal(e)
	}
	doc, e := document(raw, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	cases := value(doc, "cases").Arr
	if len(cases) < 10 {
		t.Fatalf("shared fixture has %d cases", len(cases))
	}
	c := testCapture(t)
	for _, tc := range cases {
		name := stringAt(tc, "name")
		x, e := decodeTicket(value(tc, "record"))
		if e != nil {
			t.Fatalf("%s: decode: %v", name, e)
		}
		var held []string
		for _, id := range value(tc, "held").Arr {
			held = append(held, id.Str)
		}
		if got := escalationPending(x); !slices.Equal(got, held) {
			t.Fatalf("%s: Core holds %v, Tasks %v", name, got, held)
		}
		got := blocker(x, map[string]ticket{x.id: x}, c)
		if (got == "ESCALATION_PENDING") != (len(held) != 0) {
			t.Fatalf("%s: Core blocker %q with Tasks holds %v", name, got, held)
		}
	}
}
