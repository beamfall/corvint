//go:build unix

// SPDX-License-Identifier: Apache-2.0

package linecov

import (
	"context"
	"os/exec"
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
