//go:build darwin || linux

package changee2e

import (
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

func configureServe(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killServe(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// waitServeExit returns once the serve leader has exited, leaving it unreaped (WNOWAIT) so its
// process group ID stays reserved until stop has signalled the group, as internal/procgroup does.
// A stopped or continued record (golang/go#19314) is polled past.
func waitServeExit(cmd *exec.Cmd) {
	var info [128]byte
	for {
		_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, 1, uintptr(cmd.Process.Pid), uintptr(unsafe.Pointer(&info[0])),
			uintptr(syscall.WEXITED|syscall.WNOWAIT), 0, 0)
		if errno == syscall.EINTR {
			continue
		}
		if code := *(*int32)(unsafe.Pointer(&info[8])); errno == 0 && code >= 4 && code <= 6 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		return
	}
}
