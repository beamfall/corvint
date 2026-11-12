package appmap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// WorkingTree marks a map built from an uncommitted working tree (AMAP-V0-976): the HEAD it was
// based on, empty for an unborn branch, and "unpinned" evidence.
type WorkingTree struct {
	BaseRevision string `json:"base_revision"`
	Evidence     string `json:"evidence"`
}

// snapshotMessage is the fixed commit text of a working-tree snapshot, so the same tree always
// yields the same commit.
const snapshotMessage = "author corvint <> 0 +0000\ncommitter corvint <> 0 +0000\n\ncorvint working-tree snapshot\n"

// BuildWorktree compiles the map from root's working tree (AMAP-V0-976): tracked files with their
// uncommitted edits and deletions, and untracked files the repository's ignore rules keep. The
// snapshot is a commit in a temporary repository that borrows root's object store through
// alternates; nothing is written to root, and the temporary repository is removed.
func BuildWorktree(ctx context.Context, root, manifestPath string) (*Map, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, newSourceUnavailable(err.Error(), err)
	}
	work := repo{ctx: ctx, root: abs}
	base, err := work.resolve("HEAD")
	if errors.Is(err, errRevisionNotCommit) {
		base, err = "", nil // an unborn branch: the snapshot is the untracked files alone
	}
	if err != nil {
		return nil, orProcessFailure(err, &gokernel.Error{Code: "appmap-invalid-revision", Message: err.Error()})
	}
	out, err := work.git(8192, nil, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-object-format")
	if err != nil {
		return nil, newSourceUnavailable("git-common-dir", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return nil, newSourceUnavailable("git-common-dir", nil)
	}
	common, format := fields[0], fields[1]
	tmp, err := os.MkdirTemp("", "corvint-appmap-worktree-")
	if err != nil {
		return nil, newSourceUnavailable(err.Error(), err)
	}
	defer os.RemoveAll(tmp)
	snap := repo{ctx: ctx, root: tmp}
	if _, err := snap.git(8192, nil, "init", "-q", "--object-format="+format); err != nil {
		return nil, newSourceUnavailable("snapshot init", err)
	}
	gitDir := filepath.Join(tmp, ".git")
	if err := os.WriteFile(filepath.Join(gitDir, "objects", "info", "alternates"), []byte(filepath.Join(common, "objects")+"\n"), 0o600); err != nil {
		return nil, newSourceUnavailable(err.Error(), err)
	}
	if exclude, err := os.ReadFile(filepath.Join(common, "info", "exclude")); err == nil {
		if err := os.WriteFile(filepath.Join(gitDir, "info", "exclude"), exclude, 0o600); err != nil {
			return nil, newSourceUnavailable(err.Error(), err)
		}
	}
	if base != "" {
		if _, err := snap.git(4096, nil, "read-tree", base); err != nil {
			return nil, newSourceUnavailable("snapshot read-tree", err)
		}
	}
	// add runs in the working tree with the snapshot's Git directory and index.
	if _, err := work.git(4096, nil, "--git-dir="+gitDir, "--work-tree="+abs, "add", "-A"); err != nil {
		return nil, newSourceUnavailable("snapshot add", err)
	}
	tree, err := snap.git(4096, nil, "write-tree")
	if err != nil {
		return nil, newSourceUnavailable("snapshot write-tree", err)
	}
	commit, err := snap.git(4096, []byte("tree "+strings.TrimSpace(string(tree))+"\n"+snapshotMessage), "hash-object", "-t", "commit", "-w", "--stdin")
	if err != nil {
		return nil, newSourceUnavailable("snapshot commit", err)
	}
	snapshot := strings.TrimSpace(string(commit))
	// The revision index reads the repository's HEAD, so the snapshot repository's HEAD names it.
	if _, err := snap.git(4096, nil, "update-ref", "HEAD", snapshot); err != nil {
		return nil, newSourceUnavailable("snapshot update-ref", err)
	}
	m, err := BuildRoots(ctx, tmp, manifestPath, snapshot, Roots{})
	if err != nil {
		return nil, err
	}
	m.WorkingTree = &WorkingTree{BaseRevision: base, Evidence: "unpinned"}
	if m.Digest, err = mapDigest(m); err != nil {
		return nil, err
	}
	return m, nil
}

// worktreeHint names --worktree when a manifest absent at the revision is a working-tree file of
// root (AMAP-V0-977).
func worktreeHint(root, manifestPath string) string {
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(manifestPath))); err == nil && info.Mode().IsRegular() {
		return fmt.Sprintf("; %s exists in the working tree: commit it, or build it unpinned with --worktree", manifestPath)
	}
	return ""
}
