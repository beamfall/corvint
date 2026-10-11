//go:build unix

package store

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// containGate starts the gate in its own process group, so a timeout or
// output overflow kills everything it spawned.
func containGate(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// signalGateGroup is syscall.Kill; tests observe the store's own group
// signals.
var signalGateGroup = syscall.Kill

// killGate SIGKILLs a bounded post-gate command's group. Its only caller
// signals before that command is reaped (startBounded), so the group id
// still names that command's group. No store path signals a gate's group
// after execute has reaped its leader: reapGate sweeps the group while the
// exited leader is unreaped where waitid allows, and elsewhere no group
// signal is sent (V1-1047, PGO-V0-009).
func killGate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = signalGateGroup(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

var signalNames = map[syscall.Signal]string{syscall.SIGKILL: "SIGKILL", syscall.SIGTERM: "SIGTERM", syscall.SIGINT: "SIGINT", syscall.SIGSEGV: "SIGSEGV", syscall.SIGABRT: "SIGABRT", syscall.SIGHUP: "SIGHUP", syscall.SIGPIPE: "SIGPIPE", syscall.SIGBUS: "SIGBUS", syscall.SIGQUIT: "SIGQUIT"}

// gateSignal names the signal that ended the gate, or "".
func gateSignal(state *os.ProcessState) string {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return ""
	}
	if name, ok := signalNames[status.Signal()]; ok {
		return name
	}
	return "SIG" + strconv.Itoa(int(status.Signal()))
}
