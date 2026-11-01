package affected

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Source is one explicit source universe. A disk source retains the existing
// live APIs; an FS source reads exclusively from its supplied filesystem.
type Source struct {
	ctx   context.Context
	root  string
	fsys  fs.FS
	mu    sync.Mutex
	walks map[string]*sharedWalk
	fatal error
	// record, when set, notes every disk observation for a graph cache
	// stamp (AFP-V0-391).
	record *recording
}

func DiskSource(root string) *Source { return &Source{root: root} }

// readSourceAt is the disk read behind Source.Read; tests replace it to inject
// a transient failure (AFP-V0-392).
var readSourceAt = ReadSource

// DiskSourceContext scopes both cancellation and cached walks to this reader.
func DiskSourceContext(ctx context.Context, root string) *Source {
	return &Source{root: root, ctx: ctx, walks: map[string]*sharedWalk{}}
}
func (s *Source) cancelled() error {
	if s != nil && s.ctx != nil {
		return s.ctx.Err()
	}
	return nil
}

// stopped is cancelled, or the retained fatal failure of an immutable source:
// once a read has failed (an exhausted Git budget, say) every later read
// returns that failure without consulting the filesystem, so a provider that
// tolerates one failed read cannot keep walking the source (V1-1416).
func (s *Source) stopped() error {
	if err := s.cancelled(); err != nil {
		return err
	}
	if s != nil && s.fsys != nil {
		return s.Err()
	}
	return nil
}

func FSSource(source fs.FS) *Source { return &Source{fsys: source, walks: map[string]*sharedWalk{}} }

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
	if err := s.stopped(); err != nil {
		return nil, false, err
	}
	defer func() { s.retain(errOut) }()
	if s == nil {
		return nil, false, ErrInvalidLanguage
	}
	if s.fsys == nil && s.ctx == nil {
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
	shared.once.Do(func() {
		if s.fsys == nil {
			var visit func(string)
			if s.record != nil {
				visit = func(relative string) { s.record.path(s.root, relative, false) }
			}
			shared.files, shared.bounded, shared.err = walkSourceFilesContext(s.ctx, s.root, acceptEveryName, included, visit)
		} else {
			shared.files, shared.bounded, shared.err = walkSourceFS(s.fsys, acceptEveryName, included)
		}
	})
	if shared.err != nil {
		return nil, false, shared.err
	}
	files := make([]string, 0, len(shared.files))
	for _, relative := range shared.files {
		if err := s.cancelled(); err != nil {
			return nil, false, err
		}
		if accept(path.Base(relative)) {
			files = append(files, relative)
		}
	}
	return files, shared.bounded, nil
}

// HeldIgnored is the TJAA-V0-100 exclusion this source's walks apply: the
// matcher of the hold on a disk source's root, or nil for a supplied
// filesystem, a recording source or a root no hold covers.
func (s *Source) HeldIgnored() func(string) bool {
	if s == nil || s.fsys != nil || s.record != nil {
		return nil
	}
	return HeldIgnoredMatcher(s.root)
}

// ConcurrentReads reports a disk source, whose Read may run on several
// goroutines at once. A supplied filesystem makes no such promise.
func (s *Source) ConcurrentReads() bool { return s != nil && s.fsys == nil }

func (s *Source) Read(relative string) (bodyOut []byte, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
	defer func() { s.retain(errOut) }()
	if s == nil {
		return nil, ErrInvalidLanguage
	}
	if s.fsys == nil {
		s.record.path(s.root, relative, false)
		body, err := readSourceAt(s.root, relative)
		s.record.failed(relative, err)
		return body, err
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

// ReadBounded captures a smaller input contract without first asking an
// immutable backend to Stat or Open, either of which may allocate its body.
// An opaque filesystem cannot establish that admission bound and is refused.
func (s *Source) ReadBounded(relative string, limit int) (bodyOut []byte, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
	defer func() { s.retain(errOut) }()
	if s == nil {
		return nil, ErrInvalidLanguage
	}
	if !ValidRelativePath(relative) || limit < 1 || limit > MaxSourceBytes {
		return nil, ErrInvalidUnit
	}
	var file fs.File
	var err error
	if s.fsys != nil {
		bounded, ok := s.fsys.(interface {
			OpenBounded(string, int) (fs.File, error)
		})
		if !ok {
			return nil, fmt.Errorf("%w: immutable source lacks bounded open", ErrWalkUnrepresentable)
		}
		file, err = bounded.OpenBounded(relative, limit)
	} else {
		s.record.path(s.root, relative, false)
		defer func() { s.record.failed(relative, errOut) }()
		name := filepath.Join(s.root, filepath.FromSlash(relative))
		info, statErr := os.Lstat(name)
		if statErr != nil {
			return nil, statErr
		}
		if !info.Mode().IsRegular() {
			return nil, ErrInvalidUnit
		}
		if info.Size() > int64(limit) {
			return nil, ErrWalkLimit
		}
		file, err = os.Open(name)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			bodyOut, errOut = nil, errors.Join(errOut, closeErr)
		}
		// After a successful open, every capture failure is real, even if
		// one joined cause is ErrNotExist. Only an absent optional open may
		// use retain's missing-path exemption.
		if errOut != nil && s.fsys != nil {
			s.mu.Lock()
			if s.fatal == nil {
				s.fatal = errOut
			}
			s.mu.Unlock()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrInvalidUnit
	}
	if info.Size() > int64(limit) {
		return nil, ErrWalkLimit
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if len(body) > limit {
		return nil, errors.Join(ErrWalkLimit, err)
	}
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *Source) Stat(relative string) (infoOut fs.FileInfo, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
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
	s.record.path(s.root, relative, true)
	info, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(relative)))
	s.record.failed(relative, err)
	return info, err
}
func (s *Source) Lstat(relative string) (infoOut fs.FileInfo, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
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
	s.record.path(s.root, relative, false)
	info, err := os.Lstat(filepath.Join(s.root, filepath.FromSlash(relative)))
	s.record.failed(relative, err)
	return info, err
}

// LinkedEntries names the entries directly in directory that are symbolic
// links and whose base names accept admits. A disk walk drops a link silently,
// though a build tool may still compile its target, so a provider that narrows
// by a unit's own files asks here whether it can see all of them (TSP-V0-002).
// The directory is recorded unfollowed, so an added or removed link
// invalidates a graph cache entry. An immutable tree source answers nil: its
// walk already refuses every non-regular entry.
func (s *Source) LinkedEntries(directory string, accept func(name string) bool) (linksOut []string, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
	if s == nil || !fs.ValidPath(directory) {
		return nil, ErrInvalidUnit
	}
	if s.fsys != nil {
		return nil, nil
	}
	s.record.path(s.root, directory, false)
	entries, err := os.ReadDir(filepath.Join(s.root, filepath.FromSlash(directory)))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 && accept(entry.Name()) {
			linksOut = append(linksOut, path.Join(directory, entry.Name()))
		}
	}
	return linksOut, nil
}

// LinkTarget resolves the symbolic link relative, following every link, to the
// repository-relative slash path of the regular file it names (TSP-V0-721). The
// link is recorded followed, so a retargeted link or an edited target
// invalidates a graph cache entry. A target outside the root's real path, one
// that is not a regular file and any resolution failure answer ok false, as does
// an immutable tree source, which has no on-disk links.
func (s *Source) LinkTarget(relative string) (target string, ok bool) {
	if s == nil || s.fsys != nil || s.stopped() != nil || !fs.ValidPath(relative) || relative == "." {
		return "", false
	}
	s.record.path(s.root, relative, true)
	realRoot, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(s.root, filepath.FromSlash(relative)))
	if err != nil {
		return "", false
	}
	if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	inside, err := filepath.Rel(realRoot, resolved)
	if err != nil {
		return "", false
	}
	inside = filepath.ToSlash(inside)
	if inside == "." || inside == ".." || strings.HasPrefix(inside, "../") || !fs.ValidPath(inside) {
		return "", false
	}
	return inside, true
}

// ResolvesToDependency reports whether the existing path relative resolves on disk, following
// every symlink, outside the root's real path or inside a node_modules directory below it: a
// dependency location rather than repository source (TJAA-V0-122). A path that resolves to
// repository source outside every node_modules directory, such as a link into a workspace
// directory, is not one. Each node_modules-and-later component is recorded unfollowed and the
// path itself followed, so a retargeted link invalidates a graph cache entry (AFP-V0-391). An
// immutable tree source has no on-disk resolution and answers false, as does any failure.
func (s *Source) ResolvesToDependency(relative string) bool {
	if s == nil || s.fsys != nil || s.stopped() != nil || !fs.ValidPath(relative) || relative == "." {
		return false
	}
	segments, recording := strings.Split(relative, "/"), false
	for index, segment := range segments {
		recording = recording || segment == "node_modules"
		if recording {
			s.record.path(s.root, strings.Join(segments[:index+1], "/"), false)
		}
	}
	s.record.path(s.root, relative, true)
	realRoot, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		s.record.refuse("unresolvable root")
		return false
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(s.root, filepath.FromSlash(relative)))
	if err != nil {
		s.record.failed(relative, err)
		return false
	}
	inside, err := filepath.Rel(realRoot, resolved)
	if err != nil {
		return false
	}
	inside = filepath.ToSlash(inside)
	if inside == ".." || strings.HasPrefix(inside, "../") {
		return true
	}
	return inside != "." && slices.Contains(strings.Split(path.Dir(inside), "/"), "node_modules")
}

// FilesUnder lists the regular files below the directory relative whose base
// name accept admits, as repository-relative slash paths, skipping hidden and
// SkippedDirectories directories below it as the shared walk does. Unlike Walk
// it stays cacheable: like the shared walk, it stamps the directory itself
// (an absent one is a valid stamp and lists nothing) and every directory it
// lists, so a graph cache entry pins the listing it saw (NSA-V0-800). An
// accepted entry that is not a regular file refuses ErrWalkUnrepresentable.
func (s *Source) FilesUnder(relative string, accept func(string) bool) (filesOut []string, errOut error) {
	if err := s.stopped(); err != nil {
		return nil, err
	}
	defer func() { s.retain(errOut) }()
	if s == nil || !fs.ValidPath(relative) || relative == "." {
		return nil, ErrInvalidUnit
	}
	var fsys fs.FS
	if s.fsys != nil {
		fsys = s.fsys
	} else {
		s.record.path(s.root, relative, false)
		info, err := os.Lstat(filepath.Join(s.root, filepath.FromSlash(relative)))
		if errors.Is(err, fs.ErrNotExist) || err == nil && !info.IsDir() {
			return nil, nil
		}
		if err != nil {
			s.record.failed(relative, err)
			return nil, err
		}
		fsys = os.DirFS(s.root)
	}
	if info, err := fs.Stat(fsys, relative); errors.Is(err, fs.ErrNotExist) || err == nil && !info.IsDir() {
		return nil, nil
	}
	entries := 0
	var files []string
	err := fs.WalkDir(fsys, relative, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %s", ErrWalkUnreadable, current)
		}
		if cancelled := s.cancelled(); cancelled != nil {
			return cancelled
		}
		if entries++; entries > MaxWalkEntries {
			return ErrWalkLimit
		}
		if entry.IsDir() {
			if current != relative && (strings.HasPrefix(entry.Name(), ".") || SkippedDirectories[entry.Name()]) {
				return fs.SkipDir
			}
			if s.fsys == nil {
				s.record.path(s.root, current, false)
			}
			return nil
		}
		if !accept(entry.Name()) {
			return nil
		}
		if !entry.Type().IsRegular() || !ValidRelativePath(current) {
			return fmt.Errorf("%w: %q", ErrWalkUnrepresentable, current)
		}
		files = append(files, current)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Walk visits every entry below the directory relative, hidden and skipped
// directories included, with fs.WalkDir semantics. On a disk source a walk of
// a directory that exists makes the whole graph uncacheable: BuildCached then
// reports NotStored "walked subtree <relative>" and every later run rebuilds.
// A plugin that only lists files should use FilesUnder, which stays cacheable.
func (s *Source) Walk(relative string, visit fs.WalkDirFunc) (errOut error) {
	if err := s.stopped(); err != nil {
		return err
	}
	defer func() { s.retain(errOut) }()
	if s == nil || !fs.ValidPath(relative) {
		return ErrInvalidUnit
	}
	if s.fsys != nil {
		entries := 0
		return fs.WalkDir(s.fsys, relative, func(current string, entry fs.DirEntry, err error) error {
			s.retain(err)
			if fatal := s.Err(); fatal != nil {
				return fatal
			}
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
	s.record.subtree(s.root, relative)
	return filepath.WalkDir(filepath.Join(s.root, filepath.FromSlash(relative)), func(current string, entry fs.DirEntry, err error) error {
		if cancelled := s.cancelled(); cancelled != nil {
			return cancelled
		}
		return visit(current, entry, err)
	})
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
