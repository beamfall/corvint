// SPDX-License-Identifier: Apache-2.0
//go:build unix

package linecov

import (
	"syscall"
	"time"
)

func planCPUTime() (time.Duration, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, err
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano()), nil
}
