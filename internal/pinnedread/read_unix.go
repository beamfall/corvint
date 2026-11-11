// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package pinnedread

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// platformRefusal is nil: darwin and linux qualify the no-follow receipt reads.
var platformRefusal error

// OpenFile opens the leaf of a pinned parent no-follow and non-blocking. It
// does not compare the opened file with an earlier observation: a caller that
// needs the leaf to be a regular file of a known identity checks the returned
// handle itself.
func OpenFile(parent *os.Root, name string) (*os.File, error) {
	return parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}

const receiptDirectoryFlags = os.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC

// OpenDirectory pins one directory component of parent: the component must be
// a directory before the no-follow open and the same directory after it.
func OpenDirectory(parent *os.Root, name string) (*os.Root, error) {
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so the component must be a directory before the open and the same one after.
	return pinDirectory(parent.Lstat, func(name string) (*os.File, error) {
		return parent.OpenFile(name, receiptDirectoryFlags, 0)
	}, name)
}

// OpenTopDirectory is OpenDirectory for the directory /name, opened by
// its absolute path, so the filesystem root / is never opened for reading: a
// sandbox such as Landlock cannot grant a read of / (V1-0883).
func OpenTopDirectory(name string) (*os.Root, error) {
	return pinDirectory(os.Lstat, func(name string) (*os.File, error) {
		return os.OpenFile(name, receiptDirectoryFlags, 0)
	}, string(filepath.Separator)+name)
}

func pinDirectory(lstat func(string) (fs.FileInfo, error), open func(string) (*os.File, error), name string) (*os.Root, error) {
	before, err := lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() {
		return nil, errors.New("receipt directory component is not a directory")
	}
	file, err := open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, opened) {
		return nil, errors.New("receipt directory identity changed")
	}
	// Reopen the held descriptor, never the mutable directory pathname.
	return os.OpenRoot("/dev/fd/" + strconv.FormatUint(uint64(file.Fd()), 10))
}
