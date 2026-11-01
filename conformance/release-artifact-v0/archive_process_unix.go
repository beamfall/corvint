//go:build unix

package main

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func configureContainedCommand(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	// Bounds the output copy after the leader exits or the group is signalled;
	// 250 ms could fail a successful command on a loaded host (V1-3435). The
	// group itself is retired by cleanupContainedCommand, not by this bound.
	command.WaitDelay = 5 * time.Second
	return nil
}

func cleanupContainedCommand(command *exec.Cmd, deadline time.Duration) error {
	if command.Process == nil {
		return nil
	}
	pid := command.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	expires := time.Now().Add(deadline)
	for time.Now().Before(expires) {
		err := syscall.Kill(-pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("owned subprocess group did not quiesce")
}
