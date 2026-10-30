//go:build linux

// SPDX-License-Identifier: Apache-2.0
// Derived from internal/tasks/safeopen/at_linux.go at 25971bda1ca1664d8a546d751cb2bb9bbd454daf.
// Kept private to the Core observer to preserve decision 0397 component separation.
package safeopen

import "syscall"

// Ancestors, including the filesystem root /, only need a pinned traversal
// handle. O_PATH avoids requiring directory read access (including Landlock
// READ_DIR) before the final open (V1-0883, as internal/tasks/safeopen).
// Linux's O_PATH value is absent from syscall on some supported architectures,
// including amd64. Keep the ABI value here rather than adding a dependency.
const traversalFlags = 0x200000 | syscall.O_DIRECTORY

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
