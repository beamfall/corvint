//go:build darwin || linux

package groupreap

import (
	"context"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

// hookResidueProof records, for each residue proof, whether the leader was
// still held unreaped (nil) and, when inject is non-nil, replaces the result.
func hookResidueProof(t *testing.T, inject error) *[]error {
	t.Helper()
	previous := residueProof
	t.Cleanup(func() { residueProof = previous })
	var held []error
	residueProof = func(leader int) (Probe, error) {
		held = append(held, leaderNotReaped(leader))
		if inject != nil {
			return ProbeLive, inject
		}
		return previous(leader)
	}
	return &held
}

// V1-1036 / WQO-V0-032: residue is proved while the exited leader is held
// unreaped, then the group is SIGKILLed before the reap.
func TestWaitResidueObservesAndSignalsBeforeReap(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		inject       error
		residue      bool
	}{
		{"live member", "sleep 30 & exit 0", nil, true},
		{"quiet group", "exit 0", nil, false},
		{"unproved", "exit 0", errProcProof, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := exec.CommandContext(context.Background(), "/bin/sh", "-c", tc.script)
			Contain(command)
			signals := hookGroupSignals(t, startedPID(command))
			held := hookResidueProof(t, tc.inject)
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			residue, err := WaitResidue(command)
			if err != nil {
				t.Fatal(err)
			}
			if residue != tc.residue {
				t.Fatalf("residue = %v, want %v", residue, tc.residue)
			}
			if len(*held) != 1 || (*held)[0] != nil {
				t.Fatalf("residue proof not made with the leader held unreaped: %v", *held)
			}
			requirePreReap(t, signals)
		})
	}
}

// A failed exit observation sends no group signal and reports residue, so a
// clean group is never claimed unproved.
func TestWaitResidueAfterFailedObservationIsResidueWithoutSignal(t *testing.T) {
	previous := observeExit
	t.Cleanup(func() { observeExit = previous })
	observeExit = func(int) error { return syscall.ECHILD }
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	Contain(command)
	signals := hookGroupSignals(t, startedPID(command))
	held := hookResidueProof(t, nil)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	residue, err := WaitResidue(command)
	if err != nil {
		t.Fatal(err)
	}
	if !residue || len(*held) != 0 || len(signals.snapshot()) != 0 {
		t.Fatalf("residue=%v proofs=%d signals=%d", residue, len(*held), len(signals.snapshot()))
	}
}

// A Darwin quiet probe (signal 0 EPERM) is accepted only when the process
// table shows a zombie-only group; a table error or a missing leader is
// unproved.
func TestTableGroupQuietRequiresZombieOnlyGroup(t *testing.T) {
	const leader = 4242
	table := func(members ...Process) func() (map[int]Process, error) {
		return func() (map[int]Process, error) {
			out := map[int]Process{}
			for _, p := range members {
				out[p.PID] = p
			}
			return out, nil
		}
	}
	zombie := Process{PID: leader, PGID: leader, State: StateZombie}
	for _, tc := range []struct {
		name    string
		read    func() (map[int]Process, error)
		probe   Probe
		wantErr bool
	}{
		{"zombie only", table(zombie, Process{PID: 7, PGID: leader, State: StateZombie}, Process{PID: 8, PGID: 1}), ProbeQuiet, false},
		{"live member", table(zombie, Process{PID: 7, PGID: leader, State: StateRunning}), ProbeLive, false},
		{"stopped member", table(zombie, Process{PID: 7, PGID: leader, State: StateStopped}), ProbeLive, false},
		{"leader missing", table(Process{PID: 7, PGID: leader, State: StateZombie}), ProbeLive, true},
		{"table error", func() (map[int]Process, error) { return nil, errProcessTable }, ProbeLive, true},
	} {
		probe, err := tableGroupQuiet(tc.read, leader)
		if probe != tc.probe || (err != nil) != tc.wantErr {
			t.Errorf("%s: probe=%v err=%v", tc.name, probe, err)
		}
	}
}

// V1-1036: Darwin returns EPERM for signal 0 both for a zombie-only group and
// for one whose live members this process may not signal. A live member that
// signal 0 cannot see (injected into the process table) is still residue.
func TestWaitResidueDarwinDeniedLiveMemberIsResidue(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin signal-0 EPERM ambiguity")
	}
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "exit 0")
	Contain(command)
	previous := residueTable
	t.Cleanup(func() { residueTable = previous })
	residueTable = func() (map[int]Process, error) {
		table, err := previous()
		if err == nil {
			const hidden = 1 << 30 // not a real PID; signal 0 never reaches it
			table[hidden] = Process{PID: hidden, PGID: command.Process.Pid, State: StateRunning}
		}
		return table, err
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	residue, err := WaitResidue(command)
	if err != nil {
		t.Fatal(err)
	}
	if !residue {
		t.Fatal("EPERM was taken as a zombie-only group while a live member remained")
	}
}
