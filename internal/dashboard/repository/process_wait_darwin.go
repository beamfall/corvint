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
		exitObserved <- observeLeaderExit(command.Process.Pid)
		<-reapAllowed
		done <- command.Wait()
	}()
	observation := &leaderObservation{result: exitObserved}

	timedOutOrCancelled := false
	select {
	case observation.err = <-exitObserved:
		observation.completed = true
	case <-childContext.Done():
		timedOutOrCancelled = true
	}

	exitObservationCompleted := observation.completed
	observationErr := observation.err
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
		// A failed observation (for example ECHILD) unpins the group ID, so
		// stopAndProveDarwin sends nothing and cleanup stays unproved (V1-1050).
		cleanupProven = containment.stopAndProveDarwin(cleanupDeadline, observation.pinned)
	}
	if !observation.completed {
		observation.err = <-exitObserved
		observation.completed = true
	}
	observationErr = observation.err
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
	for {
		var info [128]byte
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
			if !waitidStateChangeOnly(info) {
				return nil
			}
			time.Sleep(waitidStopPoll)
			continue
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

// waitidStopPoll paces waitProcessExitUnreaped while a child is stopped.
const waitidStopPoll = 10 * time.Millisecond

// waitidStateChangeOnly reports a waitid record that is not an exit. Darwin's
// waitid also returns for a trapped, stopped or continued child despite
// WEXITED (golang/go#19314); si_code, after si_signo and si_errno, is
// CLD_TRAPPED (4), CLD_STOPPED (5) or CLD_CONTINUED (6). Mirrors
// internal/groupreap and internal/procgroup (V1-1037).
func waitidStateChangeOnly(info [128]byte) bool {
	code := *(*int32)(unsafe.Pointer(&info[8]))
	return code >= 4 && code <= 6
}

// stopAndProveDarwin stops the group while the leader is pinned unreaped and
// sends nothing once pinned reports a failed exit observation (V1-1050).
func (containment *childContainment) stopAndProveDarwin(grace time.Duration, pinned func() bool) bool {
	if containment.pid <= 0 || !pinned() {
		return false
	}
	containment.terminate()
	until := time.Now().Add(grace)
	for darwinGroupHasLiveMembers(containment.pid) && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	for darwinGroupHasLiveMembers(containment.pid) {
		if !pinned() {
			return false
		}
		containment.force()
		time.Sleep(time.Millisecond)
	}
	return pinned()
}

func darwinGroupHasLiveMembers(pid int) bool {
	err := syscall.Kill(-pid, 0)
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}
