package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCEMV1250WorkspaceRefusesEverySpellingOfTheRepository pins the shared
// helper both flags use: a TMPDIR inside the worktree, inside the Git
// directory, or naming the worktree through a case alias on a
// case-insensitive volume is refused, and one outside is used.
func TestCEMV1250WorkspaceRefusesEverySpellingOfTheRepository(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "repo")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	inside := filepath.Join(root, "scratch-tmp")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	refused := []string{root, inside, filepath.Join(root, ".git")}
	alias := filepath.Join(parent, "REPO", "scratch-tmp")
	if aliasInfo, err := os.Stat(alias); err == nil {
		if insideInfo, err := os.Stat(inside); err == nil && os.SameFile(aliasInfo, insideInfo) {
			refused = append(refused, alias)
		}
	}
	for _, temp := range refused {
		setTempDir(t, temp)
		if dir, err := disposableWorkspace(context.Background(), root, "corvint-test-"); err == nil || !strings.Contains(err.Error(), "lies inside the repository") {
			t.Fatalf("TMPDIR %s: dir %q, err %v; want a refusal", temp, dir, err)
		}
	}
	entries, err := os.ReadDir(inside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("something was created inside the repository: %v %v", entries, err)
	}
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	setTempDir(t, outside)
	dir, err := disposableWorkspace(context.Background(), root, "corvint-test-")
	if err != nil {
		t.Fatal(err)
	}
	if resolved, _ := filepath.EvalSymlinks(outside); filepath.Dir(dir) != resolved {
		t.Fatalf("workspace %s is not under %s", dir, resolved)
	}
}

// setTempDir points os.TempDir() at dir on every GOOS: TMPDIR on Unix, and
// TMP and TEMP, which Windows consults first, so the refusal fixture controls
// the directory the helper reads wherever it runs (V1-1489).
func setTempDir(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, dir)
	}
	if os.TempDir() != dir {
		t.Fatalf("os.TempDir() is %q, want %q", os.TempDir(), dir)
	}
}
