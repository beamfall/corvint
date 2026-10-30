//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// detachClaimStore is one disposable store with one claimable ticket, set up
// so a detached claim re-runs this test binary as its child.
func detachClaimStore(t *testing.T) (string, *intent.Repository, string) {
	t.Helper()
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	id := planTicket(t, root, "detach", "P1", `["src/"]`)
	detachedEnv(t)
	t.Cleanup(cli.SetLeaseDetachPoll(20 * time.Millisecond))
	cli.JoinDetachedChildren(t, root)
	return root, repo, id
}

func headBytes(t *testing.T, repo *intent.Repository) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repo.StateDir, "head.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func detachedChildPid(t *testing.T, dir string) int {
	t.Helper()
	p := filepath.Join(dir, "child.json")
	waitFile(t, p)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var child struct {
		Pid int `json:"pid"`
	}
	if err := json.Unmarshal(raw, &child); err != nil || child.Pid <= 0 {
		t.Fatalf("child record %s: %v", raw, err)
	}
	return child.Pid
}

// TestATRV0021_DetachedClaimSurvivesKilledCaller: the caller of a detached
// claim is killed while the claim waits for the store; the claim still
// commits exactly once, attach returns it, repeated attaches and detaches
// return the same bytes without a second mutation, and the plain command
// with the same request id replays the same attempt (ATR-V0-020..022).
func TestATRV0021_DetachedClaimSurvivesKilledCaller(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	held := holdPreparation(t, repo)
	released := false
	t.Cleanup(func() {
		if !released {
			_ = held.Close()
		}
	})
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-detached"}
	caller := exec.Command(os.Args[0], append(append(append([]string{}, detachHelperPrefix...), claim...), "--detach", "--wait", "60")...)
	caller.Dir = root
	caller.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := caller.Start(); err != nil {
		t.Fatal(err)
	}
	dir := cli.LeaseDetachDir(repo, "claim-detached")
	child := detachedChildPid(t, dir)
	// Kill the caller and its whole process group mid-claim.
	if err := syscall.Kill(-caller.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = caller.Wait()
	if err := syscall.Kill(child, 0); err != nil {
		t.Fatalf("detached child did not survive its caller: %v", err)
	}
	if pending := cliRun(t, root, "claim", "--attach", "--request-id", "claim-detached"); pending.code != 75 || pending.res.Outcome != wire.OutcomeRefused || field(pending.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("attach while the store is held: code %d %s", pending.code, pending.stdout)
	}
	// Without its child record, the running child is still found by the
	// lock it holds: attach waits and a repeated detach starts nothing.
	waitRunningHeld(t, dir)
	if err := os.Remove(filepath.Join(dir, "child.json")); err != nil {
		t.Fatal(err)
	}
	if pending := cliRun(t, root, append(claim, "--detach")...); pending.code != 75 || field(pending.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("detach over an unrecorded running child: code %d %s", pending.code, pending.stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "child.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a second child started: %v", err)
	}
	released = true
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	got := cliRun(t, root, "claim", "--attach", "--request-id", "claim-detached", "--holder", "agent", "--wait", "60")
	attempt, generation := field(got.res.Items[0], "attemptId").Str, field(got.res.Items[0], "generation").Str
	if got.code != 0 || got.res.Outcome != wire.OutcomeOK || attempt == "" || generation == "" {
		t.Fatalf("attach: code %d %s", got.code, got.stdout)
	}
	head := headBytes(t, repo)
	// ATR-V0-022: attaching again is idempotent.
	if again := cliRun(t, root, "claim", "--attach", "--request-id", "claim-detached"); again.code != 0 || !bytes.Equal(again.stdout, got.stdout) {
		t.Fatalf("second attach differs: %s", again.stdout)
	}
	// ATR-V0-021: the same detach returns the kept result; no second mutation.
	if again := cliRun(t, root, append(claim, "--detach")...); again.code != 0 || !bytes.Equal(again.stdout, got.stdout) || !bytes.Equal(headBytes(t, repo), head) {
		t.Fatalf("repeated detach: code %d %s", again.code, again.stdout)
	}
	// The store committed the request once: the plain command replays it.
	plain := atm(t, root, nil, claim...)
	if !field(plain.res.Items[0], "replayed").Bool || field(plain.res.Items[0], "attemptId").Str != attempt || field(plain.res.Items[0], "generation").Str != generation || !bytes.Equal(headBytes(t, repo), head) {
		t.Fatalf("plain replay: %s", plain.stdout)
	}
}

// TestATRV0021_LostChildRestartsWithoutASecondClaim: a child that ended
// without keeping its result is reported SUPERVISOR_LOST; a repeated detach
// restarts it and the store replays the committed attempt (ATR-V0-021). A
// changed argv or holder under the same request id refuses without a start.
func TestATRV0021_LostChildRestartsWithoutASecondClaim(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-lost"}
	first := cliRun(t, root, append(claim, "--detach", "--wait", "60")...)
	attempt := field(first.res.Items[0], "attemptId").Str
	if first.code != 0 || attempt == "" {
		t.Fatalf("detach: code %d %s", first.code, first.stdout)
	}
	head := headBytes(t, repo)
	dir := cli.LeaseDetachDir(repo, "claim-lost")
	for _, name := range []string{"result.json", "child.json"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	lost := cliRun(t, root, "claim", "--attach", "--request-id", "claim-lost")
	if lost.res.Outcome != wire.OutcomeRefused || !hasCode(lost.res, wire.CodeSupervisorLost) || field(lost.res.Items[0], "state").Str != "LOST" {
		t.Fatalf("lost attach: %s", lost.stdout)
	}
	for _, changed := range [][]string{
		{"claim", id, "--holder", "other", "--request-id", "claim-lost", "--detach"},
		{"claim", id, "--holder", "agent", "--request-id", "claim-lost", "--lease-minutes", "9", "--detach"},
		{"claim", "--attach", "--request-id", "claim-lost", "--holder", "other"},
		{"pool", "acquire", "--attach", "--request-id", "claim-lost"},
	} {
		if x := cliRun(t, root, changed...); x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeRequestIDConflict) {
			t.Fatalf("%v: %s", changed, x.stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "child.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a conflicting request started a child: %v", err)
	}
	again := cliRun(t, root, append(claim, "--detach", "--wait", "60")...)
	if again.code != 0 || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "attemptId").Str != attempt || !bytes.Equal(headBytes(t, repo), head) {
		t.Fatalf("restarted detach: code %d %s", again.code, again.stdout)
	}
}

// TestATRV0022_RefusalIsReturnedVerbatim: a detached claim the store refuses
// returns that refusal, byte for byte as kept, with the same outcome, codes
// and warnings as the plain command, not a generic timeout (ATR-V0-022). The
// same holds for pool acquire.
func TestATRV0022_RefusalIsReturnedVerbatim(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	if x := atm(t, root, nil, "claim", id, "--holder", "first", "--request-id", "claim-first"); x.code != 0 {
		t.Fatalf("first claim: %s", x.stdout)
	}
	plain := cliRun(t, root, "claim", id, "--holder", "second", "--request-id", "claim-plain")
	if plain.res.Outcome == wire.OutcomeOK {
		t.Fatalf("second claim admitted: %s", plain.stdout)
	}
	detached := cliRun(t, root, "claim", id, "--holder", "second", "--request-id", "claim-refused", "--detach", "--wait", "60")
	kept, err := os.ReadFile(filepath.Join(cli.LeaseDetachDir(repo, "claim-refused"), "result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if detached.code != plain.code || !bytes.Equal(detached.stdout, kept) || detached.res.Outcome != plain.res.Outcome ||
		!equalStrings(detached.res.Codes, plain.res.Codes) || !equalStrings(detached.res.Warnings, plain.res.Warnings) || hasCode(detached.res, wire.CodeSupervisorLost) {
		t.Fatalf("detached refusal %d %s\nplain %d %s", detached.code, detached.stdout, plain.code, plain.stdout)
	}
	acquire := []string{"pool", "acquire", "--attempt", "not-an-attempt", "--generation", "1", "--pool", "lanes", "--request-id", "acquire-refused"}
	plainAcquire := cliRun(t, root, acquire...)
	detachedAcquire := cliRun(t, root, append(acquire, "--detach", "--wait", "60")...)
	if plainAcquire.res.Outcome == wire.OutcomeOK || detachedAcquire.code != plainAcquire.code || !bytes.Equal(detachedAcquire.stdout, plainAcquire.stdout) {
		t.Fatalf("detached acquire %s\nplain %s", detachedAcquire.stdout, plainAcquire.stdout)
	}
}

// TestATRV0024_WaitReturnsWithinItsBound: --wait returns PENDING within its
// bound plus a small margin while the claim cannot finish (ATR-V0-024).
func TestATRV0024_WaitReturnsWithinItsBound(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-wait"}
	for _, c := range []struct {
		args []string
		wait time.Duration
	}{
		{append(append([]string{}, claim...), "--detach", "--wait", "1"), time.Second},
		{[]string{"claim", "--attach", "--request-id", "claim-wait", "--wait", "1"}, time.Second},
		{[]string{"claim", "--attach", "--request-id", "claim-wait"}, 0},
	} {
		started := time.Now()
		x := cliRun(t, root, c.args...)
		elapsed := time.Since(started)
		if x.code != 75 || x.res.Outcome != wire.OutcomeRefused || field(x.res.Items[0], "state").Str != "PENDING" || field(x.res.Items[0], "requestId").Str != "claim-wait" {
			t.Fatalf("%v: code %d %s", c.args, x.code, x.stdout)
		}
		if elapsed < c.wait || elapsed > c.wait+2*time.Second {
			t.Fatalf("%v returned after %v, bound %v", c.args, elapsed, c.wait)
		}
	}
	// Let the child finish before the store is removed.
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-wait", "--wait", "60"); x.code != 0 {
		t.Fatalf("attach after release: %s", x.stdout)
	}
}

// TestATRV0023_CeilingEndsTheDetachedClaim: a detached claim still waiting
// at its ceiling ends with a kept, non-OK result rather than running on
// (ATR-V0-023).
func TestATRV0023_CeilingEndsTheDetachedClaim(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	held := holdPreparation(t, repo)
	defer held.Close()
	started := time.Now()
	x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-ceiling", "--detach", "--wait", "60")
	if x.res.Outcome == wire.OutcomeOK || hasCode(x.res, wire.CodeSupervisorLost) || x.code == 75 || time.Since(started) > 20*time.Second {
		t.Fatalf("ceiling: code %d after %v %s", x.code, time.Since(started), x.stdout)
	}
	if kept, err := os.ReadFile(filepath.Join(cli.LeaseDetachDir(repo, "claim-ceiling"), "result.json")); err != nil || !bytes.Equal(kept, x.stdout) {
		t.Fatalf("ceiling result not kept: %v", err)
	}
}

// TestATRV0026_WithoutDetachNothingChanges: the plain command keeps no
// detached record; a detach-like value is a value; flag misuse refuses
// MALFORMED before any record or start; finished records past retention are
// pruned (ATR-V0-025, ATR-V0-026).
func TestATRV0026_WithoutDetachNothingChanges(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	base := filepath.Join(repo.CommonDir, "taskman-requests")
	if x := atm(t, root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-plain-1"); x.code != 0 {
		t.Fatalf("plain claim: %s", x.stdout)
	}
	if x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-plain-2", "--branch", "--detach"); hasCode(x.res, wire.CodeSupervisorLost) || (len(x.res.Items) > 0 && field(x.res.Items[0], "state").Str != "") {
		t.Fatalf("--branch value read as a flag: %s", x.stdout)
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("plain claims kept a detached record: %v", err)
	}
	for _, args := range [][]string{
		{"claim", id, "--holder", "a", "--request-id", "r1", "--wait", "1"},
		{"claim", id, "--holder", "a", "--request-id", "r1", "--detach", "--attach"},
		{"claim", id, "--holder", "a", "--request-id", "r1", "--detach", "--detach"},
		{"claim", id, "--holder", "a", "--request-id", "r1", "--detach", "--wait", "601"},
		{"claim", id, "--holder", "a", "--request-id", "r1", "--detach", "--wait", "01"},
		{"claim", "--attach", "--request-id", "r1", "--lease-minutes", "5"},
		{"claim", id, "--attach", "--request-id", "r1"},
		{"claim", id, "--holder", "a", "--detach"},
		{"claim", id, "--holder", "a", "--request-id", "r1", "--detachx"},
	} {
		if x := cliRun(t, root, args...); x.res.Outcome != wire.OutcomeError || (len(x.res.Codes) > 0 && !hasCode(x.res, wire.CodeMalformed)) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	if _, err := os.Stat(base); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused flags kept a detached record: %v", err)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "never"); x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeMissingEvidence) {
		t.Fatalf("unknown handle: %s", x.stdout)
	}
	// ATR-V0-025: a finished record older than the retention is pruned when
	// a later detach starts; a fresh one is kept.
	if x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", "claim-old", "--detach", "--wait", "60"); x.code == 75 {
		t.Fatalf("old detach pending: %s", x.stdout)
	}
	old := cli.LeaseDetachDir(repo, "claim-old")
	cli.PruneLeaseDetach(repo, time.Now().Add(6*24*time.Hour))
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("a record inside retention was pruned: %v", err)
	}
	cli.PruneLeaseDetach(repo, time.Now().Add(8*24*time.Hour))
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a record past retention was kept: %v", err)
	}
}

// TestATRV0027_StartingChildIsNeverLost: a child still starting, before it
// holds its running lock, whose identity the launcher and attach cannot read
// is PENDING, never LOST, inside the startup window, and then commits; a
// recorded child is LOST only on positive evidence: no process at its pid,
// or an unconfirmed identity and no running lock after the window
// (ATR-V0-027).
func TestATRV0027_StartingChildIsNeverLost(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_START_MS", "1500")
	t.Cleanup(cli.SetLeaseDetachIdentity(func(int) (string, error) { return "", errors.New("identity probe unavailable") }))
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-starting"}
	if x := cliRun(t, root, append(claim, "--detach")...); x.code != 75 || field(x.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("detach of a starting child: code %d %s", x.code, x.stdout)
	}
	dir := cli.LeaseDetachDir(repo, "claim-starting")
	if raw, err := os.ReadFile(filepath.Join(dir, "child.json")); err != nil || !bytes.Contains(raw, []byte(`"identity":"unknown"`)) {
		t.Fatalf("child record %s: %v", raw, err)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-starting"); x.code != 75 || hasCode(x.res, wire.CodeSupervisorLost) || field(x.res.Items[0], "state").Str != "PENDING" {
		t.Fatalf("attach of a starting child: code %d %s", x.code, x.stdout)
	}
	if x := cliRun(t, root, "claim", "--attach", "--request-id", "claim-starting", "--wait", "60"); x.code != 0 || field(x.res.Items[0], "attemptId").Str == "" {
		t.Fatalf("starting child did not commit: code %d %s", x.code, x.stdout)
	}

	// Recorded children without a running lock or a result.
	gone := exec.Command("true")
	if err := gone.Run(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		pid       int
		started   time.Time
		probeFail bool
		state     string
	}{
		{"no-process", gone.Process.Pid, time.Now(), false, "LOST"},
		{"unknown-in-window", os.Getpid(), time.Now(), true, "PENDING"},
		{"unknown-after-window", os.Getpid(), time.Now().Add(-2 * time.Minute), true, "LOST"},
	} {
		restore := func() {}
		if !c.probeFail {
			restore = cli.SetLeaseDetachIdentity(supervisor.ProcessIdentity)
		}
		requestID := "claim-" + c.name
		dir := cli.LeaseDetachDir(repo, requestID)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, filepath.Join(dir, "request.json"), map[string]string{"profile": "taskman-detached-lease/0", "command": "claim", "requestId": requestID, "holder": "", "argvSha256": "0"})
		writeJSON(t, filepath.Join(dir, "child.json"), map[string]any{"pid": c.pid, "identity": "unknown", "startedAt": c.started.UTC().Format(time.RFC3339), "ceilingAt": c.started.UTC().Format(time.RFC3339)})
		x := cliRun(t, root, "claim", "--attach", "--request-id", requestID)
		restore()
		if field(x.res.Items[0], "state").Str != c.state || hasCode(x.res, wire.CodeSupervisorLost) != (c.state == "LOST") {
			t.Fatalf("%s: %s", c.name, x.stdout)
		}
	}
}

// TestATRV0026_ChildKeepsUnsupportedVersion: a detached child that finds a
// request.json another build wrote under another taskman-detached-lease
// version reports UNSUPPORTED_VERSION (CAL-V0-131), not a usage error, and
// runs nothing (ATR-V0-026).
func TestATRV0026_ChildKeepsUnsupportedVersion(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	requestID := "claim-other-version"
	dir := cli.LeaseDetachDir(repo, requestID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "request.json"), map[string]string{"profile": "taskman-detached-lease/1", "command": "claim", "requestId": requestID, "holder": "agent", "argvSha256": "0"})
	head := headBytes(t, repo)
	x := cliRun(t, root, "claim", id, "--holder", "agent", "--request-id", requestID, "--detached-child")
	if x.code == 0 || !hasCode(x.res, wire.CodeUnsupportedVersion) {
		t.Fatalf("child over another version: exit %d %s", x.code, x.stdout)
	}
	if !bytes.Equal(headBytes(t, repo), head) {
		t.Fatal("the child changed the store")
	}
	if _, err := os.Lstat(filepath.Join(dir, "result.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the child kept a result: %v", err)
	}
}

// TestATRV0028_ChildLogIsCappedAndRotated: what a child writes to child.log
// is capped, and each run starts a fresh log with the previous run's kept as
// child.1.log, so restarts never grow it (ATR-V0-028).
func TestATRV0028_ChildLogIsCappedAndRotated(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_LOG_BYTES", strconv.Itoa(cli.MaxLeaseDetachLog+4096))
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-log", "--detach", "--wait", "60"}
	dir := cli.LeaseDetachDir(repo, "claim-log")
	for run := 0; run < 3; run++ {
		x := cliRun(t, root, claim...)
		// The previous child may still hold running between its result
		// and its exit; its detach then reports it LOST once.
		for tries := 0; hasCode(x.res, wire.CodeSupervisorLost) && tries < 50; tries++ {
			time.Sleep(50 * time.Millisecond)
			x = cliRun(t, root, claim...)
		}
		if x.code != 0 {
			t.Fatalf("run %d: code %d %s", run, x.code, x.stdout)
		}
		info, err := os.Stat(filepath.Join(dir, "child.log"))
		if err != nil || info.Size() != int64(cli.MaxLeaseDetachLog) {
			t.Fatalf("run %d: child.log %v %v", run, info, err)
		}
		prev, err := os.Stat(filepath.Join(dir, "child.1.log"))
		if run == 0 && !errors.Is(err, os.ErrNotExist) || run > 0 && (err != nil || prev.Size() != int64(cli.MaxLeaseDetachLog)) {
			t.Fatalf("run %d: child.1.log %v %v", run, prev, err)
		}
		// Lose the child's result so the next detach restarts it.
		for _, name := range []string{"result.json", "child.json"} {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestATRV0029_PruneReachesEveryFinishedRecord: retention pages through the
// whole records directory, so expired finished records behind many
// unfinished ones are pruned and the unfinished ones are kept (ATR-V0-029).
func TestATRV0029_PruneReachesEveryFinishedRecord(t *testing.T) {
	_, repo, _ := detachClaimStore(t)
	base := filepath.Join(repo.CommonDir, "taskman-requests")
	var finished, unfinished []string
	for i := 0; i < 200; i++ {
		dir := cli.LeaseDetachDir(repo, "prune-"+strconv.Itoa(i))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "request.json"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if i%2 == 1 {
			unfinished = append(unfinished, dir)
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		finished = append(finished, dir)
	}
	cli.PruneLeaseDetach(repo, time.Now().Add(8*24*time.Hour))
	for _, dir := range finished {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired finished record %s kept: %v", dir, err)
		}
	}
	for _, dir := range unfinished {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("unfinished record %s pruned: %v", dir, err)
		}
	}
	if entries, err := os.ReadDir(base); err != nil || len(entries) != len(unfinished) {
		t.Fatalf("records left: %d %v", len(entries), err)
	}
}

// waitRunningHeld waits until a detached child holds its running lock.
func waitRunningHeld(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if f, err := os.Open(filepath.Join(dir, "running")); err == nil {
			err = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
			_ = f.Close()
			if errors.Is(err, syscall.EWOULDBLOCK) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the detached child never took its running lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func writeJSON(t *testing.T, p string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
