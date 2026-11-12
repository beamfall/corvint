package flows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AMAP-V0-977: without --worktree an uncommitted manifest keeps refusing as
// appmap-invalid-manifest and the message names --worktree; a manifest absent from the working
// tree too gets no hint; --worktree refuses every pinned or aliased flag and a workspace file.
func TestAMAPV0977WorktreeRefusals(t *testing.T) {
	root := t.TempDir()
	write := func(rel, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "fixture\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=", "commit", "-q", "-m", "fixture"}} {
		cmd := fixtureGitCommand(args...)
		cmd.Dir, cmd.Env = root, append(fixtureGitEnvironment(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	write("appmap.json", `{"schema":"application-map-manifest/0","app":"web","routers":[{"path":"src/routes.tsx","dialect":"react-router/0"}],"tests":{"root":"e2e","specs":["e2e/specs"]}}`)
	write("src/routes.tsx", "export const router = createBrowserRouter([{ path: '/', element: <Home /> }]);\n")
	write("e2e/specs/home.spec.ts", "test('home', async ({ page }) => { await page.goto('/'); });\n")

	code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
	if code == 0 || !strings.Contains(diag, "appmap-invalid-manifest") || !strings.Contains(diag, "--worktree") {
		t.Fatalf("default refusal: %d %s", code, diag)
	}
	code, _, diag = runFlowsCLI(root, "appmap", "build", "--manifest", "missing.json", "--no-workspace")
	if code == 0 || !strings.Contains(diag, "appmap-invalid-manifest") || strings.Contains(diag, "--worktree") {
		t.Fatalf("a manifest absent from the working tree too: %d %s", code, diag)
	}
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--worktree")
	if code != 0 || !strings.Contains(out, `"evidence":"unpinned"`) {
		t.Fatalf("--worktree build: %d %s", code, diag)
	}
	ws := filepath.Join(t.TempDir(), "workspace.json")
	for _, extra := range [][]string{
		{"--revision", "HEAD"},
		{"--repo", "other=" + root},
		{"--repo", "other=" + root, "--manifest-repo", "other"},
		{"--repo", "other=" + root, "--repo-revision", "other=HEAD"},
		{"--workspace", ws},
		{"--from", ws},
		{"--from", ws, "--verify"},
	} {
		args := append([]string{"appmap", "build", "--manifest", "appmap.json", "--worktree"}, extra...)
		code, _, diag := runFlowsCLI(root, args...)
		if code == 0 || !strings.Contains(diag, "--worktree does not combine") {
			t.Errorf("%v: %d %s", extra, code, diag)
		}
	}
	// A discovered workspace file is an implicit aliased root: --worktree refuses it unless
	// --no-workspace skips discovery.
	write(".corvint/workspace.json", `{"profile":"corvint-workspace/0","repos":{"other":{"root":"`+filepath.ToSlash(root)+`"}}}`)
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--worktree"); code == 0 || !strings.Contains(diag, "--worktree does not combine") {
		t.Errorf("implicit workspace: %d %s", code, diag)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--worktree", "--no-workspace"); code != 0 {
		t.Errorf("--no-workspace: %d %s", code, diag)
	}
}
