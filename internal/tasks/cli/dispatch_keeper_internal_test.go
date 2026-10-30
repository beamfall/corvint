package cli

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// CAL-V0-316 (acceptance 5): dispatch status shows each kept worker's
// keeper state, binding, last action and counts; a worker whose role
// declares no keepLease shows no keeper member.
func TestCALV0316_DispatchStatusShowsKeeperPerWorker(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{},
		Workers: []*dispatch.Worker{
			{ID: "w1", Role: "impl", Key: "ticket:a:q:t1", Keeper: &dispatch.KeeperState{State: "KEEPING", Attempt: "a1", Generation: "3", Due: now.Add(time.Minute), LastKept: now, LastAction: "RENEWED", Renewals: 2, Heartbeats: 5}},
			{ID: "w2", Role: "impl", Key: "ticket:a:q:t2", Keeper: &dispatch.KeeperState{State: "STOPPED", Attempt: "a2", Generation: "1", Reason: "FENCED", Due: now}},
			{ID: "w3", Role: "plain", Key: "ticket:a:q:t3"},
		}}
	v := dispatchStatusValue(&dispatch.Config{}, t.TempDir(), l, nil, now)
	workers, _ := statusField(t, v, "workers")
	k1, ok := statusField(t, workers.Arr[0], "keeper")
	if !ok {
		t.Fatal("no keeper for a kept worker")
	}
	statusStrings(t, k1, map[string]string{"state": "KEEPING", "attempt": "a1", "generation": "3", "reason": "", "lastAction": "RENEWED", "renewals": "2", "heartbeats": "5", "due": "2026-10-17T12:01:00Z", "lastKept": "2026-10-17T12:00:00Z"})
	if f, _ := statusField(t, k1, "failing"); f.Bool {
		t.Error("failing shown for a healthy keeper")
	}
	k2, _ := statusField(t, workers.Arr[1], "keeper")
	statusStrings(t, k2, map[string]string{"state": "STOPPED", "reason": "FENCED", "attempt": "a2"})
	if _, ok := statusField(t, k2, "lastKept"); ok {
		t.Error("lastKept shown for a keeper that never kept")
	}
	if _, ok := statusField(t, workers.Arr[2], "keeper"); ok {
		t.Error("keeper shown for a worker without keepLease")
	}
}
