// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package main

import (
	"syscall"
	"time"
)

// rusage returns this process's user and system CPU time so far.
func rusage() [2]time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return [2]time.Duration{}
	}
	return [2]time.Duration{time.Duration(ru.Utime.Nano()), time.Duration(ru.Stime.Nano())}
}
