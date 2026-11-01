package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
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

// snapshotMissNoticeAfter is the in-memory build time from which a snapshot
// reader names its miss on stderr (IDX-SNAP-V0-102, V1-1640). A fixture-sized
// build stays silent; a real checkout without `corvint index` hears what the
// miss cost and how to stop paying it.
var snapshotMissNoticeAfter = time.Second

// noteSnapshotMissBuild writes the one-line stderr notice for a snapshot miss
// whose in-memory build took at least snapshotMissNoticeAfter. It changes no
// stdout byte, exit code or repository state: the read stays read-only and the
// suggested `index --if-stale` run is the operator's. The snapshot is keyed by
// committed tree in the store every worktree shares, so the run is needed once
// per new tree, not once per checkout (IDX-SNAP-V0-115, V1-3133).
func noteSnapshotMissBuild(stderr io.Writer, verb, root string, built time.Duration) {
	if built < snapshotMissNoticeAfter {
		return
	}
	argv, _ := json.Marshal([]string{"corvint", "--root", root, "index", "--if-stale"})
	_, _ = fmt.Fprintf(stderr, "corvint %s: no index snapshot matches this tree, so this call built the full index in memory (%d ms); run %s once per committed tree, which every worktree of this repository then reuses\n", verb, built.Milliseconds(), argv)
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
				"mutates": false, "state": "fresh", "path": probe.Path, "store": filepath.Dir(probe.Path),
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
	if err := ctx.Err(); err != nil {
		emitError(stderr, err)
		return 2
	}
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
		"source_bytes": receipt.SourceBytes, "build_ms": buildCost.Milliseconds(),
		"store": receipt.Store, "store_shared": receipt.StoreShared, "live_heads": receipt.LiveHeads,
		"live_trees": receipt.LiveTrees, "evicted_snapshots": evictedSnapshotsPayload(receipt.EvictedSnapshots),
	}
	if receipt.PackPath != "" {
		payload["pack_bytes"] = receipt.PackBytes
	}
	if receipt.LegacyStore != "" {
		payload["legacy_store"] = receipt.LegacyStore
		payload["legacy_removed"] = evictedSnapshotsPayload(receipt.LegacyRemoved)
		payload["legacy_left"] = legacyLeftPayload(receipt.LegacyLeft)
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
binary, so a changed tree or a rebuilt Corvint never reads it. Eight snapshots
per worktree are kept, up to 64 and 1 GiB: the one just written, then this
binary's of a tree checked out at a live worktree HEAD, then this binary's
others, then another binary's live trees, then the rest, newest first.
The receipt names the store ("store") and every file it removed
("evicted_snapshots", each flagged "live_head" when its tree was checked out),
and reports the admitted sources' total bytes ("source_bytes") and the build's
elapsed milliseconds ("build_ms"). No aggregate size refuses a repository.
A worktree's own .corvint/index, the store before the shared one, is then
swept: its snapshots, companions, build-cost record and stale temporaries are
removed ("legacy_removed"), and anything else is left and named ("legacy_left").
This is the only verb that writes there;
"context" reads a matching snapshot in place of rebuilding the index and
applies the worktree's dirty paths from git status, and falls back to a full
build when none matches. A snapshot changes no packet byte: hit and miss
produce the same output.

With --if-stale, a matching snapshot writes nothing and reports state "fresh";
a missing or stale snapshot is rebuilt exactly as above. A linked worktree
whose tree already has a snapshot in the shared store reuses it.
`

// evictedSnapshotsPayload is the receipt's list of every file an index write
// removed, in removal order (IDX-SNAP-V0-025); empty, never null.
func evictedSnapshotsPayload(evicted []contextindex.EvictedSnapshot) []any {
	payload := make([]any, 0, len(evicted))
	for _, file := range evicted {
		payload = append(payload, map[string]any{
			"kind": file.Kind, "path": file.Path, "bytes": file.Bytes,
			"tree": file.Tree, "engine": file.Engine, "live_head": file.LiveHead,
		})
	}
	return payload
}

// legacyLeftPayload is the receipt's list of every entry of the superseded
// per-worktree store an index write left, with the reason (proposed
// IDX-SNAP-V0-027); empty, never null.
func legacyLeftPayload(left []contextindex.LegacyEntry) []any {
	payload := make([]any, 0, len(left))
	for _, entry := range left {
		payload = append(payload, map[string]any{"path": entry.Path, "reason": entry.Reason})
	}
	return payload
}

// loadSnapshotObserved is the dogfood event's zero-spawn hit against its own bracket's
// opening observation (proposed LCP-V0-016, V1-0881).
var loadSnapshotObserved = contextindex.LoadSnapshotObserved

// snapshotMissNoticeKey carries a reader's miss notice through a context
// whose build sits behind a path shared with a silent caller (the harness).
type snapshotMissNoticeKey struct{}

type snapshotMissNotice struct {
	stderr     io.Writer
	verb, root string
}

// withSnapshotMissNotice arms noteTimedMissBuild for the CLI `query` verb
// (IDX-SNAP-V0-104, V1-1640); a context without it stays silent.
func withSnapshotMissNotice(ctx context.Context, stderr io.Writer, verb, root string) context.Context {
	return context.WithValue(ctx, snapshotMissNoticeKey{}, snapshotMissNotice{stderr: stderr, verb: verb, root: root})
}

// timedMissBuild runs a snapshot miss's in-memory build and, when ctx carries
// a notice, names a successful build at least snapshotMissNoticeAfter long.
func timedMissBuild(ctx context.Context, build func() (*contextindex.Index, error)) (*contextindex.Index, error) {
	started := time.Now()
	index, err := build()
	if notice, ok := ctx.Value(snapshotMissNoticeKey{}).(snapshotMissNotice); ok && err == nil {
		noteSnapshotMissBuild(notice.stderr, notice.verb, notice.root, time.Since(started))
	}
	return index, err
}
