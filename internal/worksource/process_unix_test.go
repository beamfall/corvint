//go:build darwin || linux

package worksource

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

// VPO-V0-010: interruption owns and kills the whole ordinary Git process group.
// No child escapes the group; the supervisor's deferred cleanup is registered
// before launch and the assertion verifies the descendant disappears.
func TestWorkSourceGitInterruptionLeavesNoDescendant(t *testing.T) {
	source, err := newSource()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	source.Root = t.TempDir()
	source.GitPath = filepath.Join(source.Root, "git-fixture")
	write(t, source.Root, "git-fixture", []byte("#!/bin/sh\nsleep 30 &\nchild=$!\nprintf '%s\\n' \"$child\" > child.pid\nwait \"$child\"\n"), 0755)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	var gitErr error
	var pid int
	pidPath := filepath.Join(source.Root, "child.pid")
	// Cleanup also covers readiness failure; cancellation is always joined before
	// emergency child cleanup. Neither cleanup operation is the success witness.
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("Git cleanup exceeded bound")
		}
		if pid == 0 {
			raw, _ := os.ReadFile(pidPath)
			if strings.HasSuffix(string(raw), "\n") {
				pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			}
		}
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}()
	go func() {
		_, gitErr = source.Git(ctx, 100, "ignored")
		close(finished)
	}()
	// Fixture startup has its own bound: a busy package-parallel gate must not
	// cancel before the descendant exists and thereby miss the tested condition.
	readyDeadline := time.Now().Add(5 * time.Second)
	for pid == 0 {
		raw, readErr := os.ReadFile(pidPath)
		if readErr == nil && strings.HasSuffix(string(raw), "\n") {
			pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
			if err != nil || pid <= 0 {
				pid = 0
				t.Fatal("missing child identity")
			}
		} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			t.Fatal(readErr)
		}
		if pid > 0 {
			break
		}
		select {
		case <-finished:
			t.Fatalf("Git exited before fixture readiness: %v", gitErr)
		default:
		}
		if time.Now().After(readyDeadline) {
			t.Fatal("Git fixture readiness exceeded bound")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("Git descendant absent before interruption: %v", err)
	}
	started := time.Now()
	cancel()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("interruption exceeded bound")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("interruption exceeded bound")
	}
	if !errors.Is(gitErr, context.Canceled) {
		t.Fatalf("Git interruption did not return cancellation: %v", gitErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Git descendant survived interruption")
}

// VPO-V0-008: nonblocking no-follow reads reject a regular-file-to-FIFO swap
// without waiting for a writer, including when Git's assume flag hid the path.
func TestWorkSourceSpecialFileRejectedWithoutBlocking(t *testing.T) {
	root := fixture(t)
	source, err := Acquire(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	path := filepath.Join(root, "a.txt")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := source.VerifyMaterialization(context.Background(), root); err == nil {
		t.Fatal("special source file accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("special source file blocked")
	}
}
