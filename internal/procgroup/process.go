package procgroup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/processidentity"
)

const (
	defaultProcessInputLimit    = 16 << 20
	DefaultOutputLimit          = 16 << 20
	defaultProcessShutdownLimit = 2 * time.Second

	descendantCleanupOwnedProcessGroup = "owned-process-group"
	descendantCleanupUnsupported       = "descendant-cleanup-unsupported"
	// descendantCleanupAncestorProcessGroup marks a leader that joined an
	// ancestor's group: it owns no group and proves only its own pid gone.
	descendantCleanupAncestorProcessGroup = "ancestor-process-group"
	processVerdictPartial                 = "PARTIAL"
)

// OverflowPolicy controls whether a bounded capture stops its producer.
// The zero value preserves the existing fail-on-overflow contract.
type OverflowPolicy uint8

const (
	OverflowFail OverflowPolicy = iota
	OverflowTruncate
)

type Spec struct {
	Argv  []string
	Dir   string
	Env   []string
	Stdin []byte
	// Dialogue is an internal bounded stdio protocol. It must stop when its
	// streams close, never spawn unjoined work, and enforce its input bound.
	// The process lifecycle closes both streams on every stop and joins it.
	Dialogue                 func(io.Reader, io.WriteCloser) error
	Timeout                  time.Duration
	ShutdownTimeout          time.Duration
	InputLimit               int
	OutputLimit              int
	StderrLimit              int // Zero inherits the normalized OutputLimit.
	OverflowPolicy           OverflowPolicy
	RequireDescendantCleanup bool
	// JoinAncestorProcessGroup makes this leader join the process group of
	// whichever process starts it, instead of creating a new one. Set this
	// only when the caller is itself a leader running inside an ancestor's
	// owned process group (a nested Run inside a Run): it lets the
	// ancestor's group-directed kill reach this leader and its own
	// descendants directly, no matter how deeply Run calls nest. This Run
	// call in turn stops only its own pid on its own timeout or cancel,
	// since it does not own the shared group. It therefore cannot prove its
	// descendants gone: the observation reports DescendantCleanupStatus
	// "ancestor-process-group", a PARTIAL qualification, and never
	// OwnedProcessGroupCleanup; the ancestor's own Run carries that proof.
	JoinAncestorProcessGroup bool
	// BeforeStop is an audited internal, synchronous cleanup hook. It must honor
	// its context and leave no asynchronous work. The allowance is at most one
	// second (half ShutdownTimeout), in addition to the normal shutdown budget;
	// arbitrary callbacks cannot be forcibly bounded safely.
	BeforeStop func(context.Context, int) error
	// AfterStart may register a leader-bound observer; output drains start as
	// soon as the pipes exist, before this hook runs, so a child that fills a
	// pipe cannot block behind it. It has the same cooperative allowance as
	// BeforeStop; any observer it starts must be joined by BeforeStop,
	// including after an initialization failure.
	AfterStart func(context.Context, int) error
}

type Observation struct {
	Stdout []byte
	Stderr []byte
	Usage  *ResourceUsage

	ExitStatus                     int
	ExitObserved                   bool
	Signal                         string
	Err                            error
	Started                        bool
	TimedOut                       bool
	Cancelled                      bool
	OutputOverflow                 bool
	StdoutOverflow                 bool
	StderrOverflow                 bool
	WaitCompleted                  bool
	PipesDrained                   bool
	OwnedProcessGroupCleanup       bool
	DescendantCleanupStatus        string
	DescendantCleanupQualification string
}

// ResourceUsage is the reaped process's operating-system accounting. Nil means
// the host did not supply supported accounting; it must not be reported as zero.
type ResourceUsage struct {
	UserCPUNs   int64
	SystemCPUNs int64
	MaxRSSBytes int64
}

type processError struct {
	Code    string
	Verdict string
	Cause   error
}

func (failure *processError) Error() string {
	if failure.Cause == nil {
		return failure.Code
	}
	return failure.Code + ": " + failure.Cause.Error()
}

func (failure *processError) Unwrap() error { return failure.Cause }

var errProcessOutputOverflow = errors.New("process output limit exceeded")

type processCapture struct {
	mu       sync.Mutex
	data     bytes.Buffer
	limit    int
	exceeded bool
	notify   chan struct{}
	policy   OverflowPolicy
}

type processWaitChannels struct {
	exitObserved <-chan error
	waitDone     <-chan error
	identity     func(context.Context, int) (string, error)
	groupProbe   func(int, syscall.Signal) error
}

func newProcessCapture(limit int, notify chan struct{}, policy OverflowPolicy) *processCapture {
	return &processCapture{limit: limit, notify: notify, policy: policy}
}

func (capture *processCapture) Write(value []byte) (int, error) {
	capture.mu.Lock()
	remaining := capture.limit - capture.data.Len()
	if remaining > len(value) {
		remaining = len(value)
	}
	if remaining > 0 {
		_, _ = capture.data.Write(value[:remaining])
	}
	exceeded := remaining < len(value)
	if exceeded {
		capture.exceeded = true
	}
	capture.mu.Unlock()
	if exceeded && capture.policy == OverflowFail {
		select {
		case capture.notify <- struct{}{}:
		default:
		}
		return len(value), errProcessOutputOverflow
	}
	return len(value), nil
}

func (capture *processCapture) result() ([]byte, bool) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.data.Bytes()...), capture.exceeded
}

// Run executes the process with bounded output and descendant cleanup.
// The caller owns the process umask.
func Run(ctx context.Context, spec Spec) Observation {
	return run(ctx, spec, processWaitChannels{})
}

func run(ctx context.Context, spec Spec, waitChannels processWaitChannels) Observation {
	observation := Observation{
		ExitStatus:              -1,
		DescendantCleanupStatus: descendantCleanupOwnedProcessGroup,
	}
	if ctx == nil {
		observation.Err = &processError{Code: "process-spec-invalid", Cause: errors.New("context must not be nil")}
		return observation
	}
	if !processPlatformSupported() {
		observation.DescendantCleanupStatus = descendantCleanupUnsupported
		observation.DescendantCleanupQualification = processVerdictPartial
		observation.Err = &processError{Code: "process-platform-unsupported"}
		return observation
	}
	if spec.RequireDescendantCleanup {
		observation.DescendantCleanupStatus = descendantCleanupUnsupported
		observation.DescendantCleanupQualification = processVerdictPartial
		observation.Err = &processError{Code: descendantCleanupUnsupported, Verdict: processVerdictPartial}
		return observation
	}
	spec, err := normalizeProcessSpec(spec)
	if err != nil {
		observation.Err = &processError{Code: "process-spec-invalid", Cause: err}
		return observation
	}
	if err := ctx.Err(); err != nil {
		observation.Cancelled = true
		observation.Err = &processError{Code: "process-cancelled", Cause: err}
		return observation
	}

	if spec.JoinAncestorProcessGroup {
		observation.DescendantCleanupStatus = descendantCleanupAncestorProcessGroup
		observation.DescendantCleanupQualification = processVerdictPartial
	}

	command := exec.Command(spec.Argv[0], spec.Argv[1:]...)
	command.Args = append([]string(nil), spec.Argv...)
	command.Dir = spec.Dir
	command.Env = append(make([]string, 0, len(spec.Env)), spec.Env...)
	command.Stdin = bytes.NewReader(spec.Stdin)
	dialogue, err := newProcessDialogue(spec.Dialogue)
	if err != nil {
		observation.Err = err
		return observation
	}
	defer dialogue.close()
	if dialogue != nil {
		command.Stdin = dialogue.inputReader
	}
	command.WaitDelay = spec.ShutdownTimeout
	configureProcessCommand(command, spec.JoinAncestorProcessGroup)

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		observation.Err = &processError{Code: "process-pipe-failed", Cause: err}
		return observation
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		observation.Err = &processError{Code: "process-pipe-failed", Cause: err}
		return observation
	}
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter

	if err := startProcessCommand(command); err != nil {
		closeProcessPipes(stdoutReader, stdoutWriter, stderrReader, stderrWriter)
		observation.Err = &processError{Code: "process-start-failed", Cause: err}
		return observation
	}
	observation.Started = true
	dialogue.start(spec.Dialogue)
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	overflow := make(chan struct{}, 1)
	stdout := newProcessCapture(spec.OutputLimit, overflow, spec.OverflowPolicy)
	stderr := newProcessCapture(spec.StderrLimit, overflow, spec.OverflowPolicy)
	drains := make(chan error, 2)
	go drainProcessPipe(stdoutReader, dialogue.destination(stdout), drains)
	go drainProcessPipe(stderrReader, stderr, drains)

	identityReader := waitChannels.identity
	if identityReader == nil {
		identityReader = processidentity.Start
	}
	identityContext, cancelIdentity := context.WithTimeout(context.Background(), min(spec.ShutdownTimeout, 250*time.Millisecond))
	leaderStart, identityErr := identityReader(identityContext, command.Process.Pid)
	cancelIdentity()
	if identityErr != nil {
		observation.DescendantCleanupQualification = processVerdictPartial
	}
	var afterStartErr error
	if spec.AfterStart != nil {
		hookContext, cancelHook := context.WithTimeout(context.Background(), min(time.Second, spec.ShutdownTimeout/2))
		afterStartErr = callBeforeStop(hookContext, command.Process.Pid, spec.AfterStart)
		cancelHook()
	}

	exitObserved := make(chan error, 1)
	reapAllowed := make(chan struct{})
	waitDone := make(chan error, 1)
	go func() {
		exitObserved <- waitProcessExitUnreaped(command.Process.Pid)
		<-reapAllowed
		waitDone <- command.Wait()
	}()
	selectedExitObserved := (<-chan error)(exitObserved)
	if waitChannels.exitObserved != nil {
		selectedExitObserved = waitChannels.exitObserved
	}
	selectedWaitDone := (<-chan error)(waitDone)
	if waitChannels.waitDone != nil {
		selectedWaitDone = waitChannels.waitDone
	}

	runTimer := time.NewTimer(spec.Timeout)
	var exitObservationErr error
	exitObservationCompleted := false
	if afterStartErr != nil {
		stopProcessTimer(runTimer)
	} else {
		select {
		case exitObservationErr = <-selectedExitObserved:
			exitObservationCompleted = true
			stopProcessTimer(runTimer)
		case <-ctx.Done():
			observation.Cancelled = true
			stopProcessTimer(runTimer)
		case <-runTimer.C:
			observation.TimedOut = true
		case <-overflow:
			observation.OutputOverflow = true
			stopProcessTimer(runTimer)
		case <-dialogue.failed():
			stopProcessTimer(runTimer)
		}
	}

	var beforeStopErr error
	if spec.BeforeStop != nil {
		hookContext, cancelHook := context.WithTimeout(context.Background(), min(time.Second, spec.ShutdownTimeout/2))
		beforeStopErr = errors.Join(beforeStopErr, callBeforeStop(hookContext, command.Process.Pid, spec.BeforeStop))
		cancelHook()
	}
	shutdownDeadline := time.Now().Add(spec.ShutdownTimeout)
	leaderExited := exitObservationCompleted && exitObservationErr == nil
	groupPresent, terminationErr := terminateProcessGroup(command.Process.Pid, leaderExited, spec.JoinAncestorProcessGroup)
	if terminationErr != nil {
		terminationErr = fmt.Errorf("terminate owned process group: %w", terminationErr)
	}
	cleanupErr := cleanupProcessGroupBeforeReap(command.Process.Pid, groupPresent, leaderExited, spec.JoinAncestorProcessGroup, shutdownDeadline)
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("force owned process group before reap: %w", cleanupErr)
	}
	if dialogue != nil {
		beforeStopErr = errors.Join(beforeStopErr, dialogue.finish(leaderExited, time.Until(shutdownDeadline)))
	}
	if !exitObservationCompleted {
		exitObservationErr, _ = waitForProcessEvent(selectedExitObserved, shutdownDeadline)
	}
	close(reapAllowed)
	waitErr, waitCompleted := waitForProcessEvent(selectedWaitDone, shutdownDeadline)
	observation.WaitCompleted = waitCompleted
	quiescenceErr, identityFallback := proveProcessGroupQuiescent(command.Process.Pid, leaderStart, identityReader, waitChannels.groupProbe, spec.JoinAncestorProcessGroup, shutdownDeadline)
	if identityFallback {
		observation.DescendantCleanupQualification = processVerdictPartial
	}
	if quiescenceErr != nil {
		quiescenceErr = fmt.Errorf("prove owned process group quiescence: %w", quiescenceErr)
	}
	observation.OwnedProcessGroupCleanup = !spec.JoinAncestorProcessGroup && terminationErr == nil && cleanupErr == nil && quiescenceErr == nil && observation.WaitCompleted

	pipeErr, pipesDrained := waitForProcessDrains(stdoutReader, stderrReader, drains, shutdownDeadline)
	observation.PipesDrained = pipesDrained
	stdoutBytes, stdoutOverflow := stdout.result()
	stderrBytes, stderrOverflow := stderr.result()
	observation.Stdout = stdoutBytes
	observation.Stderr = stderrBytes
	observation.StdoutOverflow = stdoutOverflow
	observation.StderrOverflow = stderrOverflow
	observation.OutputOverflow = observation.OutputOverflow || stdoutOverflow || stderrOverflow
	if observation.WaitCompleted && command.ProcessState != nil {
		observation.Usage = processResourceUsage(command.ProcessState)
		observation.ExitObserved = true
		observation.ExitStatus = command.ProcessState.ExitCode()
		if status, ok := command.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			observation.Signal = status.Signal().String()
		}
	}

	observation.Err = processRunError(observation, spec.OverflowPolicy, waitErr, exitObservationErr, terminationErr, cleanupErr, quiescenceErr, pipeErr)
	if beforeStopErr != nil {
		observation.Err = errors.Join(observation.Err, &processError{Code: "process-before-stop-failed", Cause: beforeStopErr})
	}
	if afterStartErr != nil {
		observation.Err = errors.Join(observation.Err, &processError{Code: "process-after-start-failed", Cause: afterStartErr})
	}
	return observation
}

func callBeforeStop(ctx context.Context, pid int, hook func(context.Context, int) error) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = errors.New("cleanup hook panicked")
		}
		err = errors.Join(err, ctx.Err())
	}()
	return hook(ctx, pid)
}

func normalizeProcessSpec(spec Spec) (Spec, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return spec, errors.New("argv must contain an executable")
	}
	for _, argument := range spec.Argv {
		if strings.IndexByte(argument, 0) >= 0 {
			return spec, errors.New("argv must not contain NUL bytes")
		}
	}
	if !filepath.IsAbs(spec.Argv[0]) || filepath.Clean(spec.Argv[0]) != spec.Argv[0] {
		return spec, errors.New("executable must be an absolute clean path")
	}
	if spec.Dir == "" || !filepath.IsAbs(spec.Dir) || filepath.Clean(spec.Dir) != spec.Dir {
		return spec, errors.New("working directory must be an absolute clean path")
	}
	if spec.Timeout <= 0 {
		return spec, errors.New("timeout must be positive")
	}
	if spec.ShutdownTimeout == 0 {
		spec.ShutdownTimeout = defaultProcessShutdownLimit
	}
	if spec.ShutdownTimeout < 0 {
		return spec, errors.New("shutdown timeout must be positive")
	}
	if spec.InputLimit == 0 {
		spec.InputLimit = defaultProcessInputLimit
	}
	if spec.OutputLimit == 0 {
		spec.OutputLimit = DefaultOutputLimit
	}
	if spec.StderrLimit == 0 {
		spec.StderrLimit = spec.OutputLimit
	}
	if spec.InputLimit < 0 || spec.OutputLimit < 0 || spec.StderrLimit < 0 {
		return spec, errors.New("stream limits must be positive")
	}
	if spec.OverflowPolicy != OverflowFail && spec.OverflowPolicy != OverflowTruncate {
		return spec, errors.New("unknown overflow policy")
	}
	if len(spec.Stdin) > spec.InputLimit {
		return spec, errors.New("stdin exceeds its byte limit")
	}
	if spec.Dialogue != nil && len(spec.Stdin) != 0 {
		return spec, errors.New("dialogue and fixed stdin are mutually exclusive")
	}
	if err := validateProcessEnvironment(spec.Env); err != nil {
		return spec, err
	}
	return spec, nil
}

func validateProcessEnvironment(environment []string) error {
	seen := make(map[string]struct{}, len(environment))
	for _, entry := range environment {
		separator := strings.IndexByte(entry, '=')
		if separator <= 0 || strings.IndexByte(entry, 0) >= 0 {
			return errors.New("environment entries must be NAME=VALUE without NUL bytes")
		}
		name := entry[:separator]
		if _, exists := seen[name]; exists {
			return fmt.Errorf("environment contains duplicate name %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func drainProcessPipe(reader *os.File, destination io.Writer, result chan<- error) {
	_, err := io.Copy(destination, reader)
	if closer, ok := destination.(io.Closer); ok {
		_ = closer.Close()
	}
	_ = reader.Close()
	result <- err
}

func waitForProcessEvent(done <-chan error, deadline time.Time) (error, bool) {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		select {
		case err := <-done:
			return err, true
		default:
			return nil, false
		}
	}
	timer := time.NewTimer(remaining)
	defer stopProcessTimer(timer)
	select {
	case err := <-done:
		return err, true
	case <-timer.C:
		return nil, false
	}
}

func waitForProcessDrains(stdout, stderr *os.File, drains <-chan error, deadline time.Time) (error, bool) {
	var result error
	for completed := 0; completed < 2; completed++ {
		select {
		case err := <-drains:
			if err != nil && !errors.Is(err, errProcessOutputOverflow) {
				result = errors.Join(result, err)
			}
			continue
		default:
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			_ = stdout.Close()
			_ = stderr.Close()
			return result, false
		}
		timer := time.NewTimer(remaining)
		select {
		case err := <-drains:
			stopProcessTimer(timer)
			if err != nil && !errors.Is(err, errProcessOutputOverflow) {
				result = errors.Join(result, err)
			}
		case <-timer.C:
			_ = stdout.Close()
			_ = stderr.Close()
			return result, false
		}
	}
	return result, true
}

func processRunError(observation Observation, policy OverflowPolicy, waitErr, exitObservationErr, terminationErr, cleanupErr, quiescenceErr, pipeErr error) error {
	var result error
	if exitObservationErr != nil {
		result = errors.Join(result, &processError{Code: "process-exit-observation-failed", Cause: exitObservationErr})
	}
	if !observation.WaitCompleted {
		result = errors.Join(result, &processError{Code: "process-wait-timeout"})
	}
	var exitError *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitError) {
		result = errors.Join(result, &processError{Code: "process-wait-failed", Cause: waitErr})
	}
	if observation.Cancelled {
		result = errors.Join(result, &processError{Code: "process-cancelled"})
	}
	if observation.TimedOut {
		result = errors.Join(result, &processError{Code: "process-timeout"})
	}
	if observation.OutputOverflow && policy == OverflowFail {
		result = errors.Join(result, &processError{Code: "process-output-overflow"})
	}
	if terminationErr != nil || cleanupErr != nil || quiescenceErr != nil {
		result = errors.Join(result, &processError{Code: "process-cleanup-failed", Cause: errors.Join(terminationErr, cleanupErr, quiescenceErr)})
	}
	if !observation.PipesDrained || pipeErr != nil {
		result = errors.Join(result, &processError{Code: "process-pipe-drain-failed", Cause: pipeErr})
	}
	return result
}

func closeProcessPipes(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}

func stopProcessTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
