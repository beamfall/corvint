// SPDX-License-Identifier: Apache-2.0

package corpusbridge

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	// stampStatChunk is the number of entries one goroutine describes when a
	// directory's entries are split across stamp slots (DCR-V1-847). Tests
	// vary it.
	stampStatChunk = 1024
	// readStampDir reads the next batch of a directory's entries. Tests wrap it.
	readStampDir = func(f *os.File, n int) ([]os.DirEntry, error) { return f.ReadDir(n) }
)

var errStampUnavailable = errors.New("checkout stamp unavailable")

// stampRacyWindow refuses a stamp holding any entry changed this close to the
// capture, so a later change within the same timestamp granule cannot leave
// the stamp unchanged. Tests shorten it.
var stampRacyWindow = 2 * time.Second

// stampNow and stampTimes are the clock and the lstat time reader the stamp
// uses. Tests replace them to model a coarse or skewed filesystem.
var (
	stampNow   = time.Now
	stampTimes = fileTimes
)

// checkoutStamp returns one digest over the stamps of dirs, in order, or ""
// when any checkout cannot be stamped. Directories are read in parallel; each
// one's records are hashed in path order, so the digest does not depend on
// scheduling. A cancelled ctx stops the traversal at the next directory batch
// and leaves no stamp (V1-3029).
func checkoutStamp(ctx context.Context, dirs []string) string {
	s := &stamper{
		ctx:     ctx,
		horizon: stampNow().Add(-stampRacyWindow).UnixNano(),
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
// cannot be described, changed inside the racy window, or carries
// whole-second times, so a later rewrite within the same timestamp granule
// cannot keep the stamp (DCR-V1-417, DCR-V1-845). The
// file is reached through the artifact reader's own no-follow walk, so a
// symlink in any path component, the leaf included, leaves no stamp and the
// next call reads the file and is refused as a fresh read is.
func artifactFileStamp(root, path string) string {
	info, err := doccorpus.StatCorpusFile(root, path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	s := &stamper{horizon: stampNow().Add(-stampRacyWindow).UnixNano()}
	line, coarse, err := s.line(nil, path, info)
	if err != nil || coarse {
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
// platform cannot describe, one changed inside the racy window, more
// entries than the bound, or entries that all carry whole-second times make
// the stamp unavailable. A large directory's entries are described in
// chunks on free stamp slots and assembled in name order, so the record does
// not depend on scheduling (DCR-V1-847).
func (s *stamper) directory(key, abs, rel string, visit policy) error {
	if s.failed.Load() {
		return errStampUnavailable
	}
	entries, err := s.readDir(abs)
	if err != nil {
		return err
	}
	statDir, err := os.Open(abs)
	if err != nil {
		return errStampUnavailable
	}
	defer statDir.Close()
	kept := entries[:0]
	descend := make([]bool, 0, len(entries))
	for _, e := range entries {
		skip, down, err := visit(rel, e.Name(), e.IsDir())
		if err != nil {
			return err
		}
		if !skip {
			kept = append(kept, e)
			descend = append(descend, down)
		}
	}
	chunks := make([]stampChunk, (len(kept)+stampStatChunk-1)/stampStatChunk)
	var wg sync.WaitGroup
	for c := range chunks {
		part := kept[c*stampStatChunk : min((c+1)*stampStatChunk, len(kept))]
		describe := func() { chunks[c] = s.describeAt(statDir, part) }
		if c == len(chunks)-1 {
			describe()
			continue
		}
		select {
		case s.slots <- struct{}{}:
			wg.Go(func() {
				defer func() { <-s.slots }()
				describe()
			})
		default:
			describe()
		}
	}
	wg.Wait()
	// Child traversal does not use this descriptor. Close it before a
	// synchronous descent can retain one descriptor per directory level.
	statDir.Close()
	h := sha256.New()
	coarse := true
	for i := range chunks {
		c := &chunks[i]
		if c.err != nil {
			return c.err
		}
		h.Write(c.b)
		c.b = nil
		coarse = coarse && c.coarse
	}
	// Whole-second times on every entry mark a coarse-timestamp filesystem.
	// The racy window covers its granule only while the times come from this
	// host's clock; on a share whose server clock runs behind, a same-size
	// edit after the capture can keep the stamp, so none is made (DCR-V1-845).
	if len(kept) > 0 && coarse {
		return errStampUnavailable
	}
	for i, e := range kept {
		if !descend[i] || !chunks[i/stampStatChunk].dirs[i%stampStatChunk] {
			continue
		}
		name := e.Name()
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
	s.records[key+rel] = h.Sum(nil)
	s.mu.Unlock()
	return nil
}

// stampChunk is the record of one run of a directory's entries: their lines
// in order, which of them are directories, whether every one carries
// whole-second times, and the refusal that stopped it.
type stampChunk struct {
	b      []byte
	dirs   []bool
	coarse bool
	err    error
}

// describe lstats entries in order and appends each one's line, checking the
// context once per read batch.
func (s *stamper) describe(entries []os.DirEntry) stampChunk {
	c := stampChunk{dirs: make([]bool, len(entries)), coarse: true}
	// The stamp is private, call-local derived state. Fixed-width integers
	// preserve every stat bit without decimal conversion or repeated growth.
	capacity := 48 * len(entries)
	for _, e := range entries {
		capacity += len(e.Name())
	}
	c.b = make([]byte, 0, capacity)
	for i, e := range entries {
		if i%stampReadBatch == stampReadBatch-1 && s.ctx.Err() != nil {
			c.err = s.ctx.Err()
			return c
		}
		info, err := e.Info()
		if err != nil {
			c.err = errStampUnavailable
			return c
		}
		var whole bool
		if c.b, whole, c.err = s.line(c.b, e.Name(), info); c.err != nil {
			return c
		}
		c.coarse = c.coarse && whole
		c.dirs[i] = info.IsDir()
	}
	return c
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
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// line appends one entry's record and reports whether both its times are
// whole seconds.
func (s *stamper) line(b []byte, name string, info fs.FileInfo) ([]byte, bool, error) {
	ino, ctime, mtime, ok := stampTimes(info)
	if !ok || ctime >= s.horizon || mtime >= s.horizon {
		return b, false, errStampUnavailable
	}
	b = binary.LittleEndian.AppendUint64(b, uint64(len(name)))
	b = append(b, name...)
	for _, v := range []int64{int64(info.Mode()), info.Size(), int64(ino), ctime, mtime} {
		b = binary.LittleEndian.AppendUint64(b, uint64(v))
	}
	whole := ctime%int64(time.Second) == 0 && mtime%int64(time.Second) == 0
	return b, whole, nil
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
	root, _, err := s.line(nil, "", info)
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
	if root, _, err = s.line(root, ".git", info); err != nil {
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
