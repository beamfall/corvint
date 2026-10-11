//go:build darwin || linux

package upgradechanges

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// Traverse held directory descriptors with openat. O_NOFOLLOW applies to every
// caller-supplied component; os.Root confinement alone permits in-root symlinks.
// The caller owns the returned regular file and reads it within its bound.
func openRefs(path string) (*os.File, error) {
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), "/")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		fd, e := openRefAt(current.Fd(), part, flags)
		if e != nil {
			current.Close()
			return nil, e
		}
		current.Close()
		current = os.NewFile(uintptr(fd), part)
	}
	stat, err := current.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		current.Close()
		return nil, fmt.Errorf("regular file required")
	}
	return current, nil
}
