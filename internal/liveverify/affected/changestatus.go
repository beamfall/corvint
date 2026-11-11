// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

// ChangeStatus is one dirty path's change against the base the plan is
// measured from (AFP-V0-9960). It is a plan input, never a graph property,
// and is never cached (AFP-V0-9964).
type ChangeStatus string

// The change statuses. A rename is ChangeDeleted for the old path and
// ChangeAdded for the new one; similarity detection is off.
const (
	// ChangeAdded is a path absent at the base and present now, an
	// untracked file included.
	ChangeAdded ChangeStatus = "added"
	// ChangeDeleted is a path present at the base and absent now.
	ChangeDeleted ChangeStatus = "deleted"
	// ChangeTypeChanged is a path present at both sides that is not a
	// regular file at both: its entry kind differs, or it is a symlink, a
	// submodule or a directory at either side, which can redirect a listing.
	ChangeTypeChanged ChangeStatus = "type-changed"
	// ChangeModified is a regular file at both sides. It is the one status
	// that cannot change a name listing.
	ChangeModified ChangeStatus = "modified"
	// ChangeUnknown is every other case: no base, an unreadable tree, a path
	// the status source did not report. It selects as every status but
	// ChangeModified does.
	ChangeUnknown ChangeStatus = "unknown"
)

// ListingCanChange reports whether a dirty path with this status can change
// a listing of entry names and kinds: true for every status but
// ChangeModified, an unrecognised value included (AFP-V0-9962).
func ListingCanChange(status ChangeStatus) bool {
	return status != ChangeModified
}

// WithChangeStatus returns a copy of the graph whose plans read each dirty
// path's change status from status (AFP-V0-9960). A path status does not
// name, or names with a value other than the five statuses, is
// ChangeUnknown. The graph is not changed, and Select on a graph without
// this call reads every path as ChangeUnknown, which plans exactly as
// before the status existed.
func (graph *Graph) WithChangeStatus(status map[string]ChangeStatus) *Graph {
	copied := *graph
	copied.status = make(map[string]ChangeStatus, len(status))
	for relative, value := range status {
		switch value {
		case ChangeAdded, ChangeDeleted, ChangeTypeChanged, ChangeModified:
			copied.status[relative] = value
		}
	}
	return &copied
}

// ChangeStatusOf is the status a plan reads for dirtyPath: ChangeUnknown
// unless WithChangeStatus supplied one.
func (graph *Graph) ChangeStatusOf(dirtyPath string) ChangeStatus {
	if value, ok := graph.status[dirtyPath]; ok {
		return value
	}
	return ChangeUnknown
}

// listingMayChange reports whether dirtyPath's status can change a name
// listing (AFP-V0-9962).
func (graph *Graph) listingMayChange(dirtyPath string) bool {
	return ListingCanChange(graph.ChangeStatusOf(dirtyPath))
}

// ChangeEntry is what one side of a change shows at one path: Known is false
// when the side could not be read, Present when an entry is there, and
// Regular when that entry is a regular file.
type ChangeEntry struct {
	Known, Present, Regular bool
}

// ClassifyChange is the AFP-V0-9960 status of a path from its base and head
// entries. Only a regular file read at both sides is ChangeModified; a side
// that could not be read, or a path present at neither side, is
// ChangeUnknown.
func ClassifyChange(base, head ChangeEntry) ChangeStatus {
	switch {
	case !base.Known || !head.Known:
		return ChangeUnknown
	case !base.Present && !head.Present:
		return ChangeUnknown
	case !base.Present:
		return ChangeAdded
	case !head.Present:
		return ChangeDeleted
	case base.Regular && head.Regular:
		return ChangeModified
	}
	return ChangeTypeChanged
}

// changeStatusBatch bounds the paths one `git ls-tree` invocation names, so
// the argument list stays far below every host's limit.
const changeStatusBatch = 256

// ChangeStatuses computes the AFP-V0-9960 status of each path against the
// commit base, read from repository's Git objects, and the tree the plan is
// built over (the worktree, or a materialized snapshot). It returns nil, so
// every path is ChangeUnknown, when base is empty or the base tree cannot be
// read; a path either side cannot establish is left out, which is
// ChangeUnknown too. A base entry counts only when Git reports exactly the
// requested path, and a head entry only when its parent directory lists
// exactly its name, so a case-folding or normalizing file system can never
// turn a rename into ChangeModified.
func ChangeStatuses(ctx context.Context, gitExecutable, repository, tree, base string, paths []string) map[string]ChangeStatus {
	if gitExecutable == "" || repository == "" || tree == "" || base == "" || len(paths) == 0 {
		return nil
	}
	deadline, cancel := context.WithTimeout(ctx, StatusDeadline)
	defer cancel()
	run := boundedGitRunner(gitExecutable)
	modes := make(map[string]string, len(paths))
	candidates := make([]string, 0, len(paths))
	for _, relative := range paths {
		if ValidRelativePath(relative) && !strings.HasSuffix(relative, "/") {
			candidates = append(candidates, relative)
		}
	}
	for start := 0; start < len(candidates); start += changeStatusBatch {
		batch := candidates[start:min(start+changeStatusBatch, len(candidates))]
		raw, err := run(deadline, repository, MaxStatusBytes, append([]string{"--literal-pathspecs", "ls-tree", "-z", "--full-tree", base, "--"}, batch...)...)
		if err != nil || len(raw) >= MaxStatusBytes {
			return nil
		}
		for _, record := range strings.Split(string(raw), "\x00") {
			if record == "" {
				continue
			}
			meta, name, found := strings.Cut(record, "\t")
			fields := strings.Fields(meta)
			if !found || len(fields) != 3 {
				return nil
			}
			modes[name] = fields[0]
		}
	}
	listings := map[string]map[string]fs.FileMode{}
	status := make(map[string]ChangeStatus, len(candidates))
	for _, relative := range candidates {
		mode, present := modes[relative]
		baseEntry := ChangeEntry{Known: true, Present: present, Regular: mode == "100644" || mode == "100755"}
		value := ClassifyChange(baseEntry, headEntry(tree, relative, listings))
		if value != ChangeUnknown {
			status[relative] = value
		}
	}
	return status
}

// headEntry reads relative in tree through its parent directory's listing,
// cached per directory in listings (a nil map marks a parent that is absent
// or not a directory). The listing follows a parent that is a symbolic link;
// that parent is then a dirty path of its own, classified type-changed. Any
// other read failure is not Known.
func headEntry(tree, relative string, listings map[string]map[string]fs.FileMode) ChangeEntry {
	parent, name := path.Split(relative)
	entries, cached := listings[parent]
	if !cached {
		read, err := os.ReadDir(filepath.Join(tree, filepath.FromSlash(parent)))
		switch {
		case err == nil:
			entries = make(map[string]fs.FileMode, len(read))
			for _, entry := range read {
				entries[entry.Name()] = entry.Type()
			}
		case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
			entries = nil
		default:
			return ChangeEntry{}
		}
		listings[parent] = entries
	}
	kind, present := entries[name]
	return ChangeEntry{Known: true, Present: present, Regular: present && kind.IsRegular()}
}
