package cli_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV12038_ShowAndBlockersReportPolicyRequiredGates: the fixture policy
// marks gate verify required and ticket A names no gate of its own, so its
// record's requiredGates is empty while complete demands verify. ticket show
// and ticket blockers report that effective set; the record is unchanged.
func TestV12038_ShowAndBlockersReportPolicyRequiredGates(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteState(t, r)
	a := fixture.Ticket("A")
	fixture.WriteIntent(t, r, a)
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": a.Encode()})
	for _, verb := range []string{"show", "blockers"} {
		x := atm(t, r.Root, nil, "ticket", verb, "A")
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("%s: %+v %v", verb, x.res, x.res.Warnings)
		}
		it := x.res.Items[0]
		if got := field(it, "requiredGates"); len(got.Arr) != 0 {
			t.Errorf("%s: record requiredGates changed: %s", verb, wire.Encode(got))
		}
		got := field(it, "effectiveRequiredGates")
		if len(got.Arr) != 1 || got.Arr[0].Str != "verify" {
			t.Errorf("%s: effectiveRequiredGates %s, want [verify]", verb, wire.Encode(got))
		}
	}
}

// TestV12038_CompleteRefusesTheGateShowAndBlockersReport: a ticket whose own
// requiredGates is empty, under a policy that requires gate verify, is
// claimed and submitted. ticket show and ticket blockers report verify as an
// effective required gate, and complete refuses with MISSING_GATE for that
// same gate, so the read outputs match what complete enforces (V1-2038).
func TestV12038_CompleteRefusesTheGateShowAndBlockersReport(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
	root, tree := ergStore(t)
	gate := commandGate("verify", "/bin/true")
	gate.Obj.Set("required", wire.Bool(true))
	ergPolicyUpdateRoles(t, root, "3", []string{"implement"}, []string{"review"}, []string{"OPERATOR", "OWNER"}, false, gate)
	runOK := func(args ...string) run {
		t.Helper()
		r := atm(t, root, nil, args...)
		if r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, r.stdout)
		}
		return r
	}
	id := planTicket(t, root, "policy-gate", "P1", `["src/"]`)
	c := runOK("claim", id, "--holder", "tester", "--stage", "implement", "--request-id", "claim")
	attempt, generation := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
	runOK("submit", "--attempt", attempt, "--generation", generation, "--tree", tree, "--request-id", "submit")
	for _, verb := range []string{"show", "blockers"} {
		it := runOK("ticket", verb, id).res.Items[0]
		if got := field(it, "requiredGates"); len(got.Arr) != 0 {
			t.Errorf("%s: record requiredGates %s, want []", verb, wire.Encode(got))
		}
		if got := field(it, "effectiveRequiredGates"); len(got.Arr) != 1 || got.Arr[0].Str != "verify" {
			t.Errorf("%s: effectiveRequiredGates %s, want [verify]", verb, wire.Encode(got))
		}
	}
	raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(raw))
	x := atm(t, root, nil, "complete", "--attempt", attempt, "--generation", generation, "--request-id", "complete", "--commit", head)
	if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingGate) || !strings.Contains(string(x.stdout), "verify") {
		t.Fatalf("complete without the policy gate: want MISSING_GATE naming verify, got %s", x.stdout)
	}
}
