//go:build darwin || linux

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
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
	if p = programRecord(t, f); p.Phase != "ADMITTED" || len(p.Worktrees) != 0 {
		t.Fatalf("cleanup recorded phase %s worktrees %+v, want the removed record dropped (CAL-V0-366)", p.Phase, p.Worktrees)
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

// interruptedRemovalEnv carries the config, root and mode to the child owner
// of TestCALV0365_TakeoverSettlesInterruptedRemoval.
const interruptedRemovalEnv = "CORVINT_TEST_INTERRUPTED_REMOVAL"

// TestCALV0365_InterruptedRemovalChild is the cleanup owner of
// TestCALV0365_TakeoverSettlesInterruptedRemoval. It exits right after the
// first WORKTREE_REMOVE write, standing in for an owner crash; in mode
// "removed" it first lets Git remove that worktree, and in mode "half" it
// deletes only the work tree. It is skipped otherwise.
func TestCALV0365_InterruptedRemovalChild(t *testing.T) {
	raw := os.Getenv(interruptedRemovalEnv + "_CONFIG")
	if raw == "" {
		t.Skip("child of TestCALV0365_TakeoverSettlesInterruptedRemoval")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(interruptedRemovalEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.SetRunFaultForTest(repo.StateDir, func(at string) error {
		if at != "cleanup:WORKTREE_REMOVE" {
			return nil
		}
		if os.Getenv(interruptedRemovalEnv+"_MODE") == "half" {
			// git worktree remove deletes the work tree before its
			// registration; stop between the two.
			entries, err := store.ProgramRecords(context.Background(), repo)
			if err != nil || len(entries) != 1 {
				t.Fatalf("program records: %v", err)
			}
			if err := os.RemoveAll(entries[0].Worktree); err != nil {
				t.Fatal(err)
			}
		}
		if os.Getenv(interruptedRemovalEnv+"_MODE") == "removed" {
			entries, err := store.ProgramRecords(context.Background(), repo)
			if err != nil || len(entries) != 1 {
				t.Fatalf("program records: %v", err)
			}
			cmd := exec.Command("git", "worktree", "remove", entries[0].Worktree)
			cmd.Dir = repo.PrimaryWorktree
			for _, r := range entries[0].Worktrees {
				if r.Path == entries[0].Worktree && r.Repository != "" {
					cmd.Dir = os.Getenv(interruptedRemovalEnv + "_EXTRA")
				}
			}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git worktree remove: %v %s", err, out)
			}
		}
		os.Exit(3)
		return nil
	})()
	_, err = store.CleanupProgram(context.Background(), repo, operator(), "program", c)
	t.Fatalf("cleanup was not interrupted at WORKTREE_REMOVE: %v", err)
}

// TestCALV0365_TakeoverSettlesInterruptedRemoval proves a replacement owner
// settles a cleanup whose owner died after its WORKTREE_REMOVE write: the
// takeover returns the program to FINISHED with a typed outcome for the
// parked worktree (REMOVAL_ROLLED_BACK when Git still registers it, then
// removed by the checked retry; REMOVAL_COMPLETED when Git had removed it,
// or had deleted the work tree but not yet its registration),
// and the cleanup finishes the remaining worktrees; a reassign through
// OpenWorkflow settles it the same way. Before the fix the
// takeover refused ATTEMPT_LIVE and the program stayed in WORKTREE_REMOVE.
func TestCALV0365_TakeoverSettlesInterruptedRemoval(t *testing.T) {
	for _, mode := range []string{"registered", "removed", "half", "reopen"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f := designatedFixture(t)
			completedProgram(t, f)
			before := programRecord(t, f)
			config, err := json.Marshal(f.config)
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestCALV0365_InterruptedRemovalChild$", "-test.count=1")
			child.Env = append(os.Environ(), interruptedRemovalEnv+"_CONFIG="+string(config), interruptedRemovalEnv+"_ROOT="+f.s.repo.PrimaryWorktree,
				interruptedRemovalEnv+"_MODE="+mode, interruptedRemovalEnv+"_EXTRA="+f.extra)
			out, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 3 {
				t.Fatalf("child did not stop at WORKTREE_REMOVE: %v\n%s", err, out)
			}
			p := programRecord(t, f)
			if p.Phase != "WORKTREE_REMOVE" || p.OwnerPID != child.Process.Pid {
				t.Fatalf("interrupted program phase %s owner %d", p.Phase, p.OwnerPID)
			}
			parked := p.Worktree
			if mode == "reopen" {
				// A reassign takes the program over through OpenWorkflow.
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
					t.Fatalf("reopen after the interrupted removal: %v, want the idle refusal", err)
				}
				if q := programRecord(t, f); q.Assignment != 2 || q.Phase != "ADMITTED" || len(q.Worktrees) != 0 || q.Epoch != p.Epoch+1 {
					t.Fatalf("reassign after takeover: assignment %d phase %s epoch %d worktrees %+v", q.Assignment, q.Phase, q.Epoch, q.Worktrees)
				}
				if _, err := os.Lstat(parked); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("parked worktree %s remains: %v", parked, err)
				}
				return
			}
			results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
			if err != nil || len(results) == 0 {
				t.Fatalf("takeover cleanup: %+v %v", results, err)
			}
			want := store.RemovalRolledBack
			if mode != "registered" {
				want = store.RemovalCompleted
			}
			if results[0].Path != parked || results[0].Takeover != want || results[0].Removed != (mode != "registered") || (mode == "half") != strings.Contains(results[0].Reason, "git worktree prune") {
				t.Fatalf("takeover item %+v, want %s for %s", results[0], want, parked)
			}
			for _, r := range results[1:] {
				if !r.Removed || r.Takeover != "" {
					t.Fatalf("cleanup after takeover kept %+v", r)
				}
			}
			q := programRecord(t, f)
			if q.Phase != "FINISHED" || q.Epoch != p.Epoch+1 || q.OwnerPID != os.Getpid() || len(q.Worktrees) != 0 {
				t.Fatalf("settled program phase %s epoch %d (was %d) worktrees %+v", q.Phase, q.Epoch, before.Epoch, q.Worktrees)
			}
			if _, err := os.Lstat(parked); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("parked worktree %s remains: %v", parked, err)
			}
		})
	}
}

// TestCALV0366_CleanupDropsRemovedRecords proves a cleanup drops the record
// of every worktree it removes, so the program record holds only worktrees
// still on disk and a repeated cleanup neither grows it nor reports again.
// Before the fix each removed record stayed, marked removed.
func TestCALV0366_CleanupDropsRemovedRecords(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	registered := len(programRecord(t, f).Worktrees)
	if registered < 2 {
		t.Fatalf("fixture registered %d stage worktrees, want at least 2", registered)
	}
	ctx := context.Background()
	results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != registered {
		t.Fatalf("cleanup: %+v %v", results, err)
	}
	if p := programRecord(t, f); len(p.Worktrees) != 0 || p.Phase != "FINISHED" {
		t.Fatalf("cleanup kept records %+v in phase %s", p.Worktrees, p.Phase)
	}
	results, err = store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 0 || len(programRecord(t, f).Worktrees) != 0 {
		t.Fatalf("repeated cleanup: %+v %v", results, err)
	}
}
