package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
	"github.com/Beamfall/corvint/internal/worklistadapter"
	"github.com/Beamfall/corvint/internal/workqueue"
)

// Retention is bounded independently from the digest of every observed byte.
type workLimitedBuffer struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
	overrun  func()
	digest   hash.Hash
	count    int64
}

func (buffer *workLimitedBuffer) Write(raw []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.digest == nil {
		buffer.digest = sha256.New()
	}
	_, _ = buffer.digest.Write(raw)
	buffer.count += int64(len(raw))
	remaining := max(0, buffer.limit-len(buffer.data))
	buffer.data = append(buffer.data, raw[:min(remaining, len(raw))]...)
	if len(raw) > remaining && !buffer.exceeded {
		buffer.exceeded = true
		if buffer.overrun != nil {
			buffer.overrun()
		}
	}
	return len(raw), nil
}

func (buffer *workLimitedBuffer) rawIdentity() (workqueue.Count, string) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.digest == nil {
		buffer.digest = sha256.New()
	}
	return workqueue.Count(buffer.count), hex.EncodeToString(buffer.digest.Sum(nil))
}

type workAdapterRunner struct {
	ctx           context.Context
	root, path    string
	env           []string
	source        workSource
	policy        *workqueue.Policy
	total         int64
	identity      []workqueue.ExecutableIdentity
	qualification string
	executables   []*workExecutable
	bound         *workExecutable
	binding       workCorvintExecutableBinding
	verifyTarget  func(context.Context) error
	// failure and failureReceipt keep the last unsuccessful invocation's cause
	// and receipt for in-process diagnosis; the command result carries only
	// the error code and drops the receipt.
	failure        error
	failureReceipt workqueue.AdapterReceipt
}

func newWorkAdapterRunner(ctx context.Context, root, path string, source workSource, policy *workqueue.Policy) (*workAdapterRunner, error) {
	objects, identity, err := workExecutableChain(path, source.adapterRaw, source.adapterMode)
	if err != nil {
		return nil, err
	}
	runner := &workAdapterRunner{ctx: ctx, root: root, path: path, source: source, policy: policy, identity: identity, qualification: "UNQUALIFIED", executables: objects}
	if policy.MappingVersion != worklistadapter.RepositoryMapping {
		return runner, nil
	}
	fail := func(err error) (*workAdapterRunner, error) {
		runner.Close()
		return nil, err
	}
	binding, err := workParseBoundAdapter(source.adapterRaw)
	if err != nil {
		return fail(err)
	}
	if source.qualified == nil {
		return fail(errors.New("missing qualified repository source"))
	}
	bound, _, err := workOpenBoundExecutable(binding.Path, []string{source.qualified.Root, source.qualified.GitDir, source.qualified.CommonDir})
	if err != nil {
		return fail(err)
	}
	runner.bound, runner.binding = bound, binding
	runner.executables = append(runner.executables, bound)
	runner.identity = append(runner.identity, workqueue.ExecutableIdentity{FileSHA256: bound.digest, Mode: fmt.Sprintf("%04o", bound.info.Mode().Perm()), PathSHA256: workqueue.SHA256Hex([]byte(binding.Path))})
	return runner, nil
}

func (runner *workAdapterRunner) qualifyBoundExecutable() error {
	if runner.bound == nil {
		return nil
	}
	if len(runner.env) == 0 {
		return errors.New("missing private adapter environment")
	}
	if err := workVerifyBoundExecutable(runner.ctx, runner.binding, runner.bound, runner.env, runner.root); err != nil {
		return err
	}
	return nil
}

func (runner *workAdapterRunner) Close() {
	for _, object := range runner.executables {
		object.close()
	}
}

func (runner *workAdapterRunner) checkExecutables() error {
	for _, object := range runner.executables {
		if err := object.check(); err != nil {
			if object == runner.bound {
				return fmt.Errorf("%w: %v", errWorkBoundExecutableUnqualified, err)
			}
			return err
		}
	}
	return nil
}

// After the reap, an open capture pipe is reported incomplete once the drain
// has watched workDrainIdle of awake time (WQO-V0-034, WQO-V0-920). Only tick
// intervals delivered near schedule count, so a host stall that delivers EOF
// and the expiry together never misreports a complete capture. workDrainHang
// bounds the wall time of a stalled host; it is a hang detector, not a budget.
const (
	workDrainIdle = time.Second
	workDrainTick = 25 * time.Millisecond
	workDrainHang = time.Minute
)

// workDrainCapture waits for both copy goroutines. It closes the read ends
// through closeReads and reports incomplete capture when the drain expires,
// and closes them silently on cancellation, which the caller reports.
func workDrainCapture(ctx context.Context, drained <-chan error, closeReads func(), ticks, hang <-chan time.Time, now func() time.Time) error {
	var cause error
	awake, last := time.Duration(0), now()
	for pending := 2; pending > 0; pending-- {
		expired := false
		select {
		case drainErr := <-drained:
			if drainErr != nil && cause == nil {
				cause = errors.New("incomplete pipe capture")
			}
			continue
		case <-ticks:
			current := now()
			if gap := current.Sub(last); gap <= 2*workDrainTick {
				awake += gap
			}
			last = current
			if awake < workDrainIdle {
				pending++
				continue
			}
			expired = true
		case <-hang:
			expired = true
		case <-ctx.Done():
		}
		closeReads()
		if expired && cause == nil {
			cause = errors.New("incomplete pipe capture")
		}
		for ; pending > 0; pending-- {
			<-drained
		}
	}
	return cause
}

const workReapTimeout = time.Second

// workWaitResidue is groupreap.WaitResidue; tests observe or inject the
// pre-reap residue observation.
var workWaitResidue = groupreap.WaitResidue

// workStartLive is groupreap.StartLive: it records the adapter's group so an
// exit through groupreap.KillLive retires it, and workWaitResidue releases it
// before the reap (PGO-V0-009). Tests observe it.
var workStartLive = groupreap.StartLive

var errWorkBoundExecutableUnqualified = errors.New("bound Corvint executable is unqualified")

// workTextBusyRetries bounds the start retries of a just-materialized adapter
// (see workStartAdapter).
var workTextBusyRetries = []time.Duration{5 * time.Millisecond, 20 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond}

// workStartAdapter starts a fresh command from build, retrying a start that
// failed with ETXTBSY. Materialization closes each file's write descriptor,
// but a fork made concurrently by another goroutine can inherit it until that
// child's own exec closes it (close-on-exec), so exec of the materialized
// adapter can briefly fail "text file busy" on Linux (V1-2433). Nothing
// started in that case, so a bounded retry is safe; any other failure, a
// start, or cancellation returns at once; cancellation during a retry returns
// the context's error, so the command reports CANCELLED or INPUT_LIMIT rather
// than ADAPTER_FAILED. An exec.Cmd starts at most once, so each attempt builds
// a new one.
func workStartAdapter(ctx context.Context, build func() *exec.Cmd) (*exec.Cmd, error) {
	command := build()
	err := workStartLive(command)
	for _, delay := range workTextBusyRetries {
		if err == nil || !errors.Is(err, syscall.ETXTBSY) {
			break
		}
		if ctx.Err() != nil {
			return command, ctx.Err()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return command, ctx.Err()
		case <-timer.C:
		}
		command = build()
		err = workStartLive(command)
	}
	return command, err
}

func (runner *workAdapterRunner) run(operation string, argv []string, stdoutLimit int) ([]byte, workqueue.AdapterReceipt, error) {
	stdout := &workLimitedBuffer{limit: min(stdoutLimit, int(max(0, int64(workAggregateLimit)-runner.total)))}
	stderr := &workLimitedBuffer{limit: workStderrLimit}
	receipt := workqueue.AdapterReceipt{AdapterBlobOID: runner.source.adapterOID, AdapterFileSHA256: workqueue.SHA256Hex(runner.source.adapterRaw), AdapterMode: runner.source.adapterMode, Argv: append([]string{}, argv...), ContainmentClass: workContainmentClass(), ExecutableQualification: runner.qualification, InterpreterChain: runner.identity, Operation: operation, PolicyID: runner.policy.ID, State: "INCOMPLETE"}
	finish := func(err error) ([]byte, workqueue.AdapterReceipt, error) {
		receipt.StdoutBytes, receipt.StdoutRawSHA256 = stdout.rawIdentity()
		receipt.StderrBytes, receipt.StderrRawSHA256 = stderr.rawIdentity()
		runner.total += int64(receipt.StdoutBytes)
		workqueue.RefreshReceipt(&receipt)
		if err != nil {
			runner.failure, runner.failureReceipt = err, receipt
			return nil, receipt, err
		}
		return stdout.data, receipt, nil
	}
	if err := runner.ctx.Err(); err != nil {
		return finish(err)
	}
	if runner.verifyTarget != nil {
		if err := runner.verifyTarget(runner.ctx); err != nil {
			return finish(err)
		}
	}
	if err := runner.checkExecutables(); err != nil {
		return finish(err)
	}
	if len(runner.env) == 0 {
		return finish(errors.New("missing private adapter environment"))
	}
	commandArgv := append([]string(nil), argv...)
	if runner.bound != nil {
		commandArgv = append([]string{runner.bound.executionPath}, commandArgv...)
	}
	// A nil stdin is /dev/null. Explicit pipes keep Cmd.Wait independent of drains.
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		return finish(err)
	}
	defer outRead.Close()
	defer outWrite.Close()
	errRead, errWrite, err := os.Pipe()
	if err != nil {
		return finish(err)
	}
	defer errRead.Close()
	defer errWrite.Close()
	command, err := workStartAdapter(runner.ctx, func() *exec.Cmd {
		command := exec.Command(runner.path, commandArgv...)
		command.Dir, command.Env = runner.root, append([]string(nil), runner.env...)
		command.Stdout, command.Stderr = outWrite, errWrite
		workContain(command)
		return command
	})
	if err != nil {
		return finish(err)
	}
	_ = outWrite.Close()
	_ = errWrite.Close()
	overrun := make(chan struct{})
	var once sync.Once
	stdout.overrun = func() { once.Do(func() { close(overrun) }) }
	stderr.overrun = stdout.overrun
	drained := make(chan error, 2)
	go func() { _, err := io.Copy(stdout, outRead); drained <- err }()
	go func() { _, err := io.Copy(stderr, errRead); drained <- err }()
	// Residue is observed while the exited leader is held unreaped, and the
	// group is SIGKILLed before the reap, so no numeric group signal can
	// follow the reap and reach a reused group ID (V1-1036).
	var residue bool
	done := make(chan error, 1)
	go func() {
		var err error
		residue, err = workWaitResidue(command)
		done <- err
	}()
	var cause error
	reaped := false
	operationContext, cancel := context.WithCancel(runner.ctx)
	defer cancel()
	select {
	case err = <-done:
		reaped = true
	case <-overrun:
		cause = errors.New("limit")
	case <-operationContext.Done():
		cause = operationContext.Err()
	}
	if !reaped {
		workStop(command)
		reapTimer := time.NewTimer(workReapTimeout)
		select {
		case err = <-done:
			reaped = true
		case <-reapTimer.C:
			cause = errors.New("process reap incomplete")
		}
		reapTimer.Stop()
	}
	// A member still live when the leader exited is residue (WQO-V0-032).
	if reaped && residue && cause == nil {
		cause = errors.New("process residue")
	}
	ticker := time.NewTicker(workDrainTick)
	hang := time.NewTimer(workDrainHang)
	closeReads := func() { _ = outRead.Close(); _ = errRead.Close() }
	if drainCause := workDrainCapture(operationContext, drained, closeReads, ticker.C, hang.C, time.Now); drainCause != nil && cause == nil {
		cause = drainCause
	}
	ticker.Stop()
	hang.Stop()
	if reaped {
		workReceiptExit(&receipt, command)
	}
	if contextErr := operationContext.Err(); contextErr != nil {
		cause = contextErr
	}
	if identityErr := runner.checkExecutables(); identityErr != nil {
		cause = identityErr
	}
	if runner.verifyTarget != nil {
		if targetErr := runner.verifyTarget(runner.ctx); targetErr != nil {
			cause = targetErr
		}
	}
	if stdout.exceeded || stderr.exceeded {
		cause = errors.New("limit")
	}
	if cause != nil {
		return finish(cause)
	}
	if receipt.Signal != nil {
		return finish(err)
	}
	receipt.State = "PASSED"
	if err != nil {
		receipt.State = "FAILED"
	}
	return finish(err)
}

func (runner *workAdapterRunner) commandError(err error, stderr io.Writer) string {
	if errors.Is(err, errWorkBoundExecutableUnqualified) {
		return workUnqualified(stderr, workReasonBoundExecutable)
	}
	if err.Error() == "limit" || errors.Is(err, context.DeadlineExceeded) {
		return "INPUT_LIMIT"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELLED"
	}
	operation := "adapter"
	if runner.failure != nil && errors.Is(err, runner.failure) && runner.failureReceipt.Operation != "" {
		operation = runner.failureReceipt.Operation
	}
	fmt.Fprintf(stderr, "corvint work: ADAPTER_FAILED: %s: %s\n", operation, workAdapterFailure(err))
	return "ADAPTER_FAILED"
}

// workAdapterFailure is the fixed WQO-V0-684 cause text of an ADAPTER_FAILED
// error, naming the bound that expired. It never echoes adapter output, paths
// or other repository-controlled bytes.
func workAdapterFailure(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code >= 0 {
			return fmt.Sprintf("the adapter exited with status %d", code)
		}
		return "the adapter was ended by a signal"
	}
	var start *fs.PathError
	switch {
	case err.Error() == "process residue":
		return "a process of the adapter's group was still live when its leader exited (process residue)"
	case err.Error() == "process reap incomplete":
		return fmt.Sprintf("the adapter's process group was not reaped within %s of its kill (process reap incomplete)", workReapTimeout)
	case err.Error() == "incomplete pipe capture":
		return fmt.Sprintf("an output pipe stayed open for %s of awake drain time, or %s of wall time, after the reap (incomplete pipe capture)", workDrainIdle, workDrainHang)
	case err.Error() == "missing private adapter environment":
		return "the private adapter environment is missing"
	case errors.As(err, &start):
		return "the adapter could not be started"
	}
	return "the adapter invocation failed before its output was captured"
}

func workContainmentClass() string {
	if runtime.GOOS == "darwin" {
		return "DARWIN_PROCESS_GROUP_UNQUALIFIED"
	}
	if runtime.GOOS == "linux" {
		return "LINUX_PROCESS_GROUP_UNQUALIFIED"
	}
	return "UNSUPPORTED"
}
