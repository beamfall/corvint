//go:build unix

package main

import "syscall"

// processGone reports whether pid has exited, by probing it with signal 0.
// It exists so assertGone's descendant-reap assertion (which relies on a
// POSIX signal-0 probe) still compiles on non-unix platforms.
func processGone(pid int) bool {
	return syscall.Kill(pid, 0) == syscall.ESRCH
}
