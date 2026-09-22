//go:build unix

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// ownedByInvokingUser reports whether info's owner is the current user.
func ownedByInvokingUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid()
}

// lockExclusive takes an exclusive advisory lock on f, without waiting when
// nonblocking is set.
func lockExclusive(f *os.File, nonblocking bool) error {
	how := syscall.LOCK_EX
	if nonblocking {
		how |= syscall.LOCK_NB
	}
	return syscall.Flock(int(f.Fd()), how)
}

func unlock(f *os.File) { syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// signalExit returns 128+signal when the command was killed by a signal.
func signalExit(exit *exec.ExitError) (int, bool) {
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return 0, false
	}
	return 128 + int(status.Signal()), true
}
