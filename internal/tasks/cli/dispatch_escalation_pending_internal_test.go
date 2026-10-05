package cli

import (
	"slices"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ESC-V0-006: dispatch status names, sorted and apart from parked keys, the
// tickets its last native plan observation held as ESCALATION_PENDING. It
// reads only the dispatcher's own ledger, and a ledger with no such ticket
// keeps the previous status shape.
func TestIssue502_DispatchStatusShowsEscalationPending(t *testing.T) {
	now := time.Now()
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{"ticket:a:q:t9": {Parked: true}}}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "escalationPending"); ok {
		t.Fatal("a ledger with no observation shows escalation holds")
	}
	l.Seen = &dispatch.Seen{Tickets: map[string]string{
		"ticket:a:q:t3": "OPEN|ready|BLOCKED|ESCALATION_PENDING",
		"ticket:a:q:t1": "OPEN|ready|BLOCKED|ESCALATION_PENDING",
		"ticket:a:q:t2": "OPEN|ready|SELECTED|DEVELOPMENT_MODE",
		"ticket:a:q:t4": "HELD|ready|BLOCKED|TICKET_HELD",
		"ticket:a:q:t5": "OPEN|UNKNOWN|UNKNOWN|ESCALATION_PENDING|extra",
	}}
	v := dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now)
	held, ok := statusField(t, v, "escalationPending")
	var got []string
	for _, x := range held.Arr {
		got = append(got, x.Str)
	}
	if !ok || !slices.Equal(got, []string{"ticket:a:q:t1", "ticket:a:q:t3"}) {
		t.Fatalf("escalationPending = %v", got)
	}
	parked, _ := statusField(t, v, "parked")
	if len(parked.Arr) != 1 || parked.Arr[0].Str != "ticket:a:q:t9" {
		t.Fatalf("parked = %+v", parked)
	}
	delete(l.Seen.Tickets, "ticket:a:q:t1")
	delete(l.Seen.Tickets, "ticket:a:q:t3")
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "escalationPending"); ok {
		t.Fatal("an answered hold still shows")
	}
}
