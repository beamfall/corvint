//go:build darwin || linux

package taskman

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

func runRead(ctx context.Context, binary, root string, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "LANG=C"}
	containRead(cmd)
	out, stderr := &limitedBuffer{limit: 16 << 20}, &limitedBuffer{limit: 1 << 20}
	cmd.Stdout = out
	cmd.Stderr = stderr
	e := groupreap.Run(cmd)
	if e != nil || ctx.Err() != nil || out.overflow || stderr.overflow || len(stderr.raw) > 0 {
		return nil, errors.New("native executor read failed or exceeded bound")
	}
	return out.raw, nil
}

// runPortable runs the independent portable consumer (TCB-V0-017) with the
// native read's environment, except that the consumer resolves git from the
// gate's own git first (portablePath), and returns its output and exit
// status. The consumer stays in the caller's process group, so a supervisor
// that kills the gate's group (the Tasks runner on timeout or cancellation)
// also stops it and its descendants. Its own leader is killed at the
// consumer's whole-repository verification budget, and the pipe drain is
// bounded as for a read. A signal, an unbounded drain or an overflow is an
// error.
func runPortable(ctx context.Context, binary, root string, args []string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=" + portablePath(), "LC_ALL=C", "LANG=C"}
	cmd.WaitDelay = time.Minute
	out, stderr := &limitedBuffer{limit: 16 << 20}, &limitedBuffer{limit: 1 << 20}
	cmd.Stdout = out
	cmd.Stderr = stderr
	e := cmd.Run()
	var exit *exec.ExitError
	if e != nil && !errors.As(e, &exit) || ctx.Err() != nil || out.overflow || stderr.overflow || cmd.ProcessState.ExitCode() < 0 {
		return nil, 0, errors.New("portable consumer failed or exceeded bound")
	}
	return out.raw, cmd.ProcessState.ExitCode(), nil
}

// portableFixedPath is the PATH every native read's child sees and the whole
// of what the portable consumer saw before TCB-V0-043.
const portableFixedPath = "/usr/bin:/bin"

// portablePath is the PATH the portable consumer resolves git from: the
// directory of the git the gate's own reads and the native verifier resolve
// (exec.Command looks "git" up in this process's PATH, never in the child's),
// ahead of the fixed /usr/bin:/bin, so both legs of the cross-check run one
// Git (TCB-V0-043). The consumer runs nothing but git, and that git is already
// the one the native leg trusted. An unresolved or relative lookup, or a git
// already under the fixed path, keeps the fixed path alone. The directory is
// an operator fact and never enters the document.
func portablePath() string {
	git, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(git) {
		return portableFixedPath
	}
	dir := filepath.Dir(git)
	if dir == "/usr/bin" || dir == "/bin" || strings.ContainsAny(dir, ":\x00") {
		return portableFixedPath
	}
	return dir + ":" + portableFixedPath
}

// containRead owns the read's process group and bounds its pipe drain.
func containRead(cmd *exec.Cmd) {
	// Cancellation stops the leader; groupreap.Run sweeps the group before the reap.
	groupreap.Contain(cmd)
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	cmd.WaitDelay = time.Minute
}

func openInput(path string) (*os.File, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(fd), path), nil
}
