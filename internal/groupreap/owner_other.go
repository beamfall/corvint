//go:build !darwin && !linux

package groupreap

import "os/exec"

// OwnerAvailable reports whether this platform has the unreaped-leader owner.
func OwnerAvailable() bool { return false }

func containLeader(*exec.Cmd) {}

func defaultPrimitives() Primitives {
	return Primitives{
		WaitExit:   func(int) error { return ErrOwnerUnavailable },
		KillGroup:  func(int) error { return ErrOwnerUnavailable },
		ProbeGroup: func(int) (Probe, error) { return ProbeLive, ErrOwnerUnavailable },
		Reap:       func(*exec.Cmd) error { return ErrOwnerUnavailable },
	}
}
