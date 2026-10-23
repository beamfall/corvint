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

// TestATRV0310Helper runs only inside a child test binary. It starts a real
// dispatcher worker whose process group sleeps (the shape of the host.sh
// workers that survived V1-2305), records the group leader, and then, by
// mode, ends without reaping it ("leak"), registers the per-test reaper
// first ("cleanup"), or waits to be killed ("kill").
func TestATRV0310Helper(t *testing.T) {
	mode, dir, ok := strings.Cut(os.Getenv(reapHelperEnv), ":")
	if !ok {
		t.Skip("helper for TestATRV0310_LeakedWorkerGroupsAreReaped")
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
		"hosts":   map[string]any{"sleeper": map[string]any{"argv": []string{"/bin/sh", "-c", "/bin/sleep 600 & /bin/sleep 600; wait"}}},
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

// startReapHelper re-executes the test binary as TestATRV0310Helper with its
// own armed reaper, waits until it has started its worker, and returns the
// worker's group leader, the child, its combined output and a function that
// waits for the child to end and returns its error.
func startReapHelper(t *testing.T, mode string) (worker int, child *exec.Cmd, out *bytes.Buffer, wait func() error) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out = &bytes.Buffer{}
	child = exec.Command(exe, "-test.run=^TestATRV0310Helper$", "-test.count=1", "-test.timeout=15m")
	child.Env = append(os.Environ(), cli.TestRootEnv+"=", reapHelperEnv+"="+mode+":"+dir, cli.ReaperLogEnv+"="+filepath.Join(dir, "reaper.log"))
	child.Stdout, child.Stderr = out, out
	// A worker group the helper leaves behind outlives a bounded Wait, so the
	// copy of its output is not waited on past a few seconds.
	child.WaitDelay = 5 * time.Second
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	var exitErr error
	exited := make(chan struct{})
	go func() { exitErr = child.Wait(); close(exited) }()
	wait = func() error { <-exited; return exitErr }
	t.Cleanup(func() { _ = child.Process.Kill() })
	// The package timeout is the hang bound for the helper; this wait gives
	// up after two minutes. A helper that exits before it recorded a worker is
	// a failure of the setup, not of the reaper. The helper's reaper samples
	// the process table, so a killed helper's worker is also awaited in the
	// reaper's log: a binary killed before the reaper has attributed the group
	// is outside what the reaper promises.
	giveUp := time.After(2 * time.Minute)
	for ended := false; ; {
		if raw, err := os.ReadFile(filepath.Join(dir, "worker")); err == nil {
			var id string
			if _, err := fmt.Sscan(string(raw), &worker, &id); err != nil || worker <= 0 {
				t.Fatalf("worker record %q: %v", raw, err)
			}
			t.Cleanup(func() { _ = syscall.Kill(-worker, syscall.SIGKILL) })
			if log, _ := os.ReadFile(filepath.Join(dir, "reaper.log")); mode != "kill" || strings.Contains(string(log), fmt.Sprintf("group %d\n", worker)) {
				return worker, child, out, wait
			}
		} else if ended {
			t.Fatalf("helper exited (%v) before it started a worker:\n%s", exitErr, out)
		}
		select {
		case <-exited:
			ended = true // read the worker record once more before failing
		case <-giveUp:
			t.Fatalf("helper did not start a worker, or its reaper never recorded the group, within 2 minutes:\n%s", out)
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

func groupGone(t *testing.T, pgid int, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if !cli.GroupPresent(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !cli.GroupPresent(pgid)
}

// ATR-V0-310 (V1-2305): a dispatcher worker group started by a test in this
// package is gone when the test ends, whichever way it ends. A test that
// registers the per-test reaper fails when it leaks and its group is killed;
// a leak no test registered fails the package and is killed by TestMain; and
// a test binary killed outright (SIGKILL, or the package timeout) runs
// neither, so the dead-man reaper must stop the group.
func TestATRV0310_LeakedWorkerGroupsAreReaped(t *testing.T) {
	t.Run("per-test cleanup fails the leaking test and kills the group", func(t *testing.T) {
		worker, _, out, wait := startReapHelper(t, "cleanup")
		err := wait()
		if err == nil || !strings.Contains(out.String(), "outlived the test") {
			t.Fatalf("a test that leaked its worker group did not fail (%v):\n%s", err, out)
		}
		if !groupGone(t, worker, 30*time.Second) {
			t.Fatalf("worker group %d survived the failing test:\n%s", worker, out)
		}
	})
	t.Run("package check fails the run and kills the group", func(t *testing.T) {
		worker, _, out, wait := startReapHelper(t, "leak")
		err := wait()
		if err == nil || !strings.Contains(out.String(), "tests left worker process groups running") {
			t.Fatalf("a leak no test reaped did not fail the package (%v):\n%s", err, out)
		}
		if !groupGone(t, worker, 30*time.Second) {
			t.Fatalf("worker group %d survived the package check:\n%s", worker, out)
		}
	})
	t.Run("a killed test binary leaves no worker group", func(t *testing.T) {
		worker, child, out, wait := startReapHelper(t, "kill")
		if syscall.Kill(-worker, 0) != nil {
			t.Fatalf("worker group %d is not running before the kill:\n%s", worker, out)
		}
		if err := child.Process.Signal(syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		_ = wait()
		// The bound is generous for a loaded host; the reaper acts at once
		// on end of file, and without it the group never empties.
		if !groupGone(t, worker, 30*time.Second) {
			t.Fatalf("worker group %d outlived its SIGKILLed test binary (members: %s):\n%s", worker, groupMembers(worker), out)
		}
	})
}
