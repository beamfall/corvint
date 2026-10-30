//go:build !unix

package main

import (
	"context"
	"errors"
	"os/exec"
)

// runOwned cannot reach descendants here: cancellation kills only the process it
// started (no job object), so a compiler or test binary that `go` started can
// outlive it.
func runOwned(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
		case <-stopped:
		}
	}()
	err := cmd.Wait()
	close(stopped)
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	return err
}
