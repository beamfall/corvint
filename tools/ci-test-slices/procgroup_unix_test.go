//go:build unix

package main

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

// TestAFPV0041CancelStopsTheEnumerationGroup runs enumeration against a fake
// `go` whose sleeping child ignores SIGINT and holds the output pipe, and
// checks that cancellation leaves no member of the group alive.
func TestAFPV0041CancelStopsTheEnumerationGroup(t *testing.T) {
	old := stopGrace
	stopGrace = 300 * time.Millisecond
	t.Cleanup(func() { stopGrace = old })
	for name, leader := range map[string]string{
		"leader interruptible":  "",
		"leader ignores SIGINT": "trap '' INT\n",
	} {
		t.Run(name, func(t *testing.T) {
			bin, pids := t.TempDir(), filepath.Join(t.TempDir(), "pids")
			// An asynchronous command of a non-interactive shell ignores SIGINT.
			fake := write(t, filepath.Join(bin, "go"), "#!/bin/sh\n"+leader+"sleep 300 &\necho \"$$ $!\" > \"$CI_TEST_SLICES_PIDS.tmp\"\nmv \"$CI_TEST_SLICES_PIDS.tmp\" \"$CI_TEST_SLICES_PIDS\"\nwait\n")
			if err := os.Chmod(fake, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CI_TEST_SLICES_PIDS", pids)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := goList(ctx, t.TempDir(), "example.org/p")
				done <- err
			}()
			var leaderPID, sleeper int
			for deadline := time.Now().Add(time.Minute); leaderPID == 0; time.Sleep(10 * time.Millisecond) {
				if raw, err := os.ReadFile(pids); err == nil {
					f := strings.Fields(string(raw))
					leaderPID, _ = strconv.Atoi(f[0])
					sleeper, _ = strconv.Atoi(f[1])
				} else if time.Now().After(deadline) {
					t.Fatal("fake go did not start its child")
				}
			}
			t.Cleanup(func() { _ = syscall.Kill(sleeper, syscall.SIGKILL) })
			cancel()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled enumeration succeeded")
				}
			case <-time.After(time.Minute):
				t.Fatal("cancelled enumeration did not return")
			}
			// The group must already be empty; a child reparented to init may
			// still need a moment to be reaped.
			if err := syscall.Kill(-leaderPID, 0); !errors.Is(err, syscall.ESRCH) {
				t.Errorf("process group %d still has a member after cancellation: %v", leaderPID, err)
			}
			for deadline := time.Now().Add(2 * time.Second); !errors.Is(syscall.Kill(sleeper, 0), syscall.ESRCH); time.Sleep(20 * time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatalf("grandchild %d survived cancellation", sleeper)
				}
			}
		})
	}
}
