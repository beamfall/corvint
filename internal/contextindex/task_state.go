package contextindex

import "github.com/Beamfall/corvint/internal/taskprojection"

// taskStateOnly reports whether the observed worktree is dirty only in
// recognised Tasks store projection files (DIRTY-CACHE-017). Such a worktree
// is source-clean: the projection is task state, not an indexed source.
func (index *Index) taskStateOnly() bool {
	return taskprojection.OnlyProjection(index.DirtyPaths)
}

// SourceDirty reports whether the worktree has a dirty path that is not a
// recognised Tasks store projection file (DIRTY-CACHE-017).
func (index *Index) SourceDirty() bool {
	return len(index.DirtyPaths) != 0 && !index.taskStateOnly()
}

// taskStateMember is the separate task-state member a source-clean packet
// carries when only projection files are dirty, or nil (DIRTY-CACHE-017).
func (index *Index) taskStateMember() map[string]any {
	if !index.taskStateOnly() {
		return nil
	}
	paths := index.DirtyPaths
	if len(paths) > maxExclusionSamples {
		paths = paths[:maxExclusionSamples]
	}
	return map[string]any{"class": taskprojection.Class, "path_count": len(index.DirtyPaths), "paths": append([]string{}, paths...)}
}
