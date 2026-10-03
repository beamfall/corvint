//go:build darwin

// SPDX-License-Identifier: AGPL-3.0-or-later
// Derived from internal/tasks/safeopen/at_darwin.go at
// 29a6db884ed795f7694c316433896d190e1ab508; kept private for decision 0397.
package postmergeworkflow

import (
	"syscall"
	"unsafe"
)

const nativeTraversalFlags = syscall.O_RDONLY | syscall.O_DIRECTORY

// Darwin nativeOpenAt is syscall 463 (XNU syscall ABI). Go 1.27 exposes only
// syscall's private libc nativeOpenAt wrapper. Use the pinned Syscall6 ABI here;
// no linkname into Go internals or third-party dependency is required.
func nativeOpenAt(fd int, name string, flags int, perm uint32) (int, error) {
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
