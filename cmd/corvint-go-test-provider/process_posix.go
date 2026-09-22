//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	// A maximum-size 16 MiB discovery stream expands under base64 inside the
	// attached verifier response. This transport bound does not raise the
	// semantic discovery bound checked before decoding.
	directCommandOutputLimit      = int64(24 << 20)
	directCommandShutdownLimit    = 2 * time.Second
	directCommandPollInterval     = 5 * time.Millisecond
	directCommandQuiescenceWindow = 10 * time.Millisecond
	directCommandGracePeriod      = 750 * time.Millisecond
)

var (
	errDirectCommandInvalid         = errors.New("direct command: invalid request")
	errDirectCommandUnsupported     = errors.New("direct command: unsupported platform")
	errDirectCommandStart           = errors.New("direct command: start failed")
	errDirectCommandWait            = errors.New("direct command: wait failed")
	errDirectCommandWaitDeadline    = errors.New("direct command: wait did not complete before shutdown deadline")
	errDirectCommandCancelled       = errors.New("direct command: cancelled")
	errDirectCommandTimeout         = errors.New("direct command: timed out")
	errDirectCommandOutputLimit     = errors.New("direct command: output limit exceeded")
	errDirectCommandContainment     = errors.New("direct command: containment cleanup failed")
	errDirectCommandPipe            = errors.New("direct command: pipe drain failed")
	errDirectCommandPipeWait        = errors.New("direct command: pipe drain timed out")
	errDirectCommandNotQuiescent    = errors.New("direct command: process group did not quiesce")
	directCommandSignalProcessGroup = syscall.Kill
	directCommandWait               = func(command *exec.Cmd) error { return command.Wait() }
)

type directCommandResult struct {
	Stdout []byte
	Stderr []byte

	ExitCode            int
	Started             bool
	Exited              bool
	Cancelled           bool
	TimedOut            bool
	OutputLimitExceeded bool
	ProcessCleanupDone  bool
	PipesDrained        bool
	WaitCompleted       bool
}

// runDirectCommand starts exactly executable with argv, environment, and cwd.
// It never consults a shell or inherits the ambient environment.
func runDirectCommand(
	ctx context.Context,
	executable string,
	argv, environment []string,
	cwd string,
	timeout time.Duration,
) (directCommandResult, error) {
	return runDirectCommandWithFiles(ctx, executable, argv, environment, cwd, timeout, nil)
}

func runDirectCommandWithFiles(
	ctx context.Context,
	executable string,
	argv, environment []string,
	cwd string,
	timeout time.Duration,
	extraFiles []*os.File,
) (directCommandResult, error) {
	result := directCommandResult{ExitCode: -1}
	if ctx == nil || executable == "" || !filepath.IsAbs(executable) || filepath.Clean(executable) != executable ||
		cwd == "" || !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || timeout <= 0 {
		return result, errDirectCommandInvalid
	}
	if err := ctx.Err(); err != nil {
		result.Cancelled = true
		return result, errors.Join(errDirectCommandCancelled, err)
	}

	budget := newDirectCommandBudget(uint64(directCommandOutputLimit))
	stdout := &directCommandCapture{budget: budget}
	stderr := &directCommandCapture{budget: budget}
	command := exec.Command(executable, append([]string(nil), argv...)...)
	command.Args[0] = executable
	command.Dir = cwd
	command.Env = append([]string{}, environment...)
	command.Stdin = bytes.NewReader(nil)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.ExtraFiles = append([]*os.File(nil), extraFiles...)

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return result, fmt.Errorf("%w: stdout: %w", errDirectCommandPipe, err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return result, fmt.Errorf("%w: stderr: %w", errDirectCommandPipe, err)
	}
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter

	if err := command.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		_ = stderrReader.Close()
		_ = stderrWriter.Close()
		return result, fmt.Errorf("%w: %w", errDirectCommandStart, err)
	}
	result.Started = true
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	drains := make(chan error, 2)
	go drainDirectCommandPipe(stdoutReader, stdout, drains)
	go drainDirectCommandPipe(stderrReader, stderr, drains)
	waitDone := make(chan error, 1)
	waitCommand := directCommandWait
	go func() { waitDone <- waitCommand(command) }()

	timer := time.NewTimer(timeout)
	var waitErr, terminationErr error
	shutdownDeadline := time.Time{}
	select {
	case waitErr = <-waitDone:
		result.WaitCompleted = true
		stopDirectCommandTimer(timer)
	case <-ctx.Done():
		result.Cancelled = true
		stopDirectCommandTimer(timer)
		shutdownDeadline = time.Now().Add(directCommandShutdownLimit)
		terminationErr = terminateDirectCommand(command)
	case <-timer.C:
		result.TimedOut = true
		shutdownDeadline = time.Now().Add(directCommandShutdownLimit)
		terminationErr = terminateDirectCommand(command)
	case <-budget.exceeded:
		result.OutputLimitExceeded = true
		stopDirectCommandTimer(timer)
		shutdownDeadline = time.Now().Add(directCommandShutdownLimit)
		terminationErr = terminateDirectCommand(command)
	}
	if shutdownDeadline.IsZero() {
		shutdownDeadline = time.Now().Add(directCommandShutdownLimit)
	}
	cleanupErr := cleanupDirectCommandGroup(command.Process.Pid, shutdownDeadline)
	if !result.WaitCompleted {
		waitErr, result.WaitCompleted = waitForDirectCommandWait(waitDone, shutdownDeadline)
	}
	result.ProcessCleanupDone = terminationErr == nil && cleanupErr == nil && result.WaitCompleted

	pipeErr, pipeWaitExpired := waitForDirectCommandDrains(
		stdoutReader,
		stderrReader,
		drains,
		time.Until(shutdownDeadline),
	)
	result.PipesDrained = !pipeWaitExpired && pipeErr == nil
	result.Stdout = stdout.bytes()
	result.Stderr = stderr.bytes()
	result.OutputLimitExceeded = result.OutputLimitExceeded || budget.wasExceeded()
	if result.WaitCompleted && command.ProcessState != nil {
		result.Exited = command.ProcessState.Exited()
		result.ExitCode = command.ProcessState.ExitCode()
	}

	var runErr error
	if !result.WaitCompleted {
		runErr = errors.Join(runErr, errDirectCommandWaitDeadline)
	} else if waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(waitErr, &exitErr) {
			runErr = errors.Join(runErr, fmt.Errorf("%w: %w", errDirectCommandWait, waitErr))
		}
	}
	if result.Cancelled {
		runErr = errors.Join(runErr, errDirectCommandCancelled, ctx.Err())
	}
	if result.TimedOut {
		runErr = errors.Join(runErr, errDirectCommandTimeout)
	}
	if result.OutputLimitExceeded {
		runErr = errors.Join(runErr, errDirectCommandOutputLimit)
	}
	if terminationErr != nil || cleanupErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("%w: %w", errDirectCommandContainment, errors.Join(terminationErr, cleanupErr)))
	}
	if pipeWaitExpired {
		runErr = errors.Join(runErr, errDirectCommandPipeWait)
	} else if pipeErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("%w: %w", errDirectCommandPipe, pipeErr))
	}
	return result, runErr
}

func waitForDirectCommandWait(waitDone <-chan error, deadline time.Time) (error, bool) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		select {
		case err := <-waitDone:
			return err, true
		default:
			return nil, false
		}
	}
	timer := time.NewTimer(remaining)
	defer stopDirectCommandTimer(timer)
	select {
	case err := <-waitDone:
		return err, true
	case <-timer.C:
		return nil, false
	}
}

func terminateDirectCommand(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	err := directCommandSignalProcessGroup(-command.Process.Pid, syscall.SIGTERM)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return errors.Join(err, command.Process.Kill())
}

func cleanupDirectCommandGroup(pid int, deadline time.Time) error {
	if pid <= 0 {
		return errDirectCommandContainment
	}
	group := -pid
	err := directCommandSignalProcessGroup(group, syscall.SIGTERM)
	var permissionErr error
	goneSince := time.Time{}
	if errors.Is(err, syscall.ESRCH) {
		goneSince = time.Now()
	} else if errors.Is(err, syscall.EPERM) {
		permissionErr = err
	} else if err != nil {
		return err
	}
	graceDeadline := time.Now().Add(directCommandGracePeriod)
	if graceDeadline.After(deadline) {
		graceDeadline = deadline
	}
	for goneSince.IsZero() && time.Now().Before(graceDeadline) {
		timer := time.NewTimer(min(directCommandPollInterval, time.Until(graceDeadline)))
		<-timer.C
		err = directCommandSignalProcessGroup(group, 0)
		if errors.Is(err, syscall.ESRCH) {
			goneSince = time.Now()
		} else if err != nil && !errors.Is(err, syscall.EPERM) {
			return err
		}
	}
	if goneSince.IsZero() {
		err = directCommandSignalProcessGroup(group, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			goneSince = time.Now()
		} else if errors.Is(err, syscall.EPERM) {
			permissionErr = err
		} else if err != nil {
			return err
		}
	}
	for {
		err = directCommandSignalProcessGroup(group, 0)
		switch {
		case errors.Is(err, syscall.ESRCH):
			if goneSince.IsZero() {
				goneSince = time.Now()
			}
			if time.Since(goneSince) >= directCommandQuiescenceWindow {
				return nil
			}
		case errors.Is(err, syscall.EPERM):
			goneSince = time.Time{}
			permissionErr = err
		case err != nil:
			return err
		default:
			goneSince = time.Time{}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.Join(errDirectCommandNotQuiescent, permissionErr)
		}
		timer := time.NewTimer(min(directCommandPollInterval, remaining))
		<-timer.C
	}
}

type directCommandBudget struct {
	mu       sync.Mutex
	limit    uint64
	observed uint64
	stored   uint64
	overflow bool
	exceeded chan struct{}
}

func newDirectCommandBudget(limit uint64) *directCommandBudget {
	return &directCommandBudget{limit: limit, exceeded: make(chan struct{}, 1)}
}

func (budget *directCommandBudget) admit(length int) int {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	amount := uint64(length)
	if math.MaxUint64-budget.observed < amount {
		budget.observed = math.MaxUint64
	} else {
		budget.observed += amount
	}
	remaining := uint64(0)
	if budget.stored < budget.limit {
		remaining = budget.limit - budget.stored
	}
	keep := min(amount, remaining)
	budget.stored += keep
	if budget.observed > budget.limit && !budget.overflow {
		budget.overflow = true
		budget.exceeded <- struct{}{}
	}
	return int(keep)
}

func (budget *directCommandBudget) wasExceeded() bool {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.overflow
}

type directCommandCapture struct {
	budget *directCommandBudget
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (capture *directCommandCapture) Write(value []byte) (int, error) {
	keep := capture.budget.admit(len(value))
	if keep > 0 {
		capture.mu.Lock()
		_, _ = capture.buffer.Write(value[:keep])
		capture.mu.Unlock()
	}
	return len(value), nil
}

func (capture *directCommandCapture) bytes() []byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.buffer.Bytes()...)
}

func drainDirectCommandPipe(reader *os.File, capture *directCommandCapture, done chan<- error) {
	_, err := io.Copy(capture, reader)
	closeErr := reader.Close()
	done <- errors.Join(err, closeErr)
}

func waitForDirectCommandDrains(stdout, stderr *os.File, drains <-chan error, timeout time.Duration) (error, bool) {
	if timeout <= 0 {
		result, collected := collectAvailableDirectCommandDrains(drains, 2)
		if collected == 2 {
			return result, false
		}
		_ = stdout.Close()
		_ = stderr.Close()
		return result, true
	}
	timer := time.NewTimer(timeout)
	defer stopDirectCommandTimer(timer)
	var result error
	for remaining := 2; remaining > 0; remaining-- {
		select {
		case err := <-drains:
			result = errors.Join(result, err)
		case <-timer.C:
			available, collected := collectAvailableDirectCommandDrains(drains, remaining)
			result = errors.Join(result, available)
			if collected == remaining {
				return result, false
			}
			_ = stdout.Close()
			_ = stderr.Close()
			return result, true
		}
	}
	return result, false
}

func collectAvailableDirectCommandDrains(drains <-chan error, count int) (error, int) {
	var result error
	collected := 0
	for collected < count {
		select {
		case err := <-drains:
			result = errors.Join(result, err)
			collected++
		default:
			return result, collected
		}
	}
	return result, collected
}

func stopDirectCommandTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}
	select {
	case <-timer.C:
	default:
	}
}
