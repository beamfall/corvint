//go:build darwin || linux

package testacceptance

import (
	"context"
	"github.com/Beamfall/corvint/internal/procgroup"
	"os"
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"
)

func TestNEAV0006DetachedHelper(t *testing.T) {
	if slices.Contains(os.Args, "detached-child") {
		time.Sleep(30 * time.Second)
		return
	}
	if !slices.Contains(os.Args, "detached-parent") {
		return
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	child := exec.Command(exe, "-test.run=^TestNEAV0006DetachedHelper$", "--", "detached-child")
	child.Env = SafeEnvironment()
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e = child.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	time.Sleep(30 * time.Second)
}
func TestNEAV0006ObservedNestedGroupCancellation(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 650*time.Millisecond)
	defer cancel()
	o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{exe, "-test.run=^TestNEAV0006DetachedHelper$", "--", "detached-parent"}, Dir: canonicalTemp(t), Env: SafeEnvironment(), Timeout: time.Second, ObserveDescendants: true})
	if !o.Cancelled || !o.WaitCompleted || !o.OwnedProcessGroupCleanup || o.DescendantObservation == nil || !o.DescendantObservation.Absent || len(o.DescendantObservation.Processes) < 1 {
		t.Fatalf("actual detached child retirement absent: %+v", o)
	}
	if len(o.DescendantObservation.Limitations) == 0 {
		t.Fatal("bounded observer limits erased")
	}
}
