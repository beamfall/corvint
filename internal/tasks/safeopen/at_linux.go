//go:build linux

package safeopen

import "syscall"

// Ancestors only need a pinned traversal handle. O_PATH avoids requiring
// directory read access (including Landlock READ_DIR) before the final open.
const traversalFlags = syscall.O_PATH | syscall.O_DIRECTORY

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
