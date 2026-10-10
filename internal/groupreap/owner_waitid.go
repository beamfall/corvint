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

// residueProof is WaitResidue's pre-reap observation; tests observe or inject
// it. It is the owner's quiet proof, except that a Darwin quiet result (signal
// 0 EPERM) can also mean live members this process may not signal, so it is
// accepted only when the process table shows every member of the group as a
// zombie (V1-1036). Anything unproved stays residue.
var residueProof = func(leader int) (Probe, error) {
	probe, err := defaultPrimitives().QuietProof(leader)
	if err != nil || probe != ProbeQuiet || runtime.GOOS != "darwin" {
		return probe, err
	}
	return tableGroupQuiet(residueTable, leader)
}

// residueTable is readProcessTable; tests inject a member signal 0 cannot see.
var residueTable = readProcessTable

// tableGroupQuiet reports ProbeQuiet only when read lists leader and every
// process of leader's group is a zombie. It sends no signal.
func tableGroupQuiet(read func() (map[int]Process, error), leader int) (Probe, error) {
	table, err := read()
	if err != nil {
		return ProbeLive, err
	}
	if p, ok := table[leader]; !ok || p.PGID != leader {
		return ProbeLive, errProcessTable
	}
	for _, p := range table {
		if p.PGID == leader && p.State != StateZombie {
			return ProbeLive, nil
		}
	}
	return ProbeQuiet, nil
}
