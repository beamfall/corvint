package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// readBoundedFileUnderRoot reads a repository-relative path through a
// descriptor rooted at root (os.OpenRoot), so a resolved location can never
// land outside root the way a lexical filepath.Join could when a path
// component is a symlink. os.Root still follows a symlink that stays within
// root, but FPK-V0-012 and FPK-V0-031 refuse a symlinked component even
// then, so refuseSymlinkComponents runs first and rejects any existing
// component — including the final one — that is itself a symlink.
func readBoundedFileUnderRoot(root, relativePath string, bound int) ([]byte, error) {
	relative := filepath.FromSlash(relativePath)
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer opened.Close()
	if err := refuseSymlinkComponents(opened, relative); err != nil {
		return nil, err
	}
	before, err := opened.Lstat(relative)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("file is not a regular file")
	}
	file, err := opened.OpenFile(relative, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reopened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !reopened.Mode().IsRegular() || !os.SameFile(before, reopened) {
		return nil, errors.New("file identity changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bound {
		return nil, errors.New("map exceeds bound")
	}
	return data, nil
}

// refuseSymlinkComponents refuses relative when any existing path component
// under root — including the final one — is itself a symlink.
func refuseSymlinkComponents(root *os.Root, relative string) error {
	segments := strings.Split(filepath.ToSlash(relative), "/")
	prefix := ""
	for _, segment := range segments {
		if prefix == "" {
			prefix = segment
		} else {
			prefix = prefix + "/" + segment
		}
		info, err := root.Lstat(filepath.FromSlash(prefix))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("refusing path with symlinked component: %q", prefix)
		}
	}
	return nil
}

func openBoundedRegularFile(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("file is not a regular file")
	}
	file, err := openBoundedFile(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, errors.New("file identity changed")
	}
	return file, nil
}
