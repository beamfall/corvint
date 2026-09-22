package gitauth

import (
	"context"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// changedEntry is one path whose entry differs between two verified trees.
// An absent side is the zero TreeEntry.
type changedEntry struct {
	path     string
	old, new TreeEntry
}

// treePair is one directory whose bodies are compared at the next level; a
// nil body is an absent side.
type treePair struct {
	dir      string
	old, new []byte
}

// verifiedChangeSet lists every non-tree entry that differs between the base
// and target trees under the canonical exclusion, in bytewise full-path order,
// which is the order Git's tree comparison emits. Every tree body it reads is
// self-hashed against its name, and each child tree is requested by the OID
// its verified parent names, so no Git-read tree body can drop, add, or
// relabel an entry. One cat-file batch is spawned per differing directory
// level, each bounded by MaxTreeBytes, and the walk refuses to descend past
// MaxTreeDepth levels.
func (r *Repository) verifiedChangeSet(ctx context.Context, baseRevision, targetRevision string) ([]changedEntry, error) {
	if !validRevisionText(baseRevision) || !validRevisionText(targetRevision) {
		return nil, cemcode.New(cemcode.InvalidArguments, "revision must be bounded printable text")
	}
	roots, err := r.verifiedCommitTrees(ctx, []string{baseRevision, targetRevision})
	if err != nil {
		return nil, err
	}
	width := len(roots[0].oid) / 2
	pending := []treePair{{old: roots[0].body, new: roots[1].body}}
	var changed []changedEntry
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
				if path == wire.ExcludedCEMPath {
					continue
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
					changed = append(changed, changedEntry{path: path, old: old, new: new})
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
	sort.Slice(changed, func(i, j int) bool { return changed[i].path < changed[j].path })
	return changed, nil
}

// verifiedTrees reads the requested trees in one batch and self-hashes each
// body. When want is given, each record must carry the OID its verified
// parent names.
func (r *Repository) verifiedTrees(ctx context.Context, requests []string, want []TreeEntry) ([]batchRecord, error) {
	stdin := []byte(strings.Join(requests, "\x00") + "\x00")
	out, err := r.gitStdin(ctx, MaxTreeBytes, stdin, "cat-file", "--batch", "-z")
	if err != nil {
		return nil, err
	}
	return verifyTreeBatch(ctx, out, requests, want)
}

func verifyTreeBatch(ctx context.Context, out []byte, requests []string, want []TreeEntry) ([]batchRecord, error) {
	result, err := exactBatchRecords(out, len(requests))
	if err != nil {
		return nil, err
	}
	for i := range requests {
		record := result[i]
		if record.missing || record.kind != "tree" {
			return nil, unavailable("%s does not resolve to a local tree", requests[i])
		}
		if want != nil && record.oid != want[i].OID {
			return nil, unavailable("tree %s does not resolve locally", want[i].OID)
		}
		if err := requireObjectIdentity(ctx, "tree", record.oid, record.body); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *Repository) verifiedCommitTrees(ctx context.Context, revisions []string) ([]batchRecord, error) {
	prefixes := make([]string, len(revisions))
	requests := make([]string, len(revisions))
	for i, revision := range revisions {
		prefixes[i] = revision + "^{commit}"
		requests[i] = revision + "^{tree}"
	}
	stream, err := r.streamCommitBatch(ctx, prefixes, requests)
	if err != nil {
		return nil, err
	}
	roots, err := verifyTreeBatch(ctx, stream.tail, requests, nil)
	if err != nil {
		return nil, err
	}
	for i, commit := range stream.objects {
		pinned := len(revisions[i]) == stream.width && wire.IsGitOid(revisions[i])
		if commit.missing || (pinned && commit.oid != revisions[i]) {
			return nil, unavailable("commit does not resolve to its pinned identity")
		}
		if err := requireCommitTree(commit, roots[i].oid); err != nil {
			return nil, err
		}
	}
	return roots, nil
}

// fillTreePairs hands each verified child body, in request order, to the
// pair side that asked for it.
func fillTreePairs(pairs []treePair, trees []batchRecord) []treePair {
	i := 0
	for p := range pairs {
		if pairs[p].old != nil {
			pairs[p].old = trees[i].body
			i++
		}
		if pairs[p].new != nil {
			pairs[p].new = trees[i].body
			i++
		}
	}
	return pairs
}

// treeEntries decodes a verified tree body by name; a nil body is empty and
// a duplicated name is refused.
func treeEntries(raw []byte, width int) (map[string]TreeEntry, error) {
	entries := map[string]TreeEntry{}
	for len(raw) > 0 {
		name, entry, rest, err := nextTreeEntry(raw, width)
		if err != nil {
			return nil, err
		}
		if _, dup := entries[name]; dup {
			return nil, unavailable("tree body is malformed")
		}
		entries[name] = entry
		raw = rest
	}
	return entries, nil
}

func unionNames(a, b map[string]TreeEntry) []string {
	names := make([]string, 0, len(a)+len(b))
	for name := range a {
		names = append(names, name)
	}
	for name := range b {
		if _, both := a[name]; !both {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}
