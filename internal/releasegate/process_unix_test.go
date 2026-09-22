//go:build !windows

package releasegate

import "syscall"

func killProcess(pid int)      { _ = syscall.Kill(pid, syscall.SIGKILL) }
func processGone(pid int) bool { return syscall.Kill(pid, 0) == syscall.ESRCH }
