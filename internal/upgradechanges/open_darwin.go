package upgradechanges

import (
	"syscall"
	"unsafe"
)

func openRefAt(parent uintptr, name string, flags int) (int, error) {
	pointer, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	// Darwin SDK openat number; the frozen syscall package omits this constant.
	// Same descriptor-relative primitive as internal/authoritystore/files_darwin.go.
	fd, _, errno := syscall.Syscall6(463, parent, uintptr(unsafe.Pointer(pointer)), uintptr(flags), 0, 0, 0)
	if errno != 0 {
		return -1, errno
	}
	return int(fd), nil
}
