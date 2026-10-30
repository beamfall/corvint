//go:build darwin || linux

package procgroup

import (
	"os"
	"runtime"
	"syscall"
)

func processResourceUsage(state *os.ProcessState) *ResourceUsage {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok {
		return nil
	}
	rss := int64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		rss *= 1024
	}
	return &ResourceUsage{UserCPUNs: state.UserTime().Nanoseconds(), SystemCPUNs: state.SystemTime().Nanoseconds(), MaxRSSBytes: rss}
}
