//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func configureProcess(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	c.WaitDelay = 250 * time.Millisecond
}

// processGroupState observes ONLY the original process group. A descendant
// that leaves the group (setsid) is adopted by the subreaper or init and is
// outside this observation; the state deliberately claims group emptiness,
// never full-descendant emptiness.
func processGroupState(p *os.Process) (string, error) {
	if p == nil {
		return "NOT_OBSERVED", nil
	}
	for deadline := time.Now().Add(250 * time.Millisecond); ; time.Sleep(10 * time.Millisecond) {
		if err := reapExecutionChildren(); err != nil {
			return "NOT_EMPTY_OR_NOT_OBSERVED", err
		}
		err := syscall.Kill(-p.Pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return "GROUP_EMPTY_OBSERVED", nil
		}
		if err != nil {
			return "NOT_EMPTY_OR_NOT_OBSERVED", err
		}
		if time.Now().After(deadline) {
			return "NOT_EMPTY_OR_NOT_OBSERVED", nil
		}
	}
}

func cleanupProcessGroup(p *os.Process) (string, error) {
	if p == nil {
		return "NOT_OBSERVED", nil
	}
	if err := syscall.Kill(-p.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return "NOT_EMPTY_OR_NOT_OBSERVED", err
	}
	for deadline := time.Now().Add(500 * time.Millisecond); ; time.Sleep(10 * time.Millisecond) {
		if err := reapExecutionChildren(); err != nil {
			return "NOT_EMPTY_OR_NOT_OBSERVED", err
		}
		err := syscall.Kill(-p.Pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return "CLEANED_AFTER_RESIDUE", nil
		}
		if err != nil {
			return "NOT_EMPTY_OR_NOT_OBSERVED", err
		}
		if time.Now().After(deadline) {
			return "NOT_EMPTY_OR_NOT_OBSERVED", errors.New("process group did not exit")
		}
	}
}
