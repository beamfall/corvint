package gitauth

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RevisionPaths lists the path of every non-tree entry at one pinned commit
// through the verified walk, for a cost independent of how many directories
// the tree holds (DCX-V1-520): one Git operation pins the commit and its
// self-hashed root tree, then each directory level is one cat-file batch whose
// records must self-hash and carry the OID their verified parent names. A
// subtree whose path descend rejects is pruned unread (DCX-V1-521). The walk
// refuses past MaxTreeDepth levels, past MaxPathSetTreeBytes of tree output
// (git-output-exceeded) and past maxEntries listed entries, each before the
// next level is read (DCX-V1-522). Names follow Git's tree grammar (no empty,
// ".", ".." or slash-bearing segment); wire-path policy is left to the caller's
// selected paths, as with the per-tree walk this replaces. Paths are returned
// sorted.
func (r *Repository) RevisionPaths(ctx context.Context, commit string, maxEntries int, descend func(dir string) bool) ([]string, error) {
	if !wire.IsGitOid(commit) {
		return nil, unavailable("revision paths require a pinned commit")
	}
	stream, err := r.streamCommitBatch(ctx, []string{commit + "^{commit}"}, []string{commit + "^{tree}"})
	if err != nil {
		return nil, err
	}
	records, err := exactBatchRecords(stream.tail, 1)
	if err != nil {
		return nil, err
	}
	root := records[0]
	if root.missing || root.kind != "tree" || (stream.width != 0 && len(root.oid) != stream.width) {
		return nil, unavailable("revision does not resolve to a tree")
	}
	if err := requireCommitTree(stream.objects[0], root.oid); err != nil {
		return nil, err
	}
	if err := requireObjectIdentity(ctx, "tree", root.oid, root.body); err != nil {
		return nil, err
	}
	width := len(root.oid) / 2
	paths, entries, used := []string{}, 0, len(root.body)
	dirs, bodies := []string{""}, [][]byte{root.body}
	for depth := 0; len(dirs) > 0; depth++ {
		if depth == MaxTreeDepth {
			return nil, unavailable("tree is nested deeper than %d directory levels", MaxTreeDepth)
		}
		var next []string
		var want []TreeEntry
		for i, dir := range dirs {
			listed, err := treeEntries(bodies[i], width)
			if err != nil {
				return nil, err
			}
			if entries += len(listed); entries > maxEntries {
				return nil, unavailable("revision lists more than %d tree entries", maxEntries)
			}
			for name, entry := range listed {
				current := name
				if dir != "" {
					current = dir + "/" + name
				}
				if entry.Type != "tree" {
					paths = append(paths, current)
				} else if descend == nil || descend(current) {
					next, want = append(next, current), append(want, entry)
				}
			}
		}
		if len(next) == 0 {
			break
		}
		limit := MaxPathSetTreeBytes - used
		if limit <= 0 {
			return nil, cemcode.New(cemcode.GitOutputExceeded, "revision listing exceeds its %d-byte tree bound", MaxPathSetTreeBytes)
		}
		requests := make([]string, len(want))
		for i, entry := range want {
			requests[i] = entry.OID
		}
		out, err := r.gitStdin(ctx, limit, []byte(strings.Join(requests, "\x00")+"\x00"), "cat-file", "--batch", "-z")
		if err != nil {
			return nil, err
		}
		used += len(out)
		level, err := verifyTreeBatch(ctx, out, requests, want)
		if err != nil {
			return nil, err
		}
		dirs, bodies = next, make([][]byte, len(level))
		for i, record := range level {
			bodies[i] = record.body
		}
	}
	sort.Strings(paths)
	return paths, nil
}
