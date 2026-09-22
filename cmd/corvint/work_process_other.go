//go:build !unix

package main

import (
	"errors"
	"github.com/Beamfall/corvint/internal/workqueue"
	"os"
	"os/exec"
)

func workContain(command *exec.Cmd) {}
func workKillGroup(command *exec.Cmd) {
	workTerminateGroup(command)
}
func workTerminateGroup(command *exec.Cmd) bool {
	if command.Process != nil {
		return command.Process.Kill() == nil
	}
	return false
}
func workOpenExecutable(path string) (*os.File, error) {
	return nil, errors.New("unsupported executable platform")
}
func workReceiptExit(receipt *workqueue.AdapterReceipt, command *exec.Cmd) {
	if command.ProcessState != nil && command.ProcessState.ExitCode() >= 0 {
		code := workqueue.Count(command.ProcessState.ExitCode())
		receipt.ExitCode = &code
	}
}
