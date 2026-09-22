//go:build darwin || linux

package unplannedread

import (
	"os"
	"syscall"
)

func lockLedgerFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockLedgerFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
