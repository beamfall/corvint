//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/at_darwin.go at 25971bda1ca1664d8a546d751cb2bb9bbd454daf.
// Kept private to the Core observer to preserve decision 0397 component separation.
package safeopen

import (
	"syscall"
	"unsafe"
)

// Darwin openat is syscall 463 (XNU syscall ABI). Go 1.27 exposes only
// syscall's private libc openat wrapper. Use the pinned Syscall6 ABI here;
// no linkname into Go internals or third-party dependency is required.
func openat(fd int, name string, flags int, perm uint32) (int, error) {
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
