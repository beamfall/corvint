//go:build unix

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestContainedRunnerKillsDescendantOnCancellation(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := runContained(ctx, os.Args[0], []string{"-test.run=^TestArchiveProcessHelper$"}, []string{"ARCHIVE_PROCESS_HELPER=1", "ARCHIVE_PROCESS_PID=" + pidFile, "PATH=" + os.Getenv("PATH")}, ".")
		done <- err
	}()
	var childPID int
	// The descendant here is a re-exec of this whole test binary, which under
	// heavy host load (many other package test binaries competing for
	// fork/exec slots) can take much longer to start than on a quiet
	// machine; raise the bound so this only fails on a genuine hang.
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		raw, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, _ = strconv.Atoi(string(raw))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		cancel()
		t.Fatal("descendant did not start within 30s")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("runner did not return within 30s")
	}
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		err := syscall.Kill(childPID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %d survived cancellation after 30s", childPID)
}

func TestContainedRunnerSweepsDescendantAfterSuccessfulLeader(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	_, _, err := runContained(context.Background(), os.Args[0], []string{"-test.run=^TestArchiveProcessHelper$"}, []string{"ARCHIVE_PROCESS_HELPER=success", "ARCHIVE_PROCESS_PID=" + pidFile, "PATH=" + os.Getenv("PATH")}, ".")
	if err != nil {
		t.Fatalf("successful leader cleanup failed: %v", err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(string(raw))
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("success-path descendant %d survived", pid)
	}
}

func TestArchiveProcessHelper(t *testing.T) {
	mode := os.Getenv("ARCHIVE_PROCESS_HELPER")
	if mode != "1" && mode != "success" {
		return
	}
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv("ARCHIVE_PROCESS_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		os.Exit(4)
	}
	if mode == "success" {
		os.Exit(0)
	}
	_ = child.Wait()
	os.Exit(0)
}
