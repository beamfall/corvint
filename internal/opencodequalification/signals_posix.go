//go:build darwin || linux

package opencodequalification

import "syscall"

func terminateProcess(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
