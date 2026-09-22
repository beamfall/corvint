//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package gorunner

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

var signalProcessGroup = syscall.Kill

const (
	processGroupPollInterval     = 5 * time.Millisecond
	processGroupQuiescenceWindow = 10 * time.Millisecond
)

var errProcessGroupNotQuiescent = errors.New("process group did not quiesce before shutdown deadline")

func platformContainment() (Containment, bool) {
	return ContainmentProcessGroupBestEffort, true
}

func configureCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateCommand(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	err := signalProcessGroup(-command.Process.Pid, syscall.SIGKILL)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return errors.Join(err, command.Process.Kill())
}

func cleanupCommand(command *exec.Cmd, deadline time.Time) error {
	if command.Process == nil {
		return nil
	}
	processGroup := -command.Process.Pid
	err := signalProcessGroup(processGroup, syscall.SIGKILL)
	permissionErr := error(nil)
	goneSince := time.Time{}
	if errors.Is(err, syscall.ESRCH) {
		goneSince = time.Now()
	} else if errors.Is(err, syscall.EPERM) {
		permissionErr = err
	} else if err != nil {
		return err
	}
	for {
		err = signalProcessGroup(processGroup, 0)
		if errors.Is(err, syscall.ESRCH) {
			if goneSince.IsZero() {
				goneSince = time.Now()
			}
			if time.Since(goneSince) >= processGroupQuiescenceWindow {
				return nil
			}
		} else if errors.Is(err, syscall.EPERM) {
			goneSince = time.Time{}
			permissionErr = err
		} else if err != nil {
			return err
		} else {
			goneSince = time.Time{}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.Join(errProcessGroupNotQuiescent, permissionErr)
		}
		wait := min(processGroupPollInterval, remaining)
		timer := time.NewTimer(wait)
		<-timer.C
	}
}
