//go:build darwin || linux

package store_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
)

// TestATRV0920_FailedRemovalWriteReloadsProgram proves a failed write that
// would enter WORKTREE_REMOVE leaves the workflow's program equal to the
// recorded one, and that a retried CleanupWorktrees on the same Workflow
// removes each worktree only after a durable WORKTREE_REMOVE record names it
// (ATR-V0-920, V1-2480). Before the fix the unfenced write kept the
// uncommitted phase in memory.
func TestATRV0920_FailedRemovalWriteReloadsProgram(t *testing.T) {
	// Not parallel: the program write hook is process wide.
	f := designatedFixture(t)
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	grantIntegration(t, f, programScope(t, f, a))
	if _, err := w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	registered := len(programRecord(t, f).Worktrees)
	if registered < 1 {
		t.Fatal("fixture registered no stage worktree")
	}
	refused := errors.New("injected WORKTREE_REMOVE publication failure")
	var mu sync.Mutex
	armed, failNext := true, false
	// The write hook sees the composed phase before the transition, so the
	// fault refuses that write's first artifact and nothing is published.
	defer store.SetProgramWriteHookForTest(func() {
		mu.Lock()
		defer mu.Unlock()
		if armed && w.ProgramPhaseForTest() == "WORKTREE_REMOVE" {
			armed, failNext = false, true
		}
	})()
	defer store.SetStorePublishFaultForTest(f.s.repo.StateDir, func(transaction.Artifact) error {
		mu.Lock()
		defer mu.Unlock()
		if failNext {
			failNext = false
			return refused
		}
		return nil
	})()
	if _, err := w.CleanupWorktrees(); !errors.Is(err, refused) {
		t.Fatalf("cleanup was not refused at the WORKTREE_REMOVE write: %v", err)
	}
	recorded := programRecord(t, f)
	if recorded.Phase == "WORKTREE_REMOVE" || len(recorded.Worktrees) != registered {
		t.Fatalf("the refused write reached the store: phase %s, %d records", recorded.Phase, len(recorded.Worktrees))
	}
	if got := w.ProgramPhaseForTest(); got != recorded.Phase {
		t.Fatalf("in-memory phase %s ran ahead of the recorded phase %s", got, recorded.Phase)
	}
	durable := 0
	defer store.SetRunFaultForTest(f.s.repo.StateDir, func(at string) error {
		if at != "cleanup:WORKTREE_REMOVE" {
			return nil
		}
		p := programRecord(t, f)
		if p.Phase != "WORKTREE_REMOVE" || p.Worktree == "" {
			t.Errorf("a removal started without a durable WORKTREE_REMOVE record: phase %s worktree %q", p.Phase, p.Worktree)
		} else if _, err := os.Lstat(p.Worktree); err != nil {
			t.Errorf("worktree %s was gone before its removal was recorded: %v", p.Worktree, err)
		}
		durable++
		return nil
	})()
	results, err := w.CleanupWorktrees()
	if err != nil {
		t.Fatalf("retried cleanup: %v", err)
	}
	removed := 0
	for _, item := range results {
		if item.Removed {
			removed++
		}
	}
	if removed != registered || durable != registered {
		t.Fatalf("retried cleanup removed %d of %d worktrees with %d durable removal records: %+v", removed, registered, durable, results)
	}
	if p := programRecord(t, f); len(p.Worktrees) != 0 || p.Phase == "WORKTREE_REMOVE" {
		t.Fatalf("retried cleanup left phase %s with records %+v", p.Phase, p.Worktrees)
	}
}
