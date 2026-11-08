// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"syscall"
	"time"
)

func planCPUTime() (time.Duration, error) {
	var created, exited, kernel, user syscall.Filetime
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, err
	}
	if err := syscall.GetProcessTimes(process, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	ticks := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
	ticks += uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
	return time.Duration(ticks) * 100 * time.Nanosecond, nil
}
