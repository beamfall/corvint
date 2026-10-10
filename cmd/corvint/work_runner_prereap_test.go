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
