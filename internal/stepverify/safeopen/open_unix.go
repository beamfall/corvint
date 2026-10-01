//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/open_unix.go at 25971bda1ca1664d8a546d751cb2bb9bbd454daf.
// Kept private to the Core observer to preserve decision 0397 component separation.
// Package safeopen provides the narrow no-follow opening boundary for local stores.
// Every directory component is opened atomically with O_DIRECTORY|O_NOFOLLOW.
package safeopen

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const Supported = true

// beforeOpen is only used by deterministic path-replacement tests.
var beforeOpen func(string)
var openBridge = os.OpenRoot

// control keeps the descriptor alive, including against concurrent Close, for fn.
func control(f *os.File, fn func(uintptr) error) error {
	c, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var callErr error
	err = c.Control(func(fd uintptr) { callErr = fn(fd) })
	if err != nil {
		return err
	}
	return callErr
}

func child(parent *os.File, name string, flags int, perm os.FileMode) (*os.File, error) {
	if beforeOpen != nil {
		beforeOpen(name)
	}
	var fd int
	err := control(parent, func(p uintptr) error {
		var err error
		for {
			fd, err = openat(int(p), name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, uint32(perm))
			if err != syscall.EINTR {
				return err
			}
		}
	})
	if err != nil {
		return nil, &os.PathError{Op: "openat", Path: name, Err: err}
	}
	return os.NewFile(uintptr(fd), name), nil
}

func descend(parent *os.File, rel string, flags int, perm os.FileMode) (*os.File, error) {
	if rel != "." && (!filepath.IsLocal(rel) || filepath.Clean(rel) != rel || strings.Contains(rel, "\\")) {
		return nil, fmt.Errorf("unclean relative path %q", rel)
	}
	parts := strings.Split(rel, "/")
	cur, err := child(parent, ".", syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range parts[:len(parts)-1] {
		next, err := child(cur, part, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
		cur.Close()
		if err != nil {
			return nil, err
		}
		cur = next
	}
	defer cur.Close()
	return child(cur, parts[len(parts)-1], flags, perm)
}

// Root pins the absolute directory, refusing symlinks in any component. The
// descriptor-only /dev/fd bridge is used because Go exposes no File-to-Root
// constructor. The source descriptor stays pinned until conversion AND identity
// comparison finish. An absent or non-equivalent bridge fails closed.
func Root(path string) (*Directory, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("unclean absolute directory %q", path)
	}
	anchor, err := os.OpenFile("/", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer anchor.Close()
	rel := strings.TrimPrefix(path, "/")
	if rel == "" {
		rel = "."
	}
	dir, err := descend(anchor, rel, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	raw, err := rootFromFile(dir)
	if err != nil {
		return nil, err
	}
	return &Directory{root: raw}, nil
}

func rootFromFile(dir *os.File) (*os.Root, error) {
	want, err := dir.Stat()
	if err != nil {
		return nil, err
	}
	var root *os.Root
	err = control(dir, func(fd uintptr) error {
		var err error
		root, err = openBridge("/dev/fd/" + strconv.FormatUint(uint64(fd), 10))
		if err != nil {
			return err
		}
		got, err := root.Stat(".")
		if err != nil {
			root.Close()
			root = nil
			return err
		}
		if !got.IsDir() || !os.SameFile(want, got) {
			root.Close()
			root = nil
			return fmt.Errorf("descriptor bridge changed directory identity")
		}
		return nil
	})
	if err != nil && root != nil {
		root.Close()
		root = nil
	}
	return root, err
}

// InRoot opens through pinned directories; the final entry never follows a
// symlink and O_NONBLOCK prevents a replaced FIFO from blocking before Stat.
func InRoot(root *Directory, rel string, directory bool) (*File, error) {
	parent, err := root.root.Open(".")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	flags := syscall.O_RDONLY
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	raw, err := descend(parent, rel, flags, 0)
	if err != nil {
		return nil, err
	}
	return &File{file: raw}, nil
}

func SubRoot(root *Directory, rel string) (*Directory, error) {
	dir, err := InRoot(root, rel, true)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	raw, err := rootFromFile(dir.file)
	if err != nil {
		return nil, err
	}
	return &Directory{root: raw}, nil
}
