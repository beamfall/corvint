//go:build darwin || linux

package groupreap

import (
	"os/exec"
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

func defaultPrimitives() Primitives {
	return Primitives{
		WaitExit: leaderUnreaped,
		KillGroup: func(leader int) error {
			err := syscall.Kill(-leader, syscall.SIGKILL)
			if err == syscall.EPERM || err == syscall.ESRCH {
				return nil
			}
			return err
		},
		ProbeGroup: func(leader int) (Probe, error) {
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
		},
		Reap: func(command *exec.Cmd) error { return command.Wait() },
	}
}
