package affected

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Source is one explicit source universe. A disk source retains the existing
// live APIs; an FS source reads exclusively from its supplied filesystem.
type Source struct {
	root  string
	fsys  fs.FS
	mu    sync.Mutex
	walks map[string]*sharedWalk
	fatal error
}

func DiskSource(root string) *Source { return &Source{root: root} }
func FSSource(source fs.FS) *Source  { return &Source{fsys: source, walks: map[string]*sharedWalk{}} }

// Err retains the first immutable read failure, including failures a provider ignores.
// Missing optional paths and successful walk control signals are not failures.
func (s *Source) Err() error { s.mu.Lock(); defer s.mu.Unlock(); return s.fatal }
func (s *Source) retain(err error) {
	if s == nil || s.fsys == nil || err == nil || errors.Is(err, fs.ErrNotExist) || err == fs.SkipDir || err == fs.SkipAll {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fatal == nil {
		s.fatal = err
	}
}

func (s *Source) Files(accept func(string) bool) ([]string, error) {
	files, _, err := s.FilesIncluding(accept)
	return files, err
}
func (s *Source) FilesIncluding(accept func(string) bool, included ...string) (filesOut []string, boundedOut bool, errOut error) {
	defer func() { s.retain(errOut) }()
	if s == nil {
		return nil, false, ErrInvalidLanguage
	}
	if s.fsys == nil {
		return SourceFilesIncluding(s.root, accept, included...)
	}
	key := strings.Join(included, "\x00")
	s.mu.Lock()
	shared := s.walks[key]
	if shared == nil {
		shared = &sharedWalk{}
		s.walks[key] = shared
	}
	s.mu.Unlock()
	shared.once.Do(func() { shared.files, shared.bounded, shared.err = walkSourceFS(s.fsys, acceptEveryName, included) })
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
func (s *Source) Read(relative string) (bodyOut []byte, errOut error) {
	defer func() { s.retain(errOut) }()
	if s == nil {
		return nil, ErrInvalidLanguage
	}
	if s.fsys == nil {
		return ReadSource(s.root, relative)
	}
	if !ValidRelativePath(relative) {
		return nil, ErrInvalidUnit
	}
	f, err := s.fsys.Open(relative)
	if err != nil {
		return nil, err
	}
	defer func() { s.retain(f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalidUnit
	}
	if info.Size() > MaxSourceBytes {
		return nil, ErrWalkLimit
	}
	body, err := io.ReadAll(io.LimitReader(f, MaxSourceBytes+1))
	if len(body) > MaxSourceBytes {
		return nil, ErrWalkLimit
	}
	return body, err
}
func (s *Source) Stat(relative string) (infoOut fs.FileInfo, errOut error) {
	defer func() { s.retain(errOut) }()
	if s == nil || !fs.ValidPath(relative) {
		return nil, ErrInvalidUnit
	}
	if s.fsys != nil {
		info, err := fs.Stat(s.fsys, relative)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %q", ErrWalkUnrepresentable, relative)
		}
		return info, nil
	}
	return os.Stat(filepath.Join(s.root, filepath.FromSlash(relative)))
}
func (s *Source) Lstat(relative string) (infoOut fs.FileInfo, errOut error) {
	defer func() { s.retain(errOut) }()
	if s == nil || !fs.ValidPath(relative) {
		return nil, ErrInvalidUnit
	}
	if s.fsys != nil {
		info, err := fs.Stat(s.fsys, relative)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: %q", ErrWalkUnrepresentable, relative)
		}
		return info, nil
	}
	return os.Lstat(filepath.Join(s.root, filepath.FromSlash(relative)))
}
func (s *Source) Walk(relative string, visit fs.WalkDirFunc) (errOut error) {
	defer func() { s.retain(errOut) }()
	if s == nil || !fs.ValidPath(relative) {
		return ErrInvalidUnit
	}
	if s.fsys != nil {
		entries := 0
		return fs.WalkDir(s.fsys, relative, func(current string, entry fs.DirEntry, err error) error {
			s.retain(err)
			entries++
			if entries > MaxWalkEntries {
				return ErrWalkLimit
			}
			if err == nil && entry != nil && !entry.IsDir() && !entry.Type().IsRegular() {
				return fmt.Errorf("%w: %q", ErrWalkUnrepresentable, current)
			}
			return visit(current, entry, err)
		})
	}
	return filepath.WalkDir(filepath.Join(s.root, filepath.FromSlash(relative)), visit)
}

func walkSourceFS(source fs.FS, accept func(name string) bool, includedDirectories []string) ([]string, bool, error) {
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
		if !entry.Type().IsRegular() {
			// Immutable symlinks/gitlinks have no complete file universe here.
			// Refuse conservatively before language filtering can drop them.
			return fmt.Errorf("%w: %q", ErrWalkUnrepresentable, current)
		}
		if !accept(entry.Name()) {
			return nil
		}
		slashed := current
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
		err := fs.WalkDir(source, includedRoot, func(current string, entry fs.DirEntry, err error) error {
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
	err := fs.WalkDir(source, ".", func(current string, entry fs.DirEntry, err error) error {
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
			if current == "." {
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
