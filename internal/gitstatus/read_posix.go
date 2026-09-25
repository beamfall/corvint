//go:build darwin || linux

package gitstatus

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

var (
	errUnreadable = unsupported(classMetadataUnreadable, "cannot be opened")
	errReplaced   = unsupported(classMetadataDrift, "changed while it was read")
)

type pinnedDirectory struct {
	root *os.Root
	info os.FileInfo
}

// Handles are private to one status operation and shared only by its bounded
// metadata reads. Each ancestor is opened without following symlinks. Live path
// identities are checked at both status brackets, so a replaced parent cannot
// hide a configuration change behind a still-open directory handle.
//
// An ancestor that can be searched but not read (mode 0711, EAF-V0-012) has no
// handle: it is pinned by its Lstat identity alone, and its child is opened by
// absolute path. The same bracket checks then refuse a replaced ancestor or a
// child that no longer resolves to the handle that was read.
type metadataReader struct {
	directories map[string]pinnedDirectory
}

func (reader *metadataReader) Close() {
	for _, directory := range reader.directories {
		if directory.root != nil {
			directory.root.Close()
		}
	}
}

func (reader *metadataReader) directory(path string) (*os.Root, error) {
	held, err := reader.pin(path)
	if err != nil {
		return nil, err
	}
	if held.root == nil {
		return nil, errUnreadable
	}
	return held.root, nil
}

func (reader *metadataReader) pin(path string) (pinnedDirectory, error) {
	if held, ok := reader.directories[path]; ok {
		return held, nil
	}
	if reader.directories == nil {
		reader.directories = make(map[string]pinnedDirectory)
	}
	var parent pinnedDirectory
	if path != string(filepath.Separator) {
		var err error
		if parent, err = reader.pin(filepath.Dir(path)); err != nil {
			return pinnedDirectory{}, err
		}
	}
	file, err := openDirectory(parent, path)
	if os.IsPermission(err) {
		return reader.pinSearchOnly(parent, path)
	}
	if err != nil {
		return pinnedDirectory{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return pinnedDirectory{}, err
	}
	root, err := os.OpenRoot("/dev/fd/" + strconv.FormatUint(uint64(file.Fd()), 10))
	if err != nil {
		return pinnedDirectory{}, err
	}
	held := pinnedDirectory{root: root, info: info}
	reader.directories[path] = held
	return held, nil
}

func openDirectory(parent pinnedDirectory, path string) (*os.File, error) {
	const flags = os.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	if path == string(filepath.Separator) {
		return os.Open(path)
	}
	if parent.root == nil {
		return os.OpenFile(path, flags, 0)
	}
	return parent.root.OpenFile(filepath.Base(path), flags, 0)
}

// pinSearchOnly records a directory that cannot be read, so cannot be held
// open, by the identity its path names now, looked up through the parent
// handle when there is one. It must be a real directory: a symlink is refused
// exactly as the no-follow open would refuse it.
func (reader *metadataReader) pinSearchOnly(parent pinnedDirectory, path string) (pinnedDirectory, error) {
	lstat := os.Lstat
	name := path
	if parent.root != nil {
		lstat = parent.root.Lstat
		name = filepath.Base(path)
	}
	info, err := lstat(name)
	if err != nil {
		return pinnedDirectory{}, err
	}
	if !info.IsDir() {
		return pinnedDirectory{}, unsupported(classMetadataUnreadable, irregular(info.Mode()))
	}
	held := pinnedDirectory{info: info}
	reader.directories[path] = held
	return held, nil
}

func (reader *metadataReader) unchangedDirectories() error {
	for path, held := range reader.directories {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || !os.SameFile(info, held.info) {
			return unsupported(classMetadataDrift, "a metadata directory was replaced during observation")
		}
	}
	return nil
}

func irregular(mode os.FileMode) string {
	switch mode.Type() {
	case os.ModeSymlink:
		return "is a symlink"
	case os.ModeDir:
		return "is a directory"
	case os.ModeNamedPipe:
		return "is a FIFO"
	}
	return "is not a regular file"
}

// Pin every path component and never follow metadata symlinks or block on FIFO.
func readRegular(path string, limit int) ([]byte, bool, error) {
	reader := metadataReader{}
	defer reader.Close()
	data, present, _, err := reader.readRegular(path, limit)
	return data, present, err
}

func (reader *metadataReader) readRegular(path string, limit int) ([]byte, bool, time.Time, error) {
	if !filepath.IsAbs(path) {
		return nil, false, time.Time{}, unsupported(classMetadataUnreadable, "is not named by an absolute path")
	}
	parent, err := reader.directory(filepath.Dir(path))
	if os.IsNotExist(err) {
		return nil, false, time.Time{}, nil
	}
	if err != nil {
		return nil, false, time.Time{}, err
	}
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so the leaf must be regular before the open and the same file after it.
	leaf, err := parent.Lstat(filepath.Base(path))
	if os.IsNotExist(err) {
		return nil, false, time.Time{}, nil
	}
	if err != nil {
		return nil, false, time.Time{}, errUnreadable
	}
	if !leaf.Mode().IsRegular() {
		return nil, false, time.Time{}, unsupported(classMetadataUnreadable, irregular(leaf.Mode()))
	}
	file, err := parent.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, false, time.Time{}, errUnreadable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(leaf, info) {
		return nil, false, time.Time{}, errReplaced
	}
	if info.Size() > int64(limit) {
		return nil, false, time.Time{}, errTooLarge
	}
	// Stat gives an exact allocation bound. An extra byte detects growth; a
	// short read detects shrinkage, and both refuse the changing snapshot.
	data := make([]byte, int(info.Size())+1)
	n, err := io.ReadFull(file, data)
	if n != len(data)-1 || (err != io.EOF && err != io.ErrUnexpectedEOF) {
		return nil, false, time.Time{}, errReplaced
	}
	return data[:n], true, info.ModTime(), nil
}
