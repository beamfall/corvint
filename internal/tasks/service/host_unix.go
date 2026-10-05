//go:build darwin || linux

package service

import (
	"os"
	"syscall"
)

const platformSupported = true

func fileOwner(fi os.FileInfo) (uint32, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Uid, true
}

func tryLock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }

func fileID(fi os.FileInfo) (uint64, uint64) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return uint64(st.Dev), uint64(st.Ino)
}
