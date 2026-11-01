package main

import (
	"context"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// worktreeOverlayFlag is the explicit opt-in on `corvint query` and
// `corvint context` that reads tracked dirty paths from the worktree
// (DIRTY-CACHE-940, V1-0418). Absent, both verbs are byte-for-byte unchanged.
const worktreeOverlayFlag = "--worktree-overlay"

// overlayQueryContext answers a query over the HEAD snapshot (or, on a miss, a
// full in-memory HEAD build) with its tracked dirty paths re-derived from the
// worktree. The view reaches the profiles through the batch seam, so the
// receipt is the plain query receipt plus the DIRTY-CACHE-941/942 marks.
// Nothing is written (invariant 4).
func overlayQueryContext(ctx context.Context, stderr io.Writer, options options) (map[string]any, error) {
	base := snapshotIndex(ctx, options.root)
	if base == nil {
		started := time.Now()
		built, err := contextindex.Build(ctx, options.root)
		if err != nil {
			return nil, mapStandaloneQueryBuildError(err, false)
		}
		noteSnapshotMissBuild(stderr, "query", options.root, time.Since(started))
		base = built
	}
	// Not released here: the receipt may alias mapped bytes until it is
	// emitted, and like every other reader the mapping lives for the process.
	view, err := contextindex.OverlayWorktree(ctx, base)
	if err != nil {
		return nil, err
	}
	return standaloneQueryContext(ctx, options, view)
}
