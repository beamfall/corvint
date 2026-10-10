//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// inGroup starts cmd as the leader of a new process group. Cancellation
// interrupts the whole group, so `go` and the compilers, linkers and test
// binaries it started can stop; kills every member still alive after
// stopGrace; and returns only once the group is empty, so Wait never returns
// while a descendant runs. A descendant that leaves the group (setsid or
// setpgid) is not reached. A group id cannot be reused while a member remains.
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		group := -cmd.Process.Pid
		if err := syscall.Kill(group, syscall.SIGINT); errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		} else if err != nil {
			return err
		}
		if emptied(group, stopGrace) {
			return nil
		}
		if err := syscall.Kill(group, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		if !emptied(group, stopGrace) {
			return errors.New("process group survived SIGKILL")
		}
		return nil
	}
}

// emptied reports whether process group -group has no member left within d.
func emptied(group int, d time.Duration) bool {
	for deadline := time.Now().Add(d); ; time.Sleep(20 * time.Millisecond) {
		if errors.Is(syscall.Kill(group, 0), syscall.ESRCH) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
	}
}
