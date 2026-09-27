//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package groupreap

import (
	"os/exec"
	"syscall"
)

// signalGroup is syscall.Kill; tests observe when the group is signalled.
var signalGroup = syscall.Kill

// Wait reaps a started Setpgid command and SIGKILLs any process left in its
// group. Where waitid is available the group is signalled while the exited
// leader is still unreaped; elsewhere it is signalled after Wait.
func Wait(command *exec.Cmd) error {
	processID := command.Process.Pid
	if leaderUnreaped(processID) == nil {
		_ = signalGroup(-processID, syscall.SIGKILL)
		return command.Wait()
	}
	err := command.Wait()
	_ = signalGroup(-processID, syscall.SIGKILL)
	return err
}
