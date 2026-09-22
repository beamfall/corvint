// Package companionrelease builds the optional, darwin/arm64-only companion
// bundle (corvint-console, corvint-dashboard-snapshot, corvint, atm) described by
// docs/specs/public-release-v0.md PUB-V0-002/003/004. It never mutates a
// caller-supplied checkout: every read is either a Git object read against an
// explicit HEAD, or a bounded subprocess run in a scratch directory the
// caller owns. It builds, verifies, and retains a candidate; it never tags,
// pushes, or publishes anything.
package companionrelease

import (
	"context"
	"fmt"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// subprocessTimeout bounds any single tool invocation this package makes.
// Full `go build` invocations get a longer, explicit budget; short
// inspection commands (git rev-parse, go env) use this default.
const subprocessTimeout = 20 * time.Second

// runCaptured runs argv[0] with a closed environment and returns stdout.
// Every subprocess this package starts goes through procgroup.Run so process
// groups are owned and reaped; this is the sole call site.
func runCaptured(ctx context.Context, dir string, env []string, timeout time.Duration, argv ...string) (stdout, stderr []byte, err error) {
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv:            argv,
		Dir:             dir,
		Env:             env,
		Timeout:         timeout,
		ShutdownTimeout: 5 * time.Second,
		OutputLimit:     32 << 20,
	})
	if observation.Err != nil {
		return observation.Stdout, observation.Stderr, fmt.Errorf("%v: %w (stderr=%s)", argv, observation.Err, trimForError(observation.Stderr))
	}
	if !observation.ExitObserved || observation.ExitStatus != 0 {
		return observation.Stdout, observation.Stderr, fmt.Errorf("%v: exit=%d (stderr=%s)", argv, observation.ExitStatus, trimForError(observation.Stderr))
	}
	return observation.Stdout, observation.Stderr, nil
}

func runExpectedExit(ctx context.Context, dir string, env []string, timeout time.Duration, expected int, argv ...string) (stdout, stderr []byte, err error) {
	observation := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: timeout,
		ShutdownTimeout: 5 * time.Second, OutputLimit: 32 << 20})
	if observation.Err != nil {
		return observation.Stdout, observation.Stderr, observation.Err
	}
	if !observation.ExitObserved || observation.ExitStatus != expected {
		return observation.Stdout, observation.Stderr, fmt.Errorf("%v: exit=%d, want %d", argv, observation.ExitStatus, expected)
	}
	return observation.Stdout, observation.Stderr, nil
}

// runCapturedStdin is runCaptured plus stdin, for `git cat-file --batch`.
func runCapturedStdin(ctx context.Context, dir string, env []string, timeout time.Duration, stdin []byte, argv ...string) (stdout, stderr []byte, err error) {
	observation := procgroup.Run(ctx, procgroup.Spec{
		Argv:            argv,
		Dir:             dir,
		Env:             env,
		Stdin:           stdin,
		Timeout:         timeout,
		ShutdownTimeout: 5 * time.Second,
		InputLimit:      maxExportTotalBytes + (1 << 20),
		OutputLimit:     maxExportTotalBytes + (1 << 20),
	})
	if observation.Err != nil {
		return observation.Stdout, observation.Stderr, fmt.Errorf("%v: %w (stderr=%s)", argv, observation.Err, trimForError(observation.Stderr))
	}
	if !observation.ExitObserved || observation.ExitStatus != 0 {
		return observation.Stdout, observation.Stderr, fmt.Errorf("%v: exit=%d (stderr=%s)", argv, observation.ExitStatus, trimForError(observation.Stderr))
	}
	return observation.Stdout, observation.Stderr, nil
}

func trimForError(b []byte) string {
	const max = 2000
	if len(b) > max {
		return string(b[:max]) + "...(truncated)"
	}
	return string(b)
}
