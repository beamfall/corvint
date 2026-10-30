//go:build unix

package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func poolRunnerIdentity(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return poolRunnerIdentityContext(ctx, pid)
}

// poolNativeIdentity reads a process start identity from the kernel without
// listing processes (Darwin and Linux); tests replace it to reach the
// listing.
var poolNativeIdentity = supervisor.ProcessIdentity

// poolRunnerIdentityContext answers from Darwin's kern.proc.pid, or from
// Linux's /proc/<pid>/stat and /proc/stat btime, when that kernel read
// succeeds, so a loaded host's slow process listing cannot fail a pool
// preparation (PSR-V0-201, V1-1008; PSR-V0-204, V1-2425). A failed kernel
// read, and every other host, keeps the bounded listing.
func poolRunnerIdentityContext(ctx context.Context, pid int) (string, error) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if identity, e := poolNativeIdentity(pid); e == nil {
			return identity, nil
		}
	}
	return poolListedIdentity(ctx, pid)
}

// poolNativePrefix is the identity prefix poolNativeIdentity writes on this
// host, or "" where pool identities are always listed.
func poolNativePrefix() string {
	switch runtime.GOOS {
	case "darwin", "linux":
		return runtime.GOOS + ":"
	}
	return ""
}

// poolRecordedIdentity re-reads pid's identity with the reader that produced
// recorded, so a listed identity (an earlier binary's, or one recorded after
// a failed kernel read) is never compared with a kernel identity, or the
// reverse, and a live runner never looks gone (PSR-V0-201). A Linux read
// that names the recorded process returns recorded itself; one whose start
// tick matches but whose btime is further off is an error, never a gone
// runner (PSR-V0-204). The comparison is the supervisor's shared one
// (PSR-V0-227).
func poolRecordedIdentity(pid int, recorded string) (string, error) {
	if prefix := poolNativePrefix(); prefix != "" && strings.HasPrefix(recorded, prefix) {
		identity, e := poolNativeIdentity(pid)
		if e == nil && prefix == "linux:" {
			switch supervisor.CompareIdentity(identity, recorded) {
			case supervisor.IdentitySame:
				return recorded, nil
			case supervisor.IdentityUnknown:
				return "", wire.Errorf(wire.CodeQuiescenceUnproved, "pool", "runner identity uncertain: start tick matches but boot time differs")
			}
		}
		return identity, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return poolListedIdentity(ctx, pid)
}

func poolListedIdentity(ctx context.Context, pid int) (string, error) {
	raw, e := poolProbe(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=")
	if exit, ok := e.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return "", nil
	}
	return strings.TrimSpace(string(raw)), e
}
func poolGroupLive(pid int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return poolGroupLiveContext(ctx, pid)
}

// poolGroupSignal is syscall.Kill; tests observe every signal the pool sends
// to a process group itself.
var poolGroupSignal = syscall.Kill

// poolGroupLiveContext observes a pool command's group after its leader was
// reaped. It sends signal 0 only and never grants kill authority: the group
// was already SIGKILLed while the leader was held unreaped (groupreap,
// PGO-V0-008). After the reap the numeric group ID could name a reused group,
// which can only make the group look live and the run unclean, never clean.
func poolGroupLiveContext(ctx context.Context, pid int) (bool, error) {
	// CAL-V0-188: ESRCH from kill(-group, 0) proves the group has no member,
	// not even a zombie, without the full process-table listing whose fixed
	// bound a loaded host overruns (V1-0989). Any other answer, including
	// Darwin's EPERM for a zombie-only group, still needs the listing.
	if pid > 1 && poolGroupSignal(-pid, 0) == syscall.ESRCH {
		return false, nil
	}
	raw, e := poolProbe(ctx, "/bin/ps", "-axo", "pgid=,stat=")
	if e != nil {
		return true, e
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == strconv.Itoa(pid) && !strings.HasPrefix(f[1], "Z") {
			return true, nil
		}
	}
	return false, nil
}

const maxPoolCommandOutput = 64 * wire.KiB // CAL-V0-033; sweep has its separate MaxPoolSweepOutput.

// Trusted commands are confined to an owned process group. Detached processes
// are outside this qualification; uncertain exit/pipe cleanup never admits.
// A non-nil guard runs immediately before Start; its failure starts nothing
// and records SOURCE_CHANGED (PSR-V0-012).
func executePoolGuarded(ctx context.Context, def *intent.PoolCommand, root string, env []string, guard func() error) (class string, clean bool, digest wire.Digest) {
	out := &cappedOutput{max: maxPoolCommandOutput}
	cmd := exec.Command(def.Argv[0], def.Argv[1:]...)
	cmd.Dir = root
	cmd.Env = env
	pipeRead, pipeWrite, e := os.Pipe()
	if e != nil {
		return "SPAWN_FAILED", true, wire.Sum(nil)
	}
	defer pipeRead.Close()
	cmd.Stdout = pipeWrite
	cmd.Stderr = pipeWrite
	cmd.WaitDelay = time.Second
	// Early stops kill only the leader; groupreap.Wait then SIGKILLs the
	// group while the exited leader is unreaped (V1-1036).
	groupreap.Contain(cmd)
	out.kill = func() { _ = groupreap.Stop(cmd) }
	if guard != nil && guard() != nil {
		pipeWrite.Close()
		return "SOURCE_CHANGED", true, wire.Sum(nil)
	}
	if e := cmd.Start(); e != nil {
		pipeWrite.Close()
		return "SPAWN_FAILED", true, wire.Sum(nil)
	}
	pipeWrite.Close()
	readDone := make(chan struct{})
	go func() { _, _ = io.Copy(out, pipeRead); close(readDone) }()
	stopped := make(chan struct{})
	timer := time.NewTimer(time.Duration(def.TimeoutSeconds.Int()) * poolCommandSecond)
	defer timer.Stop()
	reason := make(chan string, 1)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			reason <- "INTERRUPTED"
			_ = groupreap.Stop(cmd)
		case <-timer.C:
			reason <- "TIMEOUT"
			_ = groupreap.Stop(cmd)
		case <-stopped:
		}
	}()
	err := groupreap.Wait(cmd)
	close(stopped)
	<-watcherDone
	class = "EXIT_ZERO"
	if err != nil {
		class = "EXIT_NONZERO"
	}
	if err == exec.ErrWaitDelay {
		class = "UNKNOWN"
	}
	select {
	case why := <-reason:
		class = why
	default:
	}

	// Observation only: no group signal follows the reap.
	live, e := poolGroupLive(cmd.Process.Pid)
	until := time.Now().Add(2 * time.Second)
	for e == nil && live && time.Now().Before(until) {
		time.Sleep(10 * time.Millisecond)
		live, e = poolGroupLive(cmd.Process.Pid)
	}
	clean = e == nil && !live && err != exec.ErrWaitDelay
	select {
	case <-readDone:
	case <-time.After(time.Second):
		pipeRead.Close()
		<-readDone
		clean = false
		class = "UNKNOWN"
	}
	if out.overflow {
		class = "OUTPUT_LIMIT"
	}
	return class, clean, wire.Sum(out.buf.Bytes())
}

// poolProbe is injectable only for reached-path bounded-probe tests.
var poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, sweepObservationBound(ctx, time.Second))
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.WaitDelay = sweepObservationBound(ctx, 250*time.Millisecond)
	out := &sweepCapture{max: 1 << 20, kill: cancel}
	cmd.Stdout = sweepWriter{out: true, c: out}
	cmd.Stderr = sweepWriter{out: false, c: out}
	e := cmd.Run()
	if out.overflow {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "pool probe", "output limit")
	}
	// Diagnostics never enter the parsed identity/group listing.
	return out.stdout.Bytes(), e
}

type poolCommandResult struct {
	Timing snapshot.PoolSweepTiming
	Signal *wire.Count
	Class  string
	// Bound is the run-context class (TIMEOUT or INTERRUPTED) that an
	// unproved cleanup overrides with UNKNOWN; it is never encoded.
	Bound          string
	Clean          bool
	Exit           *wire.Count
	Stdout, Stderr []byte
}
type sweepCapture struct {
	mu             sync.Mutex
	stdout, stderr bytes.Buffer
	overflow       bool
	max            int
	kill           func()
}
type sweepWriter struct {
	out bool
	c   *sweepCapture
}

func (w sweepWriter) Write(p []byte) (int, error) {
	c := w.c
	c.mu.Lock()
	defer c.mu.Unlock()
	room := c.max - c.stdout.Len() - c.stderr.Len()
	if len(p) > room {
		c.overflow = true
		if room > 0 {
			if w.out {
				c.stdout.Write(p[:room])
			} else {
				c.stderr.Write(p[:room])
			}
		}
		c.kill()
		return len(p), nil
	}
	if w.out {
		return c.stdout.Write(p)
	}
	return c.stderr.Write(p)
}

// Owned process-group observations have an absolute post-execution cleanup deadline.
func executePoolCapturedGuarded(ctx context.Context, def *intent.PoolCommand, root string, env []string, guard func() error) (result poolCommandResult) {
	start := time.Now()
	waited := start
	defer func() {
		end := time.Now()
		result.Timing.StartedAt = wire.SizeOf(uint64(start.UnixNano()))
		result.Timing.WaitReturnedAt = wire.SizeOf(uint64(waited.UnixNano()))
		result.Timing.CleanupEndedAt = wire.SizeOf(uint64(end.UnixNano()))
		result.Timing.ExecutionMillis = wire.SizeOf(uint64(waited.Sub(start).Milliseconds()))
		result.Timing.CleanupMillis = wire.SizeOf(uint64(end.Sub(waited).Milliseconds()))
		result.Timing.CleanupAllowanceMillis = poolCleanupAllowanceMillis
	}()
	result = poolCommandResult{Class: "SPAWN_FAILED", Clean: true}
	if len(def.Argv) == 0 {
		return result
	}
	run, cancel := context.WithTimeout(ctx, time.Duration(def.TimeoutSeconds.Int())*time.Second)
	defer cancel()
	deadline, _ := run.Deadline()
	result.Timing.Deadline = wire.SizeOf(uint64(deadline.UnixNano()))
	if e := run.Err(); e != nil {
		result.Class = "INTERRUPTED"
		if errors.Is(e, context.DeadlineExceeded) {
			result.Class = "TIMEOUT"
		}
		return result
	}
	cmd := exec.CommandContext(run, def.Argv[0], def.Argv[1:]...)
	cmd.Dir = root
	cmd.Env = env
	// PSR-V0-023: a sweep's pipe delay scales with its budget; outside a
	// sweep it stays the fixed 1s, and an expired delay is still UNKNOWN.
	cmd.WaitDelay = sweepScaledBound(ctx, time.Second)
	// Cancellation and overflow kill only the leader; Drain SIGKILLs the
	// group after the output drains (a pipe holder past the delay is still
	// exec.ErrWaitDelay) and while the exited leader is unreaped (V1-1036).
	groupreap.Contain(cmd)
	captured := &sweepCapture{max: snapshot.MaxPoolSweepOutput}
	// Drain's copiers can deliver an overflow before Start has set
	// cmd.Process, so kill runs under captured.mu and an early request is
	// held until Drain returns, while the leader is still unreaped.
	started, pending := false, false
	captured.kill = func() {
		if started {
			_ = groupreap.Stop(cmd)
		} else {
			pending = true
		}
	}
	cmd.Stdout = sweepWriter{true, captured}
	cmd.Stderr = sweepWriter{false, captured}
	if guard != nil && guard() != nil {
		result.Class = "SOURCE_CHANGED"
		return result
	}
	wait, e := groupreap.Drain(run, cmd, cmd.WaitDelay)
	if e != nil {
		return result
	}
	captured.mu.Lock()
	started = true
	if pending {
		_ = groupreap.Stop(cmd)
	}
	captured.mu.Unlock()
	e = wait()
	waited = time.Now()
	result.Class = "EXIT_ZERO"
	if e != nil {
		result.Class = "EXIT_NONZERO"
	}
	if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() >= 0 {
		x := wire.CountOf(int64(cmd.ProcessState.ExitCode()))
		result.Exit = &x
	}
	if cmd.ProcessState != nil {
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			sig := wire.CountOf(int64(status.Signal()))
			result.Signal = &sig
			result.Class = "SIGNAL"
		}
	}
	if result.Exit == nil && result.Signal == nil {
		result.Class = "UNKNOWN"
	}
	if run.Err() != nil {
		result.Class = "INTERRUPTED"
		if errors.Is(run.Err(), context.DeadlineExceeded) {
			result.Class = "TIMEOUT"
		}
		result.Bound = result.Class
	}
	// The allowance stays the recorded 5s; only its group probes keep the
	// sweep's scaled bounds (PSR-V0-023), never a test lift.
	cleanup, cancelCleanup := context.WithTimeout(sweepBudgetOnly(ctx), poolCleanupAllowance)
	defer cancelCleanup()
	// Observation only: no group signal follows the reap.
	live, probeErr := poolGroupLiveContext(cleanup, cmd.Process.Pid)
	probeUncertain := probeErr != nil
	for (probeErr != nil || live) && cleanup.Err() == nil {
		select {
		case <-cleanup.Done():
		case <-time.After(10 * time.Millisecond):
		}
		live, probeErr = poolGroupLiveContext(cleanup, cmd.Process.Pid)
		probeUncertain = probeUncertain || probeErr != nil
	}
	result.Clean = !probeUncertain && probeErr == nil && !live && e != exec.ErrWaitDelay
	result.Stdout = bytes.Clone(captured.stdout.Bytes())
	result.Stderr = bytes.Clone(captured.stderr.Bytes())
	if !result.Clean {
		result.Class = "UNKNOWN"
	}
	if captured.overflow {
		result.Class = "OUTPUT_LIMIT"
	}
	return result
}
