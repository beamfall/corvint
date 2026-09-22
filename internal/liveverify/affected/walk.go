package affected

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrWalkLimit reports a repository larger than the fixed walk bound.
var ErrWalkLimit = errors.New("affected: repository walk exceeds its bound")

// ErrWalkUnreadable reports a directory or entry the walk could not read.
var ErrWalkUnreadable = errors.New("affected: source walk hit an unreadable entry")

// ErrWalkUnrepresentable reports an accepted file whose repository-relative
// path is no canonical unit path, so no unit could name it.
var ErrWalkUnrepresentable = errors.New("affected: source walk hit a file path no unit can name")

const (
	// MaxWalkEntries bounds one repository walk.
	MaxWalkEntries = 400_000
	// MaxIncludedDirectoryEntries bounds each language-opted directory subtree.
	MaxIncludedDirectoryEntries = 20_000
	// MaxSourceBytes bounds one source file a plugin may read for imports.
	MaxSourceBytes = 4 << 20
)

// SkippedDirectories are not descended by SourceFiles. A language plugin may
// explicitly admit names that can hold its first-party source through
// SourceFilesIncluding.
var SkippedDirectories = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "__pycache__": true,
	".venv": true, "venv": true, ".tox": true, ".mypy_cache": true,
	".pytest_cache": true, ".ruff_cache": true, "site-packages": true,
	"dist": true, "build": true, "target": true, ".idea": true, ".vscode": true,
}

// SourceFiles walks root and returns every repository-relative file path whose
// name satisfies accept, sorted.
//
// Directories in SkippedDirectories and directories whose name begins with "."
// are not descended. Symbolic links are never followed: a link is reported as
// the link itself and, because no plugin accepts a link as source text, it is
// simply not a unit member.
func SourceFiles(root string, accept func(name string) bool) ([]string, error) {
	files, _, err := SourceFilesIncluding(root, accept)
	return files, err
}

// SourceFilesIncluding is SourceFiles with explicit language-owned exceptions
// to SkippedDirectories. Each exception applies at every depth.
func SourceFilesIncluding(root string, accept func(name string) bool, includedDirectories ...string) ([]string, bool, error) {
	if root == "" || !filepath.IsAbs(root) {
		return nil, false, fmt.Errorf("%w: root %q is not absolute", ErrInvalidLanguage, root)
	}
	shared := heldWalk(root, includedDirectories)
	if shared == nil {
		return walkSourceFiles(root, accept, includedDirectories)
	}
	shared.once.Do(func() {
		shared.files, shared.bounded, shared.err = walkSourceFiles(root, acceptEveryName, includedDirectories)
	})
	if shared.err != nil {
		return nil, false, shared.err
	}
	files := make([]string, 0, len(shared.files))
	for _, relative := range shared.files {
		if accept(path.Base(relative)) {
			files = append(files, relative)
		}
	}
	return files, shared.bounded, nil
}

// walks shares one traversal per root and directory exceptions among the
// plugins of one Build, which used to walk the repository once per plugin
// call. A traversal is shared only while a Build holds its root, so a caller
// outside Build still observes the tree as it is at the call.
var walks = struct {
	sync.Mutex
	holders map[string]int
	shared  map[walkKey]*sharedWalk
}{holders: map[string]int{}, shared: map[walkKey]*sharedWalk{}}

type walkKey struct{ root, included string }

type sharedWalk struct {
	once    sync.Once
	files   []string
	bounded bool
	err     error
}

// holdWalks shares traversals of root until the returned release is called.
func holdWalks(root string) func() {
	walks.Lock()
	walks.holders[root]++
	walks.Unlock()
	return func() {
		walks.Lock()
		defer walks.Unlock()
		walks.holders[root]--
		if walks.holders[root] > 0 {
			return
		}
		delete(walks.holders, root)
		for key := range walks.shared {
			if key.root == root {
				delete(walks.shared, key)
			}
		}
	}
}

func heldWalk(root string, includedDirectories []string) *sharedWalk {
	walks.Lock()
	defer walks.Unlock()
	if walks.holders[root] == 0 {
		return nil
	}
	key := walkKey{root: root, included: strings.Join(includedDirectories, "\x00")}
	if walks.shared[key] == nil {
		walks.shared[key] = &sharedWalk{}
	}
	return walks.shared[key]
}

func acceptEveryName(string) bool { return true }

// walkSourceFiles is one traversal of root for SourceFilesIncluding.
func walkSourceFiles(root string, accept func(name string) bool, includedDirectories []string) ([]string, bool, error) {
	included := make(map[string]bool, len(includedDirectories))
	for _, name := range includedDirectories {
		included[name] = true
	}
	files := make([]string, 0, 1024)
	entries := 0
	bounded := false
	// recordFile refuses an accepted file it cannot name: dropping it would
	// silently narrow the unit set while Select still reported BOUNDED scope.
	recordFile := func(current string, entry fs.DirEntry) error {
		if !entry.Type().IsRegular() || !accept(entry.Name()) {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return fmt.Errorf("%w: %q", ErrWalkUnrepresentable, current)
		}
		slashed := filepath.ToSlash(relative)
		if !ValidRelativePath(slashed) {
			return fmt.Errorf("%w: %q", ErrWalkUnrepresentable, slashed)
		}
		files = append(files, slashed)
		return nil
	}
	var walkIncluded func(string) (bool, error)
	walkIncluded = func(includedRoot string) (bool, error) {
		includedEntries := 1
		subtreeBounded := false
		err := filepath.WalkDir(includedRoot, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("%w: %s", ErrWalkUnreadable, current)
			}
			if current == includedRoot {
				return nil
			}
			entries++
			if entries > MaxWalkEntries {
				return ErrWalkLimit
			}
			includedEntries++
			if includedEntries > MaxIncludedDirectoryEntries {
				subtreeBounded = true
				return fs.SkipAll
			}
			name := entry.Name()
			if entry.IsDir() {
				if strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				if SkippedDirectories[name] && !included[name] {
					return fs.SkipDir
				}
				if SkippedDirectories[name] && included[name] {
					childBounded, childErr := walkIncluded(current)
					subtreeBounded = subtreeBounded || childBounded
					if childErr != nil {
						return childErr
					}
					return fs.SkipDir
				}
				return nil
			}
			return recordFile(current, entry)
		})
		return subtreeBounded, err
	}
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A subtree the walk cannot read would silently narrow the unit
			// set while Select still reported BOUNDED scope; refuse instead.
			return fmt.Errorf("%w: %s", ErrWalkUnreadable, current)
		}
		entries++
		if entries > MaxWalkEntries {
			return ErrWalkLimit
		}
		name := entry.Name()
		if entry.IsDir() {
			if current == root {
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if SkippedDirectories[name] && !included[name] {
				return fs.SkipDir
			}
			if SkippedDirectories[name] && included[name] {
				subtreeBounded, subtreeErr := walkIncluded(current)
				bounded = bounded || subtreeBounded
				if subtreeErr != nil {
					return subtreeErr
				}
				return fs.SkipDir
			}
			return nil
		}
		return recordFile(current, entry)
	})
	if err != nil {
		return nil, false, err
	}
	sort.Strings(files)
	return files, bounded, nil
}

// ReadSource reads one repository-relative source file under root, bounded by
// MaxSourceBytes. A file at or over the bound is refused rather than truncated,
// because a truncated read would silently drop import edges.
func ReadSource(root, relative string) ([]byte, error) {
	if !ValidRelativePath(relative) {
		return nil, fmt.Errorf("%w: path %q", ErrInvalidUnit, relative)
	}
	full := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %q is not a regular file", ErrInvalidUnit, relative)
	}
	if info.Size() > MaxSourceBytes {
		return nil, fmt.Errorf("%w: %q is %d bytes", ErrWalkLimit, relative, info.Size())
	}
	return os.ReadFile(full)
}
