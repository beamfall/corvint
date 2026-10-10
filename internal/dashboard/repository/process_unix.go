//go:build darwin || linux

package repository

import (
	"os"
	"os/exec"
	"syscall"
)

type childContainment struct {
	pid int
}

func prepareContainment(command *exec.Cmd) (*childContainment, bool) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &childContainment{}, true
}

func (containment *childContainment) attach(process *os.Process) bool {
	if process == nil || process.Pid <= 0 {
		return false
	}
	containment.pid = process.Pid
	return true
}

// groupSignal is syscall.Kill; tests check that terminate, force and the
// Darwin signal-0 probe run only while the leader is unreaped (V1-1047,
// PGO-V0-009).
var groupSignal = syscall.Kill

func (containment *childContainment) terminate() {
	if containment.pid > 0 {
		_ = groupSignal(-containment.pid, syscall.SIGTERM)
	}
}

func (containment *childContainment) force() {
	if containment.pid > 0 {
		_ = groupSignal(-containment.pid, syscall.SIGKILL)
	}
}

func (*childContainment) close() {}

// observeLeaderExit is the unreaped exit observation; tests inject a failed
// observation (ECHILD).
var observeLeaderExit = waitProcessExitUnreaped

// leaderObservation tracks the unreaped-exit observation from the waiting
// goroutine. It is used by that one goroutine only.
type leaderObservation struct {
	result    <-chan error
	completed bool
	err       error
}

// pinned reports whether the leader's PID, and so its group ID, still names
// this command: the observation is pending (the leader has not exited, so no
// one can have reaped it) or saw the exit while leaving it unreaped. A failed
// observation (for example ECHILD after another reaper collected the leader)
// confers no kill authority (V1-1047, as groupreap's V1-0652).
func (observation *leaderObservation) pinned() bool {
	if !observation.completed {
		select {
		case observation.err = <-observation.result:
			observation.completed = true
		default:
		}
	}
	return !observation.completed || observation.err == nil
}
