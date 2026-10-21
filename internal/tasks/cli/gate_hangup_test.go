//go:build darwin || linux

package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateHangupEnv carries the root and argv of a gate-run subprocess.
const gateHangupEnv = "CAL_GATE_HANGUP_RUNNER"

// TestCALV0319_HangupInterruptsGateRun: SIGHUP to `gate run` (a closed
// terminal or a dropped agent session) takes the same path as SIGINT and
// SIGTERM: the gate's process group is stopped and the run is recorded with
// its partial output and the interruption warning (V1-2033).
func TestCALV0319_HangupInterruptsGateRun(t *testing.T) {
	if spec := os.Getenv(gateHangupEnv); spec != "" {
		parts := strings.Split(spec, "\x1f")
		var out bytes.Buffer
		code := cli.Run(cli.Env{Cwd: parts[0], Args: parts[1:], Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: os.Stderr})
		_, _ = os.Stdout.Write(out.Bytes())
		os.Exit(code)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	// A gate argv entry is at most 128 bytes, so the pid file and the clean
	// worktree live under a short directory.
	dir, err := os.MkdirTemp("", "hup")
	if err == nil {
		t.Cleanup(func() { os.RemoveAll(dir) })
		dir, err = filepath.EvalSymlinks(dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(dir, "pid")
	gate := commandGate("verify", "/bin/sh", "-c", `printf partial; sleep 60 & echo "$$ $!" > "$0.tmp" && mv "$0.tmp" "$0"; wait`, pidFile)
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, gate)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	clean := filepath.Join(dir, "clean")
	git(t, root, "worktree", "add", "--detach", clean, "HEAD")
	id := planTicket(t, root, "hangup", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-hup")
	attempt, gen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", gen, "--tree", tree, "--request-id", "submit-hup")

	args := []string{root, "gate", "run", "--attempt", attempt, "--generation", gen, "--gate", "verify", "--request-id", "gate-hup", "--worktree", clean}
	runner := exec.Command(os.Args[0], "-test.run=^TestCALV0319_HangupInterruptsGateRun$")
	runner.Env = append(os.Environ(), gateHangupEnv+"="+strings.Join(args, "\x1f"))
	var out, errb bytes.Buffer
	runner.Stdout, runner.Stderr = &out, &errb
	// A surviving gate holds the output pipes; do not wait for it.
	runner.WaitDelay = 2 * time.Second
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- runner.Wait() }()
	// The script starts its descendant first and then publishes "group child"
	// atomically, so the signal cannot arrive before the descendant exists.
	group, child := 0, 0
	t.Cleanup(func() {
		if t.Failed() && group > 0 {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
	})
	for deadline := time.Now().Add(60 * time.Second); group == 0; time.Sleep(20 * time.Millisecond) {
		if raw, err := os.ReadFile(pidFile); err == nil {
			pids := strings.Fields(string(raw))
			if len(pids) != 2 {
				t.Fatalf("readiness file %q is not \"group child\"", raw)
			}
			if group, err = strconv.Atoi(pids[0]); err != nil {
				t.Fatal(err)
			}
			if child, err = strconv.Atoi(pids[1]); err != nil {
				t.Fatal(err)
			}
		} else if time.Now().After(deadline) {
			_ = runner.Process.Kill()
			t.Fatalf("gate never started\n%s", errb.Bytes())
		}
	}
	if err := syscall.Kill(child, 0); err != nil {
		t.Fatalf("gate descendant %d is not live before SIGHUP: %v", child, err)
	}
	if pgid, err := syscall.Getpgid(child); err != nil || pgid != group {
		t.Fatalf("gate descendant %d is in group %d (%v), want %d", child, pgid, err, group)
	}
	if err := runner.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(60 * time.Second):
		_ = runner.Process.Kill()
		t.Fatal("gate run did not exit after SIGHUP")
	}
	for i := 0; ; i++ {
		groupErr, childErr := syscall.Kill(-group, 0), syscall.Kill(child, 0)
		if errors.Is(groupErr, syscall.ESRCH) && errors.Is(childErr, syscall.ESRCH) {
			break
		}
		if i > 200 {
			t.Fatalf("gate group %d or descendant %d survived the SIGHUP (runner %s)\n%s", group, child, runner.ProcessState, errb.Bytes())
		}
		time.Sleep(10 * time.Millisecond)
	}
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("envelope: %v (runner %s)\nstdout=%s\nstderr=%s", err, runner.ProcessState, out.Bytes(), errb.Bytes())
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "gate run interrupted") {
		t.Fatalf("no interruption warning (runner %s)\nstdout=%s\nstderr=%s", runner.ProcessState, out.Bytes(), errb.Bytes())
	}
	// The run is recorded (a receipt, not a refusal); the store test pins the
	// FAILED result and its captured output.
	if res.Outcome != wire.OutcomeOK || field(res.Items[0], "outcome").Str != "COMPLETED" || field(res.Items[0], "receipt").Str == "" {
		t.Fatalf("interrupted gate run was not recorded: %s", out.Bytes())
	}
}
