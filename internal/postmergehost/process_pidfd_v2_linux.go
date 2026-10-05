//go:build linux && (amd64 || arm64)

// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package postmergehost

import "syscall"

// The generic pidfd system call numbers, shared by amd64 and arm64.
const (
	sysPidfdSendSignalV2 = 424
	sysPidfdOpenV2       = 434
)

// openProcessV2 returns a pidfd for pid. A pidfd names one process for its
// whole life, so a signal sent through it never reaches a later holder of
// the same PID.
func openProcessV2(pid int) (int, error) {
	fd, _, errno := syscall.Syscall(sysPidfdOpenV2, uintptr(pid), 0, 0)
	if errno != 0 {
		return -1, errno
	}
	syscall.CloseOnExec(int(fd))
	return int(fd), nil
}

// signalProcessV2 sends sig through a pidfd. Signal 0 fails with ESRCH only
// once the process has been reaped, so it confirms that the PID still names
// the pidfd's process.
func signalProcessV2(fd int, sig syscall.Signal) error {
	if _, _, errno := syscall.Syscall6(sysPidfdSendSignalV2, uintptr(fd), uintptr(sig), 0, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

// stopProcessV2 freezes the pidfd's process with SIGSTOP.
func stopProcessV2(fd int) error { return signalProcessV2(fd, syscall.SIGSTOP) }

// closeProcessV2 releases a pidfd from openProcessV2.
func closeProcessV2(fd int) { _ = syscall.Close(fd) }
