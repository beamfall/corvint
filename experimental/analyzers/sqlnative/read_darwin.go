//go:build darwin

package sqlnative

import (
	"runtime"
	"syscall"
	"unsafe"
)

type fileIdentity struct {
	dev, ino, mode, nlink uint64
	size                  int64
	mtimeS, mtimeN        int64
	ctimeS, ctimeN        int64
}

func identity(st syscall.Stat_t) fileIdentity {
	return fileIdentity{uint64(st.Dev), st.Ino, uint64(st.Mode), uint64(st.Nlink), st.Size, st.Mtimespec.Sec, st.Mtimespec.Nsec, st.Ctimespec.Sec, st.Ctimespec.Nsec}
}
func regular(st syscall.Stat_t) bool {
	return st.Mode&syscall.S_IFMT == syscall.S_IFREG && st.Nlink == 1
}
func directory(st syscall.Stat_t) bool { return st.Mode&syscall.S_IFMT == syscall.S_IFDIR }

// SYS_openat=463; keep pointer live.
const sysOpenAt = 463

var openAt = openat

func openat(dirfd int, name string, flags int, mode uint32) (int, error) {
	p, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0, err
	}
	fd, _, errno := syscall.Syscall6(sysOpenAt, uintptr(dirfd), uintptr(unsafe.Pointer(p)), uintptr(flags), uintptr(mode), 0, 0)
	runtime.KeepAlive(p)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}
