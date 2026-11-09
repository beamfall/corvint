//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package main

import "os"

func terminationSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// dogfoodSignalCodes are the exit statuses the former dogfood scripts' traps
// returned for each signal (DCW-V0-020).
func dogfoodSignalCodes() map[os.Signal]int {
	return map[os.Signal]int{os.Interrupt: 130}
}
