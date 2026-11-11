//go:build unix

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// GLS-V0-008: tests signalling their group cannot kill the collector, and
// cancellation closes descendant-held pipes without waiting for WaitDelay.
func TestCollectorProcessGroup(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", "kill -TERM 0")
	isolate(cmd)
	if err := cmd.Run(); err == nil {
		t.Fatal("group signal did not terminate the test process")
	}
	// A successful parent leaving a descendant is still incomplete.
	cmd = exec.CommandContext(context.Background(), "sh", "-c", "sleep 60 </dev/null >/dev/null 2>&1 &")
	isolate(cmd)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if !cleanup(cmd) {
		t.Fatal("unfinished process tree was accepted as complete")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd = exec.CommandContext(ctx, "sh", "-c", "sleep 60 & wait")
	isolate(cmd)
	cmd.WaitDelay = 5 * time.Second
	start := time.Now()
	if _, err := cmd.CombinedOutput(); err == nil {
		t.Fatal("cancelled test passed")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("descendant held output after cancellation: %s", elapsed)
	}
}

// GLS-V0-008: RunIsolated kills a cancelled command's whole process group and
// does not wait on an output pipe a descendant still holds.
func TestRunIsolatedKillsTheGroupOnCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-c", "sleep 60 & echo $!; wait")
	cmd.Stdout = &out
	start := time.Now()
	if err := RunIsolated(cmd); err == nil {
		t.Fatal("cancelled command passed")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("descendant held the command after cancellation: %s", elapsed)
	}
	child, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("child pid not reported: %q", out.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(child, 0) == nil {
		if time.Now().After(deadline) {
			syscall.Kill(child, syscall.SIGKILL)
			t.Fatalf("descendant %d survived cancellation", child)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
