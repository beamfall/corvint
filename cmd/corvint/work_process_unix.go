//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/Beamfall/corvint/internal/workqueue"
)

func workContain(command *exec.Cmd) { command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func workKillGroup(command *exec.Cmd) {
	workTerminateGroup(command)
}
func workTerminateGroup(command *exec.Cmd) bool {
	if command.Process != nil {
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) == nil
	}
	return false
}

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
