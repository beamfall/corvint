//go:build !darwin && !linux

package procgroup

import (
	"errors"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"
)

func processPlatformSupported() bool { return false }

func configureProcessCommand(*exec.Cmd, bool) {}

func startProcessCommand(*exec.Cmd) error {
	return errors.New("process execution with descendant containment is unsupported on this platform")
}

func releaseProcessCommand(int) {}

func waitProcessExitUnreaped(int) error {
	return errors.New("unreaped process observation is unsupported on this platform")
}

func groupSignal(override func(int, syscall.Signal) error, _ bool) func(int, syscall.Signal) error {
	if override != nil {
		return override
	}
	return func(int, syscall.Signal) error {
		return errors.New("owned process-group cleanup is unsupported on this platform")
	}
}

func heldGroupProof(override func(int) (bool, error)) func(int) (bool, error) {
	if override != nil {
		return override
	}
	return func(int) (bool, error) {
		return false, errors.New("owned process-group cleanup is unsupported on this platform")
	}
}

func terminateProcessGroup(int, bool, bool, func(int, syscall.Signal) error) (bool, error) {
	return false, errors.New("owned process-group cleanup is unsupported on this platform")
}

func graceHeldQuiet(*atomic.Bool, func(int) (bool, error)) func(int) bool { return nil }

func cleanupProcessGroupBeforeReap(int, bool, bool, bool, func(int, syscall.Signal) error, func(int) bool, time.Duration, time.Time) error {
	return errors.New("owned process-group cleanup is unsupported on this platform")
}

func proveProcessGroupQuiescent(int, bool, func(int) (bool, error), time.Time) error {
	return errors.New("owned process-group cleanup is unsupported on this platform")
}

func processExists(int) bool { return false }

func processGroupID(int) (int, error) {
	return 0, errors.New("process groups are unsupported on this platform")
}
