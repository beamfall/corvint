package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// TestACXV0851WorktreeDirectoryAndByPath: ACX-V0-851. Under --worktree a
// directory --path expands from the working tree, so an untracked or ignored
// file there is planned, and --by-path keeps the worktree warning.
func TestACXV0851WorktreeDirectoryAndByPath(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for name, body := range map[string]string{".gitignore": "src/ignored.py\n", "src/fresh.py": "import core\n", "src/ignored.py": "import core\n"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	receipt, raw, stderr, code := runAffectedCLI(t, root, "--worktree", "--path", "src")
	if code != 0 {
		t.Fatalf("--worktree --path src: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}
	paths := fmt.Sprint(receipt["path_list"])
	for _, want := range []string{"src/core.py", "src/fresh.py", "src/ignored.py"} {
		if !strings.Contains(paths, want) {
			t.Fatalf("--worktree --path src path_list = %s, missing %s", paths, want)
		}
	}
	if _, raw, stderr, code := runAffectedCLI(t, root, "--path", "src"); code != 2 || len(raw) != 0 || !strings.Contains(stderr, "untracked") {
		t.Fatalf("default --path src with an untracked file: exit=%d stdout=%s stderr=%s", code, raw, stderr)
	}

	byPath, raw, stderr, code := runAffectedCLI(t, root, "--worktree", "--by-path", "--path", "src/fresh.py")
	if code != 0 {
		t.Fatalf("--worktree --by-path: exit=%d stderr=%s receipt=%s", code, stderr, raw)
	}
	warning, _ := byPath["warning"].(string)
	if !strings.Contains(warning, "WORKTREE plan") || !strings.Contains(warning, head) || !strings.Contains(warning, "dirty worktree paths") || !strings.Contains(warning, "not pinned evidence") {
		t.Fatalf("--worktree --by-path warning = %q", warning)
	}
	if degradations := fmt.Sprint(byPath["degradations"]); !strings.Contains(degradations, affectedWorktreeDegradation) {
		t.Fatalf("--worktree --by-path degradations = %s", degradations)
	}
	text := renderAffectedByPathText(affectedByPathReceipt{Warning: warning})
	if !strings.Contains(text, "\nwarning ") {
		t.Fatalf("--by-path text drops the warning: %q", text)
	}
}
