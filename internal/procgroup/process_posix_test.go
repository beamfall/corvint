//go:build darwin || linux

package procgroup

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// killTestProcessGroup sends SIGKILL straight to the OS process group led by
// pgid, bypassing any grace period. It exists only so process_test.go's
// group-directed kill assertions (which are inherently POSIX process-group
// semantics) still compile on platforms where process_other.go's contract
// has no process-group equivalent (see TestRunProcessNonPOSIXContract).
func killTestProcessGroup(pgid int) error {
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

// V1-1037: a stopped or continued leader is not reported as exited. On Darwin
// waitid with WEXITED|WNOWAIT also returns for those state changes
// (golang/go#19314); treating that as an exit would terminate the group while
// the leader lives. Mirrors groupreap's TestLeaderUnreapedIgnoresStopAndContinue.
func TestWaitProcessExitUnreapedIgnoresStopAndContinue(t *testing.T) {
	command := exec.Command("/bin/sleep", "60")
	configureProcessCommand(command, false)
	if err := startProcessCommand(command); err != nil {
		t.Fatal(err)
	}
	leader := command.Process.Pid
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	if err := syscall.Kill(leader, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !processStoppedForTest(t, leader) {
		if time.Now().After(deadline) {
			t.Fatal("leader did not stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	returned := make(chan error, 1)
	go func() { returned <- waitProcessExitUnreaped(leader) }()
	select {
	case err := <-returned:
		t.Fatalf("waitProcessExitUnreaped returned for a stopped leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGCONT); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		t.Fatalf("waitProcessExitUnreaped returned for a continued leader: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(leader, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("waitProcessExitUnreaped after exit = %v", err)
		}
	case <-time.After(10 * time.Second): // hang detector, not a budget (decision 0082)
		t.Fatal("waitProcessExitUnreaped did not observe the exit")
	}
	// The exit was observed without reaping: the leader is still a zombie
	// holding its PID until Wait.
	if err := syscall.Kill(leader, 0); err != nil {
		t.Fatalf("leader was reaped by the exit observation: %v", err)
	}
}

func processStoppedForTest(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "T")
}
