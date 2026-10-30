package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// markerGateStore is a gate store whose "mark" gate creates a marker in the
// checkout's Git directory, which leaves the worktree clean, so a test can
// tell whether the program started.
func markerGateStore(t *testing.T) (*leaseStore, string) {
	t.Helper()
	s := newLeaseStore(t, commandGate("mark", ": > .git/gate-started", "30", true))
	return s, filepath.Join(s.root, ".git", "gate-started")
}

func notStarted(t *testing.T, marker string) {
	t.Helper()
	if _, err := os.Lstat(marker); err == nil {
		t.Fatal("the gate program started before admission refused the run")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// TestV13008_GateRunRefusesUnderAllBarrierBeforeSpawning: an ALL barrier
// refuses a gate run before its program starts, and the refusal is the
// canonical PAUSED one; once the barrier is gone the same gate runs.
func TestV13008_GateRunRefusesUnderAllBarrierBeforeSpawning(t *testing.T) {
	t.Parallel()
	s, marker := markerGateStore(t)
	claim, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)
	reconciliationBarrier(t, s.repo, "ALL")
	report, err := s.gate(t, "gate-paused", gateOf(claim, "mark"), 2)
	if err != nil {
		t.Fatal(err)
	}
	refusedWith(t, report, mutation.OutcomeBlocked, wire.CodePaused)
	if report.Unretryable || report.GateExecuted != nil {
		t.Fatalf("a refused admission reports an execution: %+v", report)
	}
	notStarted(t, marker)
}

// TestV13008_GateRunRefusesRejectedRoleBeforeSpawning: a role the lease
// write does not admit is refused before the gate program starts.
func TestV13008_GateRunRefusesRejectedRoleBeforeSpawning(t *testing.T) {
	t.Parallel()
	s, marker := markerGateStore(t)
	claim, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)
	at, err := time.Parse("2006-01-02T15:04:05Z", string(s.at(t, 2)))
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"WORKER", "REVIEWER"} {
		choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "gate-" + role, Root: s.root, Lease: gateOf(claim, "mark")}
		report, err := store.GateRun(context.Background(), s.repo, mutation.Binding{ID: "agent", Role: role}, choice, s.root, func() time.Time { return at })
		if err == nil && (report == nil || report.Outcome.Outcome == mutation.OutcomeCompleted) {
			t.Fatalf("%s gate run was admitted: %+v", role, report)
		}
		notStarted(t, marker)
	}
	if r, err := s.gate(t, "gate-owner", gateOf(claim, "mark"), 3); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("owner gate run: %+v %v", r, err)
	}
	if _, err := os.Lstat(marker); err != nil {
		t.Fatalf("the admitted gate did not run: %v", err)
	}
}
