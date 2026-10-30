// SPDX-License-Identifier: Apache-2.0

//go:build unix

package corpusbridge

import (
	"syscall"
	"time"
)

// cpuTime returns this process's user plus system CPU time so far.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
