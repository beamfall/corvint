//go:build darwin || linux

package affected_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestAffectedStatusCancellationLeavesNoDescendant(t *testing.T) {
	_, root := observationRepository(t)
	directory := t.TempDir()
	pidPath := filepath.Join(directory, "descendant")
	quoted := "'" + strings.ReplaceAll(pidPath, "'", "'\"'\"'") + "'"
	script := "#!/bin/sh\n/bin/sleep 30 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\"' EXIT INT TERM\nprintf '%s' \"$child\" > " + quoted + "\nwait \"$child\"\n"
	executable := filepath.Join(directory, "git")
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := affected.DirtyPaths(ctx, executable, root)
		done <- err
	}()
	pid := 0
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(pidPath); err == nil {
			pid, _ = strconv.Atoi(string(raw))
		}
		if pid > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, affected.ErrStatusUnavailable) {
			t.Fatalf("cancelled status error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled status did not terminate")
	}
	if pid <= 0 {
		t.Fatal("status did not reach the controlled descendant")
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatal("status cancellation left a descendant")
}
