//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"os"
	"os/signal"
	"sync"
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

var brokenPipeOnce sync.Once

// notifyBrokenPipe makes a write to a closed stdout or stderr fail with EPIPE instead of killing
// the process, so a hook adapter can still name the cause (AHI-044). A caught signal, unlike an
// ignored one, reverts to its default in the Git children the adapter spawns.
func notifyBrokenPipe() {
	brokenPipeOnce.Do(func() { signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE) })
}
