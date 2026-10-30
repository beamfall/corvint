//go:build darwin || linux

package groupreap

import (
	"syscall"
	"time"
	"unsafe"
)

// waitidAvailable reports that leaderUnreaped can observe an exit without
// reaping, so every group signal can precede the reap.
const waitidAvailable = true

// stopPoll paces leaderUnreaped while Darwin reports a stopped leader.
const stopPoll = 10 * time.Millisecond

// leaderUnreaped blocks until the process exits and leaves it unreaped
// (waitid WEXITED|WNOWAIT), so its PID -- and with it the process group ID --
// cannot be reused until Wait reaps it.
//
// Darwin's waitid also returns for a stopped or continued child despite
// WEXITED (golang/go#19314, observed on Darwin 25 as si_code CLD_STOPPED).
// Such a report is not an exit: it is polled past until the child exits, as
// Linux's waitid blocks through it.
func leaderUnreaped(processID int) error {
	for {
		var info [128]byte
		_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(processID),
			uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT), 0, 0)
		if errno == 0 {
			if !stateChangeOnly(info) {
				return nil
			}
			time.Sleep(stopPoll)
			continue
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

// stateChangeOnly reports a waitid record for a trapped, stopped or continued
// child: si_code, after si_signo and si_errno, is CLD_TRAPPED (4),
// CLD_STOPPED (5) or CLD_CONTINUED (6) on both Darwin and Linux.
func stateChangeOnly(info [128]byte) bool {
	code := *(*int32)(unsafe.Pointer(&info[8]))
	return code >= 4 && code <= 6
}
