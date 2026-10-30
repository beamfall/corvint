package store

// Bounded Git reads for the ERG-V0-017 review packet (GitHub #1169): the
// ancestry walk that finds the delta base, the changed paths and the
// per-file diffs. Every read stops at its byte bound and reports that it
// stopped, so the packet can name what it did not read.
import (
	"bytes"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const (
	// MaxReviewPacketAncestors bounds the ancestry walk for a delta base.
	MaxReviewPacketAncestors = 4096
	// MaxReviewPacketGitBytes bounds one Git read for the packet.
	MaxReviewPacketGitBytes = 4 * wire.MiB
)

// CommitTree is one commit and its root tree.
type CommitTree struct{ Commit, Tree string }

// boundedGit runs git in root and reads at most max bytes of its output.
// over reports that the output was longer; the bytes read are kept.
func boundedGit(root string, max int64, args ...string) (out []byte, over bool, err error) {
	c := exec.Command("git", append(append([]string{}, preflightGitConfig...), args...)...)
	c.Dir, c.Env = root, gitEnvironment()
	var stderr bytes.Buffer
	c.Stderr = &stderr
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	if err = c.Start(); err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	out, readErr := io.ReadAll(io.LimitReader(pipe, max+1))
	if over = int64(len(out)) > max; over {
		out = out[:max]
		_ = c.Process.Kill()
	}
	waitErr := c.Wait()
	switch {
	case over:
		return out, true, nil
	case readErr != nil:
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", readErr)
	case waitErr != nil:
		return nil, false, waitErr
	}
	return out, false, nil
}

// AncestorTrees lists commit's ancestors newest first (commit itself
// excluded), at most MaxReviewPacketAncestors; truncated reports more.
func AncestorTrees(root, commit string) (list []CommitTree, truncated bool, err error) {
	out, over, err := boundedGit(root, MaxReviewPacketGitBytes, "log", "--no-show-signature", "--no-color", "--format=%H %T",
		"--max-count="+strconv.Itoa(MaxReviewPacketAncestors+2), "--end-of-options", commit)
	if err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git log", "ancestry of %s: %v", commit, err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if over && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		c, t, ok := strings.Cut(line, " ")
		if i == 0 || !ok {
			continue
		}
		if len(list) == MaxReviewPacketAncestors {
			return list, true, nil
		}
		list = append(list, CommitTree{Commit: c, Tree: t})
	}
	return list, over, nil
}

// IsAncestor reports whether a is an ancestor of (or equal to) b.
func IsAncestor(root, a, b string) bool {
	_, _, err := boundedGit(root, 0, "merge-base", "--is-ancestor", "--end-of-options", a, b)
	return err == nil
}

// FirstParent is commit's first parent, or "" for a root commit.
func FirstParent(root, commit string) (string, error) {
	out, _, err := boundedGit(root, 4096, "rev-list", "--parents", "--max-count=1", "--end-of-options", commit)
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "git rev-list", "parents of %s: %v", commit, err)
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return "", nil
	}
	return f[1], nil
}

// EmptyTree is the repository's empty tree object name.
func EmptyTree(root string) (string, error) {
	out, _, err := boundedGit(root, 256, "hash-object", "-t", "tree", "--stdin")
	if err != nil {
		return "", wire.Errorf(wire.CodeUnsupported, "git hash-object", "empty tree: %v", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ReviewChangedPaths is the rename-free set of paths that differ between
// base and commit, in diff order; over reports a listing past the bound.
func ReviewChangedPaths(root, base, commit string) (paths []string, over bool, err error) {
	out, over, err := boundedGit(root, MaxReviewPacketGitBytes, "diff-tree", "-r", "--name-only", "--no-renames", "-z", "--end-of-options", base, commit)
	if err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git diff-tree", "changed paths %s..%s: %v", base, commit, err)
	}
	parts := strings.Split(string(out), "\x00")
	if over && len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	for _, p := range parts {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, over, nil
}

// ReviewFileDiff is one path's unified diff between base and commit, read
// to at most max bytes; complete is false when the diff was longer.
func ReviewFileDiff(root, base, commit, path string, max int64) (diff []byte, complete bool, err error) {
	out, over, err := boundedGit(root, max, "--literal-pathspecs", "diff", "--no-color", "--no-ext-diff", "--no-renames", "--no-textconv", "-U3", "--end-of-options", base, commit, "--", path)
	if err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git diff", "diff of %s: %v", path, err)
	}
	return out, !over, nil
}
