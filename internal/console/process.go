package console

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// ProcessError takes precedence over any captured tool envelope.
type ProcessError struct {
	Kind  string
	Cause error
}

func (e *ProcessError) Error() string { return fmt.Sprintf("console process %s: %v", e.Kind, e.Cause) }
func (e *ProcessError) Unwrap() error { return e.Cause }

// ExitError is an observed, ordinary non-zero exit of a fully reaped child.
// It is not a boundary failure: a tool may exit non-zero to carry its own
// REFUSED envelope, and only the caller can decide whether the envelope it
// read is consistent with that status.
type ExitError struct{ Status int }

func (e *ExitError) Error() string { return fmt.Sprintf("exited with status %d", e.Status) }

func runTool(ctx context.Context, dir string, env []string, timeout time.Duration, limit int, argv ...string) ([]byte, []byte, error) {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, nil, &ProcessError{"start", err}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, nil, &ProcessError{"start", err}
	}
	argv = append([]string{path}, argv[1:]...)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if env == nil {
		env = gitEnvironment()
	}
	o := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: timeout, OutputLimit: limit, StderrLimit: 64 << 10})
	var boundary error
	switch {
	case o.OutputOverflow || o.StdoutOverflow || o.StderrOverflow:
		boundary = &ProcessError{"output-limit", errors.New("stdout or stderr exceeded its collection bound")}
	case o.TimedOut:
		boundary = &ProcessError{"timeout", context.DeadlineExceeded}
	case o.Cancelled || ctx.Err() != nil:
		boundary = &ProcessError{"cancelled", context.Canceled}
	case !o.Started || !o.WaitCompleted || !o.PipesDrained || !o.OwnedProcessGroupCleanup:
		boundary = &ProcessError{"cleanup-or-start", o.Err}
	case o.Err != nil:
		boundary = &ProcessError{"infrastructure", o.Err}
	case !o.ExitObserved:
		boundary = &ProcessError{"exit-unobserved", errors.New("the child's exit status was not observed")}
	case o.Signal != "":
		boundary = &ProcessError{"signalled", errors.New("the child was terminated by " + o.Signal)}
	}
	if boundary != nil {
		return o.Stdout, o.Stderr, boundary
	}
	if o.ExitStatus != 0 {
		return o.Stdout, o.Stderr, &ExitError{o.ExitStatus}
	}
	return o.Stdout, o.Stderr, nil
}

func boundaryFailure(err error) bool { var failure *ProcessError; return errors.As(err, &failure) }

func gitEnvironment() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(key, "GIT_") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1")
}
