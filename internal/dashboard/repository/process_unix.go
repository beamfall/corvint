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

func (containment *childContainment) terminate() {
	if containment.pid > 0 {
		_ = syscall.Kill(-containment.pid, syscall.SIGTERM)
	}
}

func (containment *childContainment) force() {
	if containment.pid > 0 {
		_ = syscall.Kill(-containment.pid, syscall.SIGKILL)
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
