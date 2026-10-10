//go:build darwin || linux

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// V1-1036 / WQO-V0-032: the work runner's residue result is the pre-reap
// observation made while the exited leader is held unreaped, never a group
// signal sent after the reap. The observation is inverted here: a group with
// a live member is reported quiet, and a quiet group is reported as residue.
// A runner that still decided residue by a post-reap group SIGKILL would
// report the opposite in both cases.
func TestV11036WorkRunnerResidueIsObservedBeforeReap(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		observed     bool
	}{
		{"live member reported quiet", "#!/bin/sh\nsleep 30 &\nprintf '%s' $! > child-pid\nexit 0\n", false},
		{"quiet group reported residue", "#!/bin/sh\nexit 0\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := workWaitResidue
			t.Cleanup(func() { workWaitResidue = previous })
			calls := 0
			workWaitResidue = func(command *exec.Cmd) (bool, error) {
				calls++
				_, err := previous(command)
				return tc.observed, err
			}
			runner := workTestRunner(t, context.Background(), tc.script)
			_, receipt, err := runner.run("snapshot", []string{}, 100)
			if tc.observed {
				if err == nil || receipt.State != "INCOMPLETE" || !strings.Contains(err.Error(), "residue") {
					t.Fatalf("observed residue not reported: %+v %v", receipt, err)
				}
			} else {
				if err != nil {
					t.Fatalf("post-reap signal decided residue: %+v %v", receipt, err)
				}
				// The live member was still killed, before the reap.
				workAssertHelperGone(t, filepath.Join(runner.root, "child-pid"))
			}
			if calls != 1 {
				t.Fatalf("pre-reap residue observation calls = %d, want 1", calls)
			}
		})
	}
}

// PGO-V0-009 (V1-1056): the work adapter's process group is recorded for the
// exit kill when it starts, and released by the pre-reap wait, so an exiting
// corvint retires a running adapter's group and never names a reaped one.
func TestPGOV0009WorkRunnerRecordsItsGroupForTheExitKill(t *testing.T) {
	previousStart, previousWait := workStartLive, workWaitResidue
	t.Cleanup(func() { workStartLive, workWaitResidue = previousStart, previousWait })
	started, waited := 0, 0
	workStartLive = func(command *exec.Cmd) error {
		started++
		return previousStart(command)
	}
	workWaitResidue = func(command *exec.Cmd) (bool, error) {
		if started != 1 {
			t.Errorf("adapter waited before it was recorded (starts %d)", started)
		}
		waited++
		return previousWait(command)
	}
	runner := workTestRunner(t, context.Background(), "#!/bin/sh\nexit 0\n")
	if _, receipt, err := runner.run("snapshot", []string{}, 100); err != nil {
		t.Fatalf("run: %+v %v", receipt, err)
	}
	if started != 1 || waited != 1 {
		t.Fatalf("recorded starts = %d, pre-reap waits = %d; want 1 and 1", started, waited)
	}
}
