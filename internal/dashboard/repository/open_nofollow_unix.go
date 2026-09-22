//go:build darwin || linux

package repository

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
)

const repositoryExecutionSupported = true

// openNoFollowFile opens name without following a final symlink and without
// blocking: a FIFO or device swapped in after the caller's Lstat reaches the
// caller's fstat regular-file check instead of hanging the open.
func openNoFollowFile(parent *os.Root, name string) (*os.File, bool) {
	if parent == nil {
		return nil, false
	}
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	return file, err == nil
}

func openNoFollowDirectory(parent *os.Root, name string) (*os.File, *os.Root, bool) {
	if parent == nil {
		return nil, nil, false
	}
	file, err := parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, false
	}
	prefix := "/dev/fd"
	if runtime.GOOS == "linux" {
		prefix = "/proc/self/fd"
	}
	root, err := os.OpenRoot(fmt.Sprintf("%s/%d", prefix, file.Fd()))
	if err != nil {
		_ = file.Close()
		return nil, nil, false
	}
	return file, root, true
}
