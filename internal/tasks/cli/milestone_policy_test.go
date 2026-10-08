package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// milestonePolicy writes policy version 2 with milestones.required set to
// required (or no milestones key when omit) through policy update.
func milestonePolicy(t *testing.T, root, version, expected string, omit, required bool) {
	t.Helper()
	policy := fixture.PolicyValue()
	policy.Obj.Set("policyVersion", wire.String(version))
	if !omit {
		policy.Obj.Set("milestones", wire.ObjectValue(wire.NewObject().Set("required", wire.Bool(required))))
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy.json")
	fixture.Write(t, path, wire.EncodeFile(policy))
	if x := atm(t, root, nil, "policy", "update", "--request-id", "milestones-"+version, "--expected-policy-version", expected, "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

func withMilestone(m string) string {
	return strings.Replace(createPayloadJSON, `"milestone":null`, `"milestone":"`+m+`"`, 1)
}

// TestCALV0195_MilestoneRequiredPolicy: without the policy key, and with
// milestones.required false, CREATE with a null milestone and REFINE that
// clears one are accepted. With required true, both refuse
// VALIDATION_FAILED MILESTONE_REQUIRED and write nothing; CREATE naming a
// milestone, REFINE setting one and REFINE of other fields on a legacy
// unmilestoned ticket still succeed; the receipt history recorded under the
// earlier policy still audits CONSISTENT; the create template marks
// milestone required.
func TestCALV0195_MilestoneRequiredPolicy(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	create := func(req, payload string) run {
		t.Helper()
		return atm(t, r.Root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
	}
	refine := func(req, id, rev, payload string) run {
		t.Helper()
		return atm(t, r.Root, nil, "ticket", "refine", "--request-id", req, "--target", id, "--expected-revision", rev, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload)
	}
	ok := func(x run, what string) string {
		t.Helper()
		if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "outcome").Str != "COMPLETED" {
			t.Fatalf("%s: %s", what, x.stdout)
		}
		return field(x.res.Items[0], "ticketId").Str
	}
	headSeq := func() string {
		t.Helper()
		return field(atm(t, r.Root, nil, "queue", "status").res.Items[0], "headSeq").Str
	}
	refused := func(x run, what string) {
		t.Helper()
		if x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMilestoneRequired) || len(x.res.Items) != 1 || field(x.res.Items[0], "outcome").Str != "VALIDATION_FAILED" {
			t.Fatalf("%s was not refused MILESTONE_REQUIRED: %s", what, x.stdout)
		}
	}

	// Default: no policy key, a null milestone is accepted.
	legacy := ok(create("legacy", createPayloadJSON), "default create without milestone")
	cleared := ok(create("cleared", withMilestone("v1")), "default create with milestone")
	ok(refine("clear-default", cleared, "1", `{"milestone":null}`), "default refine clearing milestone")

	// required false is the default behaviour.
	milestonePolicy(t, r.Root, "2", "1", false, false)
	ok(create("false-null", createPayloadJSON), "required:false create without milestone")

	milestonePolicy(t, r.Root, "3", "2", false, true)
	before := headSeq()
	refused(create("required-null", createPayloadJSON), "CREATE without milestone")
	refused(refine("required-clear", cleared, "2", `{"milestone":null}`), "REFINE to a null milestone")
	refused(refine("required-clear-mixed", legacy, "1", `{"body":"x","milestone":null}`), "REFINE clearing milestone beside another field")
	if after := headSeq(); after != before {
		t.Fatalf("refusals wrote receipts: headSeq %s -> %s", before, after)
	}
	ok(create("required-named", withMilestone("v1")), "CREATE naming a milestone")
	ok(refine("legacy-body", legacy, "1", `{"body":"backfill pending"}`), "REFINE of another field on a legacy unmilestoned ticket")
	ok(refine("legacy-set", legacy, "2", `{"milestone":"v1"}`), "REFINE setting a milestone")

	tmpl := atm(t, r.Root, nil, "ticket", "create", "--template")
	m := field(field(tmpl.res.Items[0], "fields"), "milestone")
	if field(m, "nullable").Bool || !strings.Contains(string(wire.Encode(field(tmpl.res.Items[0], "fill"))), `"milestone"`) {
		t.Fatalf("template under milestones.required: %s", tmpl.stdout)
	}

	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" ||
		field(audit.res.Items[0], "projectionAgreement").Str != "AGREES" {
		t.Fatalf("receipt audit: %s", audit.stdout)
	}

	// Removing the key restores the default.
	milestonePolicy(t, r.Root, "4", "3", true, false)
	ok(create("omitted-null", createPayloadJSON), "create after removing milestones")

	for _, verb := range []string{"create", "refine"} {
		if help := atm(t, r.Root, nil, "ticket", verb, "--help", "--verbose"); !strings.Contains(string(help.stdout), "MILESTONE_REQUIRED") {
			t.Errorf("ticket %s help lacks the milestone rule: %s", verb, help.stdout)
		}
	}
}

// TestCALV0196_OpenWithoutMilestoneCount: queue status reports the count of
// OPEN tickets with a null milestone over the whole inventory (DRAFT and
// milestoned tickets excluded), --summary keeps it, and roadmap warns with
// the same count only when it is non-zero, whatever page is read.
func TestCALV0196_OpenWithoutMilestoneCount(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	count := func(want string) {
		t.Helper()
		for _, args := range [][]string{{"queue", "status"}, {"queue", "status", "--summary"}} {
			x := atm(t, r.Root, nil, args...)
			if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "openWithoutMilestone").Str != want {
				t.Fatalf("%v openWithoutMilestone want %s: %s", args, want, x.stdout)
			}
		}
	}
	roadmapWarns := func(want string) {
		t.Helper()
		x := atm(t, r.Root, nil, "roadmap", "--limit", "1")
		joined := strings.Join(x.res.Warnings, "\n")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("roadmap: %s", x.stdout)
		}
		if want == "" {
			if strings.Contains(joined, "milestone") {
				t.Fatalf("roadmap warned with no unmilestoned OPEN ticket: %s", x.stdout)
			}
			return
		}
		if !strings.Contains(joined, want+" OPEN ticket(s) have no milestone") {
			t.Fatalf("roadmap warning want %s: %s", want, x.stdout)
		}
	}
	count("0")
	roadmapWarns("")
	issue := func(req, payload string) {
		t.Helper()
		if x := atm(t, r.Root, nil, "ticket", "create", "--request-id", req, "--issued-at", "2026-10-08T12:00:00Z", "--payload", payload); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("create %s: %s", req, x.stdout)
		}
	}
	issue("a", createPayloadJSON)
	issue("b", createPayloadJSON)
	issue("c", withMilestone("v1"))
	issue("draft", strings.Replace(createPayloadJSON, `"acceptanceCriteria":["it exists"]`, `"acceptanceCriteria":[]`, 1))
	count("2")
	roadmapWarns("2")
}
