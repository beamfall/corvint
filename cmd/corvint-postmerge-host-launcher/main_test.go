// Copyright 2026 Corvint contributors.
// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/postmergeproof/procfs"
)

// TestMain runs the real launcher main when a test re-executes this binary.
func TestMain(m *testing.M) {
	if os.Getenv("CORVINT_TEST_LAUNCHER_MAIN") == "1" {
		main()
	}
	os.Exit(m.Run())
}

func TestClosedLauncherInvocation(t *testing.T) {
	valid := []string{"--profile", "/profile", "--profile-sha256", strings.Repeat("a", 64), "--request", "/request", "--request-sha256", strings.Repeat("b", 64), "--out", "/fresh"}
	if _, err := parseOptions(valid); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, valid[:8], append(append([]string{}, valid...), "--engine", "tcp://remote"), {"--profile", "/p", "--profile", "/q", "--request", "/r", "--request-sha256", "x", "--out", "/o"}, {"--internal-envelope", "x", "--engine", "remote"}} {
		if _, err := parseOptions(args); err == nil {
			t.Fatal("accepted open invocation", args)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--internal-envelope", "not-a-digest"}, io.NopCloser(strings.NewReader("execute\n")), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatal("invalid internal mode ran")
	}
}

func TestInternalProcessObserverRefusesUnknownMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--internal-process-observer", "replay"}, io.NopCloser(strings.NewReader("")), &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("unknown observer mode ran: code %d stdout %q", code, stdout.String())
	}
}

// The observer role is selected only by the first argument; the same token
// anywhere else is an invalid paired host invocation.
func TestInternalProcessObserverRoutingBoundary(t *testing.T) {
	valid := []string{"--profile", "/profile", "--profile-sha256", strings.Repeat("a", 64), "--request", "/request", "--request-sha256", strings.Repeat("b", 64), "--out", "/fresh"}
	for _, args := range [][]string{
		append(append([]string{}, valid[:8]...), "--out", "--internal-process-observer"),
		{"--profile", "--internal-process-observer", "absence-sweep", "x"},
		{"--internal-envelope", "--internal-process-observer"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, io.NopCloser(strings.NewReader("")), &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
			strings.Contains(stderr.String(), "process-observer") {
			t.Fatalf("%v routed to the observer: code %d stderr %q", args, code, stderr.String())
		}
	}
}

// SIGTERM cancels the launcher context, which ends an observer blocked on
// its stdin with a refusal instead of hanging.
func TestInternalProcessObserverStopsOnSIGTERM(t *testing.T) {
	if !procfs.Supported {
		t.Skip("NOT_OBSERVED: no Linux procfs observer on this host")
	}
	for attempt := 1; attempt <= 3; attempt++ {
		cmd := exec.Command(os.Args[0], "--internal-process-observer", "absence-sweep", "x")
		cmd.Env = append(os.Environ(), "CORVINT_TEST_LAUNCHER_MAIN=1")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Fatal("observer blocked on stdin ignored SIGTERM")
		}
		_ = stdin.Close()
		// A signal before main installed its handler kills the process with
		// the default action; retry with a longer delay.
		if status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			continue
		}
		if cmd.ProcessState.ExitCode() != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "process-observer-refused") {
			t.Fatalf("SIGTERM: code %d stdout %q stderr %q", cmd.ProcessState.ExitCode(), stdout.String(), stderr.String())
		}
		return
	}
	t.Fatal("the launcher never handled SIGTERM")
}
