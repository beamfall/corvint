//go:build unix

package perfbudget

import (
	"os"
	"runtime"
	"syscall"
)

const rssUnknownReason = "peak resident set NOT_OBSERVED: the process reported no resource usage"

// peakRSS is the exited process's maximum resident set in bytes: getrusage reports bytes on
// Darwin and kilobytes on the other Unix systems.
func peakRSS(state *os.ProcessState) *int64 {
	if state == nil {
		return nil
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage == nil {
		return nil
	}
	value := int64(usage.Maxrss)
	if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
		value *= 1024
	}
	return &value
}
