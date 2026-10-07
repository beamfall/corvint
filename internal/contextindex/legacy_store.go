package contextindex

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"
	"time"
)

// legacySweepMaxEntries bounds the entries one index write inspects in a
// worktree's superseded `.corvint/index` store (proposed IDX-SNAP-V0-027). A
// larger directory is finished by later writes.
const legacySweepMaxEntries = 4 * snapshotKeepCap

// legacyPublishedName matches the files a per-worktree store held: a gob
// snapshot, its sectioned companion, or a pack, named
// `<object-format>-<tree>-<engine>` (snapshotPath, sectionedPath, packPath).
var legacyPublishedName = regexp.MustCompile(`^(?:sha1-[0-9a-f]{40}|sha256-[0-9a-f]{64})-[0-9a-f]{16}\.(gob|sect|aip)$`)

// LegacyEntry is an entry of a superseded per-worktree store that an index
// write left in place, and why (proposed IDX-SNAP-V0-027).
type LegacyEntry struct {
	Path, Reason string
}

// sweepLegacyStore removes the files of root's per-worktree `.corvint/index`
// store once the shared store under the Git common directory has superseded
// it. Only WriteSnapshot calls it, after the shared store's write and
// eviction, so no read verb removes anything (AGENTS.md invariant 4). It
// removes only regular files it recognises as a Corvint snapshot, companion,
// build-cost record or stale writer temporary, then the store's own
// `.gitignore` and the empty directory; every other entry, and a store that is
// not a real directory, is left and named. directory is "" when there is no
// legacy store.
func sweepLegacyStore(root string, now time.Time) (directory string, removed []EvictedSnapshot, left []LegacyEntry) {
	directory = filepath.Join(root, filepath.FromSlash(snapshotSubpath))
	// Every step after these checks is relative to descriptors whose identity
	// matched a no-follow Lstat, so swapping `.corvint` or `.corvint/index`
	// for a link mid-sweep cannot redirect a removal outside the store.
	corvint, ok := openRealDirectory(nil, filepath.Join(root, ".corvint"))
	if !ok {
		return "", nil, nil
	}
	defer corvint.Close()
	if _, err := corvint.Lstat("index"); errors.Is(err, os.ErrNotExist) {
		return "", nil, nil
	}
	store, ok := openRealDirectory(corvint, "index")
	if !ok {
		return directory, nil, []LegacyEntry{{directory, "not a real directory"}}
	}
	defer store.Close()
	listing, err := store.Open(".")
	if err != nil {
		return directory, nil, []LegacyEntry{{directory, "unreadable: " + err.Error()}}
	}
	entries, readErr := listing.ReadDir(legacySweepMaxEntries)
	complete := len(entries) < legacySweepMaxEntries
	if !complete {
		more, _ := listing.ReadDir(1)
		complete = len(more) == 0
	}
	listing.Close()
	if readErr != nil && readErr != io.EOF {
		return directory, nil, []LegacyEntry{{directory, "unreadable: " + readErr.Error()}}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	staleCutoff := now.Add(-snapshotTemporaryStaleAfter)
	ignore := false
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(directory, name)
		info, err := store.Lstat(name)
		if err != nil {
			left = append(left, LegacyEntry{path, "unreadable: " + err.Error()})
			continue
		}
		if !info.Mode().IsRegular() {
			left = append(left, LegacyEntry{path, "not a regular file"})
			continue
		}
		kind := ""
		switch match := legacyPublishedName.FindStringSubmatch(name); {
		case match != nil:
			kind = map[string]string{"gob": "snapshot", "sect": "sectioned", "aip": "pack"}[match[1]]
		case name == buildCostName:
			kind = "build-cost"
		case isStoreTemporary(name):
			if info.ModTime().After(staleCutoff) {
				left = append(left, LegacyEntry{path, "writer temporary not yet stale"})
				continue
			}
			kind = "temporary"
		case name == ".gitignore":
			ignore = true
			continue
		default:
			left = append(left, LegacyEntry{path, "unrecognised entry"})
			continue
		}
		if err := store.Remove(name); err != nil {
			left = append(left, LegacyEntry{path, "remove failed: " + err.Error()})
			continue
		}
		removedFile := EvictedSnapshot{Kind: kind, Path: path, Bytes: info.Size()}
		if kind == "snapshot" || kind == "sectioned" || kind == "pack" {
			removedFile.Tree, removedFile.Engine = snapshotTreeOf(path), snapshotEngineOf(trimPublishedExtension(path))
		}
		removed = append(removed, removedFile)
	}
	// The store's ignore file keeps whatever was left out of `git status`, so
	// it goes only with the directory, and only when it is the one the
	// fallback writer publishes (writeSnapshotGitIgnore).
	if !complete || len(left) > 0 {
		if !complete {
			left = append(left, LegacyEntry{directory, "entry bound reached; a later index write continues"})
		}
		return directory, removed, left
	}
	ignorePath := filepath.Join(directory, ".gitignore")
	if ignore {
		info, err := store.Lstat(".gitignore")
		if err != nil || !info.Mode().IsRegular() || !legacyIgnoreIsCorvints(store) {
			return directory, removed, append(left, LegacyEntry{ignorePath, "unrecognised entry"})
		}
		if err := store.Remove(".gitignore"); err != nil {
			return directory, removed, append(left, LegacyEntry{ignorePath, "remove failed: " + err.Error()})
		}
		removed = append(removed, EvictedSnapshot{Kind: "ignore", Path: ignorePath, Bytes: info.Size()})
	}
	opened, err := store.Stat(".")
	if err == nil {
		var named os.FileInfo
		if named, err = corvint.Lstat("index"); err == nil && !os.SameFile(opened, named) {
			err = errors.New("replaced during the sweep")
		}
	}
	// rmdir, unlike Remove, never deletes a file or link that replaced the
	// directory after the check above, nor a directory that is not empty.
	if err == nil {
		err = syscall.Rmdir(directory)
	}
	if err != nil {
		return directory, removed, append(left, LegacyEntry{directory, "remove failed: " + err.Error()})
	}
	removed = append(removed, EvictedSnapshot{Kind: "directory", Path: directory})
	return directory, removed, left
}

// legacyIgnoreIsCorvints reports whether the store's `.gitignore` holds exactly
// what writeSnapshotGitIgnore writes.
func legacyIgnoreIsCorvints(store *os.Root) bool {
	file, err := store.Open(".gitignore")
	if err != nil {
		return false
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, 3))
	return err == nil && string(content) == "*\n"
}

func trimPublishedExtension(path string) string {
	return path[:len(path)-len(filepath.Ext(path))] + ".gob"
}

// openRealDirectory opens name, relative to parent or as a path when parent is
// nil, only when it is a real directory and the opened descriptor is that same
// directory, not one a link or a concurrent swap put in its place.
func openRealDirectory(parent *os.Root, name string) (*os.Root, bool) {
	lstat, open := os.Lstat, os.OpenRoot
	if parent != nil {
		lstat, open = parent.Lstat, parent.OpenRoot
	}
	before, err := lstat(name)
	if err != nil || !before.IsDir() {
		return nil, false
	}
	opened, err := open(name)
	if err != nil {
		return nil, false
	}
	after, err := opened.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		opened.Close()
		return nil, false
	}
	return opened, true
}
