//go:build darwin || linux

package groupreap

import (
	"context"
	"os/exec"
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
