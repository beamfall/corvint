package affected

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// graphCacheHook runs at the named stage of a cache directory or entry
// access; tests replace it to stage a replacement race (AFP-V0-390,
// AFP-V0-392). The stages are "open", before the directory is opened,
// "write", before an entry is written through the open handle, and "entry",
// before an entry is opened.
var graphCacheHook = func(string) {}

// forbiddenDirectory is a directory no graph cache handle may be at or under.
type forbiddenDirectory struct {
	path string
	info fs.FileInfo
}

// forbiddenDirectories resolves root and every excluded directory (the
// repository's Git and common directories) once per call.
func forbiddenDirectories(root string, excluded []string) []forbiddenDirectory {
	forbidden := []forbiddenDirectory{}
	for _, name := range append([]string{root}, excluded...) {
		if name == "" {
			continue
		}
		directory := forbiddenDirectory{path: filepath.Clean(name)}
		if resolved, err := filepath.EvalSymlinks(name); err == nil {
			directory.path = resolved
		}
		if info, err := os.Stat(directory.path); err == nil {
			directory.info = info
		}
		forbidden = append(forbidden, directory)
	}
	return forbidden
}

// openCacheDirectory opens the graph cache directory dir as the one handle
// every entry read and write goes through (AFP-V0-390). The deepest existing
// ancestor is opened first; each missing component is created (mode 0700)
// inside the handle above it and then opened through that handle. Every
// handle is checked by the path the kernel reports for the open directory and
// by file identity against root and excluded, so a pathname replaced by a
// symbolic link into the repository after the caller's placement check is
// refused rather than written through, and a replacement after the open
// cannot redirect the handle. The final directory must belong to this user
// and be writable by no one else, since anyone who can write an entry can
// forge one (AFP-V0-392). reason is empty on success.
func openCacheDirectory(dir, root string, excluded []string, create bool) (*os.Root, string) {
	if !filepath.IsAbs(dir) {
		return nil, "cache directory is not absolute"
	}
	forbidden := forbiddenDirectories(root, excluded)
	existing := filepath.Clean(dir)
	missing := []string{}
	for {
		_, err := os.Stat(existing)
		if err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if !errors.Is(err, fs.ErrNotExist) || parent == existing {
			return nil, "cache directory unreadable"
		}
		missing = append([]string{filepath.Base(existing)}, missing...)
		existing = parent
	}
	if len(missing) != 0 && !create {
		return nil, "no entry"
	}
	graphCacheHook("open")
	handle, err := os.OpenRoot(existing)
	if err != nil {
		return nil, "cache directory unreadable"
	}
	if reason := admitCacheHandle(handle, forbidden); reason != "" {
		_ = handle.Close()
		return nil, reason
	}
	for _, name := range missing {
		if err := handle.Mkdir(name, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			_ = handle.Close()
			return nil, "cache directory: " + err.Error()
		}
		child, err := handle.OpenRoot(name)
		_ = handle.Close()
		if err != nil {
			return nil, "cache directory unreadable"
		}
		handle = child
		if reason := admitCacheHandle(handle, forbidden); reason != "" {
			_ = handle.Close()
			return nil, reason
		}
	}
	info, err := handle.Stat(".")
	if err != nil || !privateDirectory(info) {
		_ = handle.Close()
		return nil, "cache directory is not private to this user"
	}
	return handle, ""
}

// InspectCacheDirectory reports why an existing graph cache directory dir
// could not serve root, by the same placement, handle and privacy checks the
// planner applies, without creating or writing anything (AFP-V0-439). The
// reason is empty when the planner would use dir.
func InspectCacheDirectory(dir, root string, excluded []string) string {
	handle, reason := openCacheDirectory(dir, root, excluded, false)
	if handle != nil {
		_ = handle.Close()
	}
	return reason
}

// admitCacheHandle refuses a handle whose directory is at or under a
// forbidden directory, by the kernel's path for the open directory and by the
// file identity of that path and each of its ancestors.
func admitCacheHandle(handle *os.Root, forbidden []forbiddenDirectory) string {
	file, err := handle.Open(".")
	if err != nil {
		return "cache directory unreadable"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "cache directory unreadable"
	}
	name, err := directoryPath(file)
	if err != nil || !filepath.IsAbs(name) {
		return "cache directory location unknown"
	}
	const refusal = "cache directory is inside the repository or its Git directory"
	for _, directory := range forbidden {
		if pathWithin(name, directory.path) || directory.info != nil && os.SameFile(info, directory.info) {
			return refusal
		}
	}
	for current := filepath.Dir(name); ; current = filepath.Dir(current) {
		if ancestor, err := os.Stat(current); err == nil {
			for _, directory := range forbidden {
				if directory.info != nil && os.SameFile(ancestor, directory.info) {
					return refusal
				}
			}
		}
		if filepath.Dir(current) == current {
			return ""
		}
	}
}

// pathWithin reports whether candidate is root or below it, textually.
func pathWithin(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
