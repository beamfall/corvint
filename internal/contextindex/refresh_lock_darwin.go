//go:build darwin

// SPDX-License-Identifier: Apache-2.0
// Adapted from internal/stepverify/safeopen/at_darwin.go and
// internal/tasks/authority/fixture_session_darwin.go at bc773cd1.
package contextindex

import (
	"syscall"
	"unsafe"
)

// Go's Darwin syscall package keeps openat/mkdirat private. These are the XNU
// syscall numbers used by the existing repository no-follow walkers.
func snapshotRefreshOpenat(fd int, name string, flags int, perm uint32) (int, error) {
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	r, _, errno := syscall.Syscall6(463, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(flags), uintptr(perm), 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(r), nil
}

func snapshotRefreshMkdirat(fd int, name string, perm uint32) error {
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall(475, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(perm))
	if errno != 0 {
		return errno
	}
	return nil
}
