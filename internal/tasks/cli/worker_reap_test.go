//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// reapHelperEnv carries "<mode>:<directory>" to TestATRV0310Helper.
const reapHelperEnv = "CORVINT_CLI_TEST_REAP_HELPER"

// sleeperScript is the worker of the reaper tests: a group of a shell and a
// background subshell that run until the file named by argv[0] is removed,
// so the test ends its group without signalling it.
const sleeperScript = `while [ -e "$0" ]; do /bin/sleep 1; done & while [ -e "$0" ]; do /bin/sleep 1; done; wait`

// TestATRV0310Helper runs only inside a child test binary. In the worker
// modes it starts a real dispatcher worker whose process group runs until
// the parent test removes its run file (the shape of the host.sh workers
// that survived V1-2305), records the group leader, and then, by mode, ends
// without reaping it ("leak"), registers the per-test reaper first
// ("cleanup"), or waits to be killed ("kill"). In "detach" mode it completes
// a detached claim whose child exits only well after its result is kept.
func TestATRV0310Helper(t *testing.T) {
	mode, dir, ok := strings.Cut(os.Getenv(reapHelperEnv), ":")
	if !ok {
		t.Skip("helper for TestATRV0310_LeakedWorkerGroupsAreReaped")
	}
	if mode == "detach" {
		root, _, id := detachClaimStore(t)
		t.Setenv("CORVINT_TEST_LEASE_DETACH_EXIT_MS", "1500")
		if x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "r-exit", "--detach", "--wait", "30"); x.code != 0 {
			t.Fatalf("detached claim: code %d %s", x.code, x.stdout)
		}
		return
	}
	if mode == "cleanup" {
		cli.ReapWorkersOnCleanup(t)
	}
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	planTicket(t, root, "reap", "P1", `["src/"]`)
	configDir, err := filepath.EvalSymlinks(t.TempDir()) // the config reader refuses symlinked paths
	if err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"profile": "taskman-dispatch/0", "stateDir": filepath.Join(configDir, "state"), "workRoot": root,
		"tickSeconds": 1, "globalCap": 1, "killGraceSeconds": 1,
		"hosts":   map[string]any{"sleeper": map[string]any{"argv": []string{"/bin/sh", "-c", sleeperScript, filepath.Join(dir, "run")}}},
		"roles":   []any{map[string]any{"name": "impl", "host": "sleeper", "cap": 1, "match": map[string]any{"planSelected": true}, "prompt": "implement {ticketLocal}", "idleSeconds": 3600, "wallSeconds": 7200}},
		"backoff": map[string]any{"cooldownSeconds": 3600, "parkAfter": 3},
		"heal":    map[string]any{"handoff": true, "reap": true},
	}
	raw, _ := json.Marshal(config)
	configPath := filepath.Join(configDir, "dispatch.json")
	fixture.Write(t, configPath, raw)
	base := []string{"--program", "prog", "--config", configPath}
	if x := atm(t, root, nil, append([]string{"dispatch", "--once"}, base...)...); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "workersRunning").Str != "1" {
		t.Fatalf("dispatch tick: %s %s", x.stdout, x.stderr)
	}
	status := atm(t, root, nil, append([]string{"dispatch", "status"}, base...)...)
	workers := field(status.res.Items[0], "workers").Arr
	if len(workers) != 1 {
		t.Fatalf("status: %s", status.stdout)
	}
	pid, _ := strconv.Atoi(field(workers[0], "pid").Str)
	id, err := supervisor.ProcessIdentity(pid)
	if err != nil || id == "" {
		t.Fatalf("worker %d identity %q: %v", pid, id, err)
	}
	tmp := filepath.Join(dir, "worker.tmp")
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf("%d %s\n", pid, id)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "worker")); err != nil {
		t.Fatal(err)
	}
	if mode == "kill" {
		time.Sleep(10 * time.Minute)
	}
}

// reapHelper is one child test binary running TestATRV0310Helper with its
// own armed checks and dead-man reaper.
type reapHelper struct {
	dir   string
	child *exec.Cmd
	out   *bytes.Buffer
	wait  func() error
}

// startHelper re-executes the test binary as TestATRV0310Helper in mode. The
// helper's worker group, if any, runs until the returned helper's run file
// is removed; a cleanup removes it and waits, bounded, for the group to end,
// and signals nothing.
func startHelper(t *testing.T, mode string) *reapHelper {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	h := &reapHelper{dir: t.TempDir(), out: &bytes.Buffer{}}
	if err := os.WriteFile(filepath.Join(h.dir, "run"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.child = exec.Command(exe, "-test.run=^TestATRV0310Helper$", "-test.count=1", "-test.timeout=15m")
	h.child.Env = append(os.Environ(), cli.TestRootEnv+"=", reapHelperEnv+"="+mode+":"+h.dir, cli.ReaperLogEnv+"="+filepath.Join(h.dir, "reaper.log"))
	h.child.Stdout, h.child.Stderr = h.out, h.out
	// The helper's reaper shares its output; a reaper that does not end
	// promptly is not waited on past a few seconds.
	h.child.WaitDelay = 5 * time.Second
	if err := h.child.Start(); err != nil {
		t.Fatal(err)
	}
	var exitErr error
	exited := make(chan struct{})
	go func() { exitErr = h.child.Wait(); close(exited) }()
	h.wait = func() error { <-exited; return exitErr }
	// The helper is this binary's own unreaped child until its Wait, which
	// os.Process serialises with Kill, so the kill reaches only the helper.
	t.Cleanup(func() { _ = h.child.Process.Kill(); <-exited })
	return h
}

// worker waits until the helper has started its worker and, in kill mode,
// until the helper's reaper has attributed the group, and returns the
// worker's group leader. The package timeout is the hang bound; this wait
// gives up after two minutes.
func (h *reapHelper) worker(t *testing.T, mode string) int {
	t.Helper()
	exited := make(chan struct{})
	go func() { _ = h.wait(); close(exited) }()
	giveUp := time.After(2 * time.Minute)
	worker := 0
	for ended := false; ; {
		if raw, err := os.ReadFile(filepath.Join(h.dir, "worker")); err == nil && worker == 0 {
			var id string
			if _, err := fmt.Sscan(string(raw), &worker, &id); err != nil || worker <= 0 {
				t.Fatalf("worker record %q: %v", raw, err)
			}
			t.Cleanup(func() {
				if err := os.Remove(filepath.Join(h.dir, "run")); err != nil {
					t.Error(err)
				}
				if !groupGone(t, worker, 30*time.Second) {
					t.Errorf("worker group %d did not end within 30 s of its run file's removal: %s", worker, groupMembers(worker))
				}
			})
		}
		if worker != 0 {
			if log, _ := os.ReadFile(filepath.Join(h.dir, "reaper.log")); mode != "kill" || strings.Contains(string(log), fmt.Sprintf("group %d\n", worker)) {
				return worker
			}
		} else if ended {
			t.Fatalf("helper exited before it started a worker:\n%s", h.out)
		}
		select {
		case <-exited:
			ended = true // read the worker record once more before failing
		case <-giveUp:
			t.Fatalf("helper did not start a worker, or its reaper never attributed the group, within 2 minutes:\n%s", h.out)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// groupMembers describes what still runs in a process group, for a failure
// message.
func groupMembers(pgid int) string {
	raw, _ := exec.Command("ps", "-axo", "pid=,ppid=,pgid=,stat=,comm=").Output()
	var rows []string
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Fields(line); len(f) >= 5 && f[2] == strconv.Itoa(pgid) {
			rows = append(rows, strings.Join(f, " "))
		}
	}
	return strings.Join(rows, "; ")
}

func groupRunning(t *testing.T, pgid int) bool {
	t.Helper()
	running, err := cli.GroupRunning(pgid)
	if err != nil {
		t.Fatal(err)
	}
	return running
}

func groupGone(t *testing.T, pgid int, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !groupRunning(t, pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !groupRunning(t, pgid)
}

// reapedOrReported checks what became of a leaked worker group once a check
// ran: where the reaper can prove ownership the group is killed; elsewhere it
// is named as not signalled and is still running, for no signal reached it.
func reapedOrReported(t *testing.T, worker int, out string) {
	t.Helper()
	if cli.ReaperSignals() {
		if !groupGone(t, worker, 30*time.Second) {
			t.Fatalf("worker group %d survived the check (members: %s):\n%s", worker, groupMembers(worker), out)
		}
		if !strings.Contains(out, "killed") {
			t.Fatalf("the check did not report the group it killed:\n%s", out)
		}
		return
	}
	if !strings.Contains(out, "not signalled, ownership not proven") || !strings.Contains(out, strconv.Itoa(worker)) {
		t.Fatalf("the check did not report worker group %d as left unsignalled:\n%s", worker, out)
	}
	if !groupRunning(t, worker) {
		t.Fatalf("worker group %d ended although no signal may reach it:\n%s", worker, out)
	}
}

// ATR-V0-310 (V1-2305): a dispatcher worker group started by a test in this
// package fails the test, and is killed where ownership can be proven,
// whichever way the test ends. A test that registers the per-test reaper
// fails when it leaks; a leak no test registered fails the package; and a
// test binary killed outright (SIGKILL, or the package timeout) runs
// neither, so the dead-man reaper must find the group.
func TestATRV0310_LeakedWorkerGroupsAreReaped(t *testing.T) {
	t.Run("per-test cleanup fails the leaking test", func(t *testing.T) {
		h := startHelper(t, "cleanup")
		worker := h.worker(t, "cleanup")
		err := h.wait()
		if err == nil || !strings.Contains(h.out.String(), "outlived the test") {
			t.Fatalf("a test that leaked its worker group did not fail (%v):\n%s", err, h.out)
		}
		reapedOrReported(t, worker, h.out.String())
	})
	t.Run("package check fails the run", func(t *testing.T) {
		h := startHelper(t, "leak")
		worker := h.worker(t, "leak")
		err := h.wait()
		if err == nil || !strings.Contains(h.out.String(), "tests left worker process groups running") {
			t.Fatalf("a leak no test reaped did not fail the package (%v):\n%s", err, h.out)
		}
		reapedOrReported(t, worker, h.out.String())
	})
	t.Run("a killed test binary's worker group is found by the dead-man reaper", func(t *testing.T) {
		h := startHelper(t, "kill")
		worker := h.worker(t, "kill")
		if !groupRunning(t, worker) {
			t.Fatalf("worker group %d is not running before the kill:\n%s", worker, h.out)
		}
		if err := h.child.Process.Signal(syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		_ = h.wait()
		if !strings.Contains(h.out.String(), "dead-man reaper: worker process groups outlived the test binary") {
			t.Fatalf("the dead-man reaper did not report the group of its SIGKILLed binary:\n%s", h.out)
		}
		reapedOrReported(t, worker, h.out.String())
	})
}

// ATR-V0-310 (V1-2305, review finding 7): a detached claim's result is
// readable before its child exits, so a test that reads it can end with the
// child still running. The fixture joins the children it launched before
// its cleanup ends, and the package leak check, which stays strict, then
// finds nothing. The helper's child stays 1.5 s past its kept result.
func TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck(t *testing.T) {
	h := startHelper(t, "detach")
	err := h.wait()
	if err != nil || strings.Contains(h.out.String(), "left worker process groups") || strings.Contains(h.out.String(), "dead-man reaper") {
		t.Fatalf("a completed detached claim failed the package's leak check (%v):\n%s", err, h.out)
	}
}
