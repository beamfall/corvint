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
