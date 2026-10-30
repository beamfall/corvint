package testsupport

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// V1-0730: a fixture command run under a Git hook's environment (GIT_DIR and
// GIT_INDEX_FILE exported) acts on the fixture repository, not on the
// repository the hook belongs to; the old helpers inherited os.Environ and
// were redirected.
func TestRunGitIgnoresAnInheritedGitDirectory(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	outer, fixture := t.TempDir(), t.TempDir()
	RunGit(t, git, outer, "init", "-q")
	RunGit(t, git, fixture, "init", "-q")
	t.Setenv("GIT_DIR", filepath.Join(outer, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(outer, ".git", "index"))
	t.Setenv("GIT_WORK_TREE", outer)

	inherited := exec.Command(git, "-C", fixture, "rev-parse", "--absolute-git-dir")
	if output, err := inherited.Output(); err != nil || !sameDirectory(t, strings.TrimSpace(string(output)), filepath.Join(outer, ".git")) {
		t.Fatalf("control: an inherited GIT_DIR did not redirect git: %s %v", output, err)
	}
	got := strings.TrimSpace(RunGit(t, git, fixture, "rev-parse", "--absolute-git-dir"))
	if !sameDirectory(t, got, filepath.Join(fixture, ".git")) {
		t.Fatalf("RunGit followed the inherited GIT_DIR: got %s, want %s", got, filepath.Join(fixture, ".git"))
	}
	if err := os.WriteFile(filepath.Join(fixture, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	RunGit(t, git, fixture, "add", "a.txt")
	RunGit(t, git, fixture, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "commit", "-q", "-m", "fixture")
	if log := RunGit(t, git, outer, "rev-list", "--all"); strings.TrimSpace(log) != "" {
		t.Fatalf("the fixture commit landed in the outer repository: %s", log)
	}
}

func sameDirectory(t *testing.T, a, b string) bool {
	t.Helper()
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(left, right)
}
