//go:build darwin || linux

package store

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// observedCheckout is a committed checkout holding a.txt, and the path of a
// script that records its pid in pidFile and then hangs.
func observedCheckout(t *testing.T) (root, hang, pidFile string) {
	t.Helper()
	root, dir := t.TempDir(), t.TempDir()
	pidFile = filepath.Join(dir, "pid")
	hang = filepath.Join(dir, "hang.sh")
	if err := fixture.WriteExecutable(hang, []byte("#!/bin/sh\necho $$ > "+pidFile+"\nexec sleep 600\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir, c.Env = root, gitEnvironment()
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	for name, body := range map[string]string{"a.txt": "a\n", ".gitattributes": "a.txt filter=hang\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("add", "-A")
	git("-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
	return root, hang, pidFile
}

// reaped waits a bounded time for the process pidFile names to be gone.
func reaped(t *testing.T, pidFile string) {
	t.Helper()
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		return // the hook never started
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("the observation left descendant %d running", pid)
}

// TestV13009_GateObservationRunsNoFsmonitor: a checkout whose core.fsmonitor
// is a hanging hook is still observed promptly, because the gate's worktree
// observation disables executable Git configuration (V1-3009).
func TestV13009_GateObservationRunsNoFsmonitor(t *testing.T) {
	t.Parallel()
	root, hang, pidFile := observedCheckout(t)
	c := exec.Command("git", "config", "core.fsmonitor", hang)
	c.Dir, c.Env = root, gitEnvironment()
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("config: %v %s", err, out)
	}
	done := make(chan error, 1)
	go func() {
		_, clean, err := worktreeTree(root)
		if err == nil && !clean {
			err = errors.New("the clean checkout read as dirty")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the worktree observation ran the hanging fsmonitor hook")
	}
	if _, err := os.Stat(pidFile); err == nil {
		t.Fatal("the worktree observation started the fsmonitor hook")
	}
}

// TestV13009_GateObservationEndsAtDeadline: a clean filter that hangs keeps
// git status running until the observation's context ends; the observation
// then returns LIMIT_EXCEEDED promptly and the filter, a descendant of Git,
// is killed with its process group (V1-3009).
func TestV13009_GateObservationEndsAtDeadline(t *testing.T) {
	t.Parallel()
	root, hang, pidFile := observedCheckout(t)
	c := exec.Command("git", "config", "filter.hang.clean", hang)
	c.Dir, c.Env = root, gitEnvironment()
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("config: %v %s", err, out)
	}
	// Same size, new mtime: status must read a.txt through the filter.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(root, "a.txt"), later, later); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := gateObservation(context.Background(), 2)
	defer cancel()
	started := time.Now()
	_, _, err := observeWorktreeTree(ctx, root)
	if wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("hanging status: %v", err)
	}
	if took := time.Since(started); took > 2*time.Second+gateWaitDelay+5*time.Second {
		t.Fatalf("the observation took %s past its deadline", took)
	}
	reaped(t, pidFile)

	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if _, _, err := observeWorktreeTree(cancelled, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled observation: %v", err)
	}
}

// TestV13009_GateObservationCapsOutput: an observation keeps at most
// gateObservationOutputMax bytes and reports the overflow, which the runner
// refuses and a status reads as not clean (V1-3009).
func TestV13009_GateObservationCapsOutput(t *testing.T) {
	t.Parallel()
	root, _, _ := observedCheckout(t)
	big := filepath.Join(root, "big")
	if err := os.WriteFile(big, []byte(strings.Repeat("x", gateObservationOutputMax+4096)), 0o644); err != nil {
		t.Fatal(err)
	}
	oid, _, err := observeGit(context.Background(), root, "hash-object", "-w", "--", "big")
	if err != nil {
		t.Fatal(err)
	}
	out, truncated, err := observeGit(context.Background(), root, "cat-file", "blob", strings.TrimSpace(string(oid)))
	if err != nil || !truncated || len(out) != gateObservationOutputMax {
		t.Fatalf("capped cat-file: %d %v %v", len(out), truncated, err)
	}
	if _, err := observedGitRunner(context.Background())(root, "cat-file", "blob", strings.TrimSpace(string(oid))); wire.CodeOf(err) != wire.CodeLimitExceeded {
		t.Fatalf("runner overflow: %v", err)
	}
}
