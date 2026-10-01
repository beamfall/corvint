//go:build linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/at_linux.go at 25971bda1ca1664d8a546d751cb2bb9bbd454daf.
// Kept private to the Core observer to preserve decision 0397 component separation.
package safeopen

import "syscall"

func openat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}
