//go:build darwin || linux

package groupreap

import (
	"syscall"
	"unsafe"
)

// leaderUnreaped blocks until the process exits and leaves it unreaped
// (waitid WEXITED|WNOWAIT), so its PID -- and with it the process group ID --
// cannot be reused until Wait reaps it.
func leaderUnreaped(processID int) error {
	var info [128]byte
	for {
		_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(processID),
			uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT), 0, 0)
		if errno == 0 {
			return nil
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}
