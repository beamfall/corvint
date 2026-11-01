//go:build linux && amd64

package authority

import (
	"syscall"
	"unsafe"
)

// fstatat is newfstatat(dirfd, name, st, AT_SYMLINK_NOFOLLOW) for a
// NUL-terminated name (V1-2530). The syscall package exports no Fstatat on
// linux/amd64, and golang.org/x/sys is not a dependency.
func fstatat(dirfd int, name []byte, st *syscall.Stat_t) error {
	_, _, e := syscall.Syscall6(syscall.SYS_NEWFSTATAT, uintptr(dirfd), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(st)), atSymlinkNoFollow, 0, 0)
	if e != 0 {
		return e
	}
	return nil
}
