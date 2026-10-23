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

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
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

// TestCALV0365_TakeoverSettlesInterruptedWorktreeRemove stops a program
// cleanup in a child process right after its WORKTREE_REMOVE write (before
// git removes the checkout) or right after git removed it (before the phase
// returns). A new owner's cleanup then takes the program over in
// WORKTREE_REMOVE and settles that removal with a typed outcome: FINISHED
// when the checkout is removed, ROLLED_BACK with the record kept when the
// checkout no longer binds. Either way the phase returns to FINISHED and the
// remaining worktrees are cleaned as usual (CAL-V0-365).
func TestCALV0365_TakeoverSettlesInterruptedWorktreeRemove(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stage string
		dirty       bool
		moved       bool
		want        string
	}{
		{"before-git-remove", "persisted", false, false, "FINISHED"},
		{"after-git-remove", "removed", false, false, "FINISHED"},
		{"rolled-back", "persisted", true, false, "ROLLED_BACK"},
		{"moved", "persisted", false, true, "ROLLED_BACK"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := designatedFixture(t)
			completedProgram(t, f)
			before := programRecord(t, f).Worktrees
			if len(before) < 2 {
				t.Fatalf("fixture registered %d worktrees, want at least 2", len(before))
			}
			interruptedCleanup(t, f, tc.stage)
			p := programRecord(t, f)
			if p.Phase != "WORKTREE_REMOVE" {
				t.Fatalf("interrupted cleanup left phase %s, want WORKTREE_REMOVE", p.Phase)
			}
			pending := p.Worktree
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(pending, "operator.txt"), []byte("edit\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.moved {
				// A moved checkout keeps its git registration elsewhere; it
				// is not a finished removal.
				multiGit(t, f.s.repo.PrimaryWorktree, "worktree", "move", pending, pending+"-moved")
			}
			results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
			if err != nil {
				t.Fatalf("takeover cleanup: %v", err)
			}
			if len(results) != len(before) || results[0].Path != pending || results[0].Takeover != tc.want {
				t.Fatalf("takeover cleanup results %+v, want %d items led by %s %s", results, len(before), pending, tc.want)
			}
			kept := tc.dirty || tc.moved
			if removed := results[0].Removed; removed == kept || (!removed && results[0].Reason == "") {
				t.Fatalf("settled item %+v", results[0])
			}
			for _, r := range results[1:] {
				if !r.Removed || r.Takeover != "" {
					t.Fatalf("later item %+v, want an ordinary removal", r)
				}
			}
			p = programRecord(t, f)
			if p.Phase != "FINISHED" || p.Epoch < 2 {
				t.Fatalf("settled program phase %s epoch %d", p.Phase, p.Epoch)
			}
			_, statErr := os.Lstat(pending)
			if tc.moved {
				_, statErr = os.Lstat(pending + "-moved")
			}
			if kept {
				if len(p.Worktrees) != 1 || p.Worktrees[0].Path != pending || statErr != nil {
					t.Fatalf("rolled-back removal kept %+v (checkout %v)", p.Worktrees, statErr)
				}
			} else if len(p.Worktrees) != 0 || !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("finished removal kept %+v (checkout %v)", p.Worktrees, statErr)
			}
		})
	}
}

// TestCALV0365_ReassignSettlesInterruptedWorktreeRemove proves a supervisor
// reopening the program after an interrupted cleanup takes it over in
// WORKTREE_REMOVE, finishes the removal and reassigns (CAL-V0-365).
func TestCALV0365_ReassignSettlesInterruptedWorktreeRemove(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	interruptedCleanup(t, f, "persisted")
	pending := programRecord(t, f).Worktree
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("reopen after an interrupted cleanup: %v, want the idle refusal", err)
	}
	p := programRecord(t, f)
	if p.Assignment != 2 || p.Phase != "ADMITTED" || len(p.Worktrees) != 0 {
		t.Fatalf("reassign recorded assignment %d phase %s worktrees %+v", p.Assignment, p.Phase, p.Worktrees)
	}
	if _, err := os.Lstat(pending); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted removal's checkout %s: %v", pending, err)
	}
}

// TestCALV0365_IdleReopenSettlesInterruptedWorktreeRemove proves an idle
// program (ADMITTED, no attempt) left in WORKTREE_REMOVE by a stopped cleanup
// is settled back to ADMITTED by the next OpenWorkflow before it claims
// (CAL-V0-365).
func TestCALV0365_IdleReopenSettlesInterruptedWorktreeRemove(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	kept := ""
	for _, r := range programRecord(t, f).Worktrees {
		if r.Repository == "docs" {
			kept = r.Path
		}
	}
	note := filepath.Join(kept, "note.txt")
	if err := os.WriteFile(note, []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	// The reassign runs in a child too, so no owner of the program lives.
	interruptedCleanup(t, f, "reopen")
	if p := programRecord(t, f); p.Phase != "ADMITTED" || p.CurrentAttempt != "" || len(p.Worktrees) != 1 {
		t.Fatalf("reassign recorded phase %s attempt %q worktrees %+v", p.Phase, p.CurrentAttempt, p.Worktrees)
	}
	multiGit(t, kept, "checkout", "--", "note.txt")
	interruptedCleanup(t, f, "persisted")
	if p := programRecord(t, f); p.Phase != "WORKTREE_REMOVE" || p.CurrentAttempt != "" || p.Worktree != kept {
		t.Fatalf("interrupted idle cleanup left phase %s attempt %q worktree %s", p.Phase, p.CurrentAttempt, p.Worktree)
	}
	if _, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("reopen after the interrupted idle cleanup: %v, want the idle refusal", err)
	}
	p := programRecord(t, f)
	if p.Phase != "ADMITTED" || len(p.Worktrees) != 0 {
		t.Fatalf("idle settlement recorded phase %s worktrees %+v", p.Phase, p.Worktrees)
	}
	if _, err := os.Lstat(kept); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("idle settlement left the checkout: %v", err)
	}
}

// interruptedCleanup runs program cleanup in a child test process that exits
// at the named removal step, as a killed operator command would; stage
// "reopen" instead runs one OpenWorkflow reassign there and exits.
func interruptedCleanup(t *testing.T, f *multiFixture, stage string) {
	t.Helper()
	raw, err := json.Marshal(f.config)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(config, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCALV0365Child$")
	cmd.Env = append(os.Environ(), "CORVINT_TEST_CLEANUP_STAGE="+stage, "CORVINT_TEST_CLEANUP_ROOT="+f.s.repo.PrimaryWorktree, "CORVINT_TEST_CLEANUP_CONFIG="+config)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child cleanup: %v\n%s", err, out)
	}
}

// TestCALV0365Child is interruptedCleanup's child; it does nothing unless
// that helper started it.
func TestCALV0365Child(t *testing.T) {
	stage := os.Getenv("CORVINT_TEST_CLEANUP_STAGE")
	if stage == "" {
		t.Skip("run by interruptedCleanup")
	}
	raw, err := os.ReadFile(os.Getenv("CORVINT_TEST_CLEANUP_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	var config store.ProgramConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv("CORVINT_TEST_CLEANUP_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	if stage == "reopen" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.OpenWorkflow(context.Background(), repo, operator(), "program", self, config, ""); !errors.Is(err, store.ErrProgramIdle) {
			t.Fatalf("child reassign: %v", err)
		}
		os.Exit(3)
	}
	store.SetWorktreeRemoveHookForTest(func(s string) {
		if s == stage {
			os.Exit(3)
		}
	})
	_, err = store.CleanupProgram(context.Background(), repo, operator(), "program", config)
	t.Fatalf("cleanup was not interrupted: %v", err)
}

// TestCALV0366_CleanupDropsRemovedWorktreeRecords proves a cleanup drops the
// records of the worktrees it removed instead of keeping them marked removed,
// so repeated cleanups within one assignment never grow the program record
// (CAL-V0-366).
func TestCALV0366_CleanupDropsRemovedWorktreeRecords(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	p := programRecord(t, f)
	if len(p.Worktrees) < 2 {
		t.Fatalf("fixture registered %d worktrees, want at least 2", len(p.Worktrees))
	}
	// A record an earlier version kept marked removed is dropped too.
	legacy := p
	legacy.Worktrees = append([]snapshot.WorktreeRecord{{Path: filepath.Join(t.TempDir(), "removed-earlier"), Identity: "1:1", PrivateIdentity: "1:2", Commit: p.Worktrees[0].Commit, Removed: true}}, p.Worktrees...)
	if r, err := store.ProgramTransition(context.Background(), f.s.repo, operator(), fixture.QueueID, "legacy-removed", legacy); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("legacy record: %+v %v", r, err)
	}
	assignment := p.Assignment
	for round := 0; round < 3; round++ {
		results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
		if err != nil {
			t.Fatalf("cleanup %d: %v", round, err)
		}
		if round == 0 && len(results) != len(p.Worktrees) || round > 0 && len(results) != 0 {
			t.Fatalf("cleanup %d results %+v", round, results)
		}
		got := programRecord(t, f)
		if len(got.Worktrees) != 0 || got.Assignment != assignment || got.Phase != "FINISHED" {
			t.Fatalf("cleanup %d left assignment %d phase %s worktrees %+v", round, got.Assignment, got.Phase, got.Worktrees)
		}
	}
}
