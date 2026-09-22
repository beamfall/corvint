package gitauth

import (
	"context"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CommitTree verifies a pinned commit's root tree through the contained Git
// boundary. Historical tree/tag OIDs still peel; it accepts no expressions.
func (r *Repository) CommitTree(ctx context.Context, commit string) (string, error) {
	if !wire.IsGitOid(commit) {
		return "", unavailable("tree requires a pinned commit")
	}
	stream, err := r.streamCommitBatch(ctx, []string{commit + "^{commit}", commit + "^{tree}"}, nil)
	if err != nil {
		return "", err
	}
	root := stream.objects[1]
	if root.missing || root.kind != "tree" || len(stream.tail) != 0 {
		return "", unavailable("invalid tree identity")
	}
	if err := requireCommitTree(stream.objects[0], root.oid); err != nil {
		return "", err
	}
	return root.oid, nil
}

// TreePaths returns bounded immutable paths below one literal directory in
// the order ls-tree -rt reports them. Every tree body it lists is read through
// the verified walk (CEM-CB-023): the directory resolves through self-hashed
// parents, each level beneath it is one cat-file batch whose records must
// self-hash and carry the OID the verified parent names, and the walk refuses
// past MaxTreeDepth levels or MaxTreeBytes of tree bodies in total.
func (r *Repository) TreePaths(ctx context.Context, commit, directory string) ([]string, error) {
	if !wire.IsGitOid(commit) {
		return nil, unavailable("paths require a pinned commit")
	}
	if err := wire.ValidatePath(directory); err != nil {
		return nil, err
	}
	root, exists, err := r.lookupTreeEntry(ctx, commit, directory)
	if err != nil {
		return nil, err
	}
	if !exists || root.Type != "tree" {
		return []string{}, nil
	}
	bodies, err := r.verifiedSubtrees(ctx, root)
	if err != nil {
		return nil, err
	}
	return listTree([]string{}, directory, root.OID, len(root.OID)/2, bodies)
}

// verifiedSubtrees reads root and every tree beneath it, one Git call per
// directory level, and returns the verified bodies by OID.
func (r *Repository) verifiedSubtrees(ctx context.Context, root TreeEntry) (map[string][]byte, error) {
	bodies := map[string][]byte{}
	total := 0
	pending := []TreeEntry{root}
	for depth := 0; len(pending) > 0; depth++ {
		if depth == MaxTreeDepth {
			return nil, unavailable("tree is nested deeper than %d directory levels", MaxTreeDepth)
		}
		requests := make([]string, len(pending))
		for i, entry := range pending {
			requests[i] = entry.OID
		}
		trees, err := r.verifiedTrees(ctx, requests, pending)
		if err != nil {
			return nil, err
		}
		pending = nil
		for _, tree := range trees {
			total += len(tree.body)
			if total > MaxTreeBytes {
				return nil, unavailable("tree listing exceeds %d bytes", MaxTreeBytes)
			}
			bodies[tree.oid] = tree.body
			entries, err := treeEntries(tree.body, len(tree.oid)/2)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if entry.Type == "tree" {
					pending = append(pending, entry)
				}
			}
		}
	}
	return bodies, nil
}

// listTree appends every entry of the tree named oid below dir in body order,
// descending into a subtree right after listing it, as ls-tree -rt does.
func listTree(paths []string, dir, oid string, width int, bodies map[string][]byte) ([]string, error) {
	body := bodies[oid]
	for len(body) > 0 {
		name, entry, rest, err := nextTreeEntry(body, width)
		if err != nil {
			return nil, err
		}
		path := dir + "/" + name
		if err := wire.ValidatePath(path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
		if entry.Type == "tree" {
			if paths, err = listTree(paths, path, entry.OID, width, bodies); err != nil {
				return nil, err
			}
		}
		body = rest
	}
	return paths, nil
}
