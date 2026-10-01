package store_test

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"path/filepath"
	"testing"
)

func TestCALV0048_HeartbeatFenceReplayAndLeaseInvariant(t *testing.T) {
	s := newLeaseStore(t)
	id := s.ticket(t, "heartbeat")
	c := s.lease(t, "claim-heartbeat", claimOf(id, "src/"), 1, store.NoScopeDeriver)
	path := filepath.Join(s.repo.StateDir, "attempts", c.AttemptID+".json")
	read := func() *snapshot.Attempt {
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		a, e := snapshot.DecodeAttempt(raw)
		if e != nil {
			t.Fatal(e)
		}
		return a
	}
	before := read()
	reservations, e := os.ReadFile(filepath.Join(s.repo.StateDir, "reservations.json"))
	if e != nil {
		t.Fatal(e)
	}
	req := transaction.LeaseRequest{Verb: transaction.LeaseHeartbeat, AttemptID: c.AttemptID, Generation: c.Generation}
	h := s.lease(t, "heartbeat-a", req, 2, store.NoScopeDeriver)
	if h.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("heartbeat: %+v", h)
	}
	after := read()
	if *after.LastHeartbeatAt != s.at(t, 2) || *after.Lease != *before.Lease || after.RetryCount != before.RetryCount || after.Phase != before.Phase {
		t.Fatal("heartbeat changed work lease or attempt")
	}
	res, e := os.ReadFile(filepath.Join(s.repo.StateDir, "reservations.json"))
	if e != nil || !bytes.Equal(res, reservations) {
		t.Fatal("heartbeat changed reservation")
	}
	replay := s.lease(t, "heartbeat-a", req, 80, store.NoScopeDeriver)
	if !replay.Outcome.Replayed || *read().LastHeartbeatAt != s.at(t, 2) {
		t.Fatal("replay refreshed expired heartbeat")
	}
	fenced := s.lease(t, "heartbeat-expired", req, 80, store.NoScopeDeriver)
	refusedWith(t, fenced, mutation.OutcomeRevisionConflict, wire.CodeFenced)
	s.lease(t, "reap-heartbeat", transaction.LeaseRequest{Verb: transaction.LeaseReap, AttemptID: c.AttemptID, Generation: c.Generation}, 81, store.NoScopeDeriver)
	next := s.lease(t, "claim-next", claimOf(id, "src/"), 82, store.NoScopeDeriver)
	if next.Generation == c.Generation || *read().LastHeartbeatAt != s.at(t, 82) || read().RetryReasons["EXPIRED"] != "1" {
		t.Fatal("readmission did not reset signal or charge expiry")
	}
	old := s.lease(t, "heartbeat-old-generation", req, 83, store.NoScopeDeriver)
	refusedWith(t, old, mutation.OutcomeRevisionConflict, wire.CodeFenced)
	replay = s.lease(t, "heartbeat-a", req, 84, store.NoScopeDeriver)
	if !replay.Outcome.Replayed || *read().LastHeartbeatAt != s.at(t, 82) {
		t.Fatal("old replay changed next generation")
	}
}
