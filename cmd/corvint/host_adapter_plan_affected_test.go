package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// AHI-632 (V1-0878): a plan's named file gets a PRODUCED working-tree affected plan that names its
// selected units and keeps the worktree qualification (ACX-V0-851); a refused plan and a late one
// get their NOT_PRODUCED reasons; a symlinked ancestor never admits a file outside the project.
func TestAHI632PlanAffectedOutcomes(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendAffectedLine(t, root, "core/core.go", "// planned edit")
	paths := []string{"core/core.go"}

	object := planAffectedObject(context.Background(), root, paths)
	units, _ := object["selectedUnits"].([]string)
	if object["state"] != "PRODUCED" || !slices.Contains(units, wantPlanAffectedUnit) || object["omittedSelectedUnits"] != 0 {
		t.Fatalf("plan affected object = %v, want PRODUCED selecting %s", object, wantPlanAffectedUnit)
	}
	warning, _ := object["warning"].(string)
	degradations, _ := object["degradations"].([]string)
	if object["revision"] != head || !strings.Contains(warning, "not pinned evidence") || !slices.Contains(degradations, affectedWorktreeDegradation) {
		t.Fatalf("plan affected object lost its worktree qualification: %v", object)
	}
	if summary, _ := object["summary"].(string); !strings.Contains(summary, "dirty=1") {
		t.Fatalf("plan affected summary = %q", summary)
	}

	if refused := planAffectedObject(context.Background(), t.TempDir(), paths); refused["state"] != "NOT_PRODUCED" || refused["reason"] != "plan-affected-refused" {
		t.Fatalf("a plan outside a repository = %v, want plan-affected-refused", refused)
	}
	// A deadline already inside the reserve cuts the plan off, whatever finishes first.
	late, cancel := context.WithDeadline(context.Background(), time.Now().Add(planAffectedReserve/2))
	defer cancel()
	if object := startPlanAffected(late, root, paths)(); object["state"] != "NOT_PRODUCED" || object["reason"] != "plan-affected-deadline" {
		t.Fatalf("a late plan = %v, want plan-affected-deadline", object)
	}

	// The held-back room covers the compact fallback object, so appending it fits the bound.
	if reserve, fallback := planAffectedContextReserve(paths), promptBoundReserve("\n\n"+renderPlanAffected(planAffectedRefusal(paths, "plan-affected-over-budget"))); reserve <= fallback {
		t.Fatalf("reserve %d does not cover the fallback object (%d)", reserve, fallback)
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file.go"), []byte("package outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "external")); err != nil {
		t.Fatal(err)
	}
	if got := planAffectedPaths(root, "Edit external/file.go and core/core.go."); !slices.Equal(got, paths) {
		t.Fatalf("plan paths = %q, want only %q (a symlinked ancestor leaves the project)", got, paths)
	}
}

// wantPlanAffectedUnit is a unit the fixture selects for an edit of core/core.go.
const wantPlanAffectedUnit = "go:example.com/fixture/core"
