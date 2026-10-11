//go:build linux

// SPDX-License-Identifier: AGPL-3.0-or-later
// Adapted from internal/stepverify/safeopen/at_linux.go and
// internal/tasks/authority/fixture_session_linux.go at bc773cd1.
package contextindex

import "syscall"

func snapshotRefreshOpenat(fd int, name string, flags int, perm uint32) (int, error) {
	return syscall.Openat(fd, name, flags, perm)
}

func snapshotRefreshMkdirat(fd int, name string, perm uint32) error {
	return syscall.Mkdirat(fd, name, perm)
}
