package authoritystore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func bindingGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	output, err := gitrun.Run(context.Background(), gitrun.NewBudget(1, 5*time.Second), gitrun.Options{Binary: binary, Dir: dir, Env: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid"}, StdoutLimit: 4096}, args...)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}
func bindingRepo(t *testing.T) RootDocument {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bindingGit(t, path, "init", "-q")
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	return RootDocument{RepositoryRoot: path, Git: Image{Path: binary}}
}
func TestActualCwdRepositoryBinding(t *testing.T) {
	root := bindingRepo(t)
	other := bindingRepo(t)
	nested := filepath.Join(root.RepositoryRoot, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{root.RepositoryRoot, nested, alias} {
		t.Run(path, func(t *testing.T) {
			t.Chdir(path)
			// Neither ambient shell hints nor Git repository redirection select scope.
			t.Setenv("PWD", other.RepositoryRoot)
			t.Setenv("GIT_DIR", filepath.Join(other.RepositoryRoot, ".git"))
			t.Setenv("GIT_WORK_TREE", other.RepositoryRoot)
			b, err := bindCurrentRepository(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer b.close()
			if err := b.unchanged(context.Background(), root); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("PLE-V0-009 other repository refuses global enrollment", func(t *testing.T) {
		t.Chdir(other.RepositoryRoot)
		if b, err := bindCurrentRepository(context.Background(), root); err == nil {
			b.close()
			t.Fatal("global enrollment escaped admitted repository")
		}
	})
	t.Run("admitted alias rejected", func(t *testing.T) {
		t.Chdir(nested)
		bad := root
		bad.RepositoryRoot = alias
		if b, err := bindCurrentRepository(context.Background(), bad); err == nil {
			b.close()
			t.Fatal("noncanonical admitted root")
		}
	})
	t.Run("config redirection", func(t *testing.T) {
		t.Chdir(nested)
		bindingGit(t, root.RepositoryRoot, "config", "core.worktree", other.RepositoryRoot)
		defer bindingGit(t, root.RepositoryRoot, "config", "--unset", "core.worktree")
		if b, err := bindCurrentRepository(context.Background(), root); err == nil {
			b.close()
			t.Fatal("core.worktree redirected scope")
		}
	})
}
func TestActualCwdRepositoryBindingRejectsReplacement(t *testing.T) {
	for _, mode := range []string{"root rename", "nested rename", "nested repository", "git directory replacement", "config drift", "deleted cwd"} {
		t.Run(mode, func(t *testing.T) {
			root := bindingRepo(t)
			nested := filepath.Join(root.RepositoryRoot, "nested")
			if err := os.Mkdir(nested, 0700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(nested)
			b, err := bindCurrentRepository(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			defer b.close()
			switch mode {
			case "root rename":
				moved := root.RepositoryRoot + "-moved"
				if err := os.Rename(root.RepositoryRoot, moved); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.RemoveAll(moved) })
				if err := os.Mkdir(root.RepositoryRoot, 0700); err != nil {
					t.Fatal(err)
				}
				bindingGit(t, root.RepositoryRoot, "init", "-q")
			case "nested rename":
				if err := os.Rename(nested, nested+"-moved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(nested, 0700); err != nil {
					t.Fatal(err)
				}
			case "nested repository":
				bindingGit(t, nested, "init", "-q")
			case "deleted cwd":
				if err := os.Remove(nested); err != nil {
					t.Fatal(err)
				}
			case "config drift":
				bindingGit(t, root.RepositoryRoot, "config", "core.worktree", t.TempDir())
			case "git directory replacement":
				gitdir := filepath.Join(root.RepositoryRoot, ".git")
				if err := os.Rename(gitdir, gitdir+"-old"); err != nil {
					t.Fatal(err)
				}
				bindingGit(t, root.RepositoryRoot, "init", "-q")
			}
			if err := b.unchanged(context.Background(), root); err == nil {
				t.Fatal("repository identity drift accepted")
			}
		})
	}
}
func TestActualCwdRepositoryBindingDistinguishesLinkedWorktree(t *testing.T) {
	root := bindingRepo(t)
	bindingGit(t, root.RepositoryRoot, "commit", "-q", "--allow-empty", "-m", "initial")
	linked := filepath.Join(t.TempDir(), "linked")
	bindingGit(t, root.RepositoryRoot, "worktree", "add", "-q", "--detach", linked)
	t.Chdir(linked)
	if b, err := bindCurrentRepository(context.Background(), root); err == nil {
		b.close()
		t.Fatal("shared history substituted another worktree")
	}
	linkedRoot, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	root.RepositoryRoot = linkedRoot
	b, err := bindCurrentRepository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer b.close()
}
