//go:build darwin || linux

package cli

import "syscall"

// detachAvailable: a detached supervisor runs in a new session (ATR-V0-008).
const detachAvailable = true

// readinessFD is the launcher's readiness pipe in the supervisor.
const readinessFD = 3

func detachAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }

// protectReadiness keeps the readiness pipe out of every process the
// supervisor starts, so the launcher sees EOF when the supervisor is ready or
// gone rather than when a descendant exits.
func protectReadiness() { syscall.CloseOnExec(readinessFD) }
