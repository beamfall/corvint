//go:build darwin || linux

package procgroup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This helper emits a finite amount: it cannot exhaust the test host even with
// the original unbounded capture. Arguments avoid changing the ps environment.
func TestDescendantSnapshotStderrHelper(t *testing.T) {
	if len(os.Args) < 5 || os.Args[2] != "--" {
		return
	}
	count, err := strconv.Atoi(os.Args[3])
	if err != nil {
		os.Exit(2)
	}
	data := bytes.Repeat([]byte("x"), 4096)
	for count > 0 {
		n := min(count, len(data))
		if _, err := os.Stderr.Write(data[:n]); err != nil {
			os.Exit(3)
		}
		count -= n
	}
	fmt.Fprintln(os.Stdout, "123 1 Sat Oct 10 12:34:56 2026 S")
	switch os.Args[4] {
	case "fail":
		os.Exit(7)
	case "block":
		if err := os.WriteFile(os.Args[5], []byte("ready"), 0600); err != nil {
			os.Exit(4)
		}
		time.Sleep(time.Minute)
	case "stdout":
		for range 1025 {
			if _, err := os.Stdout.Write(data); err != nil {
				os.Exit(5)
			}
		}
	}
	os.Exit(0)
}

func snapshotFixture(ctx context.Context, count int, mode string, extra ...string) *exec.Cmd {
	args := []string{"-test.run=^TestDescendantSnapshotStderrHelper$", "--", strconv.Itoa(count), mode}
	return exec.CommandContext(ctx, os.Args[0], append(args, extra...)...)
}

// PGO-V0-010: no snapshot is accepted when diagnostic capture overflows.
func TestDescendantSnapshotStderrBoundary(t *testing.T) {
	for _, tc := range []struct {
		name       string
		count      int
		mode, want string
	}{
		{"quiet", 0, "ok", ""},
		{"exact-limit", 16 << 10, "ok", ""},
		{"limit-plus-one", (16 << 10) + 1, "ok", "stderr exceeds bound"},
		{"finite-noisy", 256 << 10, "ok", "stderr exceeds bound"},
		{"overflow-and-failure", 256 << 10, "fail", "stderr exceeds bound"},
		{"command-failure", 1, "fail", "unavailable"},
		{"stdout-overflow", 0, "stdout", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := snapshotFixture(ctx, tc.count, tc.mode)
			began := time.Now()
			rows, err := descendantSnapshotCommand(cmd)
			if tc.mode == "stdout" {
				if err == nil || rows != nil {
					t.Fatal("stdout overflow accepted")
				}
				return
			}
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || rows != nil {
					t.Fatalf("rows=%v error=%v, want %s", rows, err, tc.want)
				}
			} else if err != nil || rows[123] != (ObservedProcess{PID: 123, ParentPID: 1, Start: "Sat Oct 10 12:34:56 2026", State: "S"}) {
				t.Fatalf("identity changed: %v %v", rows, err)
			}
			if tc.mode == "fail" && (err == nil || !strings.Contains(err.Error(), "unavailable")) {
				t.Fatalf("lost command failure: %v", err)
			}
			retained, overflow := cmd.Stderr.(*processCapture).result()
			if len(retained) != min(tc.count, 16<<10) || overflow != (tc.count > 16<<10) {
				t.Fatalf("retained=%d overflow=%v", len(retained), overflow)
			}
			t.Logf("emitted=%d retained=%d overflow=%v elapsed=%s error=%v", tc.count, len(retained), overflow, time.Since(began), err)
		})
	}
}

// PGO-V0-010: cancellation must reap the command and join both exec copiers.
func TestDescendantSnapshotCancellationJoinsCopies(t *testing.T) {
	for _, count := range []int{1, 256 << 10} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			ready := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			cmd := snapshotFixture(ctx, count, "block", ready)
			done := make(chan error, 1)
			go func() { _, err := descendantSnapshotCommand(cmd); done <- err }()
			// Cleanup always joins, including a failed startup acknowledgement.
			defer func() { cancel(); <-done }()
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("helper did not acknowledge stderr writes")
				}
				time.Sleep(5 * time.Millisecond)
			}
			began := time.Now()
			cancel()
			select {
			case err := <-done:
				done <- err // preserve the cleanup's join read
				if err == nil || cmd.ProcessState == nil {
					t.Fatalf("cancellation did not reap: %v", err)
				}
				if count > 16<<10 && !strings.Contains(err.Error(), "stderr exceeds bound") {
					t.Fatalf("lost overflow on cancellation: %v", err)
				}
				t.Logf("acknowledged stderr=%d cancellation/join=%s error=%v", count, time.Since(began), err)
			case <-time.After(2 * time.Second):
				t.Fatal("cancellation did not join")
			}
		})
	}
}
