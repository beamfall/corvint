//go:build darwin || linux

package testsupport

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestExitedUnreapedIgnoresStoppedChild pins V1-1046: a stopped child is not an exit (golang/go#19314), so the
// unreaped wait must keep blocking until the child really exits, and must
// leave it unreaped.
func TestExitedUnreapedIgnoresStoppedChild(t *testing.T) {
	command := exec.Command("sleep", "60")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	pid := command.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_, _ = command.Process.Wait()
	})
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	returned := make(chan error, 1)
	go func() { returned <- exitedUnreaped(pid) }()
	select {
	case err := <-returned:
		t.Fatalf("wait returned %v for a stopped child", err)
	case <-time.After(500 * time.Millisecond):
	}
	_ = syscall.Kill(pid, syscall.SIGCONT)
	select {
	case err := <-returned:
		t.Fatalf("wait returned %v for a continued child", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-returned:
		if err != nil {
			t.Fatalf("wait after exit: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("wait did not return after the child exited")
	}
	// Still unreaped: Wait must observe the signalled exit.
	if err := command.Wait(); err == nil {
		t.Fatal("Wait returned nil for a killed child")
	}
}
