package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const ergGate = "g1-review"

// ergStore is a lease store whose policy (version 2) declares one
// routing-only external review gate recorded by OWNER or OPERATOR.
func ergStore(t *testing.T) (root, tree string) {
	t.Helper()
	root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	ergPolicyUpdate(t, root, "2")
	raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(raw))
}

// ergPolicyUpdate installs the review policy at version (from version-1).
func ergPolicyUpdate(t *testing.T, root, version string) {
	t.Helper()
	ergPolicyUpdateStages(t, root, version, []string{"implement"})
}

// ergPolicyUpdateStages installs the review policy at version with the
// gate's author stages; no stages leaves the gate undeclared.
func ergPolicyUpdateStages(t *testing.T, root, version string, authorStages []string) {
	t.Helper()
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", wire.String(version))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	capacity := wire.NewObject().Set("classes", wire.Array()).Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4"))
	policy.Obj.Set("capacity", wire.ObjectValue(capacity))
	if len(authorStages) != 0 {
		def := wire.NewObject().Set("authorStages", wire.Strings(authorStages)).Set("gateId", wire.String(ergGate)).
			Set("purpose", wire.String("ROUTING_ONLY")).Set("recorderRoles", wire.Strings([]string{"OPERATOR", "OWNER"})).
			Set("requireReviewerLease", wire.Bool(false)).Set("reviewStages", wire.Strings([]string{"review"}))
		policy.Obj.Set("externalReviews", wire.Array(wire.ObjectValue(def)))
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy-"+version+".json")
	fixture.Write(t, path, wire.EncodeFile(policy))
	prev, _ := wire.ParseCount("version", version)
	if x := atm(t, root, nil, "policy", "update", "--request-id", "policy-"+version, "--expected-policy-version", strconv.FormatInt(prev.Int()-1, 10), "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

func ergGateView(t *testing.T, root, id string) (dispatch.GateView, string) {
	t.Helper()
	tickets, err := cli.ObserveTickets(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range tickets {
		if x.ID == id {
			if !x.GatesObserved {
				t.Fatalf("native observation left gates unobserved: %+v", x)
			}
			return x.Gates[ergGate], x.GateState(ergGate)
		}
	}
	t.Fatalf("ticket %s not observed", id)
	return dispatch.GateView{}, ""
}

// TestERGV0009_NativeVerdictsThroughTheCLI drives `gate record|resubmit|
// history` against a native store: a RETURN, a CAS conflict, an explicit
// resubmission, a PASS, newest-first history, workState gate exposure through
// the dispatcher observation, STALE after a newer candidate, and receipt
// audit binding every event to its receipt (a forged receipt refuses).
func TestERGV0009_NativeVerdictsThroughTheCLI(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	headSeq := func() string {
		t.Helper()
		return field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	}
	if _, state := ergGateView(t, root, id); state != dispatch.GateNone {
		t.Fatalf("before any verdict: %s", state)
	}

	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := headSeq()

	record := func(req, verdict, gen, rev string) run {
		t.Helper()
		return atm(t, root, nil, "gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
			"--expected-generation", gen, "--expected-revision", rev, "--request-id", req, "--reason", "TESTS:missing case")
	}
	if x := record("review-1", "RETURN", "0", "0"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("record RETURN: %s", x.stdout)
	}
	if v, state := ergGateView(t, root, id); state != dispatch.GateReturn || v.Generation != "1" || v.Revision != "1" || v.Head == "" {
		t.Fatalf("after RETURN: %s %+v", state, v)
	}
	conflict := record("review-2", "PASS", "0", "0")
	if conflict.res.Outcome == wire.OutcomeOK || field(conflict.res.Items[0], "outcome").Str != mutation.OutcomeRevisionConflict {
		t.Fatalf("stale counters were accepted: %s", conflict.stdout)
	}
	if x := atm(t, root, nil, "gate", "record", id, "--gate", "undeclared", "--verdict", "PASS", "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--request-id", "review-3"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeGateUnknown) {
		t.Fatalf("an undeclared gate was accepted: %s", x.stdout)
	}

	runOK("gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1")
	if v, state := ergGateView(t, root, id); state != dispatch.GateResubmitted || v.Generation != "2" || !v.Resubmitted {
		t.Fatalf("after resubmit: %s %+v", state, v)
	}
	if x := record("review-4", "PASS", "2", "2"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("record PASS: %s", x.stdout)
	}
	pass, state := ergGateView(t, root, id)
	if state != dispatch.GatePass || pass.Revision != "3" {
		t.Fatalf("after PASS: %s %+v", state, pass)
	}

	h := runOK("gate", "history", id, "--gate", ergGate, "--limit", "2")
	if len(h.res.Items) != 2 || field(h.res.Items[0], "sha256").Str != pass.Head || len(h.res.Warnings) != 1 || !h.res.Untrusted {
		t.Fatalf("history page: %s", h.stdout)
	}
	next := strings.TrimPrefix(h.res.Warnings[0], "truncated: next --cursor ")
	if rest := runOK("gate", "history", id, "--gate", ergGate, "--cursor", next); len(rest.res.Items) != 1 || len(rest.res.Warnings) != 0 {
		t.Fatalf("history tail: %s", rest.stdout)
	}
	runOK("receipt", "audit")

	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")
	if _, state := ergGateView(t, root, id); state != dispatch.GatePass {
		t.Fatalf("a released subject with no newer candidate is still current: %s", state)
	}
	c = runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")
	runOK("submit", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--tree", tree, "--request-id", "submit-b")
	if _, state := ergGateView(t, root, id); state != "STALE" {
		t.Fatalf("a newer candidate did not stale the PASS: %s", state)
	}
	if x := record("review-5", "PASS", "2", "3"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a verdict on the superseded subject was accepted: %s", x.stdout)
	}
	runOK("receipt", "audit")

	// Currency comes from the submission history, not from the newer
	// attempt's phase: once B leaves BUILT, A's PASS stays STALE and A's
	// subject cannot take a new current verdict (ERG-V0-006).
	runOK("release", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str,
		"--reason", wire.CodeHandoff, "--request-id", "release-b")
	if _, state := ergGateView(t, root, id); state != "STALE" {
		t.Fatalf("releasing the newer candidate revived the PASS: %s", state)
	}
	if x := record("review-6", "PASS", "2", "3"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a verdict on the superseded subject was accepted after release: %s", x.stdout)
	}
	runOK("receipt", "audit")

	// The pure binding fold accepts the real receipts and refuses one whose
	// actor no longer matches the event it posts.
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	blob := func(d wire.Digest) ([]byte, bool) {
		raw, err := os.ReadFile(filepath.Join(repo.StateDir, "evidence", string(d)))
		return raw, err == nil
	}
	last, err := wire.ParseCount("headSeq", headSeq())
	if err != nil {
		t.Fatal(err)
	}
	for _, forge := range []bool{false, true} {
		var fold transaction.ExternalReviewReceiptAudit
		var refused error
		for seq := uint64(1); seq <= uint64(last.Int()) && refused == nil; seq++ {
			name, err := snapshot.ReceiptName(seq)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(repo.StateDir, "receipts", name))
			if err != nil {
				t.Fatal(err)
			}
			rc, err := snapshot.DecodeReceipt(raw)
			if err != nil {
				t.Fatal(err)
			}
			if forge && rc.RequestID != nil && *rc.RequestID == "review-4" {
				rc.ActorID = "someone-else"
			}
			refused = fold.Step(rc, wire.Sum(raw), blob)
		}
		if forge != (refused != nil) || (forge && wire.CodeOf(refused) != wire.CodeJournalForked) {
			t.Fatalf("forge=%t: %v", forge, refused)
		}
	}
}

// TestERGV0009_ReviewRetriesReplay retries `gate record|resubmit` with the
// same request id and --issued-at: the CLI resubmits the retained request, so
// the retry replays even after the head moved (the default prior RETURN), the
// policy changed and the author lease ended; a changed input is refused.
func TestERGV0009_ReviewRetriesReplay(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	record := []string{"gate", "record", id, "--gate", ergGate, "--verdict", "RETURN", "--subject-receipt", subject,
		"--expected-generation", "0", "--expected-revision", "0", "--request-id", "review-1", "--reason", "TESTS:missing case", "--issued-at", "2026-10-04T12:00:00Z"}
	resubmit := []string{"gate", "resubmit", id, "--gate", ergGate, "--author-attempt", attempt, "--subject-receipt", subject,
		"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", "resubmit-1", "--issued-at", "2026-10-04T12:01:00Z"}
	runOK(record...)
	runOK(resubmit...)
	replayed := func(name string, args []string) {
		t.Helper()
		if x := runOK(args...); !field(x.res.Items[0], "replayed").Bool {
			t.Fatalf("%s: the retry was not a replay: %s", name, x.stdout)
		}
	}
	replayed("immediate resubmit", resubmit)
	replayed("record after the head moved", record)
	// A retained request id with any changed input refuses at once as
	// REQUEST_ID_CONFLICT, whatever a fresh composition would have hit.
	changedResubmit := append(append([]string{}, resubmit[:len(resubmit)-6]...), "--reason", "FIXED:a different fix", "--request-id", "resubmit-1", "--issued-at", "2026-10-04T12:01:00Z")
	changedRecord := append(append([]string{}, record[:len(record)-6]...), "--reason", "TESTS:another case", "--request-id", "review-1", "--issued-at", "2026-10-04T12:00:00Z")
	conflict := func(name string, args []string) {
		t.Helper()
		if x := atm(t, root, nil, args...); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeRequestIDConflict) {
			t.Fatalf("%s: a changed retry was not a request-id conflict: %s", name, x.stdout)
		}
	}
	conflict("changed resubmit", changedResubmit)
	conflict("changed record", changedRecord)

	ergPolicyUpdate(t, root, "3")
	runOK("release", "--attempt", attempt, "--generation", generation, "--reason", wire.CodeHandoff, "--request-id", "release-a")
	replayed("resubmit after policy and lease change", resubmit)
	replayed("record after policy change", record)
	// Without the author lease a fresh resubmit composition would refuse as
	// MALFORMED; the retained request id decides first.
	conflict("changed resubmit after policy and lease change", changedResubmit)
	conflict("changed record after policy change", changedRecord)
	// With the gate undeclared a fresh composition would refuse as
	// GATE_UNKNOWN; the retained request id still decides first.
	ergPolicyUpdateStages(t, root, "4", nil)
	conflict("changed record after the gate was undeclared", changedRecord)
	conflict("changed resubmit after the gate was undeclared", changedResubmit)
}
