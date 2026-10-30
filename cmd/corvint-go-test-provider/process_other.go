//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package main

import (
	"context"
	"errors"
	"time"
)

const directCommandOutputLimit = int64(24 << 20)

var (
	errDirectCommandInvalid      = errors.New("direct command: invalid request")
	errDirectCommandUnsupported  = errors.New("direct command: unsupported platform")
	errDirectCommandStart        = errors.New("direct command: start failed")
	errDirectCommandWait         = errors.New("direct command: wait failed")
	errDirectCommandWaitDeadline = errors.New("direct command: wait did not complete before shutdown deadline")
	errDirectCommandCancelled    = errors.New("direct command: cancelled")
	errDirectCommandTimeout      = errors.New("direct command: timed out")
	errDirectCommandOutputLimit  = errors.New("direct command: output limit exceeded")
	errDirectCommandContainment  = errors.New("direct command: containment cleanup failed")
	errDirectCommandPipe         = errors.New("direct command: pipe drain failed")
	errDirectCommandPipeWait     = errors.New("direct command: pipe drain timed out")
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

func runDirectCommand(
	context.Context,
	string,
	[]string,
	[]string,
	string,
	time.Duration,
) (directCommandResult, error) {
	return directCommandResult{ExitCode: -1}, errDirectCommandUnsupported
}

func runAuthorityCommand(
	context.Context,
	string,
	[]string,
	[]string,
	string,
	time.Duration,
	[]byte,
) (directCommandResult, error) {
	return directCommandResult{ExitCode: -1}, errDirectCommandUnsupported
}
