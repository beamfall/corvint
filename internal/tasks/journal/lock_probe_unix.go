//go:build darwin || linux

package journal

import (
	"errors"
	"os"
	"syscall"
)

// sharedLockFree takes `flock(LOCK_SH|LOCK_NB)` on f and releases it at once
// (CTS-V0-026). Contention or any other error answers false.
func sharedLockFree(f *os.File) bool {
	raw, err := f.SyscallConn()
	if err != nil {
		return false
	}
	granted := false
	if err := raw.Control(func(fd uintptr) {
		for {
			e := syscall.Flock(int(fd), syscall.LOCK_SH|syscall.LOCK_NB)
			if errors.Is(e, syscall.EINTR) {
				continue
			}
			if e == nil {
				granted = true
				_ = syscall.Flock(int(fd), syscall.LOCK_UN)
			}
			return
		}
	}); err != nil {
		return false
	}
	return granted
}
