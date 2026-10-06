//go:build darwin || linux

package testrunner

import (
	"bytes"
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// starvedReader delays its first Write past the old one-second pipe-drain
// bound, standing in for an output-copying goroutine the scheduler leaves
// unscheduled after the child has exited (V1-0391).
type starvedReader struct {
	once sync.Once
	buf  bytes.Buffer
}

func (r *starvedReader) Write(p []byte) (int, error) {
	r.once.Do(func() { time.Sleep(3 * time.Second) })
	return r.buf.Write(p)
}

// TestContainPhaseSurvivesStarvedReader: a child that exits 0 while its output reader is starved for
// several seconds returns its output instead of exec.ErrWaitDelay.
func TestContainPhaseSurvivesStarvedReader(t *testing.T) {
	command := exec.CommandContext(context.Background(), "/bin/sh", "-c", "printf ok")
	out := &starvedReader{}
	command.Stdout = out
	containPhase(command, false)
	if err := command.Run(); err != nil {
		t.Fatalf("starved reader failed a successful child: %v", err)
	}
	if got := out.buf.String(); got != "ok" {
		t.Fatalf("output = %q, want ok", got)
	}
}
