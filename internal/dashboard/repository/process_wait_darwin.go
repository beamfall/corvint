//go:build darwin

package repository

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

func (containment *childContainment) wait(command *exec.Cmd, childContext context.Context, stdin io.Reader, verifyExecutable func() bool) containmentOutcome {
	exitObserved := make(chan error, 1)
	reapAllowed := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		exitObserved <- waitProcessExitUnreaped(command.Process.Pid)
		<-reapAllowed
		done <- command.Wait()
	}()

	var observationErr error
	exitObservationCompleted := false
	timedOutOrCancelled := false
	select {
	case observationErr = <-exitObserved:
		exitObservationCompleted = true
	case <-childContext.Done():
		timedOutOrCancelled = true
	}

	observationFailed := exitObservationCompleted && observationErr != nil
	executableStable := true
	if exitObservationCompleted && observationErr == nil && !timedOutOrCancelled {
		executableStable = verifyExecutable()
	}
	residueAfterLeader := exitObservationCompleted && observationErr == nil && darwinGroupHasLiveMembers(containment.pid)
	cleanupRequired := timedOutOrCancelled || observationFailed || residueAfterLeader
	cleanupProven := true
	if cleanupRequired {
		closeReader(stdin)
		cleanupProven = containment.stopAndProveDarwin(cleanupDeadline)
	}
	if !exitObservationCompleted {
		observationErr = <-exitObserved
	}
	close(reapAllowed)
	waitErr := <-done
	if timedOutOrCancelled || observationErr != nil {
		executableStable = verifyExecutable()
	}
	if observationErr != nil {
		cleanupProven = false
	}
	return containmentOutcome{
		waitErr: waitErr, timedOutOrCancelled: timedOutOrCancelled,
		residueAfterLeader: residueAfterLeader, cleanupProven: cleanupProven,
		groupQuiescent: true, observationFailed: observationFailed,
		executableStable: executableStable,
	}
}

func waitProcessExitUnreaped(pid int) error {
	if pid <= 0 {
		return syscall.ESRCH
	}
	var info [128]byte
	for {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_WAITID,
			uintptr(1),
			uintptr(pid),
			uintptr(unsafe.Pointer(&info[0])),
			uintptr(syscall.WEXITED|syscall.WNOWAIT),
			0,
			0,
		)
		if errno == 0 {
			return nil
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

func (containment *childContainment) stopAndProveDarwin(grace time.Duration) bool {
	if containment.pid <= 0 {
		return false
	}
	containment.terminate()
	until := time.Now().Add(grace)
	for darwinGroupHasLiveMembers(containment.pid) && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if darwinGroupHasLiveMembers(containment.pid) {
		containment.force()
		for darwinGroupHasLiveMembers(containment.pid) {
			containment.force()
			time.Sleep(time.Millisecond)
		}
	}
	return true
}

func darwinGroupHasLiveMembers(pid int) bool {
	err := syscall.Kill(-pid, 0)
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}
