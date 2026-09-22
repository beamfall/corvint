//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package doccompiler

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
)

func TestRunCommandCancellationKillsProcessGroupDescendant(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "child.pid")
	scriptPath := filepath.Join(root, "runner")
	script := "#!/bin/sh\n/bin/sleep 30 &\necho $! > '" + pidPath + "'\nwait\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "state")
	if err := os.MkdirAll(stateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runCommand(ctx, scriptPath, nil, root, []string{"PATH=" + root}, 1024, 1024)
		done <- err
	}()
	pid := waitForProcessPID(t, pidPath)
	cancel()
	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled command did not return")
	}
	if errorCode(err) != "command-timeout" {
		t.Fatalf("error = %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %d survived cancellation", pid)
}

func waitForProcessPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		raw, err := os.ReadFile(path)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && parseErr == nil && pid > 0 {
			return pid
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for PID handoff at %s: read=%v parse=%v value=%q", path, err, parseErr, raw)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUnixContainmentRemainsUnqualifiedForSessionEscapes(t *testing.T) {
	qualified, description := descendantCleanupQualification()
	if qualified || !strings.Contains(description, "setsid/session escapes") {
		t.Fatalf("qualified=%v description=%q", qualified, description)
	}
}
