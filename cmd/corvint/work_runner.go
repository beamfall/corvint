package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

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
			return err
		}
	}
	return nil
}

const workDrainTimeout = 250 * time.Millisecond

const workReapTimeout = time.Second

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
	command := exec.Command(runner.path, commandArgv...)
	command.Dir, command.Env = runner.root, append([]string(nil), runner.env...)
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
	command.Stdout, command.Stderr = outWrite, errWrite
	workContain(command)
	if err = command.Start(); err != nil {
		return finish(err)
	}
	defer workKillGroup(command)
	_ = outWrite.Close()
	_ = errWrite.Close()
	overrun := make(chan struct{})
	var once sync.Once
	stdout.overrun = func() { once.Do(func() { close(overrun) }) }
	stderr.overrun = stdout.overrun
	drained := make(chan error, 2)
	go func() { _, err := io.Copy(stdout, outRead); drained <- err }()
	go func() { _, err := io.Copy(stderr, errRead); drained <- err }()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
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
		workKillGroup(command)
		reapTimer := time.NewTimer(workReapTimeout)
		select {
		case err = <-done:
			reaped = true
		case <-reapTimer.C:
			cause = errors.New("process reap incomplete")
		}
		reapTimer.Stop()
	}
	// A reaped leader cannot hold its group alive. Remaining members are residue.
	if workTerminateGroup(command) && cause == nil {
		cause = errors.New("process residue")
	}
	timer := time.NewTimer(workDrainTimeout)
	defer timer.Stop()
	for pending := 2; pending > 0; pending-- {
		select {
		case drainErr := <-drained:
			if drainErr != nil && cause == nil {
				cause = errors.New("incomplete pipe capture")
			}
		case <-timer.C:
			_ = outRead.Close()
			_ = errRead.Close()
			if cause == nil {
				cause = errors.New("incomplete pipe capture")
			}
			<-drained
		}
	}
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

func (runner *workAdapterRunner) commandError(err error) string {
	if err.Error() == "limit" || errors.Is(err, context.DeadlineExceeded) {
		return "INPUT_LIMIT"
	}
	if errors.Is(err, context.Canceled) {
		return "CANCELLED"
	}
	return "ADAPTER_FAILED"
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
