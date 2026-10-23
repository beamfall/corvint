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
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
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

// removeCrashEnv names the interruption point for the child process of
// TestCALV0365_CleanupTakesOverInterruptedRemoval.
const removeCrashEnv = "CORVINT_TEST_WORKTREE_REMOVE_CRASH"

// TestCALV0365_RemoveCrashChild is the owner process of the takeover tests: it
// runs the operator cleanup and exits at the named point of the first stage
// worktree removal without any further write, standing in for a crash. It is
// skipped otherwise.
func TestCALV0365_RemoveCrashChild(t *testing.T) {
	point := os.Getenv(removeCrashEnv)
	if point == "" {
		t.Skip("child of the CAL-V0-365 takeover tests")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(os.Getenv(removeCrashEnv+"_CONFIG")), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(removeCrashEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	store.CALTestWorktreeRemoveHook(func(at string) error {
		if at == point {
			os.Exit(3)
		}
		return nil
	})
	_, err = store.CleanupProgram(context.Background(), repo, operator(), "program", c)
	t.Fatalf("owner was not interrupted at %s: %v", point, err)
}

// crashedRemoval runs the cleanup of a completed program in a child process
// that exits at point, and returns the program record it left.
func crashedRemoval(t *testing.T, f *multiFixture, point string) snapshot.Program {
	t.Helper()
	config, err := json.Marshal(f.config)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCALV0365_RemoveCrashChild$", "-test.count=1")
	child.Env = append(os.Environ(), removeCrashEnv+"="+point, removeCrashEnv+"_CONFIG="+string(config), removeCrashEnv+"_ROOT="+f.s.repo.PrimaryWorktree)
	out, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child did not stop at %s: %v\n%s", point, err, out)
	}
	p := programRecord(t, f)
	if p.Phase != "WORKTREE_REMOVE" || p.OwnerPID != child.Process.Pid {
		t.Fatalf("interrupted program %s owner %d, want WORKTREE_REMOVE owned by %d", p.Phase, p.OwnerPID, child.Process.Pid)
	}
	return p
}

// inFlight returns the record the interrupted removal named.
func inFlight(t *testing.T, p snapshot.Program) snapshot.WorktreeRecord {
	t.Helper()
	for _, r := range p.Worktrees {
		if p.Effect == supervisor.Digest([]byte("WORKTREE_REMOVE:"+r.Path+":"+r.Identity)) {
			return r
		}
	}
	t.Fatalf("no worktree record matches the recorded effect %s", p.Effect)
	return snapshot.WorktreeRecord{}
}

// TestCALV0365_CleanupTakesOverInterruptedRemoval proves a cleanup that died
// after its WORKTREE_REMOVE write is finished by the next operator cleanup,
// with a typed outcome: ROLLED_BACK when Git had not yet removed the
// worktree (the cleanup then removes it), COMPLETED when it had (the record is
// dropped). Before CAL-V0-365 the phase was not a safe takeover phase and the
// program stayed parked in it.
func TestCALV0365_CleanupTakesOverInterruptedRemoval(t *testing.T) {
	for _, tc := range []struct{ point, resolution string }{{"recorded", store.RemovalRolledBack}, {"removed", store.RemovalCompleted}} {
		t.Run(tc.point, func(t *testing.T) {
			f := designatedFixture(t)
			completedProgram(t, f)
			stuck := crashedRemoval(t, f, tc.point)
			target := inFlight(t, stuck)
			_, statErr := os.Lstat(target.Path)
			if gone := os.IsNotExist(statErr); gone != (tc.point == "removed") {
				t.Fatalf("worktree %s gone=%v at %s", target.Path, gone, tc.point)
			}
			results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
			if err != nil {
				t.Fatalf("takeover cleanup: %v", err)
			}
			resolved := 0
			for _, r := range results {
				if r.Path == target.Path && r.Repository == target.Repository {
					resolved++
					if r.Resolution != tc.resolution || !r.Removed {
						t.Fatalf("in-flight item %+v, want %s and removed", r, tc.resolution)
					}
				} else if r.Resolution != "" {
					t.Fatalf("item %+v carries a resolution but was not in flight", r)
				}
				if !r.Removed {
					t.Fatalf("cleanup kept %+v", r)
				}
			}
			if resolved != 1 || len(results) != len(stuck.Worktrees) {
				t.Fatalf("results %+v: %d for the in-flight worktree, want 1 of %d items", results, resolved, len(stuck.Worktrees))
			}
			p := programRecord(t, f)
			if p.Phase != "FINISHED" || p.OwnerPID != os.Getpid() || p.Epoch != stuck.Epoch+1 || len(p.Worktrees) != 0 {
				t.Fatalf("settled program phase %s owner %d epoch %d worktrees %+v", p.Phase, p.OwnerPID, p.Epoch, p.Worktrees)
			}
			if list := multiGit(t, f.s.repo.PrimaryWorktree, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
				t.Fatalf("queue repository keeps a stage worktree:\n%s", list)
			}
			if list := multiGit(t, f.extra, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
				t.Fatalf("docs keeps a stage worktree:\n%s", list)
			}
		})
	}
}

// TestCALV0365_ReopenSettlesInterruptedRemoval proves a supervisor restart
// that finds the program parked in WORKTREE_REMOVE settles it and carries on
// to the reassign, instead of refusing as an unproved prior owner.
func TestCALV0365_ReopenSettlesInterruptedRemoval(t *testing.T) {
	f := designatedFixture(t)
	completedProgram(t, f)
	stuck := crashedRemoval(t, f, "removed")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("reopen after an interrupted removal: %v, want the idle refusal", err)
	}
	p := programRecord(t, f)
	if p.Phase != "ADMITTED" || p.Assignment != stuck.Assignment+1 || p.OwnerPID != os.Getpid() || len(p.Worktrees) != 0 {
		t.Fatalf("reopened program phase %s assignment %d owner %d worktrees %+v", p.Phase, p.Assignment, p.OwnerPID, p.Worktrees)
	}
}

// TestCALV0366_RemovedWorktreeRecordsAreCompacted proves a removed worktree
// leaves no record, so repeated cleanups of one assignment never grow the
// program record, and that a record an earlier release marked removed is
// dropped by the next cleanup.
func TestCALV0366_RemovedWorktreeRecordsAreCompacted(t *testing.T) {
	f := designatedFixture(t)
	completedProgram(t, f)
	full := programRecord(t, f)
	dirty := ""
	for _, r := range full.Worktrees {
		if r.Repository == "docs" {
			dirty = r.Path
			break
		}
	}
	note := filepath.Join(dirty, "note.txt")
	if err := os.WriteFile(note, []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	kept := func() []string {
		var paths []string
		for _, r := range programRecord(t, f).Worktrees {
			if r.Removed {
				t.Fatalf("a removed record is still held: %+v", r)
			}
			paths = append(paths, r.Path)
		}
		return paths
	}
	for round := 1; round <= 3; round++ {
		if _, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config); err != nil {
			t.Fatalf("cleanup %d: %v", round, err)
		}
		if got := kept(); len(got) != 1 || got[0] != dirty {
			t.Fatalf("cleanup %d holds %v of %d records, want only the dirty %s", round, got, len(full.Worktrees), dirty)
		}
	}
	// A record the earlier release marked removed: the next cleanup drops it.
	p := programRecord(t, f)
	p.Worktrees = append(p.Worktrees, snapshot.WorktreeRecord{Path: dirty + "-legacy", Identity: "1:1", PrivateIdentity: "1:2", Commit: p.Base, Removed: true})
	if r, err := store.ProgramTransition(ctx, f.s.repo, operator(), fixture.QueueID, "legacy-removed-record", p); err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("plant the legacy record: %+v %v", r, err)
	}
	if programRecord(t, f).Worktrees[len(p.Worktrees)-1].Path != dirty+"-legacy" {
		t.Fatal("legacy record not planted")
	}
	if _, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config); err != nil {
		t.Fatalf("cleanup with a legacy record: %v", err)
	}
	if got := kept(); len(got) != 1 || got[0] != dirty {
		t.Fatalf("legacy record survived: %v", got)
	}
}
