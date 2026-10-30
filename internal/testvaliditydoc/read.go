package testvaliditydoc

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrInputTooLarge identifies a receipt that exceeds the input byte bound.
var ErrInputTooLarge = errors.New("input exceeds byte bound")

// ReadFile reads a resolved, root-relative receipt without following symlinks.
// Each directory is pinned before descent, as in contextindex's blob reader.
func ReadFile(root *os.Root, name string) ([]byte, error) {
	return ReadFileBounded(root, name, MaxInputBytes)
}

// openFilesystemRoot opens / for a file directly in it; tests observe it.
var openFilesystemRoot = os.OpenRoot

// ReadAbsoluteFileBounded is ReadFileBounded for a clean absolute path. The
// first directory below / is pinned by its absolute path with the same
// no-follow identity check as every later component, so / is opened for
// reading only for a file directly in it (V1-0883).
func ReadAbsoluteFileBounded(name string, limit int64) ([]byte, error) {
	if !filepath.IsAbs(name) || filepath.Clean(name) != name {
		return nil, errors.New("receipt path is not a clean absolute path")
	}
	relative := strings.TrimPrefix(name, string(filepath.Separator))
	top, rest, nested := strings.Cut(relative, string(filepath.Separator))
	var root *os.Root
	var err error
	if nested {
		root, err = openTopDirectory(top)
	} else {
		root, err = openFilesystemRoot(string(filepath.Separator))
		rest = relative
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return ReadFileBounded(root, rest, limit)
}

// ReadFileBounded retains the no-follow receipt reader while allowing an explicit
// corpus-sized bound. Receipt callers keep ReadFile and its original 4 MiB limit.
func ReadFileBounded(root *os.Root, name string, limit int64) ([]byte, error) {
	if limit < 1 || limit > 128<<20 {
		return nil, errors.New("invalid input byte bound")
	}
	var data []byte
	err := walkReceiptParent(root, name, func(parent *os.Root, leaf string, before os.FileInfo) error {
		var err error
		data, err = readRegularBounded(parent, leaf, before, limit)
		return err
	})
	return data, err
}

// StatFile is ReadFileBounded without the read: it pins every directory
// component without following a symlink, as the read does, and returns the
// lstat record of the leaf, which must be a regular file.
// StreamFileBounded is ReadFileBounded without holding the file: the same walk,
// regular-file and identity checks, then read(r) consumes the opened file through r,
// which ends after limit+1 bytes. A file whose opened size exceeds limit is refused
// before read runs. read's error is returned as it is.
func StreamFileBounded(root *os.Root, name string, limit int64, read func(r io.Reader) error) error {
	if limit < 1 || limit > 128<<20 {
		return errors.New("invalid input byte bound")
	}
	return walkReceiptParent(root, name, func(parent *os.Root, leaf string, before os.FileInfo) error {
		if !before.Mode().IsRegular() {
			return errors.New("receipt is not a regular file")
		}
		file, err := openReceiptFile(parent, leaf)
		if err != nil {
			return err
		}
		defer file.Close()
		opened, err := file.Stat()
		if err != nil {
			return err
		}
		if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
			return errors.New("receipt file identity changed")
		}
		if opened.Size() > limit {
			return ErrInputTooLarge
		}
		return read(io.LimitReader(file, limit+1))
	})
}

func StatFile(root *os.Root, name string) (os.FileInfo, error) {
	var info os.FileInfo
	err := walkReceiptParent(root, name, func(_ *os.Root, _ string, before os.FileInfo) error {
		if !before.Mode().IsRegular() {
			return errors.New("receipt is not a regular file")
		}
		info = before
		return nil
	})
	return info, err
}

// walkReceiptParent pins each directory component of the local name without
// following a symlink and calls leaf with the pinned parent, the leaf name and
// its lstat record.
func walkReceiptParent(root *os.Root, name string, leaf func(parent *os.Root, leaf string, before os.FileInfo) error) error {
	if !filepath.IsLocal(name) {
		return errors.New("receipt path is not local")
	}
	parent := root
	for _, component := range strings.Split(filepath.Dir(name), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		next, err := openReceiptDirectory(parent, component)
		if err != nil {
			return err
		}
		defer next.Close()
		parent = next
	}
	base := filepath.Base(name)
	before, err := parent.Lstat(base)
	if err != nil {
		return err
	}
	return leaf(parent, base, before)
}

func readRegular(parent *os.Root, leaf string, before os.FileInfo) ([]byte, error) {
	return readRegularBounded(parent, leaf, before, MaxInputBytes)
}
func readRegularBounded(parent *os.Root, leaf string, before os.FileInfo, limit int64) ([]byte, error) {
	if !before.Mode().IsRegular() {
		return nil, errors.New("receipt is not a regular file")
	}
	file, err := openReceiptFile(parent, leaf)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, errors.New("receipt file identity changed")
	}
	// The opened file's size sizes the buffer, so a large input is read once
	// rather than through repeated growth; a file that grows is still cut at
	// the bound.
	data := make([]byte, 0, min(max(opened.Size(), 0), limit)+1)
	reader := io.LimitReader(file, limit+1)
	for {
		n, err := reader.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(data) == cap(data) {
			data = append(data, 0)[:len(data)]
		}
	}
	if int64(len(data)) > limit {
		return nil, ErrInputTooLarge
	}
	return data, nil
}
