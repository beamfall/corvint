//go:build unix

package store

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// containGate starts the gate in its own process group, so a timeout or
// output overflow kills everything it spawned.
func containGate(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

func killGate(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// stopGate kills a started lease gate early. Where waitid exists it signals
// only the leader through os.Process, which refuses a reaped leader; the
// group is SIGKILLed by groupreap.Wait while the exited leader is unreaped
// (V1-1047, PGO-V0-008).
func stopGate(cmd *exec.Cmd) { _ = groupreap.Stop(cmd) }

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
