//go:build linux

package supervisor

import (
	"io"
	"os"
	"os/exec"
	"testing"
)

// CAL-V0-056: a process reaped after its /proc stat file was opened fails
// the read with ESRCH. It has ended, so its identity is "" rather than an
// unreadable-identity error that would freeze its worker's recorded tree.
func TestCALV0056_ReapedDuringIdentityReadIsGone(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if id, err := ProcessIdentity(pid); err != nil || id == "" {
		t.Fatalf("live identity %q %v", id, err)
	}
	var readErr error
	readProcStat = func(name string) ([]byte, error) {
		f, err := os.Open(name)
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer f.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // reaped: the opened entry no longer names a task
		b, err := io.ReadAll(f)
		readErr = err
		return b, err
	}
	t.Cleanup(func() { readProcStat = os.ReadFile })
	id, err := ProcessIdentity(pid)
	if readErr == nil {
		t.Skip("this kernel still reads a reaped process's stat file")
	}
	if err != nil || id != "" {
		t.Fatalf("identity of a process reaped mid-read = %q, %v (read error %v)", id, err, readErr)
	}
}
