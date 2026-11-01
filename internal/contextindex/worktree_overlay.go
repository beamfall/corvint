package contextindex

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

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

const worktreeOverlayUnreadReason = "worktree content not overlaid: unreadable, not a regular file, or over the source size bound"

// OverlayWorktree returns a view of base whose tracked dirty paths carry their
// worktree bytes, cited by the Git blob id of those bytes. base must be a
// fully compiled index of HEAD (a full snapshot load or a full build); it is
// not modified, and the view shares its mapping lease, so releasing either
// releases both.
func OverlayWorktree(ctx context.Context, base *Index) (*Index, error) {
	if base == nil {
		return nil, &Error{Message: "worktree overlay needs a HEAD index"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sources := make(map[string]Source, len(base.Sources))
	for sourcePath, source := range base.Sources {
		sources[sourcePath] = source
	}
	overlaid := map[string]string{}
	dropped := map[string]string{}
	for _, dirty := range base.DirtyPaths {
		mode, ok := worktreeOverlayCandidate(base, dirty)
		if !ok {
			continue
		}
		data, read := readOverlayFile(base.Root, dirty)
		if !read {
			delete(sources, dirty)
			dropped[dirty] = worktreeOverlayUnreadReason
			if _, err := os.Lstat(filepath.Join(base.Root, filepath.FromSlash(dirty))); errors.Is(err, fs.ErrNotExist) {
				dropped[dirty] = "deleted in the worktree"
			}
			continue
		}
		blob := gitBlobHash(data, base.ObjectFormat)
		if head, ok := base.Sources[dirty]; ok && head.BlobHash == blob {
			continue
		}
		pinned := pinnedFrom(treeEntry{path: dirty, oid: blob, mode: mode, size: len(data)}, data)
		if reason := pinned.exclusionReason(); reason != "" {
			delete(sources, dirty)
			dropped[dirty] = "worktree content not overlaid: " + reason
			continue
		}
		sources[dirty] = pinned.source
		overlaid[dirty] = blob
	}
	view := &Index{
		Root: base.Root, ObjectFormat: base.ObjectFormat, CommitRevision: base.CommitRevision,
		Revision: base.Revision, StatusSHA256: base.StatusSHA256,
		Sources: sources, Exclusions: overlayExclusions(base.Exclusions, overlaid, dropped),
		DirtyPaths: base.DirtyPaths, Tracked: base.Tracked, Skipped: base.Skipped,
		Features: map[string]Record{}, Scenarios: map[string]Record{}, Documents: map[string]Record{},
		Markers: map[string][]Marker{}, Imports: map[string]map[string]struct{}{},
		ProfileID: base.ProfileID, UnsupportedSuffixCount: base.UnsupportedSuffixCount,
		bodies: base.bodies, lease: base.lease, worktreeOverlay: overlaid, committedSources: base.Sources,
	}
	if facts, err := priorFacts(base); err == nil {
		prior := &reusePrior{index: base, facts: facts, taken: make(map[string]struct{}, len(sources))}
		for sourcePath := range sources {
			if _, changed := overlaid[sourcePath]; !changed {
				prior.taken[sourcePath] = struct{}{}
			}
		}
		prior.attach(view)
	}
	view.compile()
	view.blobFacts, view.priorVocabulary = nil, nil
	return view, nil
}

// readOverlayFile reads a tracked dirty path's worktree bytes only when every
// parent directory is a real directory, before and after the read.
// readCleanFile confines opens to the repository but follows an in-repository
// directory symlink, which would let untracked or ignored content stand in for
// a tracked path; the overlay reads no bytes through one.
func readOverlayFile(root, relative string) ([]byte, bool) {
	if !realParentDirectories(root, relative) {
		return nil, false
	}
	data, read := readCleanFile(root, relative)
	if !read || !realParentDirectories(root, relative) {
		return nil, false
	}
	return data, true
}

func realParentDirectories(root, relative string) bool {
	current := root
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component == "." {
			break
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

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

// worktreeOverlayCandidate reports whether dirty may be read from the
// worktree: tracked at HEAD, through the same path screen and admitted-suffix
// rule as the index, and a regular-file mode at HEAD. Untracked paths are
// never overlaid (DIRTY-CACHE-940).
func worktreeOverlayCandidate(base *Index, dirty string) (string, bool) {
	if _, tracked := base.Tracked[dirty]; !tracked || forbiddenPath(dirty) != "" {
		return "", false
	}
	if name := path.Base(dirty); !ImpactPathAdmitted(dirty) && name != "Makefile" && name != "go.mod" {
		return "", false
	}
	mode := "100644"
	if head, ok := base.Sources[dirty]; ok {
		mode = head.Mode
	}
	return mode, mode == "100644" || mode == "100755"
}

// overlayExclusions drops the HEAD exclusion of every overlaid path, whose
// worktree bytes are now admitted, and adds one for every dropped path.
func overlayExclusions(head []Exclusion, overlaid, dropped map[string]string) []Exclusion {
	exclusions := make([]Exclusion, 0, len(head)+len(dropped))
	for _, exclusion := range head {
		_, admitted := overlaid[exclusion.Path]
		_, replaced := dropped[exclusion.Path]
		if !admitted && !replaced {
			exclusions = append(exclusions, exclusion)
		}
	}
	for droppedPath, reason := range dropped {
		exclusions = append(exclusions, Exclusion{droppedPath, reason})
	}
	sort.Slice(exclusions, func(left, right int) bool { return exclusions[left].Path < exclusions[right].Path })
	return exclusions
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

// markWorktreeRows relabels every context row citing an overlaid path as
// low-confidence worktree evidence (DIRTY-CACHE-942). A reserved row the
// authority screen already downgraded keeps its downgrade (TCP-V0-056).
func (compiler *taskContextCompiler) markWorktreeRows(rows []contextRow) []contextRow {
	if !compiler.index.WorktreeOverlaid() {
		return rows
	}
	for position := range rows {
		if _, overlaid := compiler.index.overlaidBlob(rows[position].path); overlaid && rows[position].downgrade == "" {
			rows[position].authority, rows[position].confidence = WorktreeOverlayAuthority, "low"
		}
	}
	return rows
}
