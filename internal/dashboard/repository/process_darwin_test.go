//go:build darwin

package repository

import (
	"context"
	"os/exec"
	"testing"
)

func TestDarwinWaitVerifiesExecutableBeforeReap(t *testing.T) {
	command := exec.Command("/usr/bin/true")
	containment, ok := prepareContainment(command)
	if !ok {
		t.Fatal("containment unavailable")
	}
	defer containment.close()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if !containment.attach(command.Process) {
		containment.force()
		_, _ = command.Process.Wait()
		t.Fatal("attach containment")
	}
	verified := false
	outcome := containment.wait(command, context.Background(), nil, func() bool {
		verified = true
		return command.ProcessState == nil
	})
	if outcome.waitErr != nil || !outcome.cleanupProven || !outcome.executableStable {
		t.Fatalf("outcome = %#v", outcome)
	}
	if !verified {
		t.Fatal("executable binding was not verified")
	}
	if command.ProcessState == nil {
		t.Fatal("Wait did not publish ProcessState")
	}
}

func TestWaitProcessExitUnreapedPinsLeaderUntilWait(t *testing.T) {
	command := exec.Command("/usr/bin/true")
	containment, ok := prepareContainment(command)
	if !ok {
		t.Fatal("containment unavailable")
	}
	defer containment.close()
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if waited {
			return
		}
		containment.force()
		_, _ = command.Process.Wait()
	})
	if !containment.attach(command.Process) {
		t.Fatal("attach containment")
	}
	if err := waitProcessExitUnreaped(command.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if command.ProcessState != nil {
		t.Fatal("waitid reaped the process")
	}
	if darwinGroupHasLiveMembers(command.Process.Pid) {
		t.Fatal("zombie-only process group reported live residue")
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	waited = true
	if command.ProcessState == nil {
		t.Fatal("Wait did not publish ProcessState")
	}
}
