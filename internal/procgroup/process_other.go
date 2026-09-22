//go:build !darwin && !linux

package procgroup

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

func processPlatformSupported() bool { return false }

func configureProcessCommand(*exec.Cmd, bool) {}

func startProcessCommand(*exec.Cmd) error {
	return errors.New("process execution with descendant containment is unsupported on this platform")
}

func waitProcessExitUnreaped(int) error {
	return errors.New("unreaped process observation is unsupported on this platform")
}

func terminateProcessGroup(int, bool, bool) (bool, error) {
	return false, errors.New("owned process-group cleanup is unsupported on this platform")
}

func cleanupProcessGroupBeforeReap(int, bool, bool, bool, time.Time) error {
	return errors.New("owned process-group cleanup is unsupported on this platform")
}

func proveProcessGroupQuiescent(int, string, func(context.Context, int) (string, error), func(int, syscall.Signal) error, bool, time.Time) (error, bool) {
	return errors.New("owned process-group cleanup is unsupported on this platform"), false
}

func processExists(int) bool { return false }

func processGroupID(int) (int, error) {
	return 0, errors.New("process groups are unsupported on this platform")
}
