//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package doccompiler

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
)

func TestRunCommandCancellationKillsProcessGroupDescendant(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "child.pid")
	scriptPath := filepath.Join(root, "runner")
	script := "#!/bin/sh\n/bin/sleep 3600 &\necho $! > '" + pidPath + "'\nwait\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Join(root, "state")
	if err := os.MkdirAll(stateRoot, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runCommand(ctx, scriptPath, nil, root, []string{"PATH=" + root}, 1024, 1024)
		done <- err
	}()
	pid := waitForProcessPID(t, pidPath, done)
	cancel()
	if err := <-done; errorCode(err) != "command-timeout" {
		t.Fatalf("error = %v", err)
	}
	requireDescendantGone(t, pid)
}

// waitForProcessPID waits for the runner's PID handoff. Its sync points are the handoff itself
// and the command's return, never a wall clock: under host load the shell can take longer than
// any fixed margin to write the file (V1-1481), and a hang is the package hang detector's to
// report. A command that returns before the handoff fails the test with its error.
func waitForProcessPID(t *testing.T, path string, done <-chan error) int {
	t.Helper()
	for {
		raw, err := os.ReadFile(path)
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && parseErr == nil && pid > 0 {
			// A survivor would otherwise outlive the test by its sleep.
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			return pid
		}
		select {
		case commandErr := <-done:
			t.Fatalf("command returned before the PID handoff at %s: %v (read=%v parse=%v value=%q)", path, commandErr, err, parseErr, raw)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// requireDescendantGone asserts that pid no longer runs once runCommand has returned. That
// return is the sync point: the descendant inherits the command's stdout pipe, so Wait returns
// only after every holder has closed it by exiting, or after the one-minute WaitDelay with the
// holder still alive. The descendant is then either reaped or a zombie awaiting its new
// parent's reap; any other state is a survivor. No wall-clock poll decides it (V1-1481).
func requireDescendantGone(t *testing.T, pid int) {
	t.Helper()
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return
	}
	out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	state := strings.TrimSpace(string(out))
	var exitErr *exec.ExitError
	switch {
	case err == nil && strings.HasPrefix(state, "Z"):
		return
	case errors.As(err, &exitErr) && state == "":
		return // ps found no such process: reaped between the two probes
	case err != nil:
		t.Fatalf("cannot read the state of descendant %d: %v", pid, err)
	}
	t.Fatalf("descendant %d survived cancellation in state %q", pid, state)
}

func TestUnixContainmentRemainsUnqualifiedForSessionEscapes(t *testing.T) {
	qualified, description := descendantCleanupQualification()
	if qualified || !strings.Contains(description, "setsid/session escapes") {
		t.Fatalf("qualified=%v description=%q", qualified, description)
	}
}
