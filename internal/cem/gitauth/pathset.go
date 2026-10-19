package gitauth

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// MaxPathSetTreeBytes bounds the tree bodies one ResolvePaths walk reads in
// total. It is sized for the largest corpus input inventory (100,000 paths),
// not for one path lookup (MaxTreeBytes).
const MaxPathSetTreeBytes = 32 << 20

// batchHeaderBytes over-approximates one cat-file batch header and its
// trailing LF ("<64-hex oid> blob <size>\n" plus the record LF).
const batchHeaderBytes = 96

// ResolvePaths resolves every path at one pinned commit through the verified
// walk, for a cost independent of len(paths) (DCP-V1-155): one Git operation
// pins the commit and its self-hashed root tree, then each directory level the
// paths pass through is one cat-file batch whose records must self-hash and
// carry the OID their verified parent names. Only directories on a requested
// path are read. The walk refuses past MaxTreeDepth levels and, with
// git-output-exceeded, past MaxPathSetTreeBytes of tree output in total. A
// path that is absent, or lies below a non-tree, is absent from the result.
func (r *Repository) ResolvePaths(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error) {
	if !wire.IsGitOid(commit) {
		return nil, unavailable("path resolution requires a pinned commit")
	}
	result := map[string]TreeEntry{}
	if len(paths) == 0 {
		return result, nil
	}
	// needed holds every directory a requested path passes through, by depth.
	byDepth := [][]string{}
	needed := map[string]bool{}
	for _, path := range paths {
		if err := wire.ValidatePath(path); err != nil {
			return nil, err
		}
		components := strings.Split(path, "/")
		for depth := 1; depth < len(components); depth++ {
			dir := strings.Join(components[:depth], "/")
			if needed[dir] {
				continue
			}
			needed[dir] = true
			for len(byDepth) < depth {
				byDepth = append(byDepth, nil)
			}
			byDepth[depth-1] = append(byDepth[depth-1], dir)
		}
	}
	if len(byDepth) >= MaxTreeDepth {
		return nil, unavailable("tree is nested deeper than %d directory levels", MaxTreeDepth)
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
	rootEntries, err := treeEntries(root.body, width)
	if err != nil {
		return nil, err
	}
	trees := map[string]map[string]TreeEntry{"": rootEntries}
	used := len(root.body)
	for _, dirs := range byDepth {
		sort.Strings(dirs)
		var requests, names []string
		var want []TreeEntry
		for _, dir := range dirs {
			entry, ok := lookupChild(trees, dir)
			if !ok || entry.Type != "tree" {
				continue
			}
			requests = append(requests, entry.OID)
			want = append(want, entry)
			names = append(names, dir)
		}
		if len(requests) == 0 {
			break
		}
		limit := MaxPathSetTreeBytes - used
		if limit <= 0 {
			return nil, cemcode.New(cemcode.GitOutputExceeded, "path resolution exceeds its %d-byte tree bound", MaxPathSetTreeBytes)
		}
		stdin := []byte(strings.Join(requests, "\x00") + "\x00")
		out, err := r.gitStdin(ctx, limit, stdin, "cat-file", "--batch", "-z")
		if err != nil {
			return nil, err
		}
		used += len(out)
		level, err := verifyTreeBatch(ctx, out, requests, want)
		if err != nil {
			return nil, err
		}
		for i, record := range level {
			entries, err := treeEntries(record.body, width)
			if err != nil {
				return nil, err
			}
			trees[names[i]] = entries
		}
	}
	for _, path := range paths {
		if entry, ok := lookupChild(trees, path); ok {
			result[path] = entry
		}
	}
	return result, nil
}

// lookupChild finds path's entry in its verified parent tree, if that parent
// was read.
func lookupChild(trees map[string]map[string]TreeEntry, path string) (TreeEntry, bool) {
	parent, name := "", path
	if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
		parent, name = path[:slash], path[slash+1:]
	}
	entries, ok := trees[parent]
	if !ok {
		return TreeEntry{}, false
	}
	entry, ok := entries[name]
	return entry, ok
}

// BlobHeaders reads the typed headers of distinct blob OIDs in one Git
// operation (`cat-file --batch-check`), so a caller can refuse an over-bound
// blob before any body is allocated (DCP-V1-156). The sizes are Git's claim;
// BlobBatch re-verifies every body against them and its object identity.
func (r *Repository) BlobHeaders(ctx context.Context, oids []string) (map[string]int, error) {
	sizes := map[string]int{}
	if len(oids) == 0 {
		return sizes, nil
	}
	for _, oid := range oids {
		if !wire.IsGitOid(oid) {
			return nil, cemcode.New(cemcode.InvalidArguments, "blob OID must be a full lowercase Git OID")
		}
	}
	stdin := []byte(strings.Join(oids, "\x00") + "\x00")
	out, err := r.gitStdin(ctx, len(oids)*batchHeaderBytes, stdin, "cat-file", "--batch-check", "-z")
	if err != nil {
		return nil, err
	}
	for _, oid := range oids {
		line, rest, found := bytes.Cut(out, []byte{'\n'})
		if !found {
			return nil, unavailable("cat-file batch-check output is truncated")
		}
		out = rest
		fields := strings.Fields(string(line))
		if len(fields) != 3 || fields[0] != oid || fields[1] != "blob" {
			return nil, unavailable("blob %s does not resolve locally", oid)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size > MaxBlobBytes {
			return nil, unavailable("blob %s header is malformed or exceeds %d bytes", oid, MaxBlobBytes)
		}
		sizes[oid] = size
	}
	if len(out) != 0 {
		return nil, unavailable("cat-file batch-check has extra output")
	}
	return sizes, nil
}

// BlobBatch reads distinct blobs whose admitted header sizes the caller holds
// in one Git operation (DCP-V1-156). Stdout is bounded by exactly those sizes;
// each body must have its admitted size and self-hash to its OID. Distinct
// bytes are charged once per OID against MaxTotalBlobBytes before Git runs.
func (r *Repository) BlobBatch(ctx context.Context, oids []string, sizes map[string]int) (map[string][]byte, error) {
	blobs := map[string][]byte{}
	if len(oids) == 0 {
		return blobs, nil
	}
	expected, charge := 0, int64(0)
	seen := map[string]bool{}
	for _, oid := range oids {
		size, ok := sizes[oid]
		if !ok || seen[oid] || !wire.IsGitOid(oid) {
			return nil, cemcode.New(cemcode.InvalidArguments, "blob batch needs distinct full OIDs with admitted sizes")
		}
		seen[oid] = true
		expected += size + batchHeaderBytes
		if !r.chargedOids[oid] {
			charge += int64(size)
		}
	}
	if charge > MaxTotalBlobBytes-r.blobBytes {
		return nil, unavailable("verification exceeded its %d-byte blob budget", MaxTotalBlobBytes)
	}
	stdin := []byte(strings.Join(oids, "\x00") + "\x00")
	options := r.gitOptions(expected, stdin)
	options.StdoutSizeHint = expected
	out, err := gitrun.Run(ctx, r.budget, options, append(r.pinnedArgs(), "cat-file", "--batch", "-z")...)
	if err != nil {
		return nil, err
	}
	records := batchRecords{rest: out}
	for _, oid := range oids {
		record, err := records.next()
		if err != nil {
			return nil, err
		}
		if record.missing || record.oid != oid || record.kind != "blob" || len(record.body) != sizes[oid] {
			return nil, unavailable("blob %s does not resolve locally", oid)
		}
		if err := requireObjectIdentity(ctx, "blob", oid, record.body); err != nil {
			return nil, err
		}
		blobs[oid] = record.body
	}
	if len(records.rest) != 0 {
		return nil, unavailable("cat-file batch has extra output")
	}
	if r.chargedOids == nil {
		r.chargedOids = map[string]bool{}
	}
	for _, oid := range oids {
		if !r.chargedOids[oid] {
			r.chargedOids[oid] = true
			r.blobBytes += int64(sizes[oid])
		}
	}
	return blobs, nil
}
