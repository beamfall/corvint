//go:build linux

package safeopen

import "syscall"

// Ancestors only need a pinned traversal handle. O_PATH avoids requiring
// directory read access (including Landlock READ_DIR) before the final open.
// Linux's O_PATH value is absent from syscall on some supported architectures,
// including amd64. Keep the ABI value here rather than adding a dependency.
const traversalFlags = 0x200000 | syscall.O_DIRECTORY

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
