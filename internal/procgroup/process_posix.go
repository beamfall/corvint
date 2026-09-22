//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

const (
	processGroupPollInterval = 5 * time.Millisecond
	processGroupGracePeriod  = 100 * time.Millisecond
	processGroupQuietPeriod  = 10 * time.Millisecond
	// processGroupIdentityBackoffCap bounds how far the identity-read backoff
	// below can widen. The group-presence probe (groupProbe, a signal-0 kill)
	// still runs every processGroupPollInterval so ESRCH/quiet-period exit
	// detection is unaffected; only the identity fork (ps on darwin,
	// /proc/<pid>/stat on linux) backs off while the group keeps reading
	// present, since re-confirming an unchanged identity every 5ms buys
	// nothing but fork volume for the whole child lifetime.
	processGroupIdentityBackoffCap = 50 * time.Millisecond
)

func processPlatformSupported() bool { return true }

func configureProcessCommand(command *exec.Cmd, joinAncestorGroup bool) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: !joinAncestorGroup}
}

func startProcessCommand(command *exec.Cmd) error {
	return command.Start()
}

func waitProcessExitUnreaped(pid int) error {
	if pid <= 0 {
		return errors.New("process has no leader")
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

func terminateProcessGroup(pid int, leaderExited, joinAncestorGroup bool) (bool, error) {
	if pid <= 0 {
		return false, errors.New("process group has no leader")
	}
	signal := syscall.SIGTERM
	if leaderExited {
		signal = syscall.SIGKILL
	}
	err := processTerminationSignal(joinAncestorGroup)(pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if leaderExited && errors.Is(err, syscall.EPERM) {
		return false, nil
	}
	return err == nil, err
}

func cleanupProcessGroupBeforeReap(pid int, groupPresent, leaderExited, joinAncestorGroup bool, deadline time.Time) error {
	if pid <= 0 {
		return errors.New("process group has no leader")
	}
	if !groupPresent {
		return nil
	}
	if leaderExited {
		return nil
	}
	graceDeadline := time.Now().Add(processGroupGracePeriod)
	if graceDeadline.After(deadline) {
		graceDeadline = deadline
	}
	signal := processTerminationSignal(joinAncestorGroup)
	quiescent, _ := processGroupQuiescentBy(pid, graceDeadline, "", nil, signal)
	if quiescent {
		return nil
	}
	err := signal(pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func proveProcessGroupQuiescent(pid int, leaderStart string, identityReader func(context.Context, int) (string, error), groupProbe func(int, syscall.Signal) error, joinAncestorGroup bool, deadline time.Time) (error, bool) {
	if groupProbe == nil {
		groupProbe = processTerminationSignal(joinAncestorGroup)
	}
	quiescent, identityFallback := processGroupQuiescentBy(pid, deadline, leaderStart, identityReader, groupProbe)
	if quiescent {
		return nil, identityFallback
	}
	return errors.New("owned process group did not quiesce before shutdown deadline"), identityFallback
}

func processGroupQuiescentBy(pid int, deadline time.Time, leaderStart string, identityReader func(context.Context, int) (string, error), groupProbe func(int, syscall.Signal) error) (bool, bool) {
	quietSince := time.Time{}
	identityFallback := false
	nextIdentityCheck := time.Time{}
	identityBackoff := processGroupPollInterval
	for {
		err := groupProbe(pid, 0)
		if (err == nil || errors.Is(err, syscall.EPERM)) && leaderStart != "" && identityReader != nil && !time.Now().Before(nextIdentityCheck) {
			identityContext, cancelIdentity := context.WithDeadline(context.Background(), deadline)
			currentStart, identityErr := identityReader(identityContext, pid)
			cancelIdentity()
			identityFallback = identityFallback || identityErr != nil
			if identityErr == nil && currentStart != leaderStart {
				return true, identityFallback
			}
			nextIdentityCheck = time.Now().Add(identityBackoff)
			if identityBackoff < processGroupIdentityBackoffCap {
				identityBackoff *= 2
				if identityBackoff > processGroupIdentityBackoffCap {
					identityBackoff = processGroupIdentityBackoffCap
				}
			}
		}
		if errors.Is(err, syscall.ESRCH) {
			if quietSince.IsZero() {
				quietSince = time.Now()
			}
			if time.Since(quietSince) >= processGroupQuietPeriod {
				return true, identityFallback
			}
		} else {
			quietSince = time.Time{}
			if err != nil && !errors.Is(err, syscall.EPERM) {
				return false, identityFallback
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, identityFallback
		}
		time.Sleep(min(processGroupPollInterval, remaining))
	}
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func processGroupID(pid int) (int, error) { return syscall.Getpgid(pid) }

func signalOwnedProcessGroup(pid int, signal syscall.Signal) error {
	_, _, errno := syscall.RawSyscall(syscall.SYS_KILL, uintptr(-pid), uintptr(signal), 0)
	if errno != 0 {
		return errno
	}
	return nil
}

// processTerminationSignal picks how a leader is stopped. An owning leader
// signals the whole process group it created, reaching every descendant that
// did not itself escape into a disjoint group. A leader that joined an
// ancestor's group instead (joinAncestorGroup) does not own that group and
// must not signal it — that would strike siblings, including the ancestor
// itself — so it signals only its own pid; the ancestor's own group-directed
// kill is what reaches it and its descendants together.
func processTerminationSignal(joinAncestorGroup bool) func(int, syscall.Signal) error {
	if joinAncestorGroup {
		return signalProcess
	}
	return signalOwnedProcessGroup
}

func signalProcess(pid int, signal syscall.Signal) error {
	return syscall.Kill(pid, signal)
}
