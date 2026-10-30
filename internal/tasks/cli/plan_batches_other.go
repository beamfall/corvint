//go:build !darwin && !linux

package cli

import (
	"os"
	"syscall"
)

// batchSignals cancel a plan batches read (NTP-V0-022).
var batchSignals = []os.Signal{os.Interrupt}

// batchProcAttr has no process group here; WaitDelay still bounds the wait.
func batchProcAttr() *syscall.SysProcAttr { return nil }

func batchKill(p *os.Process) error { return p.Kill() }
