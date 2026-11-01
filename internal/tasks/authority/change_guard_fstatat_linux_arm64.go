//go:build linux && arm64

package authority

import (
	"syscall"
	"unsafe"
)

// fstatat is fstatat(dirfd, name, st, AT_SYMLINK_NOFOLLOW) for a
// NUL-terminated name (V1-2530), raw so the name needs no conversion.
func fstatat(dirfd int, name []byte, st *syscall.Stat_t) error {
	_, _, e := syscall.Syscall6(syscall.SYS_FSTATAT, uintptr(dirfd), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(st)), atSymlinkNoFollow, 0, 0)
	if e != 0 {
		return e
	}
	return nil
}
