//go:build unix

package store

import (
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
	"os"
	"os/exec"
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
func poolRunnerIdentityContext(ctx context.Context, pid int) (string, error) {
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
func poolGroupLiveContext(ctx context.Context, pid int) (bool, error) {
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

// Trusted commands are confined to an owned process group. Detached processes
// are outside this qualification; uncertain exit/pipe cleanup never admits.
func executePool(ctx context.Context, def *intent.PoolCommand, root string, env []string) (class string, clean bool, digest wire.Digest) {
	out := &cappedOutput{}
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
	containGate(cmd)
	out.kill = func() { killGate(cmd) }
	if e := cmd.Start(); e != nil {
		pipeWrite.Close()
		return "SPAWN_FAILED", true, wire.Sum(nil)
	}
	pipeWrite.Close()
	readDone := make(chan struct{})
	go func() { _, _ = io.Copy(out, pipeRead); close(readDone) }()
	stopped := make(chan struct{})
	timer := time.NewTimer(time.Duration(def.TimeoutSeconds.Int()) * time.Second)
	defer timer.Stop()
	reason := make(chan string, 1)
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			reason <- "INTERRUPTED"
			killGate(cmd)
		case <-timer.C:
			reason <- "TIMEOUT"
			killGate(cmd)
		case <-stopped:
		}
	}()
	err := cmd.Wait()
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

	live, e := poolGroupLive(cmd.Process.Pid)
	if e == nil && live {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
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
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.WaitDelay = 250 * time.Millisecond
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
	Timing         snapshot.PoolSweepTiming
	Signal         *wire.Count
	Class          string
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
func executePoolCaptured(ctx context.Context, def *intent.PoolCommand, root string, env []string) (result poolCommandResult) {
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
	cmd.WaitDelay = time.Second
	containGate(cmd)
	captured := &sweepCapture{max: snapshot.MaxPoolSweepOutput}
	captured.kill = func() { killGate(cmd) }
	cmd.Stdout = sweepWriter{true, captured}
	cmd.Stderr = sweepWriter{false, captured}
	cmd.Cancel = func() error { killGate(cmd); return nil }
	if e := cmd.Start(); e != nil {
		return result
	}
	e := cmd.Wait()
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
	}
	cleanup, cancelCleanup := context.WithTimeout(context.Background(), poolCleanupAllowance)
	defer cancelCleanup()
	live, probeErr := poolGroupLiveContext(cleanup, cmd.Process.Pid)
	probeUncertain := probeErr != nil
	if probeErr != nil || live {
		killGate(cmd)
	}
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
