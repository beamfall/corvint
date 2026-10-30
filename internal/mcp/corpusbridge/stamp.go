// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// A checkout stamp is a digest of the lstat data of every worktree entry and
// of the Git control state that the reuse key and pinned-revision checks read
// (DCP-V1-314). It runs no Git process. While a later stamp equals the one
// captured before an observation or a pin verification, nothing those read
// has changed, so the bridge reuses their result instead of repeating them
// (DCP-V1-315, DCP-V1-316). Any state the stamp cannot prove unchanged makes
// it unavailable, and an unavailable stamp never matches: the bridge then
// observes and verifies exactly as before.
const (
	// stampConfigBound bounds the repository config the stamp reads.
	stampConfigBound = 1 << 20
)

var (
	// stampEntryBound bounds the entries one checkout stamp lstats; a larger
	// checkout keeps the Git observation path. It is enforced on each batched
	// directory read, so an oversized directory is never read whole (V1-3029).
	// Tests lower it.
	stampEntryBound int64 = 250_000
	// stampReadBatch is the number of entries one directory read returns.
	stampReadBatch = 1024
	// readStampDir reads the next batch of a directory's entries. Tests wrap it.
	readStampDir = func(f *os.File, n int) ([]os.DirEntry, error) { return f.ReadDir(n) }
)

var errStampUnavailable = errors.New("checkout stamp unavailable")

// stampRacyWindow refuses a stamp holding any entry changed this close to the
// capture, so a later change within the same timestamp granule cannot leave
// the stamp unchanged. Tests shorten it.
var stampRacyWindow = 2 * time.Second

// checkoutStamp returns one digest over the stamps of dirs, in order, or ""
// when any checkout cannot be stamped. Directories are read in parallel; each
// one's records are hashed in path order, so the digest does not depend on
// scheduling. A cancelled ctx stops the traversal at the next directory batch
// and leaves no stamp (V1-3029).
func checkoutStamp(ctx context.Context, dirs []string) string {
	s := &stamper{
		ctx:     ctx,
		horizon: time.Now().Add(-stampRacyWindow).UnixNano(),
		slots:   make(chan struct{}, 2*runtime.GOMAXPROCS(0)),
		records: map[string][]byte{},
	}
	for i, dir := range dirs {
		s.wg.Go(func() { s.fail(s.checkout(strconv.Itoa(i)+"\x00"+dir+"\x00", dir)) })
	}
	s.wg.Wait()
	if s.failed.Load() {
		return ""
	}
	keys := make([]string, 0, len(s.records))
	for key := range s.records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		h.Write([]byte(strconv.Itoa(len(key)) + ":" + key + "\n"))
		h.Write(s.records[key])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// artifactFileStamp is the stat record of the configured artifact file, by
// mode, size, inode, change and modification times, or "" when the file
// cannot be described or changed inside the racy window, so a later rewrite
// within the same timestamp granule cannot keep the stamp (DCR-V1-417). The
// file is reached through the artifact reader's own no-follow walk, so a
// symlink in any path component, the leaf included, leaves no stamp and the
// next call reads the file and is refused as a fresh read is.
func artifactFileStamp(root, path string) string {
	info, err := doccorpus.StatCorpusFile(root, path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	s := &stamper{horizon: time.Now().Add(-stampRacyWindow).UnixNano()}
	line, err := s.line(nil, path, info)
	if err != nil {
		return ""
	}
	return string(line)
}

// stampDirs lists the checkouts the reuse key observes and the pins resolve
// in: the primary root, each declared additional repository and the owner of
// every pinned revision, without repeats.
func stampDirs(root string, m doccorpus.Manifest, pins []string) []string {
	dirs, seen := []string{}, map[string]bool{}
	add := func(dir string) {
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	add(root)
	for _, r := range m.AdditionalRepositories {
		add(m.RevisionRoot(root, r.Revision))
	}
	for _, revision := range pins {
		add(m.RevisionRoot(root, revision))
	}
	return dirs
}

type stamper struct {
	ctx     context.Context
	horizon int64
	entries atomic.Int64
	failed  atomic.Bool
	slots   chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	records map[string][]byte
}

func (s *stamper) fail(err error) {
	if err != nil {
		s.failed.Store(true)
	}
}

// spawn reads a directory on another goroutine while a slot is free, and on
// this one otherwise.
func (s *stamper) spawn(read func() error) error {
	select {
	case s.slots <- struct{}{}:
		s.wg.Go(func() {
			defer func() { <-s.slots }()
			s.fail(read())
		})
		return nil
	default:
		return read()
	}
}

// policy decides, for one entry of the directory rel, whether it is left
// out of the stamp, whether a directory is descended into, and whether the
// entry makes the stamp unavailable.
type policy func(rel, name string, dir bool) (skip, descend bool, err error)

// directory reads one directory's entries in name order and records each
// entry's name, mode, size, inode, change and modification times under key,
// then visits each subdirectory the policy descends into. An entry the
// platform cannot describe, one changed inside the racy window, or more
// entries than the bound make the stamp unavailable.
func (s *stamper) directory(key, abs, rel string, visit policy) error {
	if s.failed.Load() {
		return errStampUnavailable
	}
	entries, err := s.readDir(abs)
	if err != nil {
		return err
	}
	var b []byte
	for i, e := range entries {
		if i%stampReadBatch == stampReadBatch-1 && s.ctx.Err() != nil {
			return s.ctx.Err()
		}
		name := e.Name()
		skip, descend, err := visit(rel, name, e.IsDir())
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return errStampUnavailable
		}
		if b, err = s.line(b, name, info); err != nil {
			return err
		}
		if !info.IsDir() || !descend {
			continue
		}
		child := name
		if rel != "" {
			child = rel + "/" + name
		}
		childAbs := filepath.Join(abs, name)
		if err := s.spawn(func() error { return s.directory(key, childAbs, child, visit) }); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.records[key+rel] = b
	s.mu.Unlock()
	return nil
}

// readDir reads the entries of one directory in batches and returns them in
// name order, as os.ReadDir does. The entry bound is checked after each batch
// and the context before each, so neither an oversized directory nor a
// cancelled call reads the rest of it (V1-3029).
func (s *stamper) readDir(abs string) ([]os.DirEntry, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, errStampUnavailable
	}
	defer f.Close()
	var entries []os.DirEntry
	for {
		if err := s.ctx.Err(); err != nil {
			return nil, err
		}
		batch, err := readStampDir(f, stampReadBatch)
		if s.entries.Add(int64(len(batch))) > stampEntryBound {
			return nil, errStampUnavailable
		}
		entries = append(entries, batch...)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errStampUnavailable
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (s *stamper) line(b []byte, name string, info fs.FileInfo) ([]byte, error) {
	ino, ctime, mtime, ok := fileTimes(info)
	if !ok || ctime >= s.horizon || mtime >= s.horizon {
		return b, errStampUnavailable
	}
	b = strconv.AppendInt(b, int64(len(name)), 10)
	b = append(b, ':')
	b = append(b, name...)
	for _, v := range []int64{int64(info.Mode()), info.Size(), int64(ino), ctime, mtime} {
		b = append(b, ' ')
		b = strconv.AppendInt(b, v, 10)
	}
	return append(b, '\n'), nil
}

// checkout records the checkout root itself, its repository control state
// and every worktree entry except the repository's own .git. A nested .git
// is a submodule or nested repository whose state the stamp does not cover,
// so it makes the stamp unavailable. Symlinks are recorded, never followed.
func (s *stamper) checkout(prefix, dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return errStampUnavailable
	}
	root, err := s.line(nil, "", info)
	if err != nil {
		return err
	}
	gitdir := filepath.Join(dir, ".git")
	info, err = os.Lstat(gitdir)
	// A gitfile (linked worktree or submodule) points at state outside the
	// checkout; only an ordinary repository directory is stamped.
	if err != nil || !info.IsDir() {
		return errStampUnavailable
	}
	if root, err = s.line(root, ".git", info); err != nil {
		return err
	}
	s.mu.Lock()
	s.records[prefix] = root
	s.mu.Unlock()
	if err := s.gitdir(prefix+"g\x00", gitdir); err != nil {
		return err
	}
	return s.directory(prefix+"w\x00", dir, "", worktreePolicy)
}

func worktreePolicy(rel, name string, dir bool) (bool, bool, error) {
	if name == ".git" {
		if rel == "" {
			return true, false, nil
		}
		return false, false, errStampUnavailable
	}
	return false, dir, nil
}

// gitdir records the repository control state: HEAD, the index, refs,
// packed refs, config, info files and every object file. Each loose object
// and pack is recorded by its own entry, so an object deleted, repacked, or
// truncated or rewritten in place changes the stamp. An
// alternate object store, a common-dir link, or a config include or worktree
// setting points outside the stamped state and makes it unavailable.
func (s *stamper) gitdir(key, gitdir string) error {
	for _, outside := range []string{"commondir", filepath.Join("objects", "info", "alternates")} {
		if _, err := os.Lstat(filepath.Join(gitdir, outside)); !errors.Is(err, fs.ErrNotExist) {
			return errStampUnavailable
		}
	}
	config, err := readStampConfig(filepath.Join(gitdir, "config"))
	if err != nil {
		return err
	}
	config = bytes.ToLower(config)
	if bytes.Contains(config, []byte("include")) || bytes.Contains(config, []byte("worktree")) {
		return errStampUnavailable
	}
	return s.directory(key, gitdir, "", controlPolicy)
}

// controlPolicy descends everywhere in the repository directory except
// large-file storage, which status and pin checks never read. Loose object
// directories are descended into: an object file modified in place changes
// only its own entry, never its directory's (DCP-V1-316).
func controlPolicy(rel, name string, dir bool) (bool, bool, error) {
	if rel == "" && name == "lfs" {
		return false, false, nil
	}
	return false, dir, nil
}

// readStampConfig reads the repository config through a bounded reader. A
// missing config is empty. A symlink is never followed, and anything other
// than a regular file of at most stampConfigBound bytes (a FIFO, device or
// directory swapped in, or an oversized file) makes the stamp unavailable
// before any blocking read or large allocation.
func readStampConfig(path string) ([]byte, error) {
	f, err := openStampFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errStampUnavailable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > stampConfigBound {
		return nil, errStampUnavailable
	}
	config, err := io.ReadAll(io.LimitReader(f, stampConfigBound+1))
	if err != nil || len(config) > stampConfigBound {
		return nil, errStampUnavailable
	}
	return config, nil
}
