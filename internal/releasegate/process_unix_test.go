//go:build !windows

package releasegate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func killProcess(pid int)      { _ = syscall.Kill(pid, syscall.SIGKILL) }
func processGone(pid int) bool { return syscall.Kill(pid, 0) == syscall.ESRCH }

// V1-1036: the post-reap descendant check is an observation. A group ID that
// still answers signal 0 after the reap (as a reused one would) refuses the
// command, and nothing but signal 0 is sent to it.
func TestV11036PostReapGroupCheckOnlyObservesAndFailsClosed(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var signals []syscall.Signal
	previous := groupProbe
	t.Cleanup(func() { groupProbe = previous })
	groupProbe = func(pid int, signal syscall.Signal) error {
		mu.Lock()
		defer mu.Unlock()
		signals = append(signals, signal)
		return nil // the group still looks present
	}
	identity := gitIdentity{path: script, sha256: sha256Hex(scriptBytes)}
	_, err = runGitAt(context.Background(), identity, directory, 1024, "rev-parse", "HEAD")
	if err == nil || !strings.Contains(err.Error(), "did not disappear") {
		t.Fatalf("misjudged group accepted: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(signals) == 0 {
		t.Fatal("post-reap check not reached")
	}
	for _, signal := range signals {
		if signal != 0 {
			t.Fatalf("post-reap check sent %v", signal)
		}
	}
}
