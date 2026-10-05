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

// TestIssue502_DispatchStatusShowsInfraRetry: status shows each
// infrastructure retry episode as a dispatcher observation, with the named
// hold labels, and whether the policy is configured; a ledger with no
// episode shows no section.
func TestIssue502_DispatchStatusShowsInfraRetry(t *testing.T) {
	now := time.Now()
	l := &dispatch.Ledger{Program: "prog"}
	if _, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "infrastructureRetry"); ok {
		t.Fatal("a ledger with no episode shows infrastructure retry")
	}
	at := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)
	l.InfraRetry = map[string]*dispatch.InfraEpisode{
		"ticket:a:q:t2": {AcceptanceRevision: "4", State: dispatch.InfraExhausted, Sessions: 4, Charged: 3, Limit: 3, CooldownUntil: at},
		"ticket:a:q:t1": {AcceptanceRevision: "1", State: dispatch.InfraRunning, Sessions: 1, Charged: 1, Limit: 3, CooldownUntil: at, Launch: "prog.impl.1.ab-2"},
	}
	v, ok := statusField(t, dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now), "infrastructureRetry")
	want := `{"episodes":[{"acceptanceRevision":"1","charged":"1","cooldownUntil":"2026-10-05T01:02:03Z","launch":"prog.impl.1.ab-2","limit":"3","sessions":"1","state":"RUNNING","ticket":"ticket:a:q:t1"},{"acceptanceRevision":"4","charged":"3","cooldownUntil":"2026-10-05T01:02:03Z","limit":"3","sessions":"4","state":"INFRA_RETRY_EXHAUSTED","ticket":"ticket:a:q:t2"}],"policy":"ABSENT","source":"DISPATCHER_OBSERVATION"}`
	if !ok || string(wire.Encode(v)) != want {
		t.Fatalf("infrastructureRetry = %s", wire.Encode(v))
	}
	v, _ = statusField(t, dispatchStatusValue(&dispatch.Config{InfrastructureRetry: &dispatch.InfraRetryConfig{}}, t.TempDir(), l, nil, now), "infrastructureRetry")
	if p, _ := v.Obj.Get("policy"); p.Str != "PRESENT" {
		t.Fatalf("policy = %+v", p)
	}
}
