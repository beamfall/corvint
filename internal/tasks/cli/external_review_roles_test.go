package cli_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestERGV0001_ReviewerAndWorkerActors drives REVIEWER and WORKER actors
// through the native writer (ERG-V0-001). Under a policy that requires a
// reviewer lease, an OPERATOR cannot attest without one; a REVIEWER records
// only with its own live review-stage lease (LEASE_BOUND) and never
// resubmits; a WORKER never records and resubmits with its own live author
// lease; neither role enters any other mutation. The receipt audit replays
// both roles' events, and the native observation and `gate state` expose the
// full ERG-V0-009 field set.
func TestERGV0001_ReviewerAndWorkerActors(t *testing.T) {
	as := func(actor string) {
		t.Setenv("CORVINT_TASKS_ACTOR", actor)
		t.Setenv("ATM_ACTOR", actor)
	}
	as("tester")
	root, tree := ergStore(t)
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER", "REVIEWER"}, true)
	id := planTicket(t, root, "reviewed", "P1", `["src/"]`)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	refused := func(why string, args ...string) {
		t.Helper()
		x := atm(t, root, nil, args...)
		if x.res.Outcome == wire.OutcomeOK || len(x.res.Items) == 0 || field(x.res.Items[0], "outcome").Str != mutation.OutcomeUnauthorized {
			t.Fatalf("%s was not refused as UNAUTHORIZED: %s", why, x.stdout)
		}
	}

	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-a")
	author, authorGen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", author, "--generation", authorGen, "--tree", tree, "--request-id", "submit-a")
	subject := field(runOK("receipt", "audit").res.Items[0], "headSeq").Str
	runOK("release", "--attempt", author, "--generation", authorGen, "--reason", wire.CodeHandoff, "--request-id", "release-a")

	record := func(req, verdict, role string, extra ...string) []string {
		return append([]string{"gate", "record", id, "--gate", ergGate, "--verdict", verdict, "--subject-receipt", subject,
			"--expected-generation", "0", "--expected-revision", "0", "--request-id", req, "--reason", "TESTS:missing case", "--role", role}, extra...)
	}
	refused("an OPERATOR attestation under a lease-required policy", record("attest", "PASS", "OPERATOR")...)

	as("rev")
	c = runOK("claim", id, "--holder", "rev", "--stage", "review", "--request-id", "claim-review")
	reviewer, reviewerGen := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	refused("a REVIEWER without a lease", record("no-lease", "RETURN", "REVIEWER")...)
	refused("a WORKER verdict", record("worker-verdict", "RETURN", "WORKER", "--reviewer-attempt", reviewer)...)
	refused("a REVIEWER on a non-review mutation", "ticket", "refine", "--request-id", "reviewer-refine", "--target", id,
		"--expected-revision", "1", "--payload", `{"body":"reviewer edit"}`, "--role", "REVIEWER")
	as("intruder")
	refused("a REVIEWER on another holder's lease", record("borrowed", "RETURN", "REVIEWER", "--reviewer-attempt", reviewer)...)
	as("rev")
	runOK(record("review-1", "RETURN", "REVIEWER", "--reviewer-attempt", reviewer)...)

	v, state := ergGateView(t, root, id)
	if state != dispatch.GateReturn || v.Trust != (dispatch.GateTrust{ActorAuthentication: "NOT_OBSERVED", Independence: "NOT_OBSERVED", Source: "LEASE_BOUND"}) ||
		v.Candidate != (dispatch.GateCandidate{Kind: "TREE", TreeOID: tree}) || v.Subject.AttemptID != author || v.Subject.ReceiptSeq != subject ||
		v.EvidenceSha256 == "" || v.EvidenceSha256 != v.Head {
		t.Fatalf("REVIEWER RETURN view: %s %+v", state, v)
	}
	items := runOK("gate", "state", id).res.Items
	if len(items) != 1 {
		t.Fatalf("gate state items: %v", items)
	}
	got := items[0]
	trust, _ := got.Obj.Get("trust")
	candidate, _ := got.Obj.Get("candidate")
	subj, _ := got.Obj.Get("subject")
	resubmitted, _ := got.Obj.Get("resubmitted")
	if field(got, "gate").Str != ergGate || field(got, "verdict").Str != "RETURN" || field(got, "status").Str != "CURRENT" ||
		field(got, "generation").Str != "1" || field(got, "revision").Str != "1" || resubmitted.Bool ||
		field(got, "evidenceSha256").Str != v.Head || field(trust, "source").Str != "LEASE_BOUND" ||
		field(candidate, "treeOid").Str != tree || field(subj, "receiptSeq").Str != subject {
		t.Fatalf("gate state: %s", wire.EncodeFile(got))
	}
	// The reviewer's own review-stage submission is outside the author
	// stages, so it does not supersede the subject (ERG-V0-006).
	runOK("submit", "--attempt", reviewer, "--generation", reviewerGen, "--tree", tree, "--request-id", "submit-review")
	runOK("release", "--attempt", reviewer, "--generation", reviewerGen, "--reason", wire.CodeReviewReturned, "--request-id", "release-review")
	if _, state := ergGateView(t, root, id); state != dispatch.GateReturn {
		t.Fatalf("the reviewer's own lease staled the RETURN: %s", state)
	}

	as("tester")
	c = runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim-b")
	fix := field(c.res.Items[0], "attemptId").Str
	resubmit := func(req, role string) []string {
		return []string{"gate", "resubmit", id, "--gate", ergGate, "--author-attempt", fix, "--subject-receipt", subject,
			"--expected-generation", "1", "--expected-revision", "1", "--reason", "FIXED:added the case", "--request-id", req, "--role", role}
	}
	refused("a REVIEWER resubmission", resubmit("reviewer-resubmit", "REVIEWER")...)
	runOK(resubmit("resubmit-1", "WORKER")...)
	if v, state := ergGateView(t, root, id); state != dispatch.GateResubmitted || v.Generation != "2" || !v.Resubmitted || v.Trust.Source != "LEASE_BOUND" {
		t.Fatalf("after WORKER resubmit: %s %+v", state, v)
	}
	runOK("receipt", "audit")
}
