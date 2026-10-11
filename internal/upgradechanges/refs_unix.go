//go:build darwin || linux

package upgradechanges

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

// Traverse held directory descriptors with openat. O_NOFOLLOW applies to every
// caller-supplied component; os.Root confinement alone permits in-root symlinks.
func readRefs(path string) ([]byte, bool, error) {
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, false, err
	}
	current := os.NewFile(uintptr(fd), "/")
	defer func() { current.Close() }()
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		fd, e := openRefAt(current.Fd(), part, flags)
		if e != nil {
			return nil, false, e
		}
		current.Close()
		current = os.NewFile(uintptr(fd), part)
	}
	stat, err := current.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		return nil, false, fmt.Errorf("regular file required")
	}
	raw, err := io.ReadAll(io.LimitReader(current, maxFileBytes+1))
	if len(raw) > maxFileBytes {
		return raw[:maxFileBytes], true, err
	}
	return raw, false, err
}
