package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", wire.String("2"))
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	capacity := wire.NewObject().Set("classes", wire.Array()).Set("maxActiveAttempts", wire.String("4")).Set("maxWorkersTotal", wire.String("4"))
	policy.Obj.Set("capacity", wire.ObjectValue(capacity))
	def := wire.NewObject().Set("authorStages", wire.Strings([]string{"implement"})).Set("gateId", wire.String(ergGate)).
		Set("purpose", wire.String("ROUTING_ONLY")).Set("recorderRoles", wire.Strings([]string{"OPERATOR", "OWNER"})).
		Set("requireReviewerLease", wire.Bool(false)).Set("reviewStages", wire.Strings([]string{"review"}))
	policy.Obj.Set("externalReviews", wire.Array(wire.ObjectValue(def)))
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy-2.json")
	fixture.Write(t, path, wire.EncodeFile(policy))
	if x := atm(t, root, nil, "policy", "update", "--request-id", "policy-2", "--expected-policy-version", "1", "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
	raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(raw))
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
			refused = fold.Step(rc, blob)
		}
		if forge != (refused != nil) || (forge && wire.CodeOf(refused) != wire.CodeJournalForked) {
			t.Fatalf("forge=%t: %v", forge, refused)
		}
	}
}
