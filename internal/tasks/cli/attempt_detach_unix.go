//go:build darwin || linux

package cli

import (
	"os"
	"syscall"
)

// detachAvailable: a detached supervisor runs in a new session (ATR-V0-008).
const detachAvailable = true

// readinessFD is the launcher's readiness pipe in the supervisor.
const readinessFD = 3

func detachAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// protectReadiness keeps the readiness pipe out of every process the
// supervisor starts, so the launcher sees EOF when the supervisor is ready or
// gone rather than when a descendant exits.
func protectReadiness() { syscall.CloseOnExec(readinessFD) }

// openRunFile opens a run file without following a final symlink and without
// blocking on a FIFO; readBounded checks the descriptor's type.
func openRunFile(p string) (*os.File, error) {
	return os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// openRunsDir opens a runs directory, following a link as os.Open does, but
// refuses anything that is not a directory (ENOTDIR) without blocking on a
// FIFO in its place.
func openRunsDir(p string) (*os.File, error) {
	return os.OpenFile(p, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
}
