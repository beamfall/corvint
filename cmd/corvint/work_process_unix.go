//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/workqueue"
)

func workContain(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func workKillGroup(command *exec.Cmd) {
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}

// workStop stops the work runner's leader early; groupreap sweeps its group
// once the exit is observed and before the reap.
func workStop(command *exec.Cmd) { _ = groupreap.Stop(command) }

func workOpenExecutable(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func workReceiptExit(receipt *workqueue.AdapterReceipt, command *exec.Cmd) {
	if command.ProcessState == nil {
		return
	}
	status, ok := command.ProcessState.Sys().(syscall.WaitStatus)
	if !ok {
		return
	}
	if status.Signaled() {
		signal := status.Signal().String()
		receipt.Signal = &signal
		return
	}
	if status.Exited() {
		code := workqueue.Count(status.ExitStatus())
		receipt.ExitCode = &code
	}
}
