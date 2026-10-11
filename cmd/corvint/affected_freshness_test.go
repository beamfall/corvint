package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func appendAffectedLine(t *testing.T, root, name, line string) {
	t.Helper()
	path := filepath.Join(root, name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, []byte("\n"+line+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-044: a --base range path outside the worktree dirty set, planned over
// a dirty worktree, carries a warning naming HEAD and the dirty count; every
// other receipt keeps its member set.
func TestAffectedMixedRangeWarning(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	appendAffectedLine(t, root, "core/core.go", "// committed edit")
	affectedGit(t, root, "commit", "-qam", "edit core")
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))

	clean, _, stderr, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 || clean["warning"] != nil {
		t.Fatalf("clean range: exit=%d warning=%v stderr=%s", code, clean["warning"], stderr)
	}

	appendAffectedLine(t, root, "leaf/leaf.go", "// MUTABLE_CANARY")
	for _, form := range [][]string{{"--base", base}, {"--full", "--base", base}} {
		receipt, raw, stderr, code := runAffectedCLI(t, root, form...)
		warning, _ := receipt["warning"].(string)
		if code != 0 || !strings.Contains(warning, head) || !strings.Contains(warning, "1 dirty worktree paths") || bytes.Contains(raw, []byte("MUTABLE_CANARY")) {
			t.Fatalf("%v: exit=%d stderr=%s receipt=%s", form, code, stderr, raw)
		}
	}
	worktreeOnly, _, _, code := runAffectedCLI(t, root)
	if code != 0 || worktreeOnly["warning"] != nil {
		t.Fatalf("worktree-only form warned: %v", worktreeOnly["warning"])
	}

	affectedGit(t, root, "checkout", "-q", "--", "leaf/leaf.go")
	appendAffectedLine(t, root, "core/core.go", "// dirty again")
	inside, _, _, code := runAffectedCLI(t, root, "--base", base)
	if code != 0 || inside["warning"] != nil {
		t.Fatalf("range inside the dirty set warned: %v", inside["warning"])
	}
}

// AFP-V0-045: --require-clean leaves a clean receipt byte-identical and
// refuses a dirty worktree with a typed error before any provider runs.
func TestAffectedRequireClean(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	_, ordinary, _, code := runAffectedCLI(t, root)
	_, strict, stderr, strictCode := runAffectedCLI(t, root, "--require-clean")
	if code != 0 || strictCode != 0 || !bytes.Equal(ordinary, strict) {
		t.Fatalf("clean strict receipt changed: exit=%d/%d stderr=%s", code, strictCode, stderr)
	}
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, form := range [][]string{
		{"--require-clean"},
		{"--full", "--require-clean", "--base", head},
		// A provider that ran would refuse with its own code instead.
		{"--require-clean", "--provider", "absent-provider.json"},
	} {
		before := treeDigest(t, root)
		_, raw, stderr, code := runAffectedCLI(t, root, form...)
		if code != 2 || len(raw) != 0 || !strings.Contains(stderr, "unsupported-affected-mixed-worktree") || !strings.Contains(stderr, head) || !strings.Contains(stderr, "1 dirty worktree paths") {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", form, code, raw, stderr)
		}
		if treeDigest(t, root) != before {
			t.Fatalf("%v wrote repository state", form)
		}
	}
	for _, form := range [][]string{
		{"--require-clean=true"},
		{"--require-clean", "--require-clean"},
		{"--require-clean", "--snapshot", "receipt.json"},
		{"discovery", "--require-clean", "--playwright-config", "a.ts", "--playwright-list", "b.json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := runContext(context.Background(), append([]string{"--root", root, "affected"}, form...), strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid-arguments") {
			t.Fatalf("%v: exit=%d stdout=%s stderr=%s", form, code, stdout.String(), stderr.String())
		}
	}
	if !strings.Contains(affectedHelp, "--require-clean") || !strings.Contains(affectedHelp, "unsupported-affected-mixed-worktree") {
		t.Fatal("affected help omits --require-clean")
	}
}
