//go:build darwin || linux

package observations

import (
	"os"
	"syscall"
)

func lockObservationFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

func unlockObservationFile(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
