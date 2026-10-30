package store

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateObservationOutputMax bounds what one worktree observation keeps of a
// Git command's output (V1-3009). A status listing past it already shows the
// worktree is not clean; any other output past it is refused.
const gateObservationOutputMax = 1 << 20

// gateObservationConfig disables the Git configuration that would run a
// program during a read-only worktree observation (V1-3009): a filesystem
// monitor hook, hooks and credential helpers. Clean and process filters stay,
// because a racily clean filtered file would otherwise read as modified; the
// deadline and the process-group kill bound them instead.
var gateObservationConfig = []string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "credential.helper="}

// observeGit runs one worktree observation in root under the sanitized Git
// environment, with executable configuration disabled and its output capped
// at gateObservationOutputMax (V1-3009). Git runs in its own process group,
// which is killed when ctx ends or the output overflows, so a hanging hook
// or filter and its descendants cannot outlive the observation. truncated
// reports an overflow; an ended ctx is returned as its error.
func observeGit(ctx context.Context, root string, args ...string) (out []byte, truncated bool, err error) {
	if err := ctx.Err(); err != nil {
		return nil, false, observationEnded(ctx)
	}
	cmd := exec.Command("git", append(append([]string{}, gateObservationConfig...), args...)...)
	cmd.Dir, cmd.Env, cmd.Stdin = root, gitEnvironment(), nil
	captured := &cappedOutput{max: gateObservationOutputMax}
	cmd.Stdout = captured
	cmd.WaitDelay = gateWaitDelay
	containGate(cmd)
	captured.kill = func() { stopGate(cmd) }
	if err := cmd.Start(); err != nil {
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	stop := context.AfterFunc(ctx, func() { stopGate(cmd) })
	err = reapGate(cmd)
	stopped := !stop()
	captured.mu.Lock()
	defer captured.mu.Unlock()
	switch {
	case stopped:
		return nil, false, observationEnded(ctx)
	case captured.overflow:
		return captured.buf.Bytes(), true, nil
	case err != nil:
		return nil, false, wire.Errorf(wire.CodeUnsupported, "git", "git observation failed: %v", err)
	}
	return captured.buf.Bytes(), false, nil
}

// observationEnded is the error of an observation whose context ended: the
// caller's own cancellation as itself, the observation's deadline as
// LIMIT_EXCEEDED.
func observationEnded(ctx context.Context) error {
	if cause := context.Cause(ctx); cause != nil && cause != context.DeadlineExceeded {
		return cause
	}
	return wire.Errorf(wire.CodeLimitExceeded, "git", "the worktree observation did not finish before its deadline")
}

// gateObservation bounds the pre- and post-gate worktree observations by the
// gate's own timeout as well as ctx (V1-3009).
func gateObservation(ctx context.Context, timeoutSeconds int64) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
}

// observedGitRunner is a gitRunner whose commands observe under ctx and
// refuse an overflowing output.
func observedGitRunner(ctx context.Context) gitRunner {
	return func(root string, args ...string) ([]byte, error) {
		out, truncated, err := observeGit(ctx, root, args...)
		if err == nil && truncated {
			err = wire.Errorf(wire.CodeLimitExceeded, "git", "git observation output exceeds %d bytes", gateObservationOutputMax)
		}
		return out, err
	}
}
