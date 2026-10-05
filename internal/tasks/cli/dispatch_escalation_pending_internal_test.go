package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ESC-V0-006: dispatch status names, sorted by ticket and apart from parked
// keys and plan reasons, the request IDs of each hold the dispatcher's last
// native observation recorded. It reads only the dispatcher's own ledger,
// and a ledger with no hold keeps the previous status shape.
func TestIssue502_DispatchStatusShowsEscalationPending(t *testing.T) {
	now := time.Now()
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{"ticket:a:q:t9": {Parked: true}}}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "escalationPending"); ok {
		t.Fatal("a ledger with no observation shows escalation holds")
	}
	l.Seen = &dispatch.Seen{Tickets: map[string]string{
		"ticket:a:q:t3": "OPEN|ready|BLOCKED|RESOURCE_COLLISION",
		"ticket:a:q:t1": "OPEN|ready|BLOCKED|ESCALATION_PENDING",
		"ticket:a:q:t2": "OPEN|ready|BLOCKED|ESCALATION_PENDING",
	}, Escalations: map[string][]string{"ticket:a:q:t3": {"q-c"}, "ticket:a:q:t1": {"q-a", "q-b"}}}
	v := dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now)
	held, ok := statusField(t, v, "escalationPending")
	if want := `[{"requests":["q-a","q-b"],"ticket":"ticket:a:q:t1"},{"requests":["q-c"],"ticket":"ticket:a:q:t3"}]`; !ok || string(wire.Encode(held)) != want {
		t.Fatalf("escalationPending = %s", wire.Encode(held))
	}
	parked, _ := statusField(t, v, "parked")
	if len(parked.Arr) != 1 || parked.Arr[0].Str != "ticket:a:q:t9" {
		t.Fatalf("parked = %+v", parked)
	}
	l.Seen.Escalations = nil
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "escalationPending"); ok {
		t.Fatal("an answered hold still shows")
	}
}
