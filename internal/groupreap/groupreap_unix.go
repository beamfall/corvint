//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package groupreap

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// signalGroup is syscall.Kill; tests observe when the group is signalled.
var signalGroup = syscall.Kill

// observeExit is leaderUnreaped; tests inject an exit-observation failure.
var observeExit = leaderUnreaped

func setpgid(attributes *syscall.SysProcAttr) bool { return attributes.Setpgid && attributes.Pgid == 0 }

func signalLiveGroup(group int, signal syscall.Signal) error { return syscall.Kill(group, signal) }

// Wait reaps a started Setpgid command and SIGKILLs any process left in its
// group. Where waitid is available the group is signalled while the exited
// leader is still unreaped; elsewhere it is signalled after Wait.
func Wait(command *exec.Cmd) error { return wait(command, nil) }

// wait runs exited after the group signal and before the reap where waitid
// allows, so detached-descendant retirement precedes any output-pipe wait. A
// group recorded by StartLive is released after that signal and before the
// reap, so KillLive never names a reaped leader's group (AHI-048).
//
// A failed exit observation on a waitid platform (for example ECHILD because
// another reaper collected the leader) sends no group signal: the leader may
// already be reaped, so its numeric group ID confers no kill authority
// (V1-0652). Only platforms without waitid keep the legacy post-reap signal.
func wait(command *exec.Cmd, exited func()) error {
	processID := command.Process.Pid
	observed := observeExit(processID)
	if observed == nil {
		_ = signalGroup(-processID, syscall.SIGKILL)
		liveGroups.release(processID)
		if exited != nil {
			exited()
		}
		return command.Wait()
	}
	liveGroups.release(processID)
	err := command.Wait()
	if errors.Is(observed, errors.ErrUnsupported) {
		_ = signalGroup(-processID, syscall.SIGKILL)
	}
	if exited != nil {
		exited()
	}
	return err
}

// Contain places command in its own process group and makes its context
// cancellation use Stop. Started through StartLive or Drain, the group is also
// recorded for KillLive until its pre-reap release (AHI-048).
func Contain(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.Cancel = func() error { return Stop(command) }
}

// Stop SIGKILLs a started command early. Where waitid lets Wait and Drain
// signal the group while the exited leader is unreaped, Stop signals only the
// leader through os.Process, which refuses a leader it has reaped, so no
// numeric group signal can follow the reap; the leader's exit then triggers
// the pre-reap group sweep. Without waitid the group is signalled directly.
func Stop(command *exec.Cmd) error {
	if command.Process == nil {
		return os.ErrProcessDone
	}
	if waitidAvailable {
		return command.Process.Kill()
	}
	err := signalGroup(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
