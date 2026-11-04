//go:build unix

package main

import (
	"os"
	"runtime"
	"syscall"
)

// peakRSSKiB reads the child's maximum resident set size; macOS reports bytes, the other Unix
// systems Go supports report KiB.
func peakRSSKiB(state *os.ProcessState) (int64, bool) {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage == nil {
		return 0, false
	}
	peak := int64(usage.Maxrss)
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		peak /= 1024
	}
	return peak, true
}

func signalNumber(state *os.ProcessState) int {
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return int(status.Signal())
	}
	return 0
}
