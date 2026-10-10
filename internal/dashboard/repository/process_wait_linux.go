//go:build linux

package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

// wait holds the exited leader unreaped while it observes residue and stops
// the group, and reaps only afterwards, so no group signal can name a group
// ID reused after the reap (V1-1047). It mirrors the Darwin path; Linux
// signal 0 cannot tell a zombie from a live member, so residue is read from
// /proc instead.
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
	// A leader may exit successfully while leaving descendants behind. Record
	// that fact before cleanup: successful cleanup is proof that residue was
	// removed, not permission to accept the command's output. An unreadable
	// group counts as residue.
	residueAfterLeader := false
	if exitObservationCompleted && observationErr == nil {
		live, err := linuxGroupLive(containment.pid)
		residueAfterLeader = live || err != nil
	}
	cleanupRequired := timedOutOrCancelled || observationFailed || residueAfterLeader
	cleanupProven := true
	if cleanupRequired {
		closeReader(stdin)
		cleanupProven = containment.stopAndProveLinux(cleanupDeadline, observation.pinned)
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
	// Observation only (signal 0): a group ID reused after the reap can only
	// make the run unavailable, never accepted.
	groupQuiescent := containment.quiescent()
	return containmentOutcome{
		waitErr: waitErr, timedOutOrCancelled: timedOutOrCancelled,
		residueAfterLeader: residueAfterLeader, cleanupProven: cleanupProven,
		groupQuiescent: groupQuiescent, observationFailed: observationFailed,
		executableStable: executableStable,
	}
}

// linuxGroupLive is the /proc process-table read; tests inject an
// unreadable /proc.
var linuxGroupLive = linuxGroupHasLiveMembers

// waitProcessExitUnreaped blocks until the process exits and leaves it
// unreaped (waitid WEXITED|WNOWAIT). A trapped, stopped or continued report
// (si_code 4..6) is not an exit and is polled past, as on Darwin (V1-1046).
func waitProcessExitUnreaped(pid int) error {
	if pid <= 0 {
		return syscall.ESRCH
	}
	for {
		var info [128]byte
		_, _, errno := syscall.Syscall6(syscall.SYS_WAITID, uintptr(1), uintptr(pid),
			uintptr(unsafe.Pointer(&info[0])), uintptr(syscall.WEXITED|syscall.WNOWAIT), 0, 0)
		if errno == 0 {
			if code := *(*int32)(unsafe.Pointer(&info[8])); code < 4 || code > 6 {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

// stopAndProveLinux stops the group while the exited or running leader is
// pinned unreaped, and sends nothing once pinned reports that the leader may
// have been reaped elsewhere. It proves quiescence from /proc. An unreadable
// process table leaves cleanup unproved but still SIGKILLs the pinned group,
// so a TERM-ignoring leader cannot block the wait and no residue is left
// merely because /proc failed.
func (containment *childContainment) stopAndProveLinux(grace time.Duration, pinned func() bool) bool {
	if containment.pid <= 0 || !pinned() {
		return false
	}
	containment.terminate()
	until := time.Now().Add(grace)
	for {
		live, err := linuxGroupLive(containment.pid)
		if !pinned() {
			return false
		}
		if err != nil {
			containment.force()
			return false
		}
		if !live {
			return true
		}
		if time.Now().After(until) {
			// SIGKILL has no second grace window: this loop is proof of
			// quiescence, not another timeout.
			containment.force()
		}
		time.Sleep(time.Millisecond)
	}
}

// linuxGroupHasLiveMembers reports whether any process in group pgid other
// than a zombie remains. A zombie with another live thread is live.
func linuxGroupHasLiveMembers(pgid int) (bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return true, err
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		dir := "/proc/" + entry.Name()
		raw, err := os.ReadFile(dir + "/stat")
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return true, err
		}
		state, group, ok := parseLinuxStat(raw)
		if !ok {
			return true, errors.New("unparsable /proc stat")
		}
		if group != pgid {
			continue
		}
		if state != 'Z' && state != 'X' {
			return true, nil
		}
		if tasks, err := os.ReadDir(dir + "/task"); err == nil && len(tasks) > 1 {
			return true, nil
		}
	}
	return false, nil
}

// parseLinuxStat returns the state and process group of a /proc/<pid>/stat
// line: "pid (comm) state ppid pgrp ...", where comm may hold spaces or ')'.
func parseLinuxStat(raw []byte) (byte, int, bool) {
	end := bytes.LastIndexByte(raw, ')')
	if end < 0 {
		return 0, 0, false
	}
	fields := bytes.Fields(raw[end+1:])
	if len(fields) < 3 || len(fields[0]) != 1 {
		return 0, 0, false
	}
	group, err := strconv.Atoi(string(fields[2]))
	if err != nil {
		return 0, 0, false
	}
	return fields[0][0], group, true
}
