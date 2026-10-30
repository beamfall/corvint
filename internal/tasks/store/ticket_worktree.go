package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TicketWorktreeDir is the directory under the declared repository root
// that holds default per-ticket worktrees (TWT-V0-003).
const TicketWorktreeDir = ".worktrees"

// TicketWorktree is the TWT-V0-006 result of one ensure: the declared
// repository's identity, the ticket branch and worktree, and the observed
// head. Dirty reports uncommitted or untracked changes; ensure never resets
// them.
type TicketWorktree struct {
	RepositoryRoot   string
	RepositorySha256 wire.Digest
	Branch           string
	Path             string
	Base             string
	BaseCommit       string
	BaseAncestor     bool
	Head             string
	Created          bool
	Dirty            bool
	DirtyEntries     int
}

// WorktreeBranch derives the ticket branch from the declared pattern and the
// ticket's local id only (TWT-V0-002).
func WorktreeBranch(p *intent.WorktreePolicy, local string) string {
	return strings.Replace(p.BranchPattern, intent.WorktreeTicketToken, local, 1)
}

// EnsureTicketWorktree creates or reuses the worktree of ticket local on its
// pattern branch in the declared repository (TWT-V0-002..006). requested is
// an optional path, absolute or relative to the repository root; it and any
// reused worktree must lie inside the root (TWT-V0-005). It writes no Tasks
// state; its only effect is `git worktree add` when no worktree exists.
func EnsureTicketWorktree(repo *intent.Repository, p *intent.WorktreePolicy, local, requested string) (*TicketWorktree, error) {
	root, err := worktreeRepositoryRoot(repo, p)
	if err != nil {
		return nil, err
	}
	branch := WorktreeBranch(p, local)
	if _, err := worktreeGit(root, "check-ref-format", "--branch", branch); err != nil {
		return nil, wire.Errorf(wire.CodeMalformed, "/worktrees/branchPattern", "the branch %q derived for %s is not a valid Git branch name", branch, local)
	}
	common, err := worktreeGit(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	out := &TicketWorktree{RepositoryRoot: root, RepositorySha256: wire.Sum([]byte(filepath.Clean(common))), Branch: branch, Base: p.Base}
	if out.BaseCommit, err = worktreeGit(root, "rev-parse", "--verify", "--quiet", "--end-of-options", p.Base+"^{commit}"); err != nil {
		return nil, wire.Errorf(wire.CodeMissingEvidence, "/worktrees/base", "the declared base %s does not name a commit in %s", p.Base, root)
	}
	want := filepath.Join(root, TicketWorktreeDir, filepath.FromSlash(local))
	if requested != "" {
		if !filepath.IsAbs(requested) {
			requested = filepath.Join(root, requested)
		}
		want = filepath.Clean(requested)
	}
	if want, err = realPathWithin(root, want); err != nil {
		return nil, err
	}
	// A concurrent ensure of the same ticket can register the worktree, or
	// hold its path half-made, between this ensure's lookup and its add; the
	// loser re-observes for a bounded settle and returns the winner
	// (TWT-V0-003).
	deadline := time.Now().Add(ensureSettle)
	for try := 0; ; try++ {
		if try > 0 {
			if time.Now().After(deadline) {
				break
			}
			time.Sleep(ensurePoll)
		}
		existing, err := branchWorktree(root, branch)
		if err != nil {
			return nil, err
		}
		if existing != "" {
			return reuseTicketWorktree(out, root, existing, want, requested != "")
		}
		afterWorktreeLookup()
		if _, err := os.Lstat(want); !os.IsNotExist(err) {
			if worktreePending(want) {
				continue
			}
			return nil, wire.Errorf(wire.CodeResourceCollision, "--path", "%s already exists and is not the worktree of branch %s", want, branch)
		}
		args := []string{"worktree", "add", "--quiet", "-b", branch, want, out.BaseCommit}
		if tip, err := worktreeGit(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			// An existing branch is attached only when the declared base
			// is an ancestor of its tip (TWT-V0-004).
			if _, err := worktreeGit(root, "merge-base", "--is-ancestor", "--end-of-options", out.BaseCommit, tip); err != nil {
				return nil, wire.Errorf(wire.CodePrerequisiteUnsatisfied, "/worktrees/base", "the declared base %s is not an ancestor of branch %s; merge the base into the branch first", p.Base, branch)
			}
			args = []string{"worktree", "add", "--quiet", want, branch}
		}
		if _, err := worktreeGit(root, args...); err == nil {
			out.Created = true
			return observeTicketWorktree(out, root, want)
		}
		// A concurrent ensure may have added it: look again.
	}
	if _, err := os.Lstat(want); !os.IsNotExist(err) {
		return nil, wire.Errorf(wire.CodeResourceCollision, "--path", "%s did not become the worktree of branch %s within %s", want, branch, ensureSettle)
	}
	return nil, wire.Errorf(wire.CodeUnsupported, "git worktree add", "the worktree for %s could not be created at %s", branch, want)
}

// LookupTicketWorktree returns the real path of the existing worktree that
// has ticket local's pattern branch checked out inside the declared
// repository root, or "" when there is none (ATR-V0-341). Unlike
// EnsureTicketWorktree it never creates a worktree and has no effect.
func LookupTicketWorktree(repo *intent.Repository, p *intent.WorktreePolicy, local string) (string, error) {
	root, err := worktreeRepositoryRoot(repo, p)
	if err != nil {
		return "", err
	}
	existing, err := branchWorktree(root, WorktreeBranch(p, local))
	if err != nil || existing == "" {
		return "", err
	}
	return realPathWithin(root, existing)
}

// Concurrent-ensure settle bounds; afterWorktreeLookup is replaced only by
// deterministic concurrent-ensure tests.
var (
	ensureSettle        = 30 * time.Second
	ensurePoll          = 20 * time.Millisecond
	afterWorktreeLookup = func() {}
)

// worktreePending reports whether path looks like a worktree another ensure
// is still adding: an empty directory, or one already holding its .git entry.
// Anything else at path is a collision.
func worktreePending(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	if len(entries) == 0 {
		return true
	}
	_, err = os.Lstat(filepath.Join(path, ".git"))
	return err == nil
}

// reuseTicketWorktree returns the branch's existing worktree unchanged
// (TWT-V0-003). A worktree outside the root, or a requested path the branch
// is not checked out at, refuses.
func reuseTicketWorktree(out *TicketWorktree, root, existing, want string, requested bool) (*TicketWorktree, error) {
	real, err := realPathWithin(root, existing)
	if err != nil {
		return nil, wire.Errorf(wire.CodeOutOfScope, "/worktrees/repository", "branch %s is checked out at %s, which is not inside the declared repository root %s", out.Branch, existing, root)
	}
	if requested && real != want {
		return nil, wire.Errorf(wire.CodeResourceCollision, "--path", "branch %s is already checked out at %s", out.Branch, real)
	}
	return observeTicketWorktree(out, root, real)
}

// observeTicketWorktree reads the head and status of path; it never resets,
// cleans or checks anything out (TWT-V0-006).
func observeTicketWorktree(out *TicketWorktree, root, path string) (*TicketWorktree, error) {
	out.Path = path
	head, err := worktreeGit(path, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return nil, err
	}
	out.Head = head
	_, err = worktreeGit(root, "merge-base", "--is-ancestor", "--end-of-options", out.BaseCommit, head)
	out.BaseAncestor = err == nil
	status, err := worktreeGit(path, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return nil, err
	}
	if status != "" {
		out.Dirty, out.DirtyEntries = true, len(strings.Split(status, "\n"))
	}
	return out, nil
}

// worktreeRepositoryRoot resolves the declared repository to the real path
// of its worktree top level.
func worktreeRepositoryRoot(repo *intent.Repository, p *intent.WorktreePolicy) (string, error) {
	root := p.Repository
	switch {
	case root == ".":
		root = repo.PrimaryWorktree
	case !filepath.IsAbs(root):
		root = filepath.Join(repo.PrimaryWorktree, filepath.FromSlash(root))
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", wire.Errorf(wire.CodeMissingEvidence, "/worktrees/repository", "the declared repository %s cannot be resolved", root)
	}
	top, err := worktreeGit(real, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(top) != real {
		return "", wire.Errorf(wire.CodeMissingEvidence, "/worktrees/repository", "the declared repository %s is not the top level of a Git worktree", root)
	}
	return real, nil
}

// realPathWithin resolves p through its longest existing ancestor and
// refuses OUT_OF_SCOPE unless the result lies strictly inside root
// (TWT-V0-005).
func realPathWithin(root, p string) (string, error) {
	p = filepath.Clean(p)
	rest := []string{}
	cur := p
	for {
		if real, err := filepath.EvalSymlinks(cur); err == nil {
			cur = real
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		rest = append([]string{filepath.Base(cur)}, rest...)
		cur = parent
	}
	real := filepath.Join(append([]string{cur}, rest...)...)
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", wire.Errorf(wire.CodeOutOfScope, "--path", "%s is not inside the declared repository root %s", p, root)
	}
	return real, nil
}

// branchWorktree returns the path of the worktree that has branch checked
// out, or "".
func branchWorktree(root, branch string) (string, error) {
	list, err := worktreeGit(root, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	path := ""
	for _, line := range strings.Split(list, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			path = strings.TrimPrefix(line, "worktree ")
		case line == "branch refs/heads/"+branch:
			return path, nil
		}
	}
	return "", nil
}

// worktreeGit runs one Git command in dir under the sanitized environment
// and returns its trimmed stdout.
func worktreeGit(dir string, args ...string) (string, error) {
	c := exec.Command("git", append([]string{"-c", "credential.helper="}, args...)...)
	c.Dir = dir
	c.Env = gitEnvironment()
	out, err := c.Output()
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "git", "git %s failed: %v", args[0], err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
