//go:build linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/at_linux.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
package postmergeworkflow

import "syscall"

// Ancestors only need a pinned traversal handle. O_PATH avoids requiring
// directory read access (including Landlock READ_DIR) before the final open.
// Linux's O_PATH value is absent from syscall on some supported architectures,
// including amd64. Keep the ABI value here rather than adding a dependency.
const nativeTraversalFlags = 0x200000 | syscall.O_DIRECTORY

func nativeOpenAt(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
