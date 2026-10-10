//go:build darwin || linux

package repository

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
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

// groupSignal is syscall.Kill; tests check that terminate and force run only
// while the leader is unreaped (V1-1047).
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

func (containment *childContainment) quiescent() bool {
	return containment.pid > 0 && !groupExists(containment.pid)
}

func (containment *childContainment) stopAndProve(grace time.Duration) bool {
	if containment.pid <= 0 {
		return false
	}
	containment.terminate()
	until := time.Now().Add(grace)
	for groupExists(containment.pid) && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if groupExists(containment.pid) {
		containment.force()
		// Never return while the group may remain. SIGKILL has no second grace
		// window: this loop is proof of quiescence, not another timeout.
		for groupExists(containment.pid) {
			containment.force()
			time.Sleep(time.Millisecond)
		}
	}
	return true
}

func groupExists(pid int) bool {
	err := syscall.Kill(-pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
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
