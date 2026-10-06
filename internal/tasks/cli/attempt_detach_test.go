//go:build darwin || linux

package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const detachHelperEnv = "ATR_DETACH_HELPER"

var detachHelperPrefix = []string{"-test.run=^TestATRV0009_DetachedSupervisorHelper$", "--"}

// TestATRV0009_DetachedSupervisorHelper is the re-executed launcher or
// supervisor of the detached-run tests; it runs only under its marker.
func TestATRV0009_DetachedSupervisorHelper(t *testing.T) {
	if os.Getenv(detachHelperEnv) != "1" {
		t.Skip("helper process for the detached-run tests")
	}
	if ms, err := strconv.Atoi(os.Getenv("CORVINT_TEST_RUN_BEAT_MS")); err == nil && ms > 0 {
		cli.SetAttemptBeatInterval(time.Duration(ms) * time.Millisecond)
	}
	if n, err := strconv.ParseInt(os.Getenv("CORVINT_TEST_RUN_SEGMENT"), 10, 64); err == nil && n > 0 {
		cli.SetRunSegmentBytes(n)
	}
	cli.SetDetachExecutable(os.Args[0], detachHelperPrefix...)
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(cli.Run(cli.Env{Cwd: cwd, Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}

// detachedEnv makes a detached launcher re-run this test binary as its
// supervisor, with a fast heartbeat and attach poll.
func detachedEnv(t *testing.T) {
	t.Helper()
	t.Setenv(detachHelperEnv, "1")
	t.Setenv("CORVINT_TEST_RUN_BEAT_MS", "200")
	t.Cleanup(cli.SetDetachExecutable(os.Args[0], detachHelperPrefix...))
	t.Cleanup(cli.SetRunPoll(20 * time.Millisecond))
}

func cliRun(t *testing.T, root string, args ...string) run {
	t.Helper()
	var out, errb bytes.Buffer
	code := cli.Run(cli.Env{Cwd: root, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &errb})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("%v: envelope does not decode: %v\nstdout=%s\nstderr=%s", args, err, out.Bytes(), errb.Bytes())
	}
	return run{code: code, stdout: out.Bytes(), stderr: errb.Bytes(), res: res}
}

func attach(t *testing.T, root string, a *store.Report, extra ...string) run {
	t.Helper()
	return cliRun(t, root, append([]string{"run", "--attach", "--attempt", a.AttemptID}, extra...)...)
}

func readPid(t *testing.T, p string) int {
	t.Helper()
	waitFile(t, p)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func releaseAttempt(t *testing.T, root string, a *store.Report, requestID string) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	lease := transaction.LeaseRequest{Verb: transaction.LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation}
	report, err := store.Lease(context.Background(), repo, mutation.Binding{ID: "tester", Role: "OWNER"}, store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: root, Lease: lease, Derive: store.NoScopeDeriver}, wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z")))
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("release: %+v %v", report, err)
	}
}

// heartbeatAfter waits, bounded, until the attempt records a heartbeat in a
// later second than after: the detached supervisor beats on its own.
func heartbeatAfter(t *testing.T, root, attemptID string, after time.Time) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		rec, err := store.AttemptRecord(context.Background(), repo, attemptID)
		if err != nil {
			t.Fatal(err)
		}
		if rec.LastHeartbeatAt != nil {
			if at, err := time.Parse(time.RFC3339, string(*rec.LastHeartbeatAt)); err == nil && at.After(after.Truncate(time.Second)) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no heartbeat after the launcher exited at %s; last %v", after.Format(time.RFC3339), rec.LastHeartbeatAt)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestATRV0008_DetachedRunSurvivesItsLauncher: the launcher process exits
// while the command runs outside its process group; a successor attaches,
// waits, and reads the recorded outcome and the bounded output.
func TestATRV0008_DetachedRunSurvivesItsLauncher(t *testing.T) {
	root, a := attemptStore(t)
	detachedEnv(t)
	t.Setenv("CORVINT_TEST_RUN_SEGMENT", "64")
	dir := t.TempDir()
	pidFile, release := filepath.Join(dir, "pid"), filepath.Join(dir, "go")
	script := "echo $$ > " + pidFile + ".tmp && mv " + pidFile + ".tmp " + pidFile + "; while [ ! -f " + release + " ]; do sleep 0.05; done; " +
		"i=0; while [ $i -lt 10 ]; do echo line-$i-0123456789; i=$((i+1)); done; exit 3"
	args := []string{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "60", "--detach", "--", "/bin/sh", "-c", script}
	launcher := exec.Command(os.Args[0], append(append([]string{}, detachHelperPrefix...), args...)...)
	launcher.Dir = root
	launcher.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out, errb bytes.Buffer
	launcher.Stdout, launcher.Stderr = &out, &errb
	t.Cleanup(func() {
		if t.Failed() {
			_ = os.WriteFile(release, nil, 0o600)
		}
	})
	if err := launcher.Run(); err != nil {
		t.Fatalf("launcher: %v\nstdout=%s\nstderr=%s", err, out.Bytes(), errb.Bytes())
	}
	launcherExited := time.Now().UTC()
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 || field(res.Items[0], "state").Str != "RUNNING" {
		t.Fatalf("launch envelope %v: %s", err, out.Bytes())
	}
	runID := field(res.Items[0], "runId").Str
	command := readPid(t, pidFile)
	// The launcher has exited and nothing is left in its process group, yet
	// the command still runs: a group stop of the launcher cannot reach it.
	if err := syscall.Kill(-launcher.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("launcher group still has members: %v", err)
	}
	if err := syscall.Kill(command, 0); err != nil {
		t.Fatalf("command did not survive its launcher: %v", err)
	}
	heartbeatAfter(t, root, a.AttemptID, launcherExited)
	if r := attach(t, root, a, "--wait", "0"); r.code != 75 || r.res.Outcome != wire.OutcomeRefused || field(r.res.Items[0], "state").Str != "RUNNING" || field(r.res.Items[0], "runId").Str != runID {
		t.Fatalf("pending attach: code %d %s", r.code, r.stdout)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := attach(t, root, a, "--wait", "60")
	if r.code != 3 || r.res.Outcome != wire.OutcomeOK || field(r.res.Items[0], "class").Str != transaction.RunExit || field(r.res.Items[0], "childExit").Str != "3" {
		t.Fatalf("finished attach: code %d %s", r.code, r.stdout)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunExit || o.ExitCode == nil || *o.ExitCode != 3 || o.RunID != runID {
		t.Fatalf("outcome %+v", o)
	}
	item := r.res.Items[0]
	total, _ := strconv.Atoi(field(item, "outputBytes").Str)
	dropped, _ := strconv.Atoi(field(item, "outputDroppedBytes").Str)
	output := field(item, "output").Str
	current, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	previous, _ := os.ReadFile(filepath.Join(filepath.Dir(output), "output.1.log"))
	if total != 10*len("line-0-0123456789\n") || len(current) > 64 || len(previous) > 64 || total-dropped != len(current)+len(previous) || !strings.HasSuffix(string(current), "line-9-0123456789\n") {
		t.Fatalf("bounded output: total %d dropped %d current %q previous %q", total, dropped, current, previous)
	}
	if again := attach(t, root, a, "--run", runID); again.code != 3 || !bytes.Equal(again.stdout, r.stdout) {
		t.Fatalf("second attach differs: code %d\n%s\n%s", again.code, again.stdout, r.stdout)
	}
}

// TestATRV0013_FencedAttemptKillsTheDetachedCommand: releasing the attempt
// fences the detached run; its supervisor retires the command's group and
// records LOST_LEASE, which an attach reads.
func TestATRV0013_FencedAttemptKillsTheDetachedCommand(t *testing.T) {
	root, a := attemptStore(t)
	detachedEnv(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	start := time.Now()
	launched := runAttempt(t, root, a, gen(a), []string{"--timeout", "60", "--detach"}, "/bin/sh", "-c", "echo $$ > "+pidFile+".tmp && mv "+pidFile+".tmp "+pidFile+"; sleep 60 & wait")
	if launched.code != 0 || field(launched.res.Items[0], "state").Str != "RUNNING" {
		t.Fatalf("launch: code %d %s", launched.code, launched.stdout)
	}
	group := readPid(t, pidFile)
	t.Cleanup(func() {
		if t.Failed() {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		}
	})
	releaseAttempt(t, root, a, "release-under-detached-run")
	r := attach(t, root, a, "--wait", "60")
	if r.code != 125 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeFenced) {
		t.Fatalf("fenced attach: code %d %s", r.code, r.stdout)
	}
	if o := recordedOutcome(t, root, r); o.Class != transaction.RunLostLease || o.Cleanup != transaction.CleanupReleased || o.LostLease == nil || *o.LostLease != wire.CodeFenced {
		t.Fatalf("fenced outcome %+v", o)
	}
	if err := syscall.Kill(-group, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("command group survived the fence: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("fence took %s", elapsed)
	}
}

// TestATRV0009_DetachedPreLaunchRefusalIsReplayed: a detached run of an
// already released attempt starts nothing; the launcher prints the
// supervisor's refusal with the attached runner's status.
func TestATRV0009_DetachedPreLaunchRefusalIsReplayed(t *testing.T) {
	root, a := attemptStore(t)
	detachedEnv(t)
	releaseAttempt(t, root, a, "release-before-detached-run")
	marker := filepath.Join(t.TempDir(), "ran")
	r := runAttempt(t, root, a, gen(a), []string{"--timeout", "5", "--detach"}, "/bin/sh", "-c", "touch "+marker)
	if r.code != 125 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeFenced) || len(r.res.Items) != 0 {
		t.Fatalf("pre-launch refusal: code %d %s", r.code, r.stdout)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the command ran")
	}
	if again := attach(t, root, a); again.code != 125 || !bytes.Equal(again.stdout, r.stdout) {
		t.Fatalf("attach of the refused run: code %d %s", again.code, again.stdout)
	}
}

// writeRecord plants a run record for the attach-only tests.
func writeRecord(t *testing.T, base, runID string, rec map[string]any) string {
	t.Helper()
	dir := filepath.Join(base, runID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func plantedRecord(a *store.Report, runID, state string, pid int, identity string) map[string]any {
	return map[string]any{"profile": "taskman-attempt-run-record/0", "runId": runID, "attemptId": a.AttemptID, "generation": gen(a),
		"argvSha256": strings.Repeat("0", 64), "timeoutSeconds": 60, "state": state, "launchedAt": "2026-10-06T00:00:00Z",
		"supervisorPid": pid, "supervisorIdentity": identity, "commandPid": pid, "commandIdentity": nil,
		"endedAt": nil, "exitStatus": nil, "resultSha256": nil, "outputBytes": 0, "outputDroppedBytes": 0}
}

// TestATRV0012_AttachRefusesAProcessIdentityMismatch: a recorded PID whose
// start identity differs, or that is gone, is a lost supervisor, never a
// live run; nothing is signalled. A live identity waits; a kept result that
// does not match its record is refused.
func TestATRV0012_AttachRefusesAProcessIdentityMismatch(t *testing.T) {
	root, a := attemptStore(t)
	t.Cleanup(cli.SetRunPoll(20 * time.Millisecond))
	base, err := cli.RunDirForTest(root, a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	self, err := supervisor.ProcessIdentity(os.Getpid())
	if err != nil || self == "" {
		t.Fatalf("own identity: %q %v", self, err)
	}
	mismatch := plantedRecord(a, "00000000000000a1", "RUNNING", os.Getpid(), self+"-other")
	mismatch["commandIdentity"] = self
	writeRecord(t, base, "00000000000000a1", mismatch)
	r := attach(t, root, a, "--run", "00000000000000a1", "--wait", "0")
	if r.code != 1 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeSupervisorLost) || !strings.Contains(strings.Join(r.res.Warnings, " "), "still running unsupervised") {
		t.Fatalf("identity mismatch: code %d %s", r.code, r.stdout)
	}
	gone := exec.Command("/bin/sh", "-c", "exit 0")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	writeRecord(t, base, "00000000000000a2", plantedRecord(a, "00000000000000a2", "STARTING", gone.Process.Pid, self))
	if r := attach(t, root, a, "--run", "00000000000000a2", "--wait", "0"); r.code != 1 || !hasCode(r.res, wire.CodeSupervisorLost) {
		t.Fatalf("gone supervisor: code %d %s", r.code, r.stdout)
	}
	writeRecord(t, base, "00000000000000a3", plantedRecord(a, "00000000000000a3", "RUNNING", os.Getpid(), self))
	if r := attach(t, root, a, "--run", "00000000000000a3", "--wait", "0"); r.code != 75 || r.res.Outcome != wire.OutcomeRefused || len(r.res.Codes) != 0 || field(r.res.Items[0], "state").Str != "RUNNING" {
		t.Fatalf("live supervisor: code %d %s", r.code, r.stdout)
	}
	finished := plantedRecord(a, "00000000000000a4", "FINISHED", os.Getpid(), self)
	finished["endedAt"], finished["exitStatus"], finished["resultSha256"] = "2026-10-06T00:00:01Z", 0, strings.Repeat("0", 64)
	dir := writeRecord(t, base, "00000000000000a4", finished)
	if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := attach(t, root, a, "--run", "00000000000000a4"); r.code != 1 || r.res.Outcome != wire.OutcomeError || !hasCode(r.res, wire.CodeMalformed) {
		t.Fatalf("tampered result: code %d %s", r.code, r.stdout)
	}
	unknown := plantedRecord(a, "00000000000000a5", "RUNNING", os.Getpid(), self)
	unknown["extra"] = "x"
	writeRecord(t, base, "00000000000000a5", unknown)
	if r := attach(t, root, a, "--run", "00000000000000a5"); r.code != 1 || !hasCode(r.res, wire.CodeMalformed) {
		t.Fatalf("unknown record key: code %d %s", r.code, r.stdout)
	}
	// Several runs and no --run: listed, not guessed.
	if r := attach(t, root, a); r.code != 1 || r.res.Outcome != wire.OutcomeRefused || len(r.res.Items) != 5 {
		t.Fatalf("ambiguous attach: code %d %s", r.code, r.stdout)
	}
}

// TestATRV0011_DetachAndAttachUsage: malformed detached and attach forms
// refuse before any effect; an attempt without runs has nothing to attach.
func TestATRV0011_DetachAndAttachUsage(t *testing.T) {
	root, a := attemptStore(t)
	for _, args := range [][]string{
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--detach", "--detach", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--detach", "--supervise", "0123456789abcdef", "--", "true"},
		{"run", "--attempt", a.AttemptID, "--generation", gen(a), "--timeout", "5", "--supervise", "XYZ", "--", "true"},
		{"run", "--attach", "--attempt", a.AttemptID, "--", "true"},
		{"run", "--attach", "--attempt", a.AttemptID, "--run", "not-a-run"},
		{"run", "--attach", "--attempt", a.AttemptID, "--wait", "-1"},
		{"run", "--attach", "--attempt", a.AttemptID, "--generation", gen(a)},
		{"run", "--attach", "--attach", "--attempt", a.AttemptID},
		{"run", "--attach", "--attempt"},
	} {
		if r := cliRun(t, root, args...); r.code != 1 || r.res.Outcome != wire.OutcomeError {
			t.Fatalf("%v: code %d %s", args, r.code, r.stdout)
		}
	}
	if r := attach(t, root, a); r.code != 1 || r.res.Outcome != wire.OutcomeRefused || !hasCode(r.res, wire.CodeMissingEvidence) {
		t.Fatalf("no runs: code %d %s", r.code, r.stdout)
	}
	base, err := cli.RunDirForTest(root, a.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused form created %s: %v", base, err)
	}
}
