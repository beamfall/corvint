//go:build darwin || linux

package doccompiler

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
)

// TestTrustedNavCancellation proves TPN-V0-006 for the actual bounded runner.
func TestTrustedNavCancellation(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "grandchild.pid")
	script := filepath.Join(root, "runner")
	body := "#!/bin/sh\n/bin/sh -c '/bin/sleep 3600 & echo $! > \"" + pidPath + "\"; wait' &\nwait\n"
	if err := execfile.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runCommand(ctx, script, nil, root, []string{"PATH=/usr/bin:/bin"}, 1024, 1024)
		done <- err
	}()
	pid := waitForProcessPID(t, pidPath, done)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("cancelled command succeeded")
	}
	requireDescendantGone(t, pid)
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
