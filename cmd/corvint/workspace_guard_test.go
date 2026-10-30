package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// WSD-V0-146 (#1933): witness, review and cem read one repository. A named workspace refuses as
// workspace-unsupported naming the verb, the ignored aliases and the remedy; so does a discovered
// one unless --no-workspace, except for a paired witness, whose --range flags name each repository.
func TestWSDV0146SingleRootVerbsRefuseWorkspace(t *testing.T) {
	root, tests := t.TempDir(), t.TempDir()
	for _, dir := range []string{root, tests} {
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = dir, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, b)
			}
		}
	}
	head := func(dir string) string {
		b, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}
	base := head(root)
	named := filepath.Join(t.TempDir(), "workspace.json")
	if err := os.WriteFile(named, []byte(`{"profile":"corvint-workspace/0","repos":{"tests":{"root":"`+tests+`"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (int, string) {
		var out, diag bytes.Buffer
		code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &diag)
		return code, diag.String()
	}
	verbs := map[string][]string{
		"witness": {"witness", "--base", base},
		"review":  {"review", "--base", base},
		"cem":     {"cem", "status"},
	}
	for verb, args := range verbs {
		code, diag := run(append(args, "--workspace", named)...)
		if code != 2 || !strings.Contains(diag, "workspace-unsupported") || !strings.Contains(diag, verb+" reads only --root") || !strings.Contains(diag, "aliases tests") || !strings.Contains(diag, workspaceRemedies[verb]) {
			t.Fatalf("%s --workspace: %d %s", verb, code, diag)
		}
		if code, diag = run(append(args, "--workspace", named, "--no-workspace")...); code != 2 || !strings.Contains(diag, "does not combine") {
			t.Fatalf("%s --workspace --no-workspace: %d %s", verb, code, diag)
		}
	}
	// Without a workspace file the verbs run as before; record each outcome to compare.
	before := map[string]int{}
	for verb, args := range verbs {
		before[verb], _ = run(args...)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(named)
	if err := os.WriteFile(filepath.Join(root, ".corvint", "workspace.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// review requires a clean worktree; the declaration is local configuration.
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte(".corvint/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for verb, args := range verbs {
		code, diag := run(args...)
		if code != 2 || !strings.Contains(diag, "workspace-unsupported") || !strings.Contains(diag, "--no-workspace runs "+verb+" on --root alone") {
			t.Fatalf("%s discovered: %d %s", verb, code, diag)
		}
		if code, diag = run(append(args, "--no-workspace")...); code != before[verb] || strings.Contains(diag, "workspace-") {
			t.Fatalf("%s --no-workspace: %d (want %d) %s", verb, code, before[verb], diag)
		}
	}
	paired := []string{"witness", "--range", "app=" + base + ".." + base, "--range", "tests=" + head(tests) + ".." + head(tests), "--range-root", "tests=" + tests}
	if _, diag := run(paired...); strings.Contains(diag, "workspace-") {
		t.Fatalf("paired witness refused a discovered workspace: %s", diag)
	}
	if code, diag := run(append(paired, "--workspace", named)...); code != 2 || !strings.Contains(diag, "workspace-unsupported") {
		t.Fatalf("paired witness --workspace: %d %s", code, diag)
	}
}
