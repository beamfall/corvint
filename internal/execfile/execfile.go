// Package execfile writes files that the same process will execute.
//
// A file written and then executed by one process can fail to start with
// ETXTBSY ("text file busy") on Linux when another goroutine forks between
// the writer's open and close: the child inherits the write descriptor and
// keeps the file open for writing until its own exec closes it (the
// descriptor is close-on-exec). Retrying the start only shortens the window
// (V1-2433); it does not remove the race, and a starved child can hold the
// descriptor past any retry budget (V1-2476, V1-2550).
//
// Go's syscall.ForkLock is write-locked across every fork made through
// os/exec and os.StartProcess, so a writer that holds its read side while the
// descriptor is open excludes every concurrent fork from inheriting it: a
// fork that completed before the open never saw the descriptor, and a fork
// that starts after the close cannot. The writer therefore holds
// syscall.ForkLock.RLock from open to close. Forks block only for the length
// of one file write, and the lock exists on every GOOS Go supports.
package execfile

import (
	"io/fs"
	"os"
	"syscall"
)

// Guard runs write while holding syscall.ForkLock for reading, so no process
// forked by this program during write inherits a descriptor write opens. The
// callback MUST open and close every descriptor it creates before returning
// and MUST NOT start a process itself: a fork under the read lock deadlocks.
func Guard(write func() error) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return write()
}

// WriteFile is os.WriteFile for a file the process will execute: the write
// descriptor is open only while the fork lock excludes concurrent forks.
func WriteFile(name string, data []byte, perm fs.FileMode) error {
	return Guard(func() error { return os.WriteFile(name, data, perm) })
}
