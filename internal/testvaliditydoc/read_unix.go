//go:build darwin || linux

package testvaliditydoc

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

func openReceiptFile(parent *os.Root, name string) (*os.File, error) {
	return parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}

const receiptDirectoryFlags = os.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC

func openReceiptDirectory(parent *os.Root, name string) (*os.Root, error) {
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so the component must be a directory before the open and the same one after.
	return pinDirectory(parent.Lstat, func(name string) (*os.File, error) {
		return parent.OpenFile(name, receiptDirectoryFlags, 0)
	}, name)
}

// openTopDirectory is openReceiptDirectory for the directory /name, opened by
// its absolute path, so the filesystem root / is never opened for reading: a
// sandbox such as Landlock cannot grant a read of / (V1-0883).
func openTopDirectory(name string) (*os.Root, error) {
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
