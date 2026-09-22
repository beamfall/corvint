//go:build linux

package sqlnative

import "syscall"

type fileIdentity struct {
	dev, ino, mode, nlink uint64
	size                  int64
	mtimeS, mtimeN        int64
	ctimeS, ctimeN        int64
}

func identity(st syscall.Stat_t) fileIdentity {
	return fileIdentity{uint64(st.Dev), st.Ino, uint64(st.Mode), uint64(st.Nlink), st.Size, st.Mtim.Sec, st.Mtim.Nsec, st.Ctim.Sec, st.Ctim.Nsec}
}

func regular(st syscall.Stat_t) bool {
	return st.Mode&syscall.S_IFMT == syscall.S_IFREG && st.Nlink == 1
}
func directory(st syscall.Stat_t) bool { return st.Mode&syscall.S_IFMT == syscall.S_IFDIR }

var openAt = syscall.Openat
