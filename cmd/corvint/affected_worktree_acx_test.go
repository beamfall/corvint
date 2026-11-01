package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestACXV0851WorktreePathPlan: ACX-V0-851 (V1-3327). On a dirty worktree
// --path still refuses by default, while --worktree plans the named paths
// against the working tree and labels the receipt as non-evidence.
func TestACXV0851WorktreePathPlan(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendAffectedLine(t, root, "core/core.go", "// local edit")

	_, raw, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go")
	if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") {
		t.Fatalf("default dirty --path: exit=%d stdout=%s stderr=%s", code, raw, stderr)
	}

	receipt, raw, stderr, code := runAffectedCLI(t, root, "--worktree", "--path", "core/core.go")
	if code != 0 || receipt["plan"] == nil {
		t.Fatalf("--worktree: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}
	warning, _ := receipt["warning"].(string)
	if !strings.Contains(warning, "WORKTREE plan") || !strings.Contains(warning, head) || !strings.Contains(warning, "1 dirty worktree paths") || !strings.Contains(warning, "not pinned evidence") {
		t.Fatalf("--worktree warning = %q", warning)
	}
	degradations := fmt.Sprint(receipt["degradations"])
	if !strings.Contains(degradations, affectedWorktreeDegradation) {
		t.Fatalf("--worktree degradations = %s", degradations)
	}
	if paths := fmt.Sprint(receipt["path_list"]); paths != "[core/core.go]" {
		t.Fatalf("--worktree path_list = %s", paths)
	}

	for _, arguments := range [][]string{
		{"--worktree"},
		{"--worktree", "--path", "core/core.go", "--require-clean"},
		{"--worktree", "--worktree", "--path", "core/core.go"},
		{"--worktree=yes", "--path", "core/core.go"},
	} {
		_, raw, stderr, code := runAffectedCLI(t, root, arguments...)
		if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "--worktree") {
			t.Fatalf("%q: exit=%d stdout=%s stderr=%s", arguments, code, raw, stderr)
		}
	}
	if !strings.Contains(affectedHelp, "--worktree") || !slices.ContainsFunc(strings.Split(affectedHelp, "\n"), func(line string) bool { return strings.Contains(line, "worktree-path-plan") }) {
		t.Fatal("affected help does not document --worktree")
	}
}
