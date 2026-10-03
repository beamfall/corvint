//go:build darwin || linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/open_unix.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
// Native replay owns this narrow no-follow boundary (decision 0397).
// Every directory component is opened atomically with O_DIRECTORY|O_NOFOLLOW.
package postmergeworkflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// nativeBeforeOpen is only used by deterministic path-replacement tests.
var nativeBeforeOpen func(string)
var nativeOpenBridge = os.OpenRoot

// nativeOpenControl keeps the descriptor alive, including against concurrent Close, for fn.
func nativeOpenControl(f *os.File, fn func(uintptr) error) error {
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

func nativeOpenChild(parent *os.File, name string, flags int, perm os.FileMode) (*os.File, error) {
	if nativeBeforeOpen != nil {
		nativeBeforeOpen(name)
	}
	var fd int
	err := nativeOpenControl(parent, func(p uintptr) error {
		var err error
		for {
			fd, err = nativeOpenAt(int(p), name, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, uint32(perm))
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

func nativeOpenDescend(parent *os.File, rel string, flags int, perm os.FileMode) (*os.File, error) {
	if rel != "." && (!filepath.IsLocal(rel) || filepath.Clean(rel) != rel || strings.Contains(rel, "\\")) {
		return nil, fmt.Errorf("unclean relative path %q", rel)
	}
	parts := strings.Split(rel, "/")
	cur, err := nativeOpenChild(parent, ".", nativeTraversalFlags, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range parts[:len(parts)-1] {
		next, err := nativeOpenChild(cur, part, nativeTraversalFlags, 0)
		cur.Close()
		if err != nil {
			return nil, err
		}
		cur = next
	}
	defer cur.Close()
	return nativeOpenChild(cur, parts[len(parts)-1], flags, perm)
}

// nativeOpenRoot pins the absolute directory, refusing symlinks in any component. The
// descriptor-only /dev/fd bridge is used because Go exposes no nativeOpenFile-to-nativeOpenRoot
// constructor. The source descriptor stays pinned until conversion AND identity
// comparison finish. An absent or non-equivalent bridge fails closed.
func nativeOpenRoot(path string) (*os.Root, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("unclean absolute directory %q", path)
	}
	anchor, err := os.OpenFile("/", nativeTraversalFlags|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer anchor.Close()
	rel := strings.TrimPrefix(path, "/")
	if rel == "" {
		rel = "."
	}
	dir, err := nativeOpenDescend(anchor, rel, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return nativeRootFromFile(dir)
}

func nativeRootFromFile(dir *os.File) (*os.Root, error) {
	want, err := dir.Stat()
	if err != nil {
		return nil, err
	}
	var root *os.Root
	err = nativeOpenControl(dir, func(fd uintptr) error {
		var err error
		root, err = nativeOpenBridge("/dev/fd/" + strconv.FormatUint(uint64(fd), 10))
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

// nativeOpenInRoot opens through pinned directories; the final entry never follows a
// symlink and O_NONBLOCK prevents a replaced FIFO from blocking before Stat.
func nativeOpenInRoot(root *os.Root, rel string, flags int, perm os.FileMode, directory bool) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	return nativeOpenDescend(parent, rel, flags, perm)
}

func nativeOpenSubRoot(root *os.Root, rel string) (*os.Root, error) {
	dir, err := nativeOpenInRoot(root, rel, os.O_RDONLY, 0, true)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return nativeRootFromFile(dir)
}

func nativeOpenFile(path string) (*os.File, error) {
	root, err := nativeOpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return nativeOpenInRoot(root, filepath.Base(path), os.O_RDONLY, 0, false)
}
