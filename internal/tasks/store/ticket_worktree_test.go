package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func worktreeGitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// worktreeRepo makes a resolved repository with one commit on main.
func worktreeRepo(t *testing.T) (*intent.Repository, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktreeGitT(t, root, "init", "-q", "-b", "main")
	worktreeGitT(t, root, "commit", "-q", "--allow-empty", "-m", "base")
	return &intent.Repository{PrimaryWorktree: root, CommonDir: filepath.Join(root, ".git")}, root
}

// TestTWTV0002_TWTV0003_TWTV0006_EnsureIsIdempotentAndNeverResets: the branch
// is the pattern with the ticket's local id; a repeated ensure returns the
// same path and branch without creating anything; a dirty worktree is
// reported with its entries left in place; the result carries the
// repository identity and the head.
func TestTWTV0002_TWTV0003_TWTV0006_EnsureIsIdempotentAndNeverResets(t *testing.T) {
	repo, root := worktreeRepo(t)
	p := &intent.WorktreePolicy{Repository: ".", Base: "main", BranchPattern: "ticket/{ticket}-work"}
	if b := store.WorktreeBranch(p, "V1-0042"); b != "ticket/V1-0042-work" {
		t.Fatalf("branch: %s", b)
	}
	first, err := store.EnsureTicketWorktree(repo, p, "V1-0042", "")
	if err != nil {
		t.Fatal(err)
	}
	base := worktreeGitT(t, root, "rev-parse", "main")
	common := worktreeGitT(t, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !first.Created || first.Branch != "ticket/V1-0042-work" || first.Path != filepath.Join(root, store.TicketWorktreeDir, "V1-0042") ||
		first.Head != base || first.BaseCommit != base || !first.BaseAncestor || first.Dirty ||
		first.RepositoryRoot != root || first.RepositorySha256 != wire.Sum([]byte(filepath.Clean(common))) {
		t.Fatalf("first: %+v", first)
	}
	if got := worktreeGitT(t, first.Path, "branch", "--show-current"); got != first.Branch {
		t.Fatalf("checked out %s", got)
	}
	// Dirty the worktree and commit on the branch; ensure reports, never resets.
	worktreeGitT(t, first.Path, "commit", "-q", "--allow-empty", "-m", "work")
	if err := os.WriteFile(filepath.Join(first.Path, "scratch.txt"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := store.EnsureTicketWorktree(repo, p, "V1-0042", "")
	if err != nil {
		t.Fatal(err)
	}
	head := worktreeGitT(t, first.Path, "rev-parse", "HEAD")
	if again.Created || again.Path != first.Path || again.Branch != first.Branch || !again.Dirty || again.DirtyEntries != 1 || again.Head != head || head == base {
		t.Fatalf("again: %+v", again)
	}
	if raw, err := os.ReadFile(filepath.Join(first.Path, "scratch.txt")); err != nil || string(raw) != "keep me\n" {
		t.Fatalf("dirty file changed: %q %v", raw, err)
	}
	// The same path is returned when the caller names it.
	if same, err := store.EnsureTicketWorktree(repo, p, "V1-0042", filepath.Join(store.TicketWorktreeDir, "V1-0042")); err != nil || same.Path != first.Path {
		t.Fatalf("named same path: %v", err)
	}
	if _, err := store.EnsureTicketWorktree(repo, p, "V1-0042", "elsewhere"); wire.CodeOf(err) != wire.CodeResourceCollision {
		t.Fatalf("different path for a checked-out branch: %v", err)
	}
}

// TestTWTV0004_TWTV0005_RefusesOutsideRootAndUnrelatedBranch: a path outside
// the declared root refuses OUT_OF_SCOPE before any Git write, and an
// existing ticket branch that does not descend from the base is not attached.
func TestTWTV0004_TWTV0005_RefusesOutsideRootAndUnrelatedBranch(t *testing.T) {
	repo, root := worktreeRepo(t)
	p := &intent.WorktreePolicy{Repository: ".", Base: "main", BranchPattern: "t/{ticket}"}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range []string{filepath.Join(outside, "wt"), "../escape", ".", root} {
		if _, err := store.EnsureTicketWorktree(repo, p, "V1-0001", req); wire.CodeOf(err) != wire.CodeOutOfScope {
			t.Fatalf("%s: want OUT_OF_SCOPE, got %v", req, err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnsureTicketWorktree(repo, p, "V1-0001", "link/wt"); wire.CodeOf(err) != wire.CodeOutOfScope {
		t.Fatalf("symlink escape: %v", err)
	}
	if out := worktreeGitT(t, root, "branch", "--list", "t/V1-0001"); out != "" {
		t.Fatalf("a refused ensure created a branch: %s", out)
	}
	worktreeGitT(t, root, "checkout", "-q", "--orphan", "t/V1-0002")
	worktreeGitT(t, root, "commit", "-q", "--allow-empty", "-m", "unrelated")
	worktreeGitT(t, root, "checkout", "-q", "main")
	if _, err := store.EnsureTicketWorktree(repo, p, "V1-0002", ""); wire.CodeOf(err) != wire.CodePrerequisiteUnsatisfied {
		t.Fatalf("unrelated branch: %v", err)
	}
	if _, err := store.EnsureTicketWorktree(repo, &intent.WorktreePolicy{Repository: ".", Base: "nope", BranchPattern: "t/{ticket}"}, "V1-0003", ""); wire.CodeOf(err) != wire.CodeMissingEvidence {
		t.Fatalf("unknown base: %v", err)
	}
}
