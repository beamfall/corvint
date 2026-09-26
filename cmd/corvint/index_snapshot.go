package main

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

type indexInvocation struct {
	root    string
	ifStale bool
}

var loadSnapshot = contextindex.LoadSnapshot
var loadQuerySnapshot = contextindex.LoadQuerySnapshot
var loadSharedQuerySnapshot = contextindex.LoadSharedQuerySnapshot
var loadSnapshotDeferred = contextindex.LoadSnapshotDeferred

// parseIndexInvocation recognises `[--root PATH] index [--if-stale]`; any
// other shape is not an index invocation.
func parseIndexInvocation(arguments []string) (indexInvocation, bool, error) {
	if _, requested, _ := parseHelpInvocation(arguments); requested {
		return indexInvocation{}, false, nil
	}
	position := commandPositionAfterRoots(arguments)
	if position < 0 || arguments[position] != "index" {
		return indexInvocation{}, false, nil
	}
	root := "."
	for index := 0; index < position; index++ {
		if arguments[index] == "--root" {
			root, index = arguments[index+1], index+1
			continue
		}
		root = strings.TrimPrefix(arguments[index], "--root=")
	}
	rest := arguments[position+1:]
	if len(rest) > 1 || len(rest) == 1 && rest[0] != "--if-stale" {
		return indexInvocation{}, true, argumentError("unrecognized arguments: " + strings.Join(rest, " "))
	}
	resolved, err := resolveExplicitRoot(root)
	if err != nil {
		return indexInvocation{}, true, err
	}
	return indexInvocation{root: resolved, ifStale: len(rest) == 1}, true, nil
}

// snapshotIndex returns the committed tree's snapshot when `corvint index`
// wrote one for this tree and this binary, else nil for the caller's own build
// (IDX-SNAP-V0-008). A read that cannot observe the repository is a miss, not
// an error: the build the caller falls back to reports that failure in its own
// vocabulary, so a repository Corvint cannot read says exactly what it said
// before the snapshot existed. It writes nothing (invariant 4).
func snapshotIndex(ctx context.Context, root string) *contextindex.Index {
	return snapshotHit(loadSnapshot(ctx, root))
}

// snapshotHit is the index a hit returned, or nil for a miss or an error.
func snapshotHit(index *contextindex.Index, hit bool, err error) *contextindex.Index {
	if err != nil || !hit {
		return nil
	}
	return index
}

// deferredSnapshotIndex is snapshotIndex whose pack hit verifies each body
// when a read first touches it; its reader goes through overSnapshot.
func deferredSnapshotIndex(ctx context.Context, root string) *contextindex.Index {
	return snapshotHit(loadSnapshotDeferred(ctx, root))
}

// snapshotOrBuild is snapshotIndex's whole-file read, else contextindex.Build
// (IDX-SNAP-V0-020). It serves readers that reach Source.Data directly, which a
// deferred pack body leaves nil, so they take no deferred load; a miss builds
// exactly as they did before.
func snapshotOrBuild(ctx context.Context, root string) (*contextindex.Index, error) {
	if index := snapshotIndex(ctx, root); index != nil {
		return index, nil
	}
	return contextindex.Build(ctx, root)
}

// overSnapshot runs compute over the snapshot deferred reads, else over the
// index build returns; a nil index is a miss. A deferred pack hit verifies
// each body when compute first reads it (IDX-SNAP-V0-015). When one of those
// reads found a body failing verification, the result is discarded and
// compute reruns over eager's read, which refuses the pack whole and falls
// back to the gob snapshot, then build. A deferred miss goes straight to
// build, so a miss observes the repository once.
func overSnapshot[T any](deferred, eager func() *contextindex.Index, build func() (*contextindex.Index, error), compute func(*contextindex.Index) (T, error)) (T, error) {
	index := deferred()
	if index != nil {
		result, err := compute(index)
		if index.SnapshotRefusal() == nil {
			return result, err
		}
		index = eager()
	}
	if index == nil {
		built, err := build()
		if err != nil {
			var none T
			return none, err
		}
		index = built
	}
	return compute(index)
}

// runIndex builds the index of the committed tree and writes its snapshot
// (index-snapshot-v0). It is the one verb that writes the snapshot store;
// the packet verbs only read what it wrote.
func runIndex(ctx context.Context, invocation indexInvocation, stdout, stderr io.Writer) int {
	if invocation.ifStale {
		probe, fresh, err := contextindex.ProbeSnapshot(ctx, invocation.root)
		if err != nil {
			emitError(stderr, err)
			return 2
		}
		if fresh {
			payload := map[string]any{
				"mutates": false, "state": "fresh", "path": probe.Path,
				"tree": probe.Tree, "commit": probe.Commit, "engine": probe.Engine,
			}
			encoded, err := gokernel.CanonicalJSON(payload)
			if err != nil {
				emitError(stderr, err)
				return 2
			}
			if _, err := stdout.Write(append(encoded, '\n')); err != nil {
				emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write index snapshot probe"})
				return 2
			}
			return 0
		}
	}
	started := time.Now()
	index, err := contextindex.BuildForSnapshot(ctx, invocation.root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	buildCost := time.Since(started)
	receipt, err := contextindex.WriteSnapshot(index)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// The record only lets a hook skip a build that cannot fit its deadline;
	// without it the hook builds as before, so a failed write is not a failed
	// index (IDX-SNAP-V0-012).
	_ = contextindex.RecordBuildCost(invocation.root, buildCost)
	payload := map[string]any{
		"ok": true, "mutates": true, "command": "index", "profile": "corvint-index-snapshot/1",
		"path": receipt.Path, "bytes": receipt.Bytes, "tree": receipt.Tree, "commit": receipt.Commit,
		"engine": receipt.Engine, "sources": receipt.Sources, "symbols": receipt.Symbols, "evicted": receipt.Evicted,
	}
	if receipt.PackPath != "" {
		payload["pack_bytes"] = receipt.PackBytes
	}
	encoded, err := gokernel.CanonicalJSON(payload)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(encoded, '\n')); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write index snapshot receipt"})
		return 2
	}
	return 0
}

const indexHelp = `Write the index snapshot of the committed tree for the packet verbs to read.

Usage:
  corvint [--root PATH] index [--if-stale]

Writes corvint/index/<object-format>-<tree>-<engine>.gob under the Git common
directory, shared by every linked worktree (index-snapshot-v0, experimental):
the compiled index of HEAD's tree, keyed by the tree id and by a digest of this
binary, so a changed tree or a rebuilt Corvint never reads it. The newest eight
snapshots are kept. This is the only verb that writes there;
"context" reads a matching snapshot in place of rebuilding the index and
applies the worktree's dirty paths from git status, and falls back to a full
build when none matches. A snapshot changes no packet byte: hit and miss
produce the same output.

With --if-stale, a matching snapshot writes nothing and reports state "fresh";
a missing or stale snapshot is rebuilt exactly as above.
`
