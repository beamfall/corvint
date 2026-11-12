// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package runevidence

import (
	"errors"
	"os"
	"syscall"
)

// noFollow refuses to open a final symlink, so a linked log or lock file cannot carry a write
// outside the store.
const noFollow = syscall.O_NOFOLLOW

// openLock opens the lock file, creating it 0600 and never following a final symlink.
func openLock(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|noFollow, 0o600)
}

// tryLock takes an exclusive advisory lock without waiting. held is false, with no error, when
// another process holds it. The kernel drops the lock if the process dies, so no stale lock
// needs recovery (RVD-V1-002).
func tryLock(file *os.File) (held bool, err error) {
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlockFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
