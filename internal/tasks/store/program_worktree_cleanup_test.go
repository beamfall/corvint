//go:build darwin || linux

package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// completedProgram drives the designated fixture's program through
// implement, review and integration, so its attempt is COMPLETED.
func completedProgram(t *testing.T, f *multiFixture) {
	t.Helper()
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	grantIntegration(t, f, programScope(t, f, a))
	if _, err := w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
}

// TestATRV0297_ReassignRemovesPriorStageWorktrees proves a reassign removes
// the prior assignment's clean stage worktrees, keeps a dirty one registered
// instead of orphaning it, and that the operator cleanup removes the kept
// worktree from the idle program once it is clean (ATR-V0-296..298).
func TestATRV0297_ReassignRemovesPriorStageWorktrees(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	dirty := ""
	for _, r := range programRecord(t, f).Worktrees {
		if r.Repository == "docs" && !r.Removed {
			dirty = r.Path
		}
	}
	if dirty == "" {
		t.Fatal("no docs stage worktree was registered")
	}
	note := filepath.Join(dirty, "note.txt")
	if err := os.WriteFile(note, []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("reopen after completion: %v, want the idle refusal", err)
	}
	p := programRecord(t, f)
	if p.Assignment != 2 || p.Phase != "ADMITTED" || p.CurrentAttempt != "" {
		t.Fatalf("reassign recorded assignment %d phase %s attempt %q", p.Assignment, p.Phase, p.CurrentAttempt)
	}
	if len(p.Worktrees) != 1 || p.Worktrees[0].Path != dirty || p.Worktrees[0].Removed {
		t.Fatalf("reassign kept %+v, want only the dirty %s", p.Worktrees, dirty)
	}
	if list := multiGit(t, f.s.repo.PrimaryWorktree, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("reassign left a clean queue stage worktree:\n%s", list)
	}
	results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 1 || results[0].Removed || results[0].Reason == "" {
		t.Fatalf("cleanup of a dirty worktree: %+v %v, want it kept with a reason", results, err)
	}
	multiGit(t, dirty, "checkout", "--", "note.txt")
	results, err = store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 1 || !results[0].Removed || results[0].Path != dirty {
		t.Fatalf("cleanup of the restored worktree: %+v %v", results, err)
	}
	// The removed worktree's record is dropped (CAL-V0-366).
	if p = programRecord(t, f); p.Phase != "ADMITTED" || len(p.Worktrees) != 0 {
		t.Fatalf("cleanup recorded phase %s worktrees %+v", p.Phase, p.Worktrees)
	}
	if list := multiGit(t, f.extra, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("docs keeps a stage worktree:\n%s", list)
	}
}

// TestATRV0296_CleanupRefusesLiveAttempt proves the operator cleanup refuses
// while the program's attempt is live and leaves its stage worktree in place.
func TestATRV0296_CleanupRefusesLiveAttempt(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	w := openProgram(t, f)()
	if a, err := w.RunRole(context.Background(), "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	_, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if wire.CodeOf(err) != wire.CodeAttemptLive {
		t.Fatalf("cleanup during a live attempt: %v, want ATTEMPT_LIVE", err)
	}
	for _, r := range programRecord(t, f).Worktrees {
		if _, err := os.Lstat(r.Path); r.Removed || err != nil {
			t.Fatalf("stage worktree %s touched: removed %v %v", r.Path, r.Removed, err)
		}
	}
}
