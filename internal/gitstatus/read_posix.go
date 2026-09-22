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

type pinnedDirectory struct {
	root *os.Root
	info os.FileInfo
}

// Handles are private to one status operation and shared only by its bounded
// metadata reads. Each ancestor is opened without following symlinks. Live path
// identities are checked at both status brackets, so a replaced parent cannot
// hide a configuration change behind a still-open directory handle.
type metadataReader struct {
	directories map[string]pinnedDirectory
}

func (reader *metadataReader) Close() {
	for _, directory := range reader.directories {
		directory.root.Close()
	}
}

func (reader *metadataReader) directory(path string) (*os.Root, error) {
	if held, ok := reader.directories[path]; ok {
		return held.root, nil
	}
	if reader.directories == nil {
		reader.directories = make(map[string]pinnedDirectory)
	}
	var file *os.File
	var err error
	if path == string(filepath.Separator) {
		file, err = os.Open(path)
	} else {
		parent, parentErr := reader.directory(filepath.Dir(path))
		if parentErr != nil {
			return nil, parentErr
		}
		file, err = parent.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot("/dev/fd/" + strconv.FormatUint(uint64(file.Fd()), 10))
	if err != nil {
		return nil, err
	}
	reader.directories[path] = pinnedDirectory{root: root, info: info}
	return root, nil
}

func (reader *metadataReader) unchangedDirectories() error {
	for path, held := range reader.directories {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || !os.SameFile(info, held.info) {
			return errUnsafe
		}
	}
	return nil
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
		return nil, false, time.Time{}, errUnsafe
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
	if err != nil || !leaf.Mode().IsRegular() {
		return nil, false, time.Time{}, errUnsafe
	}
	file, err := parent.OpenFile(filepath.Base(path), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, false, time.Time{}, errUnsafe
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(leaf, info) || info.Size() > int64(limit) {
		return nil, false, time.Time{}, errUnsafe
	}
	// Stat gives an exact allocation bound. An extra byte detects growth; a
	// short read detects shrinkage, and both refuse the changing snapshot.
	data := make([]byte, int(info.Size())+1)
	n, err := io.ReadFull(file, data)
	if n != len(data)-1 || (err != io.EOF && err != io.ErrUnexpectedEOF) {
		return nil, false, time.Time{}, errUnsafe
	}
	return data[:n], true, info.ModTime(), nil
}
