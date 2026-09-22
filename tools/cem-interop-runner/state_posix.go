//go:build unix

package main

import (
	"errors"
	"os"
	"syscall"
)

// lockOwnerUID reports the file's owning UID via the platform stat struct.
func lockOwnerUID(info os.FileInfo) (int, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(stat.Uid), true
}

// flockExclusive takes a non-blocking exclusive advisory lock on fd. It
// reports locked=false (with a nil error) when the lock is already held, so
// the caller can retry until its own timeout.
func flockExclusive(fd int) (locked bool, err error) {
	err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return false, nil
	}
	return false, err
}

func flockUnlock(fd int) {
	_ = syscall.Flock(fd, syscall.LOCK_UN)
}
