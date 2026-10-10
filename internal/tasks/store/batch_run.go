package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// BatchCommandRun is what one configured batch or qualification command
// observed (TOL-V0-028..033, GitHub #714): the lease runner's process
// class, its exit code when it exited, and its capped combined output.
type BatchCommandRun struct {
	Class    string
	ExitCode string
	Output   []byte
}

// Passed reports an exit with status 0.
func (r BatchCommandRun) Passed() bool { return r.Class == "EXIT" && r.ExitCode == "0" }

// RunBatchCommand runs argv in dir with exactly env, under the lease
// runner's process containment, timeout and output cap. Its process group
// is killed after it exits, so a descendant cannot outlive it, and when ctx
// ends (the caller's interrupt).
func RunBatchCommand(ctx context.Context, argv []string, dir string, env []string, timeoutSeconds int) BatchCommandRun {
	def := &intent.GateDefinition{GateID: "batch", Kind: "COMMAND", Argv: argv, Cwd: "WORKTREE",
		TimeoutSeconds: wire.CountOf(int64(timeoutSeconds))}
	out := &cappedOutput{}
	run := execute(ctx, def, dir, env, out)
	killGateGroup(run.group)
	r := BatchCommandRun{Class: run.class, Output: bytes.Clone(out.buf.Bytes())}
	if run.exitCode != nil {
		r.ExitCode = string(*run.exitCode)
	}
	return r
}

// BatchEnvironment passes exactly the declared names that are set, then the
// runner's own variables, as the lease runner passes a gate's environment.
func BatchEnvironment(names []string, extra ...string) []string {
	env, _ := gateEnvironment(names)
	return append(env, extra...)
}

// WorktreeState returns the commit checked out at root and, when the
// worktree's tracked files differ from it, how ("" when they do not).
// Untracked files, such as a test runner's output directory, are not read.
func WorktreeState(root string) (head, changed string, err error) {
	out, err := preflightGit(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", "", err
	}
	head = strings.TrimSpace(string(out))
	status, err := preflightGit(root, "status", "--porcelain=v1", "-z", "--untracked-files=no", "--ignore-submodules=none")
	if err != nil {
		return "", "", err
	}
	if len(status) > 0 {
		first, _, _ := strings.Cut(string(status), "\x00")
		changed = "tracked changes, first " + strings.TrimSpace(first)
	}
	return head, changed, nil
}

// ResolveCommit resolves rev to a commit id in root.
func ResolveCommit(root, rev string) (string, error) {
	return resolvePreflightObject(root, rev+"^{commit}")
}

// ChangedPaths is the rename-free set of paths that differ between two
// commits, in diff order.
func ChangedPaths(root, from, to string) ([]string, error) { return changedPaths(root, from, to) }

// FixtureDigest digests the Git object id of each path at commit (a blob or
// a whole tree; the root tree for "."), "ABSENT" for a path the commit does
// not hold, so two runs at one digest saw the same fixture evidence
// (TOL-V0-031).
func FixtureDigest(root, commit string, paths []string) (wire.Digest, error) {
	clean := make([]string, 0, len(paths))
	for _, p := range paths {
		clean = append(clean, path.Clean(p))
	}
	sort.Strings(clean)
	var b strings.Builder
	for i, p := range clean {
		if i > 0 && p == clean[i-1] {
			continue
		}
		oid := "ABSENT"
		if p == "." {
			// ls-tree lists no entry named ".": the repository root is the
			// commit's root tree.
			tree, err := gitOutput(root, "rev-parse", "--verify", commit+"^{tree}")
			if err != nil {
				return "", err
			}
			b.WriteString(p + "\x00" + strings.TrimSpace(string(tree)) + "\n")
			continue
		}
		out, err := gitOutput(root, "ls-tree", "-z", "--full-tree", commit, "--", p)
		if err != nil {
			return "", err
		}
		for _, entry := range strings.Split(string(out), "\x00") {
			meta, name, ok := strings.Cut(entry, "\t")
			if f := strings.Fields(meta); ok && name == p && len(f) == 3 {
				oid = f[2]
			}
		}
		b.WriteString(p + "\x00" + oid + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return wire.Digest(hex.EncodeToString(sum[:])), nil
}

// WithDetachedWorktree runs fn in one temporary detached worktree of commit
// created under the system temporary directory, with repository hooks and
// the file-system monitor disabled, and removes it on every path; a failed
// removal is an error naming the repair (TOL-V0-032).
func WithDetachedWorktree(root, commit string, fn func(wt string) error) (err error) {
	dir, err := os.MkdirTemp("", "corvint-tasks-qualify-")
	if err != nil {
		return wire.Errorf(wire.CodeUnsupported, "--base", "cannot create a temporary directory: %v", err)
	}
	wt := filepath.Join(dir, "worktree")
	defer func() {
		_, rerr := preflightGit(root, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(dir)
		if rerr != nil && err == nil {
			err = wire.Errorf(wire.CodeUnsupported, "--base", "the temporary worktree %s could not be removed; run `git worktree prune`", wt)
		}
	}()
	if _, err := preflightGit(root, "worktree", "add", "--detach", "--quiet", wt, commit); err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(wt); err == nil {
		wt = resolved
	}
	return fn(wt)
}
