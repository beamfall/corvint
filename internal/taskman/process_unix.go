//go:build darwin || linux

package taskman

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
// native read's environment and returns its output and exit status. The
// consumer stays in the caller's process group, so a supervisor that kills
// the gate's group (the Tasks runner on timeout or cancellation) also stops
// it and its descendants. Its own leader is killed at the consumer's
// whole-repository verification budget, and the pipe drain is bounded as for
// a read. A signal, an unbounded drain or an overflow is an error.
func runPortable(ctx context.Context, binary, root string, args []string) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "LANG=C"}
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
