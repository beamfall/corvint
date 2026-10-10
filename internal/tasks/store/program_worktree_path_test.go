//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0086_LongWorkRootStageDispatches is the V1-0772 regression: a
// supervised stage worktree whose path exceeds 128 bytes (an agent TMPDIR or
// a deep operator work root) dispatches and builds, because the attempt
// records it as a PathText rather than an Identifier. Before the fix DISPATCH
// refused LIMIT_EXCEEDED after the worktree and its program records existed.
func TestCALV0086_LongWorkRootStageDispatches(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, false, true, nil)
	f.config.WorkRoot = filepath.Join(fixture.TempDirOutside(t), strings.Repeat("w", 160))
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement under a long work root: %v", err)
	}
	if a.WorktreePath == nil || len(*a.WorktreePath) <= wire.MaxIdentifierBytes || !strings.HasPrefix(*a.WorktreePath, f.config.WorkRoot+"/") {
		t.Fatalf("recorded worktree path %v", a.WorktreePath)
	}
}

// TestCALV0086_OverlongWorktreeRefusedBeforeMutation proves a stage
// worktree path the attempt cannot carry (over MaxPathTextBytes) is refused
// with LIMIT_EXCEEDED before any directory, program record or Git worktree
// exists.
func TestCALV0086_OverlongWorktreeRefusedBeforeMutation(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, false, false, nil)
	base := fixture.TempDirOutside(t)
	f.config.WorkRoot = filepath.Join(base, strings.Repeat(strings.Repeat("w", 200)+"/", 21))
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	programs := filepath.Join(f.s.repo.StateDir, "programs.json")
	before, _ := os.ReadFile(programs)
	if _, err = w.RunRole(ctx, "implementer", ""); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("overlong worktree path: want %s, got %v", wire.CodeLimitExceeded, err)
	}
	if after, _ := os.ReadFile(programs); !bytes.Equal(before, after) {
		t.Fatal("refusal changed the program inventory")
	}
	if entries, _ := os.ReadDir(base); len(entries) != 0 {
		t.Fatal("refusal created work directories")
	}
	if list := multiGit(t, f.s.repo.PrimaryWorktree, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("refusal registered a worktree:\n%s", list)
	}
}

// TestCALV0086_UnprovedStopIsNotFinished proves a stage whose drain cannot
// prove quiescence (a setsid process still holds the host output) ends the
// role with a truthful non-retryable SURVIVORS error, leaving the program and
// attempt BLOCKED_RECOVERY and the owner unreleased. Before the fix the role
// went on to journal the program FINISHED and was refused MALFORMED
// ("program transition BLOCKED_RECOVERY -> FINISHED"), the load failure that
// masked V1-0772. SURVIVORS also takes precedence over the stage's own
// failure, here its stage wall, whose text the error keeps; before that fix
// the wall's context deadline reached the caller as MALFORMED. A stage over an
// existing candidate (review, here) that also dirtied the worktree is
// classified the same way, before any candidate check; before that fix it
// returned "read-only stage changed candidate" and left the attempt STOPPING.
func TestCALV0086_UnprovedStopIsNotFinished(t *testing.T) {
	for _, tc := range []struct {
		name  string
		role  string
		files []string
		wall  int
		cause string
	}{
		{name: "after a clean exit", role: "implementer", files: []string{"escape"}},
		{name: "after the stage wall", role: "implementer", files: []string{"escape", "slow"}, wall: 2, cause: context.DeadlineExceeded.Error()},
		{name: "over a candidate it changed", role: "reviewer", files: []string{"review-escape"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := buildProgramFixture(t, false, false, nil)
			if tc.wall != 0 {
				f.config.WallSeconds = tc.wall
			}
			self, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if tc.role == "reviewer" {
				if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
					t.Fatalf("implement: %+v %v", a, err)
				}
			}
			for _, name := range tc.files {
				if err := os.WriteFile(filepath.Join(f.scripts, name), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			a, err := w.RunRole(ctx, tc.role, "")
			if wire.CodeOf(err) != wire.CodeSurvivors || !wire.RetryForbidden(err) || !strings.Contains(err.Error(), tc.cause) {
				t.Fatalf("unproved stop: want non-retryable %s keeping %q, got %s %v", wire.CodeSurvivors, tc.cause, wire.CodeOf(err), err)
			}
			if a == nil || a.Phase != "BLOCKED_RECOVERY" || a.Quiescence != "SURVIVORS" {
				t.Fatalf("attempt after unproved stop: %+v", a)
			}
			entries, err := store.ProgramRecords(ctx, f.s.repo)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range entries {
				if p.ID == "program" && (p.Phase != "BLOCKED_RECOVERY" || p.OwnerReleased || p.Quiescence == "PROVED") {
					t.Fatalf("program after unproved stop: phase %s released %v quiescence %s", p.Phase, p.OwnerReleased, p.Quiescence)
				}
			}
		})
	}
}

// TestCALV0086_WatcherToleratesTransientReadFailure proves the stage watcher
// does not cancel a running stage on an unlocked program read that fails
// while a concurrent writer stages its journal (observed under load as
// "MALFORMED: staging/a00: unassigned stage slot"): failures shorter than the
// watcher's bounded tolerance leave the stage to finish and build.
func TestCALV0086_WatcherToleratesTransientReadFailure(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, false, false, nil)
	if err := os.WriteFile(filepath.Join(f.scripts, "slow"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var reads atomic.Int32
	defer store.SetRunFaultForTest(f.s.repo.StateDir, func(point string) error {
		if point == "watch-read" {
			reads.Add(1)
			return wire.Errorf(wire.CodeMalformed, "staging/a00", "unassigned stage slot")
		}
		return nil
	})()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	a, err := w.RunRole(ctx, "implementer", "")
	if err != nil || a.Phase != "BUILT" {
		t.Fatalf("stage under transient watcher read failures: %v", err)
	}
	if reads.Load() == 0 {
		t.Fatal("the watcher never read during the stage")
	}
}
