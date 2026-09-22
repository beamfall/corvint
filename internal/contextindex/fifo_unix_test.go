//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestReadCleanFileRejectsFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe.go")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if data, ok := readCleanFile(root, "pipe.go"); ok || data != nil {
		t.Fatalf("FIFO admitted: ok=%v data=%q", ok, data)
	}
	if generated, ok := verifyCleanFile(root, "pipe.go", "", "sha1"); ok || generated {
		t.Fatalf("streaming verifier admitted FIFO: ok=%v generated=%v", ok, generated)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FIFO read blocked for %s", elapsed)
	}
}

func TestBuildCancellationKillsGitDescendants(t *testing.T) {
	root := testRepository(t)
	bin := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "child-pid")
	wrapper := fmt.Sprintf("#!/bin/sh\nsleep 60 &\nchild=$!\nprintf '%%s\\n' \"$child\" > %s\nwait \"$child\"\n", strconv.Quote(pidFile))
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	completed := make(chan error, 1)
	go func() {
		_, err := Build(ctx, root)
		completed <- err
	}()
	var rawPID []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rawPID, _ = os.ReadFile(pidFile)
		if len(rawPID) != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(rawPID) == 0 {
		t.Fatal("Git descendant did not start")
	}
	cancel()
	err := <-completed
	if err == nil || !strings.Contains(err.Error(), "cancelled") && !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("Build error = %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Git descendant %d survived cancellation", pid)
}
