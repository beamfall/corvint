package contextindex

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
)

// RangeBoundRefusalBeforeLoad returns the exact over-bound refusal that
// RangeImpact (or ExpandedRangeImpact when expanded) would return over the
// loaded index, without loading or decoding the index snapshot, or nil when
// that outcome is not proved; a nil answer leaves the caller's ordinary path
// unchanged (ERI-V0-261, V1-1881). It answers only when every refusal that
// precedes the range count provably passes:
//   - the limit is in range and base is a full object ID of HEAD's format;
//   - the repository is not history-cut and the worktree status is empty, so
//     the drift and worktree checks over a loaded index pass;
//   - a snapshot file for HEAD's tree and the running engine exists, so an
//     index of this tree was built; a body that later fails to load makes the
//     caller build the same clean tree again;
//   - the root go.mod is admitted as a source exactly as Build admits it and
//     declares a slash-qualified module, so the module refusal passes;
//   - base resolves to a commit that is the only merge base with HEAD;
//
// and rangeCertainRefusal then proves the path refusal (ERI-V0-262) or the
// bound refusal. Any Git failure or doubt answers nil. It writes nothing.
func RangeBoundRefusalBeforeLoad(ctx context.Context, root, base string, limit int, expanded bool) error {
	return rangeBoundRefusalBeforeLoad(ctx, root, base, limit, expanded, false)
}

// PerPathRangeBoundRefusalBeforeLoad is RangeBoundRefusalBeforeLoad for the
// per-path compiler (ERI-V0-305): a path refusal is not whole-range there, so
// only the bound refusal is proved early.
func PerPathRangeBoundRefusalBeforeLoad(ctx context.Context, root, base string, limit int, expanded bool) error {
	return rangeBoundRefusalBeforeLoad(ctx, root, base, limit, expanded, true)
}

func rangeBoundRefusalBeforeLoad(ctx context.Context, root, base string, limit int, expanded, perPath bool) error {
	if limit < 1 || limit > maxLimit {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, gitDeadline)
	defer cancel()
	identity, err := readIdentity(ctx, root)
	if err != nil || identity.historyCut || !validObjectID(base, identity.objectFormat) {
		return nil
	}
	if !snapshotFilePresent(root, identity) {
		return nil
	}
	if raw, err := git(ctx, root, maxStatusBytes, nil, "status", "--porcelain=v1", "-z", "--untracked-files=all"); err != nil || len(raw) != 0 {
		return nil
	}
	if !rootGoModuleQualified(ctx, root, identity.treeRevision) {
		return nil
	}
	resolvedRaw, err := git(ctx, root, maxIdentityBytes, nil, "rev-parse", base+"^{commit}", base+"^{tree}")
	if err != nil {
		return nil
	}
	resolved := strings.Split(strings.TrimSuffix(string(resolvedRaw), "\n"), "\n")
	if len(resolved) != 2 || !validObjectID(resolved[1], identity.objectFormat) {
		return nil
	}
	mergeBases, err := git(ctx, root, maxIdentityBytes, nil, "merge-base", "--all", base, identity.commitRevision)
	if err != nil || strings.TrimSpace(string(mergeBases)) != base {
		return nil
	}
	pathLimit := maxImpactPaths
	if expanded {
		pathLimit = maxExpandedRangePaths
	}
	probe := &Index{Root: root, Revision: identity.treeRevision, ObjectFormat: identity.objectFormat}
	if perPath {
		if raw, err := git(ctx, root, maxStatusBytes, nil, "diff-tree", "--no-commit-id", "--raw", "-r", "-z", "--no-renames", resolved[1], identity.treeRevision); err == nil &&
			rangeCertainlyExceeds(ctx, probe, resolved[1], raw, maxPerPathRangeRecords) {
			return rangePathBoundError(maxPerPathRangeRecords)
		}
		return nil
	}
	return rangeCertainRefusal(ctx, probe, resolved[1], pathLimit)
}

// snapshotFilePresent reports whether the snapshot file a loader would read for
// this tree and the running engine exists as a regular file. It reads no byte
// of it.
func snapshotFilePresent(root string, identity repositoryIdentity) bool {
	directory := SnapshotDirectory(root)
	if refuseLinkedSnapshotDirectory(root, directory) != nil {
		return false
	}
	var path string
	if packEnabled() {
		path = packPath(directory, identity.objectFormat, identity.treeRevision, analyzerEngine())
	} else {
		engineID := engine()
		if engineID == "" {
			return false
		}
		path = snapshotPath(directory, identity.objectFormat, identity.treeRevision, engineID)
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// rootGoModuleQualified reports whether Build over this clean tree would set
// a slash-qualified Index.Module: the root go.mod is a regular blob that
// admittedEntries keeps and pinnedFrom neither excludes nor finds undecodable,
// and goModulePattern finds a module path containing "/" in it.
func rootGoModuleQualified(ctx context.Context, root, tree string) bool {
	if forbiddenPath("go.mod") != "" {
		return false
	}
	listing, err := git(ctx, root, maxIdentityBytes, nil, "ls-tree", "-l", "-z", "--full-tree", tree, "--", "go.mod")
	if err != nil {
		return false
	}
	meta, name, ok := bytes.Cut(bytes.TrimSuffix(listing, []byte{0}), []byte{'\t'})
	fields := strings.Fields(string(meta))
	if !ok || string(name) != "go.mod" || len(fields) != 4 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
		return false
	}
	size, err := strconv.Atoi(fields[3])
	if err != nil || size > maxSourceBytes {
		return false
	}
	data, err := git(ctx, root, maxSourceBytes, nil, "cat-file", "blob", fields[2])
	if err != nil {
		return false
	}
	pinned := pinnedFrom(treeEntry{path: "go.mod", oid: fields[2], mode: fields[0]}, data)
	if pinned.exclusionReason() != "" || !pinned.source.Valid {
		return false
	}
	match := goModulePattern.FindStringSubmatch(string(data))
	return match != nil && strings.Contains(match[1], "/")
}
