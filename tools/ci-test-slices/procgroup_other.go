//go:build !unix

package main

import "os/exec"

// inGroup cannot reach descendants here: cancellation kills only the process
// it started (no job object), so a compiler or test binary that `go` started
// can outlive it.
func inGroup(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Kill() }
}
