//go:build unix

package gitrun

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

func shell() Options {
	return Options{Binary: "/bin/sh", StdoutLimit: 1 << 16, Env: []string{"PATH=/usr/bin:/bin"},
		PerOpTimeout: 5 * time.Second}
}

func run(t *testing.T, budget *Budget, options Options, script string) ([]byte, error) {
	t.Helper()
	return Run(context.Background(), budget, options, "-c", script)
}

func TestStdoutCaptured(t *testing.T) {
	out, err := run(t, NewDefaultBudget(), shell(), "echo hello")
	if err != nil || string(out) != "hello\n" {
		t.Fatalf("out %q err %v", out, err)
	}
}

func TestBoundedStdinCaptured(t *testing.T) {
	options := shell()
	options.Stdin = []byte("one\ntwo\n")
	out, err := run(t, NewDefaultBudget(), options, "cat")
	if err != nil || string(out) != string(options.Stdin) {
		t.Fatalf("out %q err %v", out, err)
	}
}

func TestTypedExitFailure(t *testing.T) {
	_, err := run(t, NewDefaultBudget(), shell(), "printf 'doom\\nsecond line\\n' >&2; exit 3")
	if cemcode.CodeOf(err) != cemcode.GitExitFailure {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "doom") {
		t.Fatalf("stderr excerpt missing: %v", err)
	}
	if err.Error() != "git-exit-failure: Git exited unsuccessfully: doom" {
		t.Fatalf("error bytes changed: %v", err)
	}
	details, ok := cemcode.GitExitFailureDetails(err)
	if !ok || details.ExitCode != 3 || string(details.Stderr) != "doom\nsecond line\n" {
		t.Fatalf("details=%#v available=%v", details, ok)
	}
}

func TestTypedTimeout(t *testing.T) {
	options := shell()
	options.PerOpTimeout = 200 * time.Millisecond
	started := time.Now()
	_, err := run(t, NewDefaultBudget(), options, "sleep 30")
	if cemcode.CodeOf(err) != cemcode.GitTimeout {
		t.Fatalf("got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("timeout path did not reap promptly: %v", elapsed)
	}
}

func TestTypedCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	_, err := Run(ctx, NewDefaultBudget(), shell(), "-c", "sleep 30")
	if cemcode.CodeOf(err) != cemcode.GitCancelled {
		t.Fatalf("got %v", err)
	}
}

func TestTypedOutputBound(t *testing.T) {
	options := shell()
	options.StdoutLimit = 1024
	_, err := run(t, NewDefaultBudget(), options, "dd if=/dev/zero bs=1024 count=64 2>/dev/null | tr '\\0' 'x'")
	if cemcode.CodeOf(err) != cemcode.GitOutputExceeded {
		t.Fatalf("got %v", err)
	}
}

func TestTypedBudgetExhaustion(t *testing.T) {
	budget := NewBudget(1, time.Minute)
	if _, err := run(t, budget, shell(), "true"); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, budget, shell(), "true")
	if cemcode.CodeOf(err) != cemcode.GitBudgetExceeded {
		t.Fatalf("got %v", err)
	}
}

func TestTypedBudgetDeadline(t *testing.T) {
	budget := NewBudget(100, -time.Second)
	_, err := run(t, budget, shell(), "true")
	if cemcode.CodeOf(err) != cemcode.GitTimeout {
		t.Fatalf("got %v", err)
	}
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func waitForDeath(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("descendant %d survived cleanup after %s", pid, 30*time.Second)
}

// TestDescendantCleanupAfterNormalExit proves a grandchild that outlives the
// direct child is swept when the child exits normally.
func TestDescendantCleanupAfterNormalExit(t *testing.T) {
	out, err := run(t, NewDefaultBudget(), shell(), "sleep 30 >/dev/null 2>&1 & echo $!")
	if err != nil {
		t.Fatal(err)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(out)))
	if convErr != nil {
		t.Fatal(convErr)
	}
	waitForDeath(t, pid)
}

// TestDescendantCleanupOnTimeout proves the whole process group dies on the
// timeout path, where the group is killed before the child is reaped.
func TestDescendantCleanupOnTimeout(t *testing.T) {
	options := shell()
	options.PerOpTimeout = 300 * time.Millisecond
	options.Dir = t.TempDir()
	_, err := Run(context.Background(), NewDefaultBudget(), options, "-c",
		"sleep 30 >/dev/null 2>&1 & echo $! > grandchild; sleep 30")
	if cemcode.CodeOf(err) != cemcode.GitTimeout {
		t.Fatalf("got %v", err)
	}
	data, readErr := readAll(options.Dir + "/grandchild")
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(data))
	if convErr != nil {
		t.Fatal(convErr)
	}
	waitForDeath(t, pid)
}

func readAll(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

// TestTypedStartFailure completes the taxonomy: an unstartable binary is
// git-start-failed, never collapsed into an exit failure.
func TestTypedStartFailure(t *testing.T) {
	options := shell()
	options.Binary = "/nonexistent-cem-binary"
	_, err := run(t, NewDefaultBudget(), options, "true")
	if cemcode.CodeOf(err) != cemcode.GitStartFailed {
		t.Fatalf("got %v", err)
	}
}

// TestDescendantCleanupOnCancellation proves the whole process group dies on
// the cancellation path.
func TestDescendantCleanupOnCancellation(t *testing.T) {
	options := shell()
	options.Dir = t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, err := Run(ctx, NewDefaultBudget(), options, "-c",
		"sleep 30 >/dev/null 2>&1 & echo $! > grandchild; sleep 30")
	if cemcode.CodeOf(err) != cemcode.GitCancelled {
		t.Fatalf("got %v", err)
	}
	data, readErr := readAll(options.Dir + "/grandchild")
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(data))
	if convErr != nil {
		t.Fatal(convErr)
	}
	waitForDeath(t, pid)
}

// TestStdinPathDescendantCleanupOnCancellation covers the cat-file --batch
// transport shape: bounded stdin is present while cancellation reaps the group.
func TestStdinPathDescendantCleanupOnCancellation(t *testing.T) {
	options := shell()
	options.Dir = t.TempDir()
	options.Stdin = bytes.Repeat([]byte("object-id\n"), 1024)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, err := Run(ctx, NewDefaultBudget(), options, "-c",
		"sleep 30 >/dev/null 2>&1 & echo $! > grandchild; sleep 30; cat")
	if cemcode.CodeOf(err) != cemcode.GitCancelled {
		t.Fatalf("got %v", err)
	}
	// The 150ms cancel fires on a fixed wall-clock delay independent of
	// whether the shell has forked and written the grandchild pid file yet;
	// under heavy host load that fork/write can lag past 150ms, so poll for
	// the file instead of a single read.
	pid := readPIDWithRetry(t, options.Dir+"/grandchild")
	waitForDeath(t, pid)
}

func readPIDWithRetry(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		data, readErr := readAll(path)
		if readErr == nil {
			if pid, convErr := strconv.Atoi(strings.TrimSpace(data)); convErr == nil {
				return pid
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("pid file %s did not contain a parseable pid within %s", path, 30*time.Second)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestDescendantCleanupOnOutputOverflow proves the whole process group dies on
// the output-bound path.
func TestDescendantCleanupOnOutputOverflow(t *testing.T) {
	options := shell()
	options.Dir = t.TempDir()
	options.StdoutLimit = 1 << 10
	_, err := Run(context.Background(), NewDefaultBudget(), options, "-c",
		"sleep 30 >/dev/null 2>&1 & echo $! > grandchild; yes overflow")
	if cemcode.CodeOf(err) != cemcode.GitOutputExceeded {
		t.Fatalf("got %v", err)
	}
	data, readErr := readAll(options.Dir + "/grandchild")
	if readErr != nil {
		t.Fatal(readErr)
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(data))
	if convErr != nil {
		t.Fatal(convErr)
	}
	waitForDeath(t, pid)
}

// TestTimeoutReapIsBoundedWhenEscapedDescendantHoldsPipes proves a descendant
// that left the child's process group while holding stdout and stderr cannot
// hold the post-kill reap past the pipe-drain bound.
func TestTimeoutReapIsBoundedWhenEscapedDescendantHoldsPipes(t *testing.T) {
	previous := pipeDrainDelay
	pipeDrainDelay = 100 * time.Millisecond
	t.Cleanup(func() { pipeDrainDelay = previous })
	options := shell()
	options.PerOpTimeout = 300 * time.Millisecond
	options.Dir = t.TempDir()
	started := time.Now()
	_, err := Run(context.Background(), NewDefaultBudget(), options, "-c", "set -m; sleep 30 & echo $! > escaped; sleep 30")
	elapsed := time.Since(started)
	if data, readErr := readAll(options.Dir + "/escaped"); readErr == nil {
		if pid, convErr := strconv.Atoi(strings.TrimSpace(data)); convErr == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
	if cemcode.CodeOf(err) != cemcode.GitTimeout || elapsed > 10*time.Second {
		t.Fatalf("got %v after %s", err, elapsed)
	}
}
