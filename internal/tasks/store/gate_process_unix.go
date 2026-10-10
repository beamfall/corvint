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

func killGate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// killGateGroup kills what remains of a gate's process group after the
// gate itself has exited, such as a descendant it left running in the
// background. A group with a live member keeps its id, so the id cannot
// name another group while the descendant survives.
func killGateGroup(group int) {
	if group > 0 {
		_ = syscall.Kill(-group, syscall.SIGKILL)
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
