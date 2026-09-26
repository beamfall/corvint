// Package journal audits immutable experimental ledgers. It never writes,
// locks, redoes, authenticates actors or qualifies execution.
package journal

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/safeopen"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Entry carries directory observations only, never a permission grant.
type Entry struct {
	Name string
	Info os.FileInfo
}

// Source supplies bounded bytes and bounded directory metadata. Missing paths
// must return os.ErrNotExist; nil, nil is a present empty file. Every other
// error is preserved as a refusal.
// List must stop at max+one listing chunk, before inspecting excess entries.
// Implementations must forbid symlinks and nonregular files on Read.
type Source interface {
	Read(path string, max int) ([]byte, error)
	List(path string, max int) (Listing, error)
}

// Native addresses private state and the primary worktree's .taskman using
// the archive namespace. Paths are explicit read inputs, not authority.
type Native struct{ StateDir, PrimaryWorktree string }

func (n Native) path(p string) (string, error) {
	if !filepath.IsAbs(n.StateDir) || filepath.Clean(n.StateDir) != n.StateDir || !filepath.IsAbs(n.PrimaryWorktree) || filepath.Clean(n.PrimaryWorktree) != n.PrimaryWorktree {
		return "", wire.Errorf(wire.CodeMalformed, p, "native source requires clean absolute roots")
	}
	if p != "." && (!filepath.IsLocal(p) || filepath.ToSlash(filepath.Clean(p)) != p || strings.Contains(p, "\\")) {
		return "", wire.Errorf(wire.CodeMalformed, p, "unclean source path")
	}
	if p == "intent" {
		return filepath.Join(n.PrimaryWorktree, intent.Dir), nil
	}
	if strings.HasPrefix(p, "intent/") {
		return filepath.Join(n.PrimaryWorktree, intent.Dir, strings.TrimPrefix(p, "intent/")), nil
	}
	return filepath.Join(n.StateDir, p), nil
}

// Listing observes the directory itself from the same handle as Entries.
// Missing DirectoryInfo is a refusal, including for an empty directory.
type Listing struct {
	DirectoryInfo os.FileInfo
	Entries       []Entry
}

func (n Native) Read(p string, max int) (raw []byte, err error) {
	s := newNativeRead(n)
	defer func() { err = joinReadClose(err, s.close()) }()
	return s.Read(p, max)
}
func (n Native) List(p string, max int) (out Listing, err error) {
	s := newNativeRead(n)
	defer func() { err = joinReadClose(err, s.close()) }()
	return s.List(p, max)
}

// nativeRead lives for exactly one audit attempt. It retains every listed
// no-follow parent; file reads never reopen the original absolute parent.
type nativeRead struct {
	native   Native
	roots    map[string]*os.Root
	absent   map[string]bool
	listed   map[string]os.FileInfo
	closeErr error
}

func newNativeRead(n Native) *nativeRead {
	return &nativeRead{n, map[string]*os.Root{}, map[string]bool{}, map[string]os.FileInfo{}, nil}
}

var afterReadNames func(string) // deterministic enumeration/metadata race witness
var closeReadRoot = (*os.Root).Close
var closeReadFile = (*os.File).Close

func (s *nativeRead) close() error {
	err := s.closeErr
	for p, r := range s.roots {
		err = errors.Join(err, closeReadRoot(r))
		delete(s.roots, p)
	}
	return err
}
func moved(p string) error {
	return wire.Errorf(wire.CodeSnapshotMoved, p, "previously observed identity disappeared or changed")
}
func (s *nativeRead) parent(p string) (*os.Root, error) {
	if r := s.roots[p]; r != nil {
		return r, nil
	}
	abs, e := s.native.path(p)
	if e != nil {
		return nil, e
	}
	var root *os.Root
	if p == "." || p == "intent" {
		root, e = safeopen.Root(abs)
	} else {
		parent, e2 := s.parent(filepath.ToSlash(filepath.Dir(p)))
		if e2 != nil {
			return nil, e2
		}
		root, e = safeopen.SubRoot(parent, filepath.Base(p))
	}
	if e != nil {
		if os.IsNotExist(e) && s.listed[p] != nil {
			return nil, moved(p)
		}
		return nil, e
	}
	s.roots[p] = root
	return root, nil
}
func (s *nativeRead) binding(p string, root *os.Root) (err error) {
	abs, e := s.native.path(p)
	if e != nil {
		return e
	}
	named, e := safeopen.Root(abs)
	if os.IsNotExist(e) {
		return moved(p)
	}
	if e != nil {
		return e
	}
	defer func() { err = s.closed(err, closeReadRoot(named)) }()
	a, e := named.Stat(".")
	if e != nil {
		return e
	}
	b, e := root.Stat(".")
	if e != nil {
		return e
	}
	if !os.SameFile(a, b) {
		return moved(p)
	}
	return nil
}
func (s *nativeRead) List(p string, max int) (out Listing, err error) {
	root, e := s.parent(p)
	if os.IsNotExist(e) {
		s.absent[p] = true
		return out, e
	}
	if e != nil {
		return out, e
	}
	if s.absent[p] {
		return out, moved(p)
	}
	if e = s.binding(p, root); e != nil {
		return out, e
	}
	dir, e := safeopen.InRoot(root, ".", os.O_RDONLY, 0, true)
	if e != nil {
		return out, e
	}
	defer func() { err = s.closed(err, closeReadFile(dir)) }()
	info, e := dir.Stat()
	if e != nil {
		return out, e
	}
	if old := s.listed[p]; old != nil && !os.SameFile(old, info) {
		return out, moved(p)
	}
	out.DirectoryInfo = info
	names, e := intent.ReadDirNames(dir, p, p, max)
	if e != nil {
		return out, e
	}
	if afterReadNames != nil {
		afterReadNames(p)
	}
	for _, name := range names {
		info, e := root.Lstat(name)
		if os.IsNotExist(e) {
			return out, moved(p + "/" + name)
		}
		if e != nil {
			return out, e
		}
		out.Entries = append(out.Entries, Entry{name, info})
		child := name
		if p != "." {
			child = p + "/" + name
		}
		s.listed[child] = info
	}
	return out, nil
}
func (s *nativeRead) Read(p string, max int) (raw []byte, err error) {
	if _, e := s.native.path(p); e != nil {
		return nil, e
	}
	root, e := s.parent(filepath.ToSlash(filepath.Dir(p)))
	if e != nil {
		return nil, e
	}
	f, e := safeopen.InRoot(root, filepath.Base(p), os.O_RDONLY, 0, false)
	if os.IsNotExist(e) && s.listed[p] != nil {
		return nil, moved(p)
	}
	if e != nil {
		return nil, e
	}
	defer func() { err = s.closed(err, closeReadFile(f)) }()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, wire.Errorf(wire.CodeUnsupportedFilesystem, p, "not a regular file")
	}
	if old := s.listed[p]; old != nil && !os.SameFile(old, st) {
		return nil, moved(p)
	}
	if max < 0 || st.Size() > int64(max) {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p, "file exceeds bound")
	}
	raw, e = io.ReadAll(io.LimitReader(f, int64(max)+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > max {
		return nil, wire.Errorf(wire.CodeLimitExceeded, p, "consumed bytes exceed bound")
	}
	return raw, nil
}

func joinReadClose(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	return wire.Errorf(wire.CodeUnsupportedFilesystem, "read lifetime", "%v", errors.Join(primary, cleanup))
}
func (s *nativeRead) closed(primary, cleanup error) error {
	s.closeErr = errors.Join(s.closeErr, cleanup)
	return joinReadClose(primary, cleanup)
}
