//go:build darwin || linux

package testvaliditydoc

import (
	"errors"
	"os"
	"strconv"
	"syscall"
)

func openReceiptFile(parent *os.Root, name string) (*os.File, error) {
	return parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
}

func openReceiptDirectory(parent *os.Root, name string) (*os.Root, error) {
	// os.Root resolves a symlink that stays inside the root even with O_NOFOLLOW,
	// so the component must be a directory before the open and the same one after.
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() {
		return nil, errors.New("receipt directory component is not a directory")
	}
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
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
