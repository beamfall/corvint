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
// uncommitted edits and deletions, and untracked files the repository's ignore rules keep. root
// must be the top of its worktree, as a declared root must (AFP-V0-156): a pinned build reads
// committed objects from the top whichever directory --root names, but a working-tree read stays
// inside --root, so a subdirectory is refused rather than widened. The snapshot is a commit in a temporary
// repository that borrows the repository's object store through alternates and carries the
// repository configuration that decides what `git add` stores (snapshotConfig) and its
// info/exclude and info/attributes files. No file, ref or index of the repository is written, but
// Git may refresh the modification time of an object of the shared store that the snapshot also
// needs, and an object it cannot refresh there (a read-only store) is written to the temporary
// repository instead. The temporary repository is removed when the build returns, including when
// a termination signal cancels ctx; a process killed outright leaves it in the system temporary
// directory. A sparse checkout is refused: its excluded paths would read as deleted.
func BuildWorktree(ctx context.Context, root, manifestPath string) (*Map, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, newSourceUnavailable(err.Error(), err)
	}
	work := repo{ctx: ctx, root: abs}
	if top, err := work.atWorktreeTop(); err != nil || !top {
		return nil, orProcessFailure(err, rootUnavailable("--root", "is not the top of a Git worktree: --worktree snapshots the worktree from its top, so --root must name the top"))
	}
	config, err := work.snapshotConfig()
	if err != nil {
		return nil, err
	}
	if sparse, ok := config["core.sparsecheckout"]; ok && gitTrue(sparse) {
		return nil, rootUnavailable("--root", "is a sparse checkout: --worktree would read every path outside the sparse patterns as deleted; build from a full checkout or a commit")
	}
	delete(config, "core.sparsecheckout")
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
	for _, name := range []string{"exclude", "attributes"} {
		if body, err := os.ReadFile(filepath.Join(common, "info", name)); err == nil {
			if err := os.WriteFile(filepath.Join(gitDir, "info", name), body, 0o600); err != nil {
				return nil, newSourceUnavailable(err.Error(), err)
			}
		}
	}
	// `git init` probed the temporary directory's file system; the repository's own answers apply.
	for _, key := range snapshotKeys {
		_, _ = snap.git(4096, nil, "config", "--unset-all", key) // exit 5: the key was not set
		if v, ok := config[key]; ok {
			if _, err := snap.git(4096, nil, "config", key, v); err != nil {
				return nil, newSourceUnavailable("snapshot config", err)
			}
		}
	}
	if base != "" {
		if _, err := snap.git(4096, nil, "read-tree", base); err != nil {
			return nil, newSourceUnavailable("snapshot read-tree", err)
		}
	}
	// add runs in the working tree with the snapshot's Git directory and index.
	if _, err := work.git(4096, nil, "--git-dir="+gitDir, "--work-tree="+work.root, "add", "-A"); err != nil {
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

// snapshotKeys are the repository settings that decide what `git add` stores for a working-tree
// file, or which files it skips (AMAP-V0-976). Corvint's Git environment reads no global or
// system configuration, so the repository's own (local, worktree and included) configuration is
// the whole answer, as it is for every other Corvint Git read.
var snapshotKeys = []string{"core.autocrlf", "core.eol", "core.safecrlf", "core.filemode", "core.symlinks",
	"core.ignorecase", "core.precomposeunicode", "core.excludesfile"}

// snapshotConfig reads snapshotKeys and core.sparsecheckout from the repository's configuration;
// the last value of a repeated key wins, as in Git.
func (r repo) snapshotConfig() (map[string]string, error) {
	pattern := `^core\.(` + strings.Join(append(trimCore(snapshotKeys), "sparsecheckout"), "|") + `)$`
	out, err := r.git(1<<16, nil, "config", "-z", "--get-regexp", pattern)
	var failure *gitSourceFailure
	if errors.As(err, &failure) && failure.cause == nil && failure.exitStatus == 1 {
		return map[string]string{}, nil // no key is set
	}
	if err != nil {
		return nil, orProcessFailure(err, newSourceUnavailable("repository config", err))
	}
	config := map[string]string{}
	for _, entry := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		key, value, found := strings.Cut(entry, "\n")
		if !found {
			value = "true" // a key with no value is a true boolean
		}
		config[strings.ToLower(key)] = value
	}
	return config, nil
}

func trimCore(keys []string) []string {
	out := make([]string, len(keys))
	for k, key := range keys {
		out[k] = strings.TrimPrefix(key, "core.")
	}
	return out
}

// gitTrue reads a Git boolean conservatively: only false, no, off, 0 and the empty value are
// false; every other value, a nonzero integer such as 2 included, counts as true.
func gitTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "no", "off", "0", "":
		return false
	}
	return true
}

// atWorktreeTop reports whether r's root is the top of a Git worktree: an empty prefix inside a
// worktree proves it without naming the top as a path (AFP-V0-156).
func (r repo) atWorktreeTop() (bool, error) {
	where, err := r.git(4096, nil, "rev-parse", "--is-inside-work-tree", "--show-prefix")
	if err != nil {
		return false, err
	}
	return string(where) == "true\n\n", nil
}

// worktreeHint names --worktree when a manifest absent at the revision is a working-tree file of
// r's root and that root is the top of its worktree, so --worktree can build it (AMAP-V0-977).
func worktreeHint(r repo, manifestPath string) string {
	if top, err := r.atWorktreeTop(); err != nil || !top {
		return ""
	}
	if info, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(manifestPath))); err == nil && info.Mode().IsRegular() {
		return fmt.Sprintf("; %s exists in the working tree: commit it, or build it unpinned with --worktree", manifestPath)
	}
	return ""
}
