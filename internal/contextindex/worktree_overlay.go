package contextindex

import ()

// Worktree overlay (DIRTY-CACHE-940..942, V1-0418). An explicit opt-in
// `--worktree-overlay` call reads the tracked dirty paths from the worktree
// instead of HEAD, and only those: the HEAD index (a verified snapshot, or a
// full in-memory build on a miss) is the reuse prior, so every other source
// keeps its pinned bytes, per-source facts and term postings. The view is
// unshared and in-memory; nothing is written. Rows citing an overlaid path are
// worktree evidence, never committed authority.

// WorktreeOverlayAuthority labels a row whose bytes were read from the
// worktree rather than the HEAD tree (DIRTY-CACHE-942). Its trust class is
// tool-output, so it satisfies no governance, critical or basis requirement.
const WorktreeOverlayAuthority = "worktree-overlay"

// WorktreeOverlayScope is the overlay receipt's freshness scope
// (DIRTY-CACHE-941), distinct from DIRTY-CACHE-012's `git+working-tree`.
const WorktreeOverlayScope = "git+worktree-overlay"

// committedSource returns the HEAD source for sourcePath: on an overlay view
// the committed bytes, never the worktree ones, so worktree edits cannot
// supply project-owned authority (DIRTY-CACHE-942).
func (index *Index) committedSource(sourcePath string) (Source, bool) {
	if index.committedSources != nil {
		source, ok := index.committedSources[sourcePath]
		return source, ok
	}
	source, ok := index.Sources[sourcePath]
	return source, ok
}

// WorktreeOverlaid reports whether index is a worktree overlay view.
func (index *Index) WorktreeOverlaid() bool { return index != nil && index.worktreeOverlay != nil }

// overlaidBlob is the worktree blob id an overlay view read for sourcePath.
func (index *Index) overlaidBlob(sourcePath string) (string, bool) {
	if index == nil || index.worktreeOverlay == nil {
		return "", false
	}
	blob, ok := index.worktreeOverlay[sourcePath]
	return blob, ok
}

// worktreeOverlayUncertainty is the coverage line a query receipt adds when a
// result cites an overlaid path (DIRTY-CACHE-942).
const worktreeOverlayUncertainty = "worktree overlay results are advisory: their bytes were read from uncommitted worktree files, not the committed tree"

// markWorktreeResults returns results with every result citing an overlaid
// path rewritten as advisory worktree evidence: low confidence, the
// worktree-overlay authority and the worktree blob id (DIRTY-CACHE-941/942).
// A result is cloned before it changes, so callers' maps are never written.
// On any index that is not an overlay view it returns results unchanged.
func markWorktreeResults(index *Index, results []map[string]any) []map[string]any {
	if !index.WorktreeOverlaid() || len(index.worktreeOverlay) == 0 {
		return results
	}
	marked, copied := results, false
	for position, result := range results {
		evidenceRows := anySlice(result["evidence"])
		rewritten, cites := make([]any, len(evidenceRows)), false
		for row, raw := range evidenceRows {
			entry, ok := raw.(map[string]any)
			if !ok {
				rewritten[row] = raw
				continue
			}
			blob, overlaid := index.overlaidBlob(stringValue(entry["path"]))
			if !overlaid {
				rewritten[row] = raw
				continue
			}
			clone := make(map[string]any, len(entry))
			for key, value := range entry {
				clone[key] = value
			}
			clone["authority"], clone["confidence"], clone["blob_hash"] = WorktreeOverlayAuthority, "low", blob
			rewritten[row], cites = clone, true
		}
		if !cites {
			continue
		}
		clone := make(map[string]any, len(result)+2)
		for key, value := range result {
			clone[key] = value
		}
		clone["evidence"], clone["advisory"], clone["worktree_overlay"] = rewritten, true, true
		if !copied {
			marked, copied = append([]map[string]any(nil), results...), true
		}
		marked[position] = clone
	}
	return marked
}

// worktreeOverlayFreshness adds the overlay scope and count to a receipt's
// freshness member (DIRTY-CACHE-941); a non-overlay receipt is unchanged.
func worktreeOverlayFreshness(index *Index, freshness map[string]any) {
	if !index.WorktreeOverlaid() {
		return
	}
	freshness["scope"] = WorktreeOverlayScope
	freshness["overlay_path_count"] = len(index.worktreeOverlay)
}
