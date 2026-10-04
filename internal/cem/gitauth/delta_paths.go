package gitauth

import (
	"context"
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"sort"
)

// DeltaPath describes an immutable non-tree path change, including the CEM sidecar.
// A missing side has a zero TreeEntry. Renames remain a deletion and addition.
type DeltaPath struct {
	Path     string
	Old, New TreeEntry
}

// DeltaPaths verifies the complete bounded tree difference without CEM exclusions.
func (r *Repository) DeltaPaths(ctx context.Context, baseRevision, targetRevision string) ([]DeltaPath, error) {
	if !wire.IsGitOid(baseRevision) || !wire.IsGitOid(targetRevision) {
		return nil, cemcode.New(cemcode.InvalidArguments, "revision must be bounded printable text")
	}
	roots, err := r.verifiedCommitTrees(ctx, []string{baseRevision, targetRevision})
	if err != nil {
		return nil, err
	}
	width := len(roots[0].oid) / 2
	pending := []treePair{{old: roots[0].body, new: roots[1].body}}
	var changed []DeltaPath
	for depth := 0; len(pending) > 0; depth++ {
		var requests []string
		var want []TreeEntry
		var next []treePair
		for _, pair := range pending {
			oldEntries, err := treeEntries(pair.old, width)
			if err != nil {
				return nil, err
			}
			newEntries, err := treeEntries(pair.new, width)
			if err != nil {
				return nil, err
			}
			for _, name := range unionNames(oldEntries, newEntries) {
				path := name
				if pair.dir != "" {
					path = pair.dir + "/" + name
				}
				old, new := oldEntries[name], newEntries[name]
				if old == new {
					continue
				}
				child := treePair{dir: path}
				if old.Type == "tree" {
					requests = append(requests, old.OID)
					want = append(want, old)
					child.old = []byte{}
					old = TreeEntry{}
				}
				if new.Type == "tree" {
					requests = append(requests, new.OID)
					want = append(want, new)
					child.new = []byte{}
					new = TreeEntry{}
				}
				if child.old != nil || child.new != nil {
					next = append(next, child)
				}
				if old != new {
					if len(changed) >= 200_000 {
						return nil, unavailable("delta changed-path bound exceeded")
					}
					changed = append(changed, DeltaPath{Path: path, Old: old, New: new})
				}
			}
		}
		if len(requests) == 0 {
			break
		}
		if depth == MaxTreeDepth {
			return nil, unavailable("change set is nested deeper than %d directory levels", MaxTreeDepth)
		}
		trees, err := r.verifiedTrees(ctx, requests, want)
		if err != nil {
			return nil, err
		}
		pending = fillTreePairs(next, trees)
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Path < changed[j].Path })
	return changed, nil
}
