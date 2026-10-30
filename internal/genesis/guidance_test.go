package genesis

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryGuidanceNestedGitOutputBudget(t *testing.T) {
	t.Run("RGV-V0-011 nested Git output stays within both budgets", func(t *testing.T) {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		root := genesisRepository(t, git, "sha1")
		genesisGit(t, git, root, "update-index", "--index-version=4")
		snapshot, err := OpenGuidance(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		before := snapshot.output
		if err = snapshot.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
		if snapshot.output <= before {
			t.Fatal("nested private status output was not charged")
		}
		if _, err = snapshot.run(context.Background(), 32<<20, "rev-parse", "HEAD"); err != nil {
			t.Fatalf("request was not narrowed to parent ceiling: %v", err)
		}
		snapshot.output = (64 << 20) - 8
		if _, err = snapshot.run(context.Background(), 32<<20, "rev-parse", "HEAD"); err == nil || !strings.Contains(err.Error(), "output-budget-exceeded") {
			t.Fatalf("remaining global output cap was bypassed: %v", err)
		}
	})
}

// RGV-V0-050 (V1-2053): a dirty-worktree refusal names the dirty paths, bounded, and an
// untracked path ignored only by a global excludes file is named with the reason and the fix.
func TestGuidanceDirtyRefusalNamesPathsAndGlobalExcludes(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := genesisRepository(t, git, "sha1")
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "git", "ignore"), []byte("local.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.WriteFile(filepath.Join(root, "local.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status := exec.Command(git, "status", "--porcelain=v1")
	status.Dir = root
	if out, err := status.Output(); err != nil || len(out) != 0 {
		t.Fatalf("the global excludes file did not ignore the fixture for the user's Git: %q %v", out, err)
	}
	_, err = OpenGuidance(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), `1 path(s): ?? "local.json"`) || !strings.Contains(err.Error(), "global excludes file") || !strings.Contains(err.Error(), ".git/info/exclude") {
		t.Fatalf("globally ignored path refusal = %v", err)
	}

	if err := os.Remove(filepath.Join(root, "local.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = OpenGuidance(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), `1 path(s): M "README.md"`) || strings.Contains(err.Error(), "global excludes") {
		t.Fatalf("tracked modification refusal = %v", err)
	}

	for i := range 10 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("extra-%02d.txt", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, err = OpenGuidance(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "11 path(s)") || !strings.Contains(err.Error(), "(+3 more)") || strings.Contains(err.Error(), "extra-09") {
		t.Fatalf("bounded refusal = %v", err)
	}
}
