//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"os"
	"syscall"
)

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

// dogfoodSignalCodes are the exit statuses the former dogfood scripts' traps
// returned for each signal (DCW-V0-020).
func dogfoodSignalCodes() map[os.Signal]int {
	return map[os.Signal]int{syscall.SIGHUP: 129, os.Interrupt: 130, syscall.SIGTERM: 143}
}
