//go:build unix

package store

import (
	"os/exec"
	"testing"
	"time"
)

// TestTOLV0027_PreflightStatusStopsAtFirstEntry: the post-gate status read
// settles on its first byte and stops the command, rather than reading
// every untracked entry to the end. The command here writes one entry and
// then would run for ten minutes; the read must return dirty at once.
func TestTOLV0027_PreflightStatusStopsAtFirstEntry(t *testing.T) {
	type answer struct {
		dirty bool
		err   error
	}
	done := make(chan answer, 1)
	c := exec.Command("sh", "-c", `printf '?? untracked\0'; exec /bin/sleep 600`)
	go func() {
		dirty, err := writesAnything(c)
		done <- answer{dirty, err}
	}()
	select {
	case a := <-done:
		if !a.dirty || a.err != nil {
			t.Fatalf("status with an entry: dirty=%v err=%v", a.dirty, a.err)
		}
	case <-time.After(20 * time.Second):
		if c.Process != nil {
			_ = c.Process.Kill()
		}
		t.Fatal("the status read did not stop at the first entry")
	}
	if dirty, err := writesAnything(exec.Command("true")); dirty || err != nil {
		t.Fatalf("empty status: dirty=%v err=%v", dirty, err)
	}
	if _, err := writesAnything(exec.Command("false")); err == nil {
		t.Fatal("a failed status read was not an error")
	}
}
