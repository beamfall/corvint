package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
)

// ATR-V0-123 (V1-1833, issue 1147): dispatch status shows, per worker, the
// pool member the dispatcher holds for its attempt; a worker without a hold
// shows none.
func TestATRV0123_DispatchStatusShowsPoolHold(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	h := dispatch.PoolHold{Profile: dispatch.PoolHoldProfile, Worker: "w1", State: dispatch.PoolHoldHeld, Attempt: "att-1", Generation: "3", Ticket: "ticket:a:q:t1", Pool: "db", Member: "a", Allocation: "sha-1", Acquired: now.Add(-time.Minute)}
	raw, _ := json.Marshal(h)
	path := dispatch.PoolHoldPath(dir, "w1")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	c := &dispatch.Config{Roles: []dispatch.Role{{Name: "impl", HoldPool: true}}}
	l := &dispatch.Ledger{Program: "prog", Backoff: map[string]*dispatch.BackoffState{}, Workers: []*dispatch.Worker{
		{ID: "w1", Role: "impl", Ticket: "ticket:a:q:t1", Pool: "db", State: "RUNNING", Started: now, LastActive: now},
		{ID: "w2", Role: "impl", Ticket: "ticket:a:q:t2", State: "RUNNING", Started: now, LastActive: now},
	}}
	workers, _ := statusField(t, dispatchStatusValueFor(c, dir, l, nil, now, nil), "workers")
	if len(workers.Arr) != 2 {
		t.Fatalf("workers %+v", workers)
	}
	hold, ok := statusField(t, workers.Arr[0], "poolHold")
	if !ok {
		t.Fatal("w1 shows no poolHold")
	}
	for key, want := range map[string]string{"state": "HELD", "attempt": "att-1", "generation": "3", "pool": "db", "member": "a", "allocation": "sha-1"} {
		if got, _ := statusField(t, hold, key); got.Str != want {
			t.Errorf("poolHold.%s %q, want %q", key, got.Str, want)
		}
	}
	if _, ok := statusField(t, workers.Arr[1], "poolHold"); ok {
		t.Error("w2 shows a poolHold it does not have")
	}
}
