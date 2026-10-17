//go:build darwin || linux

package cli

import (
	"os"
	"syscall"
)

// batchSignals cancel a plan batches read; its Git runs are killed and the
// throw-away directory removed before it refuses (NTP-V0-022).
var batchSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// batchProcAttr starts each Git run in its own process group, so the deadline
// kills any descendant with it.
func batchProcAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }

func batchKill(p *os.Process) error { return syscall.Kill(-p.Pid, syscall.SIGKILL) }
