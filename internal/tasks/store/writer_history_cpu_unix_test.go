//go:build unix

package store

import (
	"syscall"
	"time"
)

// processCPU is this process's user plus system CPU time, all threads
// included. Unlike wall time, it is not stretched by other processes' load.
func processCPU() (time.Duration, bool) {
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		return 0, false
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano()), true
}
