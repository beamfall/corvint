package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

const (
	sourceBlobLimit = 1048576
	sourceGitPath   = "/usr/bin/git"
	// sourceShutdownLimit bounds TERM, KILL, reap and stream drain for one
	// stopped Git command (procgroup.Spec.ShutdownTimeout).
	sourceShutdownLimit = 1500 * time.Millisecond
)

var (
	sourceGitOptions     = []string{"--no-pager", "--no-replace-objects", "--literal-pathspecs", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}
	sourceGitEnvironment = []string{"PATH=/usr/bin:/bin", "HOME=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C"}
)

// sourceGitExecutableKey carries an in-process test substitute for
// /usr/bin/git through the context; no command-line path sets it.
type sourceGitExecutableKey struct{}

type sourceRefusal string

func (e sourceRefusal) Error() string { return string(e) }

// sourceGit runs one sanitized Git read in an owned process group: cancellation
// sends TERM to the group, then KILL, and reaps descendants before returning.
// errSourceOutputOverflow is sourceGit's error when Git's stdout exceeded the
// caller's limit. It lets a blob read tell a budget refusal from a failed read
// (V1-2258); callers that do not check it see an ordinary error.
var errSourceOutputOverflow = errors.New("git output exceeded its byte limit")

func sourceGit(ctx context.Context, root string, limit int, arguments ...string) ([]byte, error) {
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, sourceRefusal("repository-root")
	}
	executable, _ := ctx.Value(sourceGitExecutableKey{}).(string)
	if executable == "" {
		executable = sourceGitPath
	}
	argv := append(append([]string{executable}, sourceGitOptions...), append([]string{"-C", directory}, arguments...)...)
	grace, shutdown := sourceShutdown(ctx)
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: directory, Env: sourceGitEnvironment, Timeout: sourceDeadline(ctx), ShutdownTimeout: shutdown, TerminateGrace: grace, OutputLimit: limit, StderrLimit: 8192})
	if observation.StdoutOverflow {
		return nil, errSourceOutputOverflow
	}
	if observation.Err != nil || observation.ExitStatus != 0 {
		return nil, sourceRefusal("git-failed")
	}
	return observation.Stdout, nil
}

func sourceGitText(ctx context.Context, root string, limit int, arguments ...string) (string, error) {
	raw, err := sourceGit(ctx, root, limit, arguments...)
	return strings.TrimSuffix(string(raw), "\n"), err
}

// sourceGitDeadline is the frozen 10-second bound on one source view or handoff
// capture and on each Git command it runs (ESV-V0-003).
const sourceGitDeadline = 10 * time.Second

// sourceDeadlineKey carries an in-process test replacement for sourceGitDeadline
// through the context, so tests that verify refusal semantics rather than latency
// cannot have host load expire the bound first (decision 0082, V1-1302); no
// command-line path sets it.
type sourceDeadlineKey struct{}

// sourceDeadline is the bound under ctx: the sourceDeadlineKey replacement if set,
// else sourceGitDeadline.
func sourceDeadline(ctx context.Context) time.Duration {
	if bound, ok := ctx.Value(sourceDeadlineKey{}).(time.Duration); ok {
		return bound
	}
	return sourceGitDeadline
}

// sourceTerminateGraceKey carries an in-process test grace for a stopped Git
// command to handle SIGTERM before SIGKILL, added to sourceShutdownLimit, so a
// test that witnesses TERM handling does not race host scheduling against
// procgroup's 100 ms default (V1-2319); no command-line path sets it.
type sourceTerminateGraceKey struct{}

// sourceShutdown returns the TERM grace (zero keeps procgroup's default) and
// the shutdown bound under ctx.
func sourceShutdown(ctx context.Context) (time.Duration, time.Duration) {
	if grace, ok := ctx.Value(sourceTerminateGraceKey{}).(time.Duration); ok && grace > 0 {
		return grace, grace + sourceShutdownLimit
	}
	return 0, sourceShutdownLimit
}
