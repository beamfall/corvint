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

// V1-1091: /proc/stat btime can differ by one second between reads. A live
// process's identity must not, so the first btime read is reused.
func TestV11091IdentityStableAcrossBtimeJitter(t *testing.T) {
	bootTime.Lock()
	saved := bootTime.value
	bootTime.value = ""
	bootTime.Unlock()
	reads := 0
	readBootStat = func(string) ([]byte, error) {
		reads++
		return []byte("cpu 1 2 3\nbtime " + []string{"1789760570", "1789760571"}[reads%2] + "\nprocesses 9\n"), nil
	}
	t.Cleanup(func() {
		readBootStat = os.ReadFile
		bootTime.Lock()
		bootTime.value = saved
		bootTime.Unlock()
	})
	first, err := ProcessIdentity(os.Getpid())
	if err != nil || first == "" {
		t.Fatalf("identity %q %v", first, err)
	}
	for i := 0; i < 10; i++ {
		if again, err := ProcessIdentity(os.Getpid()); err != nil || again != first {
			t.Fatalf("identity changed under btime jitter: %q then %q (%v)", first, again, err)
		}
	}
	if reads != 1 {
		t.Fatalf("btime read %d times, want once", reads)
	}
}
