//go:build darwin || linux

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
)

const removeCrashEnv = "CORVINT_TEST_CALV0365_POINT"

// TestCALV0365_InterruptedRemoveChild is the program owner that
// TestCALV0365_InterruptedWorktreeRemoveTakenOver stops inside a cleanup.
func TestCALV0365_InterruptedRemoveChild(t *testing.T) {
	point := os.Getenv(removeCrashEnv)
	if point == "" {
		t.Skip("child of TestCALV0365_InterruptedWorktreeRemoveTakenOver")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(os.Getenv(removeCrashEnv+"_CONFIG")), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(removeCrashEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	store.SetRunFaultForTest(repo.StateDir, func(at string) error {
		if at == point {
			os.Exit(3)
		}
		return nil
	})
	_, err = store.CleanupProgram(context.Background(), repo, operator(), "program", c)
	t.Fatalf("owner was not interrupted at %s: %v", point, err)
}

// recordAt returns the program's worktree record at path, if any.
func recordAt(p snapshot.Program, path string) *snapshot.WorktreeRecord {
	for i := range p.Worktrees {
		if p.Worktrees[i].Path == path {
			return &p.Worktrees[i]
		}
	}
	return nil
}

// TestCALV0365_InterruptedWorktreeRemoveTakenOver proves an owner that stops
// after a WORKTREE_REMOVE write leaves a program a different process takes
// over (CAL-V0-365): stopped before `git worktree remove`, the takeover
// finishes the removal; stopped after it, the takeover records it; with the
// worktree dirtied meanwhile, the takeover rolls back to FINISHED and keeps
// the record. Each settles with a typed outcome, through `program cleanup` or
// through the reopen that reassigns the program.
func TestCALV0365_InterruptedWorktreeRemoveTakenOver(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, point   string
		dirty, reopen bool
		want          string
	}{
		{"before-remove", "worktree-remove:written", false, false, store.RemovalFinished},
		{"after-remove", "worktree-remove:removed", false, false, store.RemovalFinished},
		{"dirtied", "worktree-remove:written", true, false, store.RemovalRolledBack},
		{"reopen", "worktree-remove:written", false, true, store.RemovalFinished},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := designatedFixture(t)
			completedProgram(t, f)
			config, err := json.Marshal(f.config)
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(os.Args[0], "-test.run=^TestCALV0365_InterruptedRemoveChild$", "-test.count=1")
			child.Env = append(os.Environ(), removeCrashEnv+"="+tc.point, removeCrashEnv+"_CONFIG="+string(config), removeCrashEnv+"_ROOT="+f.s.repo.PrimaryWorktree)
			out, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 3 {
				t.Fatalf("child did not stop at %s: %v\n%s", tc.point, err, out)
			}
			p := programRecord(t, f)
			if p.Phase != "WORKTREE_REMOVE" || p.OwnerPID != child.Process.Pid || recordAt(p, p.Worktree) == nil {
				t.Fatalf("interrupted program %+v", p)
			}
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(p.Worktree, "operator.txt"), []byte("edit\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.reopen {
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
					t.Fatalf("reopen of an interrupted removal: %v, want the reassign's idle refusal", err)
				}
				q := programRecord(t, f)
				if q.Assignment != p.Assignment+1 || q.Phase != "ADMITTED" || q.OwnerPID != os.Getpid() || q.Epoch != p.Epoch+1 || recordAt(q, p.Worktree) != nil {
					t.Fatalf("reopen after the takeover: %+v", q)
				}
				if _, err := os.Lstat(p.Worktree); !os.IsNotExist(err) {
					t.Fatalf("interrupted worktree left on disk: %v", err)
				}
				return
			}
			results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
			if err != nil || len(results) == 0 {
				t.Fatalf("takeover cleanup: %+v %v", results, err)
			}
			first := results[0]
			if first.Path != p.Worktree || first.Outcome != tc.want || first.Removed != (tc.want == store.RemovalFinished) {
				t.Fatalf("settled removal %+v, want %s for %s", first, tc.want, p.Worktree)
			}
			for _, r := range results[1:] {
				if r.Outcome != "" {
					t.Fatalf("only the interrupted removal carries an outcome: %+v", r)
				}
			}
			q := programRecord(t, f)
			if q.Phase != "FINISHED" || q.OwnerPID != os.Getpid() || q.Epoch != p.Epoch+1 {
				t.Fatalf("program after the takeover: %+v", q)
			}
			_, statErr := os.Lstat(p.Worktree)
			if tc.dirty {
				if first.Reason == "" || recordAt(q, p.Worktree) == nil || statErr != nil {
					t.Fatalf("rolled-back removal: %+v record %v on disk %v", first, recordAt(q, p.Worktree), statErr)
				}
				return
			}
			if recordAt(q, p.Worktree) != nil || !os.IsNotExist(statErr) {
				t.Fatalf("finished removal kept its record %v or worktree %v", recordAt(q, p.Worktree), statErr)
			}
		})
	}
}

// TestCALV0366_RemovedWorktreeRecordsDropped proves a cleanup drops the
// record of every worktree it removes, so repeated cleanups of one assignment
// leave only the kept worktrees recorded and never grow them (CAL-V0-366).
func TestCALV0366_RemovedWorktreeRecordsDropped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := designatedFixture(t)
	completedProgram(t, f)
	before := programRecord(t, f)
	if len(before.Worktrees) < 2 {
		t.Fatalf("fixture registered %d stage worktrees", len(before.Worktrees))
	}
	dirty := before.Worktrees[0].Path
	if err := os.WriteFile(filepath.Join(dirty, "operator.txt"), []byte("edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for round := 1; round <= 3; round++ {
		results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
		if err != nil {
			t.Fatalf("cleanup %d: %v", round, err)
		}
		p := programRecord(t, f)
		if len(p.Worktrees) != 1 || p.Worktrees[0].Path != dirty || p.Worktrees[0].Removed || p.Assignment != before.Assignment {
			t.Fatalf("cleanup %d recorded %+v, want only the kept %s", round, p.Worktrees, dirty)
		}
		if round == 1 && len(results) != len(before.Worktrees) {
			t.Fatalf("first cleanup reported %d items for %d records", len(results), len(before.Worktrees))
		}
		if round > 1 && (len(results) != 1 || results[0].Removed) {
			t.Fatalf("cleanup %d reported %+v, want only the kept worktree", round, results)
		}
	}
}
