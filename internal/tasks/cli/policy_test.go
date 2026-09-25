package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// nextPolicyFile writes the canonical fixture policy at version n outside the
// repository and returns its path and bytes.
func nextPolicyFile(t *testing.T, n string) (string, []byte) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String(n))
	raw := wire.EncodeFile(v)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy-"+n+".json")
	fixture.Write(t, path, raw)
	return path, raw
}

// sameStore checks the state and intent trees only: a writer that refuses
// after taking the lock leaves the lock file behind by design.
func sameStore(t *testing.T, r *fixture.Repo, state, intents []fixture.Entry, what string) {
	t.Helper()
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatalf("%s: store changed", what)
	}
}

func TestTMV0030_AS07_CLIPolicyUpdateAuditAndReplay(t *testing.T) {
	r := receiptFixture(t)
	path, raw := nextPolicyFile(t, "2")
	args := []string{"policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", path}
	x := atm(t, r.Root, nil, args...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %+v", x.res)
	}
	item := x.res.Items[0]
	if field(item, "outcome").Str != "COMPLETED" || field(item, "receipt").Str != "000000000003.json" || field(item, "oldPolicySha256").Str != string(wire.Sum(fixture.PolicyBytes())) || field(item, "newPolicySha256").Str != string(wire.Sum(raw)) {
		t.Fatalf("result: %+v", item)
	}
	projected, err := os.ReadFile(filepath.Join(r.IntentDir, "policy.json"))
	if err != nil || string(projected) != string(raw) {
		t.Fatalf("projection: %v", err)
	}
	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" || field(audit.res.Items[0], "projectionAgreement").Str != "AGREES" || field(audit.res.Items[0], "headSeq").Str != "3" {
		t.Fatalf("audit: %+v", audit.res)
	}
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	replay := atm(t, r.Root, nil, args...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || field(replay.res.Items[0], "receipt").Str != "" {
		t.Fatalf("replay: %+v", replay.res)
	}
	other, _ := nextPolicyFile(t, "3")
	conflict := atm(t, r.Root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "2", "--file", other)
	if conflict.res.Outcome != wire.OutcomeRefused || !hasCode(conflict.res, wire.CodeRequestIDConflict) {
		t.Fatalf("conflict: %+v", conflict.res)
	}
	sameStore(t, r, state, intents, "policy replay and conflict")
}

func TestTMV0030_AS07_CLIPolicyUpdateRefusals(t *testing.T) {
	r := receiptFixture(t)
	next, _ := nextPolicyFile(t, "2")
	skip, _ := nextPolicyFile(t, "3")
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	for _, c := range []struct {
		args    []string
		outcome string
	}{
		{[]string{"--request-id", "p", "--expected-policy-version", "0", "--file", next}, "REVISION_CONFLICT"},
		{[]string{"--request-id", "p", "--expected-policy-version", "1", "--file", skip}, "VALIDATION_FAILED"},
		{[]string{"--request-id", "p", "--expected-policy-version", "1", "--file", next, "--role", "WORKER"}, "UNAUTHORIZED"},
		{[]string{"--request-id", "p", "--expected-policy-version", "1", "--file", next, "--role", "REVIEWER"}, "UNAUTHORIZED"},
	} {
		x := atm(t, r.Root, nil, append([]string{"policy", "update"}, c.args...)...)
		if x.res.Outcome != wire.OutcomeRefused || field(x.res.Items[0], "outcome").Str != c.outcome {
			t.Fatalf("%v: %+v", c.args, x.res)
		}
	}
	for _, args := range [][]string{
		{},
		{"--request-id", "p", "--expected-policy-version", "1"},
		{"--request-id", "p", "--file", next},
		{"--expected-policy-version", "1", "--file", next},
		{"--request-id", "p", "--expected-policy-version", "one", "--file", next},
		{"--request-id", "p", "--expected-policy-version", "1", "--file", filepath.Join(t.TempDir(), "absent.json")},
		{"--request-id", "p", "--expected-policy-version", "1", "--file", next, "--file", next},
		{"--request-id", "p", "--expected-policy-version", "1", "--file", next, "--reason", "x"},
		{"--request-id", "", "--expected-policy-version", "1", "--file", next},
	} {
		x := atm(t, r.Root, nil, append([]string{"policy", "update"}, args...)...)
		if x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
	if x := atm(t, r.Root, nil, "policy", "show"); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("unknown policy verb: %+v", x.res)
	}
	sameStore(t, r, state, intents, "refused policy updates")
}

// A candidate pins the policy digest, so a committed policy update makes it
// stale for release readiness (TM-V0-028, TM-V0-030).
func TestTMV0030_AS38_PolicyUpdateInvalidatesReleaseCandidate(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	fixture.Write(t, filepath.Join(r.Root, "source.txt"), []byte("candidate source\n"))
	git(t, r.Root, "init")
	git(t, r.Root, "config", "user.email", "fixture@example.invalid")
	git(t, r.Root, "config", "user.name", "Fixture")
	git(t, r.Root, "add", ".taskman", "source.txt")
	git(t, r.Root, "commit", "-m", "fixture baseline")
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	ticketID := createTicket(t, r.Root, "candidate-ticket")
	create := atm(t, r.Root, nil, "release", "create", "--request-id", "candidate-release", "--target", "v1-0", "--issued-at", "2026-09-20T12:01:00Z", "--payload", releaseCreatePayload("v1-0", "Version 1.0", ticketID, ""))
	if create.res.Outcome != wire.OutcomeOK {
		t.Fatalf("release create: %+v", create.res)
	}
	capture := atm(t, r.Root, nil, "release", "candidate", "--request-id", "capture-v1", "--target", "v1-0", "--expected-revision", "1", "--issued-at", "2026-09-20T12:02:00Z")
	if capture.res.Outcome != wire.OutcomeOK {
		t.Fatalf("candidate capture: %+v", capture.res)
	}
	stale := func() bool {
		t.Helper()
		x := atm(t, r.Root, nil, "release", "readiness", "v1-0")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("readiness: %+v", x.res)
		}
		for _, v := range field(x.res.Items[0], "missing").Arr {
			if v.Str == "candidate-source-or-policy" {
				return true
			}
		}
		return false
	}
	if stale() {
		t.Fatal("fresh candidate reported stale")
	}
	path, _ := nextPolicyFile(t, "2")
	if x := atm(t, r.Root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %+v", x.res)
	}
	if !stale() {
		t.Fatal("candidate captured under the old policy stayed fresh")
	}
}
