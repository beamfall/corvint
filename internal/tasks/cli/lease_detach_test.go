//go:build darwin || linux

package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
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

// TestCALV0321_DetachedClaimSurvivesKilledCaller: the caller of a detached
// claim is killed while the claim waits for the store; the claim still
// commits exactly once, attach returns it, repeated attaches and detaches
// return the same bytes without a second mutation, and the plain command
// with the same request id replays the same attempt (CAL-V0-320..322).
func TestCALV0321_DetachedClaimSurvivesKilledCaller(t *testing.T) {
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
	// CAL-V0-322: attaching again is idempotent.
	if again := cliRun(t, root, "claim", "--attach", "--request-id", "claim-detached"); again.code != 0 || !bytes.Equal(again.stdout, got.stdout) {
		t.Fatalf("second attach differs: %s", again.stdout)
	}
	// CAL-V0-321: the same detach returns the kept result; no second mutation.
	if again := cliRun(t, root, append(claim, "--detach")...); again.code != 0 || !bytes.Equal(again.stdout, got.stdout) || !bytes.Equal(headBytes(t, repo), head) {
		t.Fatalf("repeated detach: code %d %s", again.code, again.stdout)
	}
	// The store committed the request once: the plain command replays it.
	plain := atm(t, root, nil, claim...)
	if !field(plain.res.Items[0], "replayed").Bool || field(plain.res.Items[0], "attemptId").Str != attempt || field(plain.res.Items[0], "generation").Str != generation || !bytes.Equal(headBytes(t, repo), head) {
		t.Fatalf("plain replay: %s", plain.stdout)
	}
}

// TestCALV0321_LostChildRestartsWithoutASecondClaim: a child that ended
// without keeping its result is reported SUPERVISOR_LOST; a repeated detach
// restarts it and the store replays the committed attempt (CAL-V0-321). A
// changed argv or holder under the same request id refuses without a start.
func TestCALV0321_LostChildRestartsWithoutASecondClaim(t *testing.T) {
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

// TestCALV0322_RefusalIsReturnedVerbatim: a detached claim the store refuses
// returns that refusal, byte for byte as kept, with the same outcome, codes
// and warnings as the plain command, not a generic timeout (CAL-V0-322). The
// same holds for pool acquire.
func TestCALV0322_RefusalIsReturnedVerbatim(t *testing.T) {
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

// TestCALV0324_WaitReturnsWithinItsBound: --wait returns PENDING within its
// bound plus a small margin while the claim cannot finish (CAL-V0-324).
func TestCALV0324_WaitReturnsWithinItsBound(t *testing.T) {
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

// TestCALV0323_CeilingEndsTheDetachedClaim: a detached claim still waiting
// at its ceiling ends with a kept, non-OK result rather than running on
// (CAL-V0-323).
func TestCALV0323_CeilingEndsTheDetachedClaim(t *testing.T) {
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

// TestCALV0326_WithoutDetachNothingChanges: the plain command keeps no
// detached record; a detach-like value is a value; flag misuse refuses
// MALFORMED before any record or start; finished records past retention are
// pruned (CAL-V0-325, CAL-V0-326).
func TestCALV0326_WithoutDetachNothingChanges(t *testing.T) {
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
	// CAL-V0-325: a finished record older than the retention is pruned when
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
