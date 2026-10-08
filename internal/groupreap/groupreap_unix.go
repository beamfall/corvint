//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package groupreap

import (
	"os/exec"
	"syscall"
)

// signalGroup is syscall.Kill; tests observe when the group is signalled.
var signalGroup = syscall.Kill

func setpgid(attributes *syscall.SysProcAttr) bool { return attributes.Setpgid && attributes.Pgid == 0 }

func signalLiveGroup(group int, signal syscall.Signal) error { return syscall.Kill(group, signal) }

// Wait reaps a started Setpgid command and SIGKILLs any process left in its
// group. Where waitid is available the group is signalled while the exited
// leader is still unreaped; elsewhere it is signalled after Wait.
func Wait(command *exec.Cmd) error { return wait(command, nil) }

// WaitPipes reaps a started command with command.Wait, so a descendant that
// still holds its output pipes is reported through WaitDelay rather than
// killed. It releases a recorded group once the leader exits, still unreaped
// where waitid allows (AHI-048); retiring the rest of the group stays with the
// caller.
func WaitPipes(command *exec.Cmd) error {
	processID := command.Process.Pid
	_ = leaderUnreaped(processID)
	liveGroups.release(processID)
	return command.Wait()
}

// wait runs exited after the group signal and before the reap where waitid
// allows, so detached-descendant retirement precedes any output-pipe wait.
func wait(command *exec.Cmd, exited func()) error {
	processID := command.Process.Pid
	if leaderUnreaped(processID) == nil {
		_ = signalGroup(-processID, syscall.SIGKILL)
		liveGroups.release(processID)
		if exited != nil {
			exited()
		}
		return command.Wait()
	}
	liveGroups.release(processID)
	err := command.Wait()
	_ = signalGroup(-processID, syscall.SIGKILL)
	if exited != nil {
		exited()
	}
	return err
}
