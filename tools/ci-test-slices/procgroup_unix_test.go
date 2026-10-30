//go:build unix

package main

import (
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

// recordHost wraps the platform owner host and records each group signal and
// the reap, so a test can pin that every group signal precedes the reap and
// that only signal-0 probes follow it (PGO-V0-019, V1-1387).
type recordHost struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordHost) add(call string) {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
}

func (r *recordHost) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

func (r *recordHost) primitives(base groupreap.Primitives) groupreap.Primitives {
	real := groupreap.HostPrimitives()
	base.KillGroup = func(leader int) error { r.add("kill"); return real.KillGroup(leader) }
	base.SignalGroup = func(leader int, signal syscall.Signal) error {
		r.add("signal-" + signal.String())
		return real.SignalGroup(leader, signal)
	}
	base.QuietProof = real.QuietProof
	base.ProbeGroup = func(leader int) (groupreap.Probe, error) { r.add("probe"); return real.ProbeGroup(leader) }
	base.Reap = func(command *exec.Cmd) error { r.add("reap"); return command.Wait() }
	return base
}

// requireSignalsBeforeReap fails unless want signals were sent, all before
// the one reap, and nothing but signal-0 probes followed the reap.
func requireSignalsBeforeReap(t *testing.T, calls []string, want ...string) {
	t.Helper()
	reap := slices.Index(calls, "reap")
	if reap < 0 || slices.Index(calls[reap+1:], "reap") >= 0 {
		t.Fatalf("calls = %v, want exactly one reap", calls)
	}
	for _, call := range calls[reap+1:] {
		if call != "probe" {
			t.Fatalf("calls = %v: %s after the reap", calls, call)
		}
	}
	for _, signal := range want {
		if i := slices.Index(calls, signal); i < 0 || i > reap {
			t.Fatalf("calls = %v, want %s before the reap", calls, signal)
		}
	}
}

// TestAFPV0041CancelStopsTheEnumerationGroup runs enumeration against a fake
// `go` whose sleeping child ignores SIGINT and holds the output pipe, and
// checks that cancellation leaves no member of the group alive.
func TestAFPV0041CancelStopsTheEnumerationGroup(t *testing.T) {
	old := stopGrace
	stopGrace = 300 * time.Millisecond
	t.Cleanup(func() { stopGrace = old })
	if !groupreap.OwnerAvailable() {
		if err := runOwned(context.Background(), exec.Command("true")); !errors.Is(err, groupreap.ErrOwnerUnavailable) {
			t.Fatalf("runOwned without an owner = %v, want ErrOwnerUnavailable", err)
		}
		t.Skip("no process-group owner on this platform")
	}
	for name, leader := range map[string]string{
		"leader interruptible":  "",
		"leader ignores SIGINT": "trap '' INT\n",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := &recordHost{}
			oldHost := host
			host = recorder.primitives(host)
			t.Cleanup(func() { host = oldHost })
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
			// The interrupt and the kill both precede the reap (V1-1387).
			requireSignalsBeforeReap(t, recorder.seen(), "signal-interrupt", "kill")
		})
	}
}

// TestPGOV0019NormalExitRetiresLingeringMemberBeforeReap runs a subprocess
// that exits leaving a member that holds no pipe, and checks the group is
// SIGKILLed while the leader is unreaped, reaped once, then only probed.
func TestPGOV0019NormalExitRetiresLingeringMemberBeforeReap(t *testing.T) {
	if !groupreap.OwnerAvailable() {
		t.Skip("no process-group owner on this platform")
	}
	recorder := &recordHost{}
	oldHost := host
	host = recorder.primitives(host)
	t.Cleanup(func() { host = oldHost })
	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd := child("/bin/sh", "-c", "sleep 300 >/dev/null 2>&1 & echo $! > \"$1\"", "sh", pidFile)
	var out strings.Builder
	cmd.Stdout = &out
	if err := runOwned(context.Background(), cmd); err != nil {
		t.Fatalf("runOwned: %v", err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	member, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	t.Cleanup(func() { _ = syscall.Kill(member, syscall.SIGKILL) })
	if err := syscall.Kill(-cmd.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("group %d still has a member: %v", cmd.Process.Pid, err)
	}
	requireSignalsBeforeReap(t, recorder.seen(), "kill")
	if slices.Contains(recorder.seen(), "signal-interrupt") {
		t.Fatalf("calls = %v: interrupt without cancellation", recorder.seen())
	}
}
