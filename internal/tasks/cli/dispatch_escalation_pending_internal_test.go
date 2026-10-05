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

// ESC-V0-007, ESC-V0-009: dispatch status shows each infrastructure retry
// episode apart from parked keys, with its count against the configured
// bound, reason and charged requests; no episode keeps the previous shape.
func TestESCV0007_DispatchStatusShowsInfrastructureRetry(t *testing.T) {
	now := time.Now()
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "infrastructureRetry"); ok {
		t.Fatal("a ledger with no episode shows infrastructure retry")
	}
	two := 2
	c := &dispatch.Config{InfrastructureRetry: &dispatch.InfrastructureRetry{MaxRetries: &two}}
	l.InfraRetry = map[string]*dispatch.InfraRetry{
		"ticket:a:q:t2": {AcceptanceRevision: "1", State: dispatch.InfraExhausted, Count: 2, Sessions: []string{"w1", "w2", "w3"}, Requests: []string{"r1"}, Reason: dispatch.InfraRetryExhausted},
		"ticket:a:q:t1": {AcceptanceRevision: "3", State: dispatch.InfraWait, Count: 1, Sessions: []string{"w4"}, Requests: []string{"r2"}, NextEligible: time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)},
	}
	v, ok := statusField(t, dispatchStatusValue(c, t.TempDir(), l, nil, now), "infrastructureRetry")
	want := `[{"acceptanceRevision":"3","count":"1","maxRetries":"2","nextEligible":"2026-10-05T01:02:03Z","requests":["r2"],"state":"RETRY_WAIT","ticket":"ticket:a:q:t1"},` +
		`{"acceptanceRevision":"1","count":"2","maxRetries":"2","reason":"INFRA_RETRY_EXHAUSTED","requests":["r1"],"state":"EXHAUSTED","ticket":"ticket:a:q:t2"}]`
	if !ok || string(wire.Encode(v)) != want {
		t.Fatalf("infrastructureRetry = %s", wire.Encode(v))
	}
}
