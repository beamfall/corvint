package gitauth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"time"
)

// RevisionFS exposes verified immutable Git objects as a source filesystem.
// It never consults worktree files or follows symlinks/gitlinks. The caller
// owns the repository/object-session lifetime; reads are serial like Repository.
func (r *Repository) RevisionFS(ctx context.Context, commit string, maxBlobBytes int) (fs.FS, error) {
	if maxBlobBytes < 1 || maxBlobBytes > MaxBlobBytes {
		return nil, fmt.Errorf("invalid immutable source byte bound")
	}
	tree, err := r.CommitTree(ctx, commit)
	if err != nil {
		return nil, err
	}
	return &revisionFS{ctx: ctx, repo: r, commit: commit, root: tree, maxBlobBytes: maxBlobBytes, dirs: map[string][]fs.DirEntry{}, resolved: map[string]TreeEntry{}, sizes: map[string]int64{}}, nil
}

type revisionFS struct {
	ctx          context.Context
	repo         *Repository
	commit, root string
	dirs         map[string][]fs.DirEntry
	resolved     map[string]TreeEntry // child entries of verified listed trees
	sizes        map[string]int64     // verified blob sizes by OID, for Stat
	entries      int
	maxBlobBytes int
}

func (s *revisionFS) entry(name string) (TreeEntry, error) {
	if !fs.ValidPath(name) {
		return TreeEntry{}, fs.ErrInvalid
	}
	if name == "." {
		return TreeEntry{Mode: "040000", Type: "tree", OID: s.root}, nil
	}
	if entry, ok := s.resolved[name]; ok {
		return entry, nil
	}
	// Resolve through the parent's verified listing instead of one Git
	// operation per path: each tree body is self-hashed and linked to the
	// OID its verified parent names, so a path costs one operation per
	// directory on first use and none afterwards (DLT-V0-003).
	parent, err := s.entry(path.Dir(name))
	if err != nil {
		return TreeEntry{}, err
	}
	if parent.Type != "tree" {
		return TreeEntry{}, fs.ErrNotExist
	}
	if _, err := s.ReadDir(path.Dir(name)); err != nil {
		return TreeEntry{}, err
	}
	entry, ok := s.resolved[name]
	if !ok {
		return TreeEntry{}, fs.ErrNotExist
	}
	return entry, nil
}
func (s *revisionFS) Open(name string) (fs.File, error) {
	return s.open(name, s.maxBlobBytes)
}

// OpenBounded applies the caller's smaller admission limit before any blob
// body is allocated, including when this repository has already read the OID.
func (s *revisionFS) OpenBounded(name string, limit int) (fs.File, error) {
	if limit < 1 {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return s.open(name, min(limit, s.maxBlobBytes))
}

func (s *revisionFS) open(name string, limit int) (fs.File, error) {
	entry, err := s.entry(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if entry.Type == "tree" {
		entries, err := s.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &revisionDirectory{info: revisionInfo{name: path.Base(name), mode: fs.ModeDir | 0755}, entries: entries}, nil
	}
	if entry.Type != "blob" || !(entry.Mode == "100644" || entry.Mode == "100755") {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	data, err := s.repo.BlobBytesBounded(s.ctx, entry.OID, limit)
	if err != nil {
		return nil, err
	}
	s.sizes[entry.OID] = int64(len(data))
	mode := fs.FileMode(0644)
	if entry.Mode == "100755" {
		mode = 0755
	}
	return &revisionFile{Reader: bytes.NewReader(data), info: revisionInfo{name: path.Base(name), mode: mode, size: int64(len(data))}}, nil
}
func (s *revisionFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if entries, ok := s.dirs[name]; ok {
		return append([]fs.DirEntry(nil), entries...), nil
	}
	entry, err := s.entry(name)
	if err != nil {
		return nil, err
	}
	if entry.Type != "tree" {
		return nil, fs.ErrInvalid
	}
	trees, err := s.repo.verifiedTrees(s.ctx, []string{entry.OID}, []TreeEntry{entry})
	if err != nil {
		return nil, err
	}
	directory := name
	items, err := treeEntries(trees[0].body, len(entry.OID)/2)
	if err != nil {
		return nil, err
	}
	s.entries += len(items)
	if s.entries > 400_000 {
		return nil, fmt.Errorf("immutable source tree entry bound exceeded")
	}
	out := make([]fs.DirEntry, 0, len(items))
	for name, item := range items {
		mode := fs.ModeIrregular
		switch item.Mode {
		case "100644":
			mode = 0644
		case "040000":
			mode = fs.ModeDir | 0755
		case "100755":
			mode = 0755
		case "120000":
			mode = fs.ModeSymlink | 0777
		case "160000":
			mode = fs.ModeIrregular
		}
		s.resolved[path.Join(directory, name)] = item
		out = append(out, revisionEntry{revisionInfo: revisionInfo{name: name, mode: mode}, source: s, path: path.Join(directory, name)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	s.dirs[name] = out
	return append([]fs.DirEntry(nil), out...), nil
}

// Stat returns immutable mode/size and never follows a non-regular object.
func (s *revisionFS) Stat(name string) (fs.FileInfo, error) {
	entry, err := s.entry(name)
	if err != nil {
		return nil, err
	}
	mode := fs.ModeIrregular
	switch entry.Mode {
	case "040000":
		mode = fs.ModeDir | 0755
	case "120000":
		mode = fs.ModeSymlink | 0777
	case "160000":
		mode = fs.ModeIrregular
	case "100644", "100755":
		if size, ok := s.sizes[entry.OID]; ok {
			mode = 0644
			if entry.Mode == "100755" {
				mode = 0755
			}
			return revisionInfo{name: path.Base(name), mode: mode, size: size}, nil
		}
		f, e := s.Open(name)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		return f.Stat()
	}
	return revisionInfo{name: path.Base(name), mode: mode}, nil
}

type revisionInfo struct {
	name string
	mode fs.FileMode
	size int64
}

func (i revisionInfo) Name() string       { return i.name }
func (i revisionInfo) Size() int64        { return i.size }
func (i revisionInfo) Mode() fs.FileMode  { return i.mode }
func (i revisionInfo) ModTime() time.Time { return time.Time{} }
func (i revisionInfo) IsDir() bool        { return i.mode.IsDir() }
func (i revisionInfo) Sys() any           { return nil }

type revisionEntry struct {
	revisionInfo
	source *revisionFS
	path   string
}

func (i revisionEntry) Type() fs.FileMode          { return i.mode.Type() }
func (i revisionEntry) Info() (fs.FileInfo, error) { return i.source.Stat(i.path) }

type revisionFile struct {
	*bytes.Reader
	info revisionInfo
}

func (f *revisionFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *revisionFile) Close() error               { return nil }

type revisionDirectory struct {
	info    revisionInfo
	entries []fs.DirEntry
	offset  int
}

func (d *revisionDirectory) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *revisionDirectory) Read([]byte) (int, error)   { return 0, fs.ErrInvalid }
func (d *revisionDirectory) Close() error               { return nil }
func (d *revisionDirectory) ReadDir(n int) ([]fs.DirEntry, error) {
	if n <= 0 {
		out := append([]fs.DirEntry(nil), d.entries[d.offset:]...)
		d.offset = len(d.entries)
		return out, nil
	}
	if d.offset == len(d.entries) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.entries) {
		end = len(d.entries)
	}
	out := append([]fs.DirEntry(nil), d.entries[d.offset:end]...)
	d.offset = end
	return out, nil
}
