//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// recordedExecution installs an executionHost that records each group signal,
// probe and the reap around the real host, and returns the recording.
func recordedExecution(t *testing.T, killErr error) func() []string {
	t.Helper()
	if !groupreap.OwnerAvailable() {
		var x ownedExecution
		if out := x.run(context.Background(), exec.Command("true")); !out.unsupported || out.status != "UNSUPPORTED_EXECUTION_AUTHORITY" {
			t.Fatalf("run without an owner = %#v, want UNSUPPORTED_EXECUTION_AUTHORITY", out)
		}
		t.Skip("no process-group owner on this platform")
	}
	var mu sync.Mutex
	var calls []string
	add := func(call string) {
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
	}
	old := executionHost
	executionHost = func() groupreap.Primitives {
		host := old()
		kill, probe := host.KillGroup, host.ProbeGroup
		host.KillGroup = func(leader int) error {
			add("kill")
			if killErr != nil {
				return killErr
			}
			return kill(leader)
		}
		host.ProbeGroup = func(leader int) (groupreap.Probe, error) { add("probe"); return probe(leader) }
		host.Reap = func(command *exec.Cmd) error { add("reap"); return command.Wait() }
		return host
	}
	t.Cleanup(func() { executionHost = old })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(calls)
	}
}

// requireKillBeforeReap fails unless the group was SIGKILLed before the one
// reap and only signal-0 probes followed it (PGO-V0-019).
func requireKillBeforeReap(t *testing.T, calls []string) {
	t.Helper()
	reap := slices.Index(calls, "reap")
	if reap < 0 || slices.Contains(calls[reap+1:], "reap") {
		t.Fatalf("calls = %v, want exactly one reap", calls)
	}
	if kill := slices.Index(calls, "kill"); kill < 0 || kill > reap {
		t.Fatalf("calls = %v, want the group kill before the reap", calls)
	}
	for _, call := range calls[reap+1:] {
		if call != "probe" {
			t.Fatalf("calls = %v: %s after the reap", calls, call)
		}
	}
}

func shellCommand(t *testing.T, script string) (*exec.Cmd, string) {
	t.Helper()
	pidFile := filepath.Join(t.TempDir(), "pid")
	return exec.Command("/bin/sh", "-c", script, "sh", pidFile), pidFile
}

func memberFrom(t *testing.T, pidFile string) int {
	t.Helper()
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	return member
}

// TestPGOV0019ShadowCancelKillsTheGroupBeforeTheReap stops a candidate whose
// shell waits on a background member: the group SIGKILL precedes the reap,
// the member is gone, and the run's group is observed empty.
func TestPGOV0019ShadowCancelKillsTheGroupBeforeTheReap(t *testing.T) {
	seen := recordedExecution(t, nil)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	cmd := exec.Command("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $!; wait")
	cmd.Stdout = writer
	ctx, cancel := context.WithCancel(context.Background())
	var x ownedExecution
	done := make(chan executionOutcome, 1)
	go func() { done <- x.run(ctx, cmd) }()
	line := make(chan string, 1)
	go func() { text, _ := bufio.NewReader(reader).ReadString('\n'); line <- text }()
	var member int
	select {
	case text := <-line:
		member, err = strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			t.Fatalf("member pid %q: %v", text, err)
		}
		t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	case <-time.After(10 * time.Second):
		t.Fatal("candidate never reported its member")
	}
	cancel()
	out := <-done
	// Close the parent's write end only after run returned: os/exec reads the
	// *os.File in Start on run's goroutine, and the member's line gives no
	// happens-before edge back to that read.
	writer.Close()
	if out.group != "GROUP_EMPTY_OBSERVED" || out.status != "" {
		t.Fatalf("outcome = %#v, want GROUP_EMPTY_OBSERVED", out)
	}
	if err := syscall.Kill(member, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("member %d survived: %v", member, err)
	}
	requireKillBeforeReap(t, seen())
}

// TestPGOV0019ShadowResidueIsKilledBeforeTheReap runs a candidate that exits
// leaving a member that holds no pipe: the run is residue, the member is
// SIGKILLed while the leader is unreaped, and only probes follow the reap.
func TestPGOV0019ShadowResidueIsKilledBeforeTheReap(t *testing.T) {
	seen := recordedExecution(t, nil)
	cmd, pidFile := shellCommand(t, "sleep 300 >/dev/null 2>&1 & echo $! > \"$1\"")
	var x ownedExecution
	out := x.run(context.Background(), cmd)
	member := memberFrom(t, pidFile)
	if out.group != "CLEANED_AFTER_RESIDUE" || out.status != "PROCESS_GROUP_RESIDUE" || out.runErr != nil {
		t.Fatalf("outcome = %#v, want CLEANED_AFTER_RESIDUE", out)
	}
	if err := syscall.Kill(member, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("member %d survived: %v", member, err)
	}
	requireKillBeforeReap(t, seen())
}

// TestPGOV0019ShadowEarlyOutputStopIsApplied covers an output-limit stop that
// fires before the owner is installed: it is applied once the owner exists,
// still before the reap.
func TestPGOV0019ShadowEarlyOutputStopIsApplied(t *testing.T) {
	seen := recordedExecution(t, nil)
	var x ownedExecution
	x.stop()
	out := x.run(context.Background(), exec.Command("/bin/sh", "-c", "exec sleep 300"))
	if out.group != "GROUP_EMPTY_OBSERVED" || out.runErr == nil {
		t.Fatalf("outcome = %#v, want a killed run with an empty group", out)
	}
	requireKillBeforeReap(t, seen())
}

// TestPGOV0019ShadowUnprovedRetirementIsHold fails the group SIGKILL: the
// owner HOLDs without reaping, and the run is CLEANUP_ERROR, never success.
func TestPGOV0019ShadowUnprovedRetirementIsHold(t *testing.T) {
	seen := recordedExecution(t, errors.New("injected kill failure"))
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	t.Cleanup(func() { _ = cmd.Wait() })
	var x ownedExecution
	out := x.run(context.Background(), cmd)
	if out.status != "CLEANUP_ERROR" || out.reason != "PROCESS_GROUP_HOLD" || out.group != "NOT_EMPTY_OR_NOT_OBSERVED" {
		t.Fatalf("outcome = %#v, want CLEANUP_ERROR/PROCESS_GROUP_HOLD", out)
	}
	if slices.Contains(seen(), "reap") {
		t.Fatalf("calls = %v: a HOLD must not reap", seen())
	}
}

// TestPGOV0019ShadowResidueProofErrorIsCleanupError keeps a failed pre-reap
// residue proof visible while the group is still retired.
func TestPGOV0019ShadowResidueProofErrorIsCleanupError(t *testing.T) {
	seen := recordedExecution(t, nil)
	old := heldGroupQuiet
	heldGroupQuiet = func(int) (bool, error) { return false, errors.New("injected proof failure") }
	t.Cleanup(func() { heldGroupQuiet = old })
	var x ownedExecution
	out := x.run(context.Background(), exec.Command("/bin/sh", "-c", "exit 0"))
	if out.status != "CLEANUP_ERROR" || out.reason != "PROCESS_GROUP_STATE" || out.cleanup != "ERROR" {
		t.Fatalf("outcome = %#v, want CLEANUP_ERROR/PROCESS_GROUP_STATE", out)
	}
	requireKillBeforeReap(t, seen())
}
