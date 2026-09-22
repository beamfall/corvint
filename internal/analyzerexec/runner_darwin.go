//go:build darwin

package analyzerexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var errOutputLimit = errors.New("analyzer output limit")
var executeContained = func(command *exec.Cmd) error { return command.Run() }

func containedBackendSupported() bool { return true }

type limitedBuffer struct {
	limit    int
	buffer   bytes.Buffer
	overflow bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	if b.overflow || len(value) > b.limit-b.buffer.Len() {
		b.overflow = true
		return 0, errOutputLimit
	}
	return b.buffer.Write(value)
}

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

// runContained is deliberately Darwin-only. sandbox-exec's deny-by-default
// profile permits only the immediate pre-launch verified owner-private staged
// executable and standard I/O; it denies repository access, network, and child
// creation.
// This is an unqualified runtime mechanism, not analyzer-profile support or
// promotion.
func runContained(ctx context.Context, sealed sealedArtifact, plan Plan) (Result, error) {
	if err := sealed.validateForLaunch(ctx); err != nil {
		return Result{CleanupState: CleanupNotRun, Termination: TerminationNotRun, CleanupError: CleanupErrorNone}, err
	}
	stdout, stderr := limitedBuffer{limit: plan.MaxStdoutBytes}, limitedBuffer{limit: plan.MaxStderrBytes}
	profile := sandboxProfile(sealed.launchPath)
	command := containedCommand(ctx, profile, sealed.launchPath)
	command.Stdin = bytes.NewReader(plan.Request)
	command.Stdout, command.Stderr = &stdout, &stderr
	started := time.Now()
	err := executeContained(command)
	termination := TerminationNotRun
	if command.ProcessState != nil {
		termination = TerminationExited
	}
	result := Result{Stdout: cloneBytes(stdout.Bytes()), Stderr: cloneBytes(stderr.Bytes()), Started: command.ProcessState != nil, Completed: command.ProcessState != nil, Elapsed: time.Since(started), Termination: termination}
	if stdout.overflow || stderr.overflow {
		result.Termination = TerminationFailed
		return result, &Error{Failure: Limit}
	}
	if ctx.Err() != nil {
		if !result.Started {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return result, &Error{Failure: Timeout}
			}
			return result, &Error{Failure: Cancelled}
		}
		result.Termination = TerminationCancelled
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Termination = TerminationTimedOut
			return result, &Error{Failure: Timeout}
		}
		return result, &Error{Failure: Cancelled}
	}
	if err != nil {
		result.Termination = TerminationFailed
		return result, &Error{Failure: Process}
	}
	return result, nil
}

// sandboxProfile deliberately names only the immediate pre-launch verified
// staged path, loader paths, and standard streams. In particular, never widen
// /System: its Data mount aliases operator-owned repository paths on Darwin.
// The root directory's own entry list (a literal, never a subpath) is the one
// read a payload needs to start on macOS 26 (decision 0145).
func sandboxProfile(executablePath string) string {
	executable := sandboxLiteral(executablePath)
	directory := sandboxLiteral(filepath.Dir(executablePath))
	return `(version 1)
(deny default)
(deny file-read* (subpath "/System"))
(deny file-read* (subpath "/System/Volumes/Data"))
(deny mach-lookup)
(deny mach-register)
(allow process-exec* (literal ` + executable + `))
(allow file-read* (literal ` + executable + `))
(allow file-map-executable (literal ` + executable + `))
(allow file-read* (subpath ` + directory + `))
(allow file-read* (literal "/dev/fd/0"))
(allow file-write* (literal "/dev/fd/1"))
(allow file-write* (literal "/dev/fd/2"))
(allow file-read* (subpath "/usr/lib"))
(allow file-read-data (literal "/"))
(allow sysctl-read)
`
}

// SandboxProfile returns the exact contained-launch profile for executablePath.
// It launches nothing; it exists so a test outside this package can run the
// identical profile without the 100 ms plan cap and observe a host refusal or
// payload failure directly.
func SandboxProfile(executablePath string) string { return sandboxProfile(executablePath) }

func containedCommand(ctx context.Context, profile, executable string) *exec.Cmd {
	command := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, executable)
	command.Dir = "/"
	command.Env = []string{}
	return command
}

func sandboxLiteral(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + replacer.Replace(value) + `"`
}

func cloneBytes(value []byte) []byte {
	result := make([]byte, len(value))
	copy(result, value)
	return result
}
