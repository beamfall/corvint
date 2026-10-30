package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

const (
	childDeadline       = 10 * time.Second
	wholeDeadline       = 30 * time.Second
	cleanupDeadline     = 250 * time.Millisecond
	maxChildStdoutBytes = 8 << 20
	maxChildStderrBytes = 64 << 10
	maxScanStdoutBytes  = 128 << 20
	maxChildren         = 4_096
	maxCatChildBytes    = 4_194_304
	maxCatTotalBytes    = 8_388_608
)

var commonArguments = []string{
	"--no-optional-locks",
	"--literal-pathspecs",
	"-c", "core.fsmonitor=false",
	"-c", "core.untrackedCache=false",
	"-c", "core.excludesFile=",
	"-c", "credential.helper=",
	"-c", "submodule.recurse=false",
	"-c", "fetch.recurseSubmodules=false",
	"-c", "protocol.allow=never",
	"-c", "protocol.file.allow=never", "-c", "advice.graftFileDeprecated=false",
}

type boundedCapture struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    uint64
	seen     uint64
	exceeded bool
	onLimit  func()
	once     sync.Once
}

func (capture *boundedCapture) Write(value []byte) (int, error) {
	capture.mu.Lock()
	remaining := uint64(0)
	if capture.seen < capture.limit {
		remaining = capture.limit - capture.seen
	}
	if uint64(len(value)) <= remaining {
		_, _ = capture.buffer.Write(value)
		capture.seen += uint64(len(value))
		capture.mu.Unlock()
		return len(value), nil
	}
	if remaining != 0 {
		_, _ = capture.buffer.Write(value[:int(remaining)])
	}
	capture.seen = capture.limit + 1
	capture.exceeded = true
	capture.mu.Unlock()
	capture.once.Do(func() {
		if capture.onLimit != nil {
			capture.onLimit()
		}
	})
	return len(value), nil
}

func (capture *boundedCapture) result() ([]byte, uint64, bool) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.buffer.Bytes()...), capture.seen, capture.exceeded
}

type commandResult struct {
	stdout                       []byte
	exit                         int
	executableRefusedBeforeStart bool
}

type containmentOutcome struct {
	waitErr             error
	timedOutOrCancelled bool
	residueAfterLeader  bool
	cleanupProven       bool
	groupQuiescent      bool
	observationFailed   bool
	executableStable    bool
}

func (authority *Authority) run(ctx context.Context, cwd string, inserted, tail []string) (commandResult, *Failure) {
	if len(tail) > 0 && tail[0] == "status" {
		run := func(ctx context.Context, directory string, limit int, args ...string) ([]byte, error) {
			result, failure := authority.runWithInput(ctx, directory, nil, args, nil, uint64(min(limit, maxChildStdoutBytes)), false)
			if failure != nil {
				return nil, failure
			}
			if result.exit != 0 {
				return nil, unavailable(ReasonUnavailable)
			}
			return result.stdout, nil
		}
		raw, err := gitstatus.Status(ctx, cwd, maxChildStdoutBytes, run, tail...)
		if err != nil {
			var failure *Failure
			if errors.As(err, &failure) {
				return commandResult{}, failure
			}
			return commandResult{}, unavailable(ReasonUnavailable)
		}
		return commandResult{stdout: raw}, nil
	}
	return authority.runWithInput(ctx, cwd, inserted, tail, nil, maxChildStdoutBytes, false)
}

func (authority *Authority) runWithInput(
	ctx context.Context,
	cwd string,
	inserted, tail []string,
	stdin io.Reader,
	stdoutLimit uint64,
	catFile bool,
) (commandResult, *Failure) {
	if authority.budget == nil || !authority.budget.reserveChild() {
		authority.objectFailed = true
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	arguments := make([]string, 0, len(commonArguments)+len(inserted)+len(tail))
	arguments = append(arguments, commonArguments...)
	arguments = append(arguments, inserted...)
	arguments = append(arguments, tail...)

	childContext, cancel := context.WithTimeout(authority.life, childDeadline)
	stopCaller := func() bool { return true }
	if ctx != nil {
		stopCaller = context.AfterFunc(ctx, cancel)
	}
	defer func() {
		stopCaller()
		cancel()
	}()

	command := exec.Command(authority.executable, arguments...)
	command.Args[0] = authority.executable
	command.Dir = cwd
	command.Env = append([]string(nil), authority.environment...)
	if gitstatus.Isolated(ctx) {
		command.Env = append(command.Env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(cwd))
	}
	command.Stdin = stdin
	command.WaitDelay = cleanupDeadline
	if ctx != nil && ctx.Err() != nil {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if authority.life.Err() != nil {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if !verifyExecutableBinding(authority.executable, authority.executableFile, authority.executableBefore) {
		authority.drifted = true
		return commandResult{executableRefusedBeforeStart: true}, unavailable(ReasonUnavailable)
	}
	containment, ok := prepareContainment(command)
	if !ok {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	defer containment.close()
	stdout := &boundedCapture{limit: stdoutLimit, onLimit: cancel}
	stderr := &boundedCapture{limit: maxChildStderrBytes, onLimit: cancel}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil || !containment.attach(command.Process) {
		if command.Process != nil {
			containment.force()
			_, _ = command.Process.Wait()
		}
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}

	outcome := containment.wait(command, childContext, stdin, func() bool {
		return verifyExecutableBinding(authority.executable, authority.executableFile, authority.executableBefore)
	})
	waitErr := outcome.waitErr
	timedOutOrCancelled := outcome.timedOutOrCancelled
	if !outcome.cleanupProven {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if !outcome.executableStable {
		authority.drifted = true
	}
	if !outcome.groupQuiescent {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if !outcome.executableStable {
		return commandResult{}, unavailable(ReasonUnavailable)
	}
	stdoutBytes, observedStdout, stdoutExceeded := stdout.result()
	_, _, stderrExceeded := stderr.result()
	if !authority.budget.addStdout(observedStdout) {
		authority.objectFailed = true
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if catFile && !authority.budget.addCatOutput(observedStdout) {
		authority.objectFailed = true
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	callerCancelled := ctx != nil && ctx.Err() != nil
	if timedOutOrCancelled || outcome.residueAfterLeader || outcome.observationFailed || callerCancelled || childContext.Err() != nil || stdoutExceeded || stderrExceeded {
		return commandResult{}, unavailable(ReasonObjectUnavailable)
	}
	if waitErr == nil {
		return commandResult{stdout: stdoutBytes, exit: 0}, nil
	}
	var exitError *exec.ExitError
	if errors.As(waitErr, &exitError) {
		return commandResult{stdout: stdoutBytes, exit: exitError.ExitCode()}, nil
	}
	return commandResult{}, unavailable(ReasonObjectUnavailable)
}

func closeReader(reader io.Reader) {
	closer, ok := reader.(io.Closer)
	if !ok || closer == nil {
		return
	}
	_ = closer.Close()
}
