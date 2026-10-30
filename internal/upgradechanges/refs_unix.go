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
// The first component is opened by its absolute path with the same flags, so
// the filesystem root / is never opened for reading: a sandbox such as Landlock
// cannot grant that read (V1-2365, as internal/flowdocs for V1-0883).
// The caller owns the returned regular file and reads it within its bound.
func openRefs(path string) (*os.File, error) {
	var current *os.File
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= syscall.O_DIRECTORY
		}
		var fd int
		var e error
		if current == nil {
			if part == "" {
				return nil, syscall.ENOENT
			}
			fd, e = openRefsTop("/"+part, flags, 0)
		} else {
			fd, e = openRefAt(current.Fd(), part, flags)
			current.Close()
		}
		if e != nil {
			return nil, e
		}
		current = os.NewFile(uintptr(fd), part)
	}
	stat, err := current.Stat()
	if err != nil || !stat.Mode().IsRegular() {
		current.Close()
		return nil, fmt.Errorf("regular file required")
	}
	return current, nil
}

// openRefsTop is syscall.Open; a test observes the absolute names it opens.
var openRefsTop = syscall.Open
