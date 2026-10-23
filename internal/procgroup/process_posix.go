//go:build darwin || linux

package procgroup

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"github.com/Beamfall/corvint/internal/groupreap"
)

const (
	processGroupPollInterval = 5 * time.Millisecond
	processGroupGracePeriod  = 100 * time.Millisecond
	processGroupQuietPeriod  = 10 * time.Millisecond
	// processGroupProofBackoffCap bounds how far the interval between
	// held-group quiescence proofs widens; each Linux proof scans /proc and a
	// Darwin proof that sees only zombies reads the process table.
	processGroupProofBackoffCap = 50 * time.Millisecond
)

func processPlatformSupported() bool { return true }

func configureProcessCommand(command *exec.Cmd, joinAncestorGroup bool) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: !joinAncestorGroup}
}

// startProcessCommand records a group the command leads for the adapter exit
// kill (AHI-048, AHI-049) until releaseProcessCommand, which run calls after
// its pre-reap group cleanup and before it allows the reap. A start after that
// kill is refused with groupreap.ErrExiting.
func startProcessCommand(command *exec.Cmd) error {
	return groupreap.StartLive(command)
}

// releaseProcessCommand is groupreap.ReleaseLive; tests observe when run calls it.
var releaseProcessCommand = groupreap.ReleaseLive

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
			if !waitidStateChangeOnly(info) {
				return nil
			}
			time.Sleep(processGroupPollInterval)
			continue
		}
		if errno != syscall.EINTR {
			return errno
		}
	}
}

// heldGroupProof is override when a test sets one, else groupreap.HeldGroupQuiet.
func heldGroupProof(override func(int) (bool, error)) func(int) (bool, error) {
	if override != nil {
		return override
	}
	return groupreap.HeldGroupQuiet
}

// groupSignal is override when a test sets one, else the termination signal.
func groupSignal(override func(int, syscall.Signal) error, joinAncestorGroup bool) func(int, syscall.Signal) error {
	if override != nil {
		return override
	}
	return processTerminationSignal(joinAncestorGroup)
}

func terminateProcessGroup(pid int, leaderExited, joinAncestorGroup bool, override func(int, syscall.Signal) error) (bool, error) {
	if pid <= 0 {
		return false, errors.New("process group has no leader")
	}
	signal := syscall.SIGTERM
	if leaderExited {
		signal = syscall.SIGKILL
	}
	err := groupSignal(override, joinAncestorGroup)(pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if leaderExited && errors.Is(err, syscall.EPERM) {
		return false, nil
	}
	return err == nil, err
}

func cleanupProcessGroupBeforeReap(pid int, groupPresent, leaderExited, joinAncestorGroup bool, override func(int, syscall.Signal) error, heldQuiet func(int) bool, grace time.Duration, deadline time.Time) error {
	if pid <= 0 {
		return errors.New("process group has no leader")
	}
	if !groupPresent {
		return nil
	}
	if leaderExited {
		return nil
	}
	if grace <= 0 {
		grace = processGroupGracePeriod
	}
	graceDeadline := time.Now().Add(grace)
	if graceDeadline.After(deadline) {
		graceDeadline = deadline
	}
	signal := groupSignal(override, joinAncestorGroup)
	if processGroupQuiescentBy(pid, graceDeadline, signal, heldQuiet) {
		return nil
	}
	err := signal(pid, syscall.SIGKILL)
	if err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// proveProcessGroupQuiescent runs before the reap, while the exited leader is
// held unreaped and its PID pins the group ID, so it never probes a numeric
// group ID that may since name another group (V1-1055). A leader that joined
// an ancestor's group leads none, and its observed exit is its quiescence.
// Otherwise it polls quietProof (heldGroupProof, groupreap.HeldGroupQuiet: Linux /proc,
// Darwin signal 0 confirmed by the process table) until deadline. A live
// member at the deadline or a proof failure leaves the cleanup unproved.
func proveProcessGroupQuiescent(pid int, joinAncestorGroup bool, quietProof func(int) (bool, error), deadline time.Time) error {
	if joinAncestorGroup {
		return nil
	}
	interval := processGroupPollInterval
	for {
		quiet, err := quietProof(pid)
		if err != nil {
			return fmt.Errorf("owned process group quiescence unproved: %w", err)
		}
		if quiet {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("owned process group did not quiesce before shutdown deadline")
		}
		time.Sleep(min(interval, remaining))
		interval = min(2*interval, processGroupProofBackoffCap)
	}
}

// processGroupQuiescentBy polls the live leader's group with signal 0 during
// the pre-reap grace period; ESRCH held for the quiet period is quiescence.
// While signal 0 still reaches the group, heldQuiet (graceHeldQuiet, nil off
// Linux) may prove it quiet.
func processGroupQuiescentBy(pid int, deadline time.Time, groupProbe func(int, syscall.Signal) error, heldQuiet func(int) bool) bool {
	quietSince := time.Time{}
	for {
		err := groupProbe(pid, 0)
		if heldQuiet != nil && (err == nil || errors.Is(err, syscall.EPERM)) && heldQuiet(pid) {
			return true
		}
		if errors.Is(err, syscall.ESRCH) {
			if quietSince.IsZero() {
				quietSince = time.Now()
			}
			if time.Since(quietSince) >= processGroupQuietPeriod {
				return true
			}
		} else {
			quietSince = time.Time{}
			if err != nil && !errors.Is(err, syscall.EPERM) {
				return false
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false
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

// waitidStateChangeOnly reports a waitid record that is not an exit. Darwin's
// waitid also returns for a stopped or continued child despite WEXITED
// (golang/go#19314), so waitProcessExitUnreaped polls past such a record, as
// internal/groupreap's leaderUnreaped does (V1-1037). si_code, after si_signo
// and si_errno, is CLD_TRAPPED (4), CLD_STOPPED (5) or CLD_CONTINUED (6) on
// both Darwin and Linux.
func waitidStateChangeOnly(info [128]byte) bool {
	code := *(*int32)(unsafe.Pointer(&info[8]))
	return code >= 4 && code <= 6
}
