//go:build darwin || linux

package criterionexperiment

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTasksTransportBoundsAndCancellation(t *testing.T) {
	for name, body := range map[string]string{"overflow": "while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done", "cancel": "sleep 30 & wait"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "verifier")
			raw := []byte("#!/bin/sh\n" + body + "\n")
			if e := os.WriteFile(p, raw, 0700); e != nil {
				t.Fatal(e)
			}
			cfg := TasksVerifierConfig{p, Digest(raw)}
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			start := time.Now()
			if _, e := tasksResult(ctx, "", cfg, "verify", nil); e == nil {
				t.Fatal("unbounded transport accepted")
			}
			if time.Since(start) > 3*time.Second {
				t.Fatal("transport did not retire promptly")
			}
		})
	}
}
func TestTasksExecutableIndependentPin(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tasks")
	raw := []byte("#!/bin/sh\nexit 0\n")
	_ = os.WriteFile(p, raw, 0700)
	for _, c := range []TasksVerifierConfig{{p, strings.Repeat("a", 64)}, {"relative", Digest(raw)}, {p, ""}} {
		if e := c.check(); e == nil {
			t.Fatal("invalid pin admitted")
		}
	}
	link := p + "-link"
	_ = os.Symlink(p, link)
	if e := (TasksVerifierConfig{link, Digest(raw)}).check(); e == nil {
		t.Fatal("symlink admitted")
	}
}

func TestTasksCancellationRetiresDescendant(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "child.pid")
	exe := filepath.Join(dir, "tasks")
	raw := []byte("#!/bin/sh\nsleep 30 &\necho $! > '" + pidPath + "'\nwait\n")
	if e := os.WriteFile(exe, raw, 0700); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, e := tasksResult(ctx, "", TasksVerifierConfig{exe, Digest(raw)}, "verify", nil); done <- e }()
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, e := os.ReadFile(pidPath)
		if e == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancel admitted")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel did not finish")
	}
	if pid == 0 {
		t.Fatal("descendant did not start")
	}
	if e := syscall.Kill(pid, 0); e != syscall.ESRCH {
		t.Fatalf("descendant still present: %v", e)
	}
}
