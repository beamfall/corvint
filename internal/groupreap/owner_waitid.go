//go:build darwin || linux

package groupreap

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// OwnerAvailable reports whether this platform has the unreaped-leader owner.
func OwnerAvailable() bool { return true }

func containLeader(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
}

// defaultRetirementMode is quiet-first on both platforms: Linux proves quiet
// from /proc because signal 0 cannot tell zombies from live members.
func defaultRetirementMode() RetirementMode { return RequirePreReapQuiet }

func defaultPrimitives() Primitives {
	probe := func(leader int) (Probe, error) {
		switch err := syscall.Kill(-leader, 0); err {
		case nil:
			return ProbeLive, nil
		case syscall.EPERM:
			return ProbeQuiet, nil
		case syscall.ESRCH:
			return ProbeAbsent, nil
		default:
			return ProbeLive, err
		}
	}
	quiet := probe
	if runtime.GOOS == "linux" {
		quiet = func(leader int) (Probe, error) { return procGroupQuiet("/proc", leader, os.Getpid()) }
	}
	return Primitives{
		WaitExit: leaderUnreaped,
		KillGroup: func(leader int) error {
			err := syscall.Kill(-leader, syscall.SIGKILL)
			if runtime.GOOS != "linux" && (err == syscall.EPERM || err == syscall.ESRCH) {
				return nil
			}
			return err
		},
		ProbeGroup: probe,
		QuietProof: quiet,
		Reap:       func(command *exec.Cmd) error { return command.Wait() },
	}
}
