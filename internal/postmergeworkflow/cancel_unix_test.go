//go:build darwin || linux

package postmergeworkflow

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestCancellationRetiresDescendant(t *testing.T) {
	root, ff, pf, _, p := setup(t, "hang")
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	p.Args = append(p.Args, pidFile)
	writeJSON(t, pf, p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := Replay(ctx, root, ff, pf, "change-1"); done <- err }()
	var pid int
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(pidFile)
		if err == nil {
			pid, _ = strconv.Atoi(string(b))
			break
		}
		select {
		case err := <-done:
			t.Fatalf("adapter ended before child: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("no descendant observed")
	}
	time.Sleep(80 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("process cleanup did not finish")
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("descendant remains")
}

func TestNonregularInputDoesNotBlock(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "fixture.pipe")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeRead(pipe, MaxBytes); err == nil {
		t.Fatal("native FIFO admitted")
	}
	if _, err := readBounded(pipe); err == nil {
		t.Fatal("FIFO admitted")
	}
	if _, err := readExecutable(pipe); err == nil {
		t.Fatal("FIFO executable admitted")
	}
}
