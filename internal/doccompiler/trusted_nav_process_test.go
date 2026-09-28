//go:build darwin || linux

package doccompiler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestTrustedNavCancellation proves TPN-V0-006 for the actual bounded runner.
func TestTrustedNavCancellation(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "grandchild.pid")
	script := filepath.Join(root, "runner")
	body := "#!/bin/sh\n/bin/sh -c '/bin/sleep 60 & echo $! > \"" + pidPath + "\"; wait' &\nwait\n"
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runCommand(ctx, script, nil, root, []string{"PATH=/usr/bin:/bin"}, 1024, 1024)
		done <- err
	}()
	pid := waitForProcessPID(t, pidPath)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled command succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup timed out")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("grandchild survived interruption")
}

func TestTrustedNavRefusesFIFOInputs(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "request")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadTrustedNavFile(pipe, 1024); errorCode(err) != "trusted-nav-input" {
		t.Fatalf("FIFO input: %v", err)
	}
	if _, err := InspectTrustedNavigation(t.Context(), pipe, true); errorCode(err) != "trusted-nav-environment" {
		t.Fatalf("FIFO interpreter: %v", err)
	}
}
