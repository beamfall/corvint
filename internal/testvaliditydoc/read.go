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

	if !filepath.IsLocal(name) {
		return nil, errors.New("receipt path is not local")
	}
	parent := root
	for _, component := range strings.Split(filepath.Dir(name), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		next, err := openReceiptDirectory(parent, component)
		if err != nil {
			return nil, err
		}
		defer next.Close()
		parent = next
	}
	leaf := filepath.Base(name)
	before, err := parent.Lstat(leaf)
	if err != nil {
		return nil, err
	}
	return readRegularBounded(parent, leaf, before, limit)
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
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrInputTooLarge
	}
	return data, nil
}
