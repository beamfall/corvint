//go:build darwin

package supervisor

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestCALV0086_DrainProvesReapedZombieGroupGone reproduces the V1-0772 load
// failure on Darwin: once the lane leader exits but before its Wait reaps it,
// kill(-group, 0) answers EPERM. drain must wait for the reap and prove the
// group gone rather than report survivors.
func TestCALV0086_DrainProvesReapedZombieGroupGone(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "read line")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	started, err := ProcessIdentity(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	g := newOwnedGroup(Boot{PID: cmd.Process.Pid, Started: started})
	in.Close()
	deadline := time.Now().Add(10 * time.Second)
	for syscall.Kill(-cmd.Process.Pid, 0) != syscall.EPERM {
		if time.Now().After(deadline) {
			t.Fatal("leader never became an unreaped zombie")
		}
		time.Sleep(10 * time.Millisecond)
	}
	reaped := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		reaped <- cmd.Wait()
	}()
	if !g.drain() {
		t.Fatal("a zombie-only group reaped during the drain was reported unclean")
	}
	<-reaped
}
