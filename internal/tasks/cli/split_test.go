package cli_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const splitIssuedAt = "2026-10-31T12:00:00Z"

// splitParent creates a ticket with three acceptance criteria and one
// attached-evidence entry, and returns its ID and the repository root.
func splitParent(t *testing.T) (string, string) {
	t.Helper()
	return splitParentUnder(t, fixture.PolicyBytes())
}

func splitParentUnder(t *testing.T, policy []byte) (string, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), policy)
	atm(t, r.Root, nil, "init")
	payload := strings.Replace(strings.Replace(createPayloadJSON, `"acceptanceCriteria":["it exists"]`,
		`"acceptanceCriteria":["first holds","second holds","third holds"]`, 1),
		`"body":null`, `"body":"Parent body."`, 1)
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-10-30T12:00:00Z", "--payload", payload)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	d := string(wire.Sum([]byte("focused log")))
	ev := atm(t, r.Root, nil, "ticket", "attach-evidence", "--target", id, "--expected-revision", "1",
		"--request-id", "ev-1", "--issued-at", "2026-10-30T12:01:00Z",
		"--payload", `{"evidence":["`+d+`"],"reason":"focused log"}`)
	if ev.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %+v\n%s", ev.res, ev.stdout)
	}
	return id, r.Root
}

func showRecord(t *testing.T, root, id string) (wire.Value, wire.Value) {
	t.Helper()
	x := atm(t, root, nil, "ticket", "show", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("ticket show %s: %+v", id, x.res)
	}
	return field(x.res.Items[0], "record"), x.res.Items[0]
}

func splitArgs(id string, extra ...string) []string {
	return append([]string{"ticket", "split", id, "--criteria", "2,3", "--expected-revision", "2",
		"--issued-at", splitIssuedAt}, extra...)
}

// TestTLFV0884_SplitMovesCriteriaToALinkedChild: the named criteria move to a
// new child that supersedes the parent's lineage link, carries the parent's
// evidence, and the parent keeps the rest with a body note naming the child.
func TestTLFV0884_SplitMovesCriteriaToALinkedChild(t *testing.T) {
	id, root := splitParent(t)
	x := atm(t, root, nil, splitArgs(id, "--request-id", "split-1")...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("split: %+v\n%s", x.res, x.stdout)
	}
	item := x.res.Items[0]
	child := field(item, "child").Str
	if child == "" || len(field(item, "steps").Arr) != 3 {
		t.Fatalf("split result: %s", wire.Encode(item))
	}
	for i, want := range []string{"split-1/create", "split-1/evidence/0", "split-1/refine"} {
		if got := field(field(item, "steps").Arr[i], "requestId").Str; got != want {
			t.Fatalf("step %d request ID %q, want %q", i, got, want)
		}
	}
	parent, _ := showRecord(t, root, id)
	if got := strs(field(parent, "acceptanceCriteria")); len(got) != 1 || got[0] != "first holds" {
		t.Fatalf("parent criteria: %v", got)
	}
	if body := field(parent, "body").Str; !strings.HasPrefix(body, "Parent body.") || !strings.Contains(body, "moved to "+child) {
		t.Fatalf("parent body: %q", body)
	}
	rec, shown := showRecord(t, root, child)
	if got := strs(field(rec, "acceptanceCriteria")); len(got) != 2 || got[0] != "second holds" || got[1] != "third holds" {
		t.Fatalf("child criteria: %v", got)
	}
	if !strings.Contains(string(wire.Encode(field(rec, "supersedes"))), id) {
		t.Fatalf("child supersedes: %s", wire.Encode(field(rec, "supersedes")))
	}
	if ev := field(shown, "attachedEvidence"); len(ev.Arr) != 1 || !strings.Contains(field(ev.Arr[0], "reason").Str, "focused log") {
		t.Fatalf("child evidence: %s", wire.Encode(ev))
	}
	if !auditConsistent(t, root) {
		t.Fatal("receipt audit is not CONSISTENT")
	}
}

// TestTLFV0884_InterruptedSplitResumes: a split interrupted after the child
// CREATE leaves the parent unchanged; a rerun with the same request ID and
// issuedAt replays the CREATE and finishes; a third run writes nothing.
func TestTLFV0884_InterruptedSplitResumes(t *testing.T) {
	id, root := splitParent(t)
	restore := cli.SetSplitAfterStep(func(step string) error {
		if step == "create" {
			return errors.New("interrupted for the test")
		}
		return nil
	})
	x := atm(t, root, nil, splitArgs(id, "--request-id", "split-2")...)
	restore()
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("interrupted split reported OK: %s", x.stdout)
	}
	parent, _ := showRecord(t, root, id)
	if got := strs(field(parent, "acceptanceCriteria")); len(got) != 3 {
		t.Fatalf("parent changed by an interrupted split: %v", got)
	}

	y := atm(t, root, nil, splitArgs(id, "--request-id", "split-2")...)
	if y.res.Outcome != wire.OutcomeOK {
		t.Fatalf("resumed split: %+v\n%s", y.res, y.stdout)
	}
	child := field(y.res.Items[0], "child").Str
	parent, _ = showRecord(t, root, id)
	if got := strs(field(parent, "acceptanceCriteria")); len(got) != 1 {
		t.Fatalf("parent after resume: %v", got)
	}
	tickets := atm(t, root, nil, "ticket", "search", "--kind", "FEATURE")
	if n := len(tickets.res.Items); n != 2 {
		t.Fatalf("resume made %d tickets, want 2: %s", n, tickets.stdout)
	}

	z := atm(t, root, nil, splitArgs(id, "--request-id", "split-2")...)
	if z.res.Outcome != wire.OutcomeOK || !field(z.res.Items[0], "alreadySplit").Bool || field(z.res.Items[0], "child").Str != child {
		t.Fatalf("third run: %+v\n%s", z.res, z.stdout)
	}
	if !auditConsistent(t, root) {
		t.Fatal("receipt audit is not CONSISTENT")
	}
}

// TestTLFV0884_SplitRefusals: every malformed or stale split refuses before
// any write.
func TestTLFV0884_SplitRefusals(t *testing.T) {
	id, root := splitParent(t)
	cases := []struct {
		name string
		args []string
		code string
	}{
		{"all criteria", []string{"ticket", "split", id, "--criteria", "1,2,3", "--expected-revision", "2", "--request-id", "s"}, wire.CodeMalformed},
		{"out of range", []string{"ticket", "split", id, "--criteria", "4", "--expected-revision", "2", "--request-id", "s"}, wire.CodeMalformed},
		{"repeat", []string{"ticket", "split", id, "--criteria", "2,2", "--expected-revision", "2", "--request-id", "s"}, wire.CodeMalformed},
		{"zero", []string{"ticket", "split", id, "--criteria", "0", "--expected-revision", "2", "--request-id", "s"}, wire.CodeMalformed},
		{"no criteria", []string{"ticket", "split", id, "--expected-revision", "2", "--request-id", "s"}, wire.CodeMalformed},
		{"stale", []string{"ticket", "split", id, "--criteria", "2", "--expected-revision", "1", "--request-id", "s"}, wire.CodeStaleTicket},
		{"no revision", []string{"ticket", "split", id, "--criteria", "2", "--request-id", "s"}, ""},
		{"no request id", []string{"ticket", "split", id, "--criteria", "2", "--expected-revision", "2"}, ""},
		{"long request id", []string{"ticket", "split", id, "--criteria", "2", "--expected-revision", "2", "--request-id", strings.Repeat("q", 53)}, ""},
	}
	for _, c := range cases {
		x := atm(t, root, nil, c.args...)
		// A usage refusal carries no code; the others name theirs.
		if x.res.Outcome == wire.OutcomeOK || (c.code != "" && !hasCode(x.res, c.code)) {
			t.Fatalf("%s: want %s, got %+v\n%s", c.name, c.code, x.res, x.stdout)
		}
	}
	if n := len(atm(t, root, nil, "ticket", "search", "--kind", "FEATURE").res.Items); n != 1 {
		t.Fatalf("refusals wrote tickets: %d", n)
	}
	parent, _ := showRecord(t, root, id)
	if field(parent, "revision").Str != "2" {
		t.Fatalf("refusals changed the parent: %s", wire.Encode(parent))
	}
}

// TestTLFV0885_DryRunWritesNothing: --dry-run prints the child payload and the
// parent's resulting criteria and records nothing.
func TestTLFV0885_DryRunWritesNothing(t *testing.T) {
	id, root := splitParent(t)
	x := atm(t, root, nil, "ticket", "split", id, "--criteria", "2", "--expected-revision", "2", "--dry-run")
	if x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "dryRun").Bool {
		t.Fatalf("dry run: %+v\n%s", x.res, x.stdout)
	}
	item := x.res.Items[0]
	if got := strs(field(field(item, "childPayload"), "acceptanceCriteria")); len(got) != 1 || got[0] != "second holds" {
		t.Fatalf("dry-run child criteria: %v", got)
	}
	if got := strs(field(item, "parentAcceptanceCriteria")); len(got) != 2 || got[1] != "third holds" {
		t.Fatalf("dry-run parent criteria: %v", got)
	}
	if n := len(atm(t, root, nil, "ticket", "search", "--kind", "FEATURE").res.Items); n != 1 {
		t.Fatalf("dry run wrote tickets: %d", n)
	}
	parent, _ := showRecord(t, root, id)
	if field(parent, "revision").Str != "2" {
		t.Fatalf("dry run changed the parent: %s", wire.Encode(parent))
	}
}

// TestTLFV0885_ParentCompletesOnRemainingCriteria: after a split the parent
// completes on the criteria it kept, and the receipt audit stays CONSISTENT.
func TestTLFV0885_ParentCompletesOnRemainingCriteria(t *testing.T) {
	id, root := splitParent(t)
	if x := atm(t, root, nil, splitArgs(id, "--request-id", "split-5")...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("split: %+v\n%s", x.res, x.stdout)
	}
	parent, _ := showRecord(t, root, id)
	rev := field(parent, "revision").Str
	done := atm(t, root, nil, "ticket", "complete-manual", "--target", id, "--expected-revision", rev,
		"--request-id", "done-1", "--issued-at", "2026-10-31T13:00:00Z", "--payload", `{"evidence":[],"reason":"manual"}`)
	if done.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %+v\n%s", done.res, done.stdout)
	}
	parent, _ = showRecord(t, root, id)
	if field(parent, "status").Str != "COMPLETED" {
		t.Fatalf("parent status %s", field(parent, "status").Str)
	}
	if !auditConsistent(t, root) {
		t.Fatal("receipt audit is not CONSISTENT")
	}
}

// TestTLFV0884_SplitKeepsExecutionConstraints: the child of a parent with
// requiredRoles, requiresPool and executionPrerequisites carries all three
// once it opens; the prerequisites arrive with the child REFINE that sets
// its criteria, so the child is never claimable without them.
func TestTLFV0884_SplitKeepsExecutionConstraints(t *testing.T) {
	v := fixture.PolicyValue()
	v.Obj.Set("capacity", wire.ObjectValue(wire.NewObject().Set("maxActiveAttempts", wire.String("1")).Set("maxWorkersTotal", wire.String("4")).Set("classes", wire.Array())))
	v.Obj.Set("pools", wire.Array(wire.ObjectValue(wire.NewObject().Set("id", wire.String("lanes")).Set("members", wire.Strings([]string{"a"})))))
	id, root := splitParentUnder(t, wire.EncodeFile(v))
	other := atm(t, root, nil, "ticket", "create", "--request-id", "req-other", "--issued-at", "2026-10-30T12:02:00Z",
		"--payload", strings.Replace(createPayloadJSON, `"title":"Console ticket"`, `"title":"Prerequisite ticket"`, 1))
	if other.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create prerequisite: %+v\n%s", other.res, other.stdout)
	}
	pre := field(other.res.Items[0], "ticketId").Str
	roles := `{"implement":["BUILDER"],"integrate":["VERIFIER"],"review":["REVIEWER"]}`
	refine := atm(t, root, nil, "ticket", "refine", "--target", id, "--expected-revision", "2",
		"--request-id", "constrain", "--issued-at", "2026-10-30T12:03:00Z", "--payload",
		`{"executionPrerequisites":[{"gateId":null,"obligation":"COMPLETED","stages":["implement"],"ticketId":"`+pre+`"}],"requiredRoles":`+roles+`,"requiresPool":"lanes"}`)
	if refine.res.Outcome != wire.OutcomeOK {
		t.Fatalf("constrain parent: %+v\n%s", refine.res, refine.stdout)
	}
	x := atm(t, root, nil, "ticket", "split", id, "--criteria", "2,3", "--expected-revision", "3",
		"--issued-at", splitIssuedAt, "--request-id", "split-c")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("split: %+v\n%s", x.res, x.stdout)
	}
	steps := field(x.res.Items[0], "steps").Arr
	if len(steps) != 4 || field(steps[1], "requestId").Str != "split-c/child" {
		t.Fatalf("split steps: %s", wire.Encode(field(x.res.Items[0], "steps")))
	}
	rec, _ := showRecord(t, root, field(x.res.Items[0], "child").Str)
	if field(rec, "status").Str != "OPEN" || len(strs(field(rec, "acceptanceCriteria"))) != 2 {
		t.Fatalf("child: %s", wire.Encode(rec))
	}
	if got := string(wire.Encode(field(rec, "requiredRoles"))); got != roles {
		t.Fatalf("child requiredRoles %s, want %s", got, roles)
	}
	if field(rec, "requiresPool").Str != "lanes" {
		t.Fatalf("child requiresPool: %s", wire.Encode(field(rec, "requiresPool")))
	}
	if ps := field(rec, "executionPrerequisites"); len(ps.Arr) != 1 || field(ps.Arr[0], "ticketId").Str != pre {
		t.Fatalf("child executionPrerequisites: %s", wire.Encode(ps))
	}
	if !auditConsistent(t, root) {
		t.Fatal("receipt audit is not CONSISTENT")
	}
}

// TestTLFV0884_ResumeRefusesARefinedChild: when another writer changes the
// child's criteria after an interruption, the resumed parent REFINE refuses
// under the writer lock and the parent keeps every criterion.
func TestTLFV0884_ResumeRefusesARefinedChild(t *testing.T) {
	id, root := splitParent(t)
	restore := cli.SetSplitAfterStep(func(step string) error {
		if step == "evidence/0" {
			return errors.New("interrupted for the test")
		}
		return nil
	})
	x := atm(t, root, nil, splitArgs(id, "--request-id", "split-g")...)
	restore()
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("interrupted split reported OK: %s", x.stdout)
	}
	var child string
	for _, item := range atm(t, root, nil, "ticket", "search", "--kind", "FEATURE").res.Items {
		if tid := field(item, "ticketId").Str; tid != id {
			child = tid
		}
	}
	if r := atm(t, root, nil, "ticket", "refine", "--target", child, "--expected-revision", "2", "--request-id", "narrow",
		"--issued-at", "2026-10-31T12:30:00Z", "--payload", `{"acceptanceCriteria":["second holds"]}`); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("refine child: %+v\n%s", r.res, r.stdout)
	}
	y := atm(t, root, nil, splitArgs(id, "--request-id", "split-g")...)
	if y.res.Outcome == wire.OutcomeOK || !hasCode(y.res, wire.CodeStaleTicket) {
		t.Fatalf("resume over a refined child: %+v\n%s", y.res, y.stdout)
	}
	parent, _ := showRecord(t, root, id)
	if got := strs(field(parent, "acceptanceCriteria")); len(got) != 3 || field(parent, "revision").Str != "2" {
		t.Fatalf("parent changed: %s", wire.Encode(parent))
	}
}

// TestTLFV0884_RecordedRefineMustBeThisSplit: a Q/refine recorded by another
// refinement, or by a split of other criteria, is REQUEST_ID_CONFLICT, not
// alreadySplit.
func TestTLFV0884_RecordedRefineMustBeThisSplit(t *testing.T) {
	id, root := splitParent(t)
	if r := atm(t, root, nil, "ticket", "refine", "--target", id, "--expected-revision", "2", "--request-id", "split-r/refine",
		"--issued-at", "2026-10-31T11:00:00Z", "--payload", `{"body":"Plain body edit."}`); r.res.Outcome != wire.OutcomeOK {
		t.Fatalf("plain refine: %+v\n%s", r.res, r.stdout)
	}
	if x := atm(t, root, nil, splitArgs(id, "--request-id", "split-r")...); !hasCode(x.res, wire.CodeRequestIDConflict) {
		t.Fatalf("split over a plain refine: %+v\n%s", x.res, x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "split", id, "--criteria", "2,3", "--expected-revision", "3",
		"--issued-at", splitIssuedAt, "--request-id", "split-s"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("split: %+v\n%s", x.res, x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "split", id, "--criteria", "2", "--expected-revision", "3",
		"--issued-at", splitIssuedAt, "--request-id", "split-s"); !hasCode(x.res, wire.CodeRequestIDConflict) {
		t.Fatalf("rerun with other criteria: %+v\n%s", x.res, x.stdout)
	}
}

// TestTLFV0884_ParentBodyAtTheLimitRefusesFirst: a parent body that cannot
// take the split note refuses LIMIT_EXCEEDED before any write, in the dry run
// as in the real run.
func TestTLFV0884_ParentBodyAtTheLimitRefusesFirst(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	payload := strings.Replace(strings.Replace(createPayloadJSON, `"acceptanceCriteria":["it exists"]`,
		`"acceptanceCriteria":["first holds","second holds"]`, 1),
		`"body":null`, `"body":"`+strings.Repeat("b", wire.MaxBodyBytes-40)+`"`, 1)
	created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "big", "--issued-at", "2026-10-30T12:00:00Z", "--payload", payload)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	for _, extra := range [][]string{{"--dry-run"}, {"--request-id", "split-big"}} {
		args := append([]string{"ticket", "split", id, "--criteria", "2", "--expected-revision", "1", "--issued-at", splitIssuedAt}, extra...)
		if x := atm(t, r.Root, nil, args...); !hasCode(x.res, wire.CodeLimitExceeded) {
			t.Fatalf("%v: %+v\n%s", extra, x.res, x.stdout)
		}
	}
	if n := len(atm(t, r.Root, nil, "ticket", "search", "--kind", "FEATURE").res.Items); n != 1 {
		t.Fatalf("refused split wrote tickets: %d", n)
	}
}

// TestTLFV0884_EvidenceProvenanceSurvivesRepeatedDigests: a digest attached
// at two acceptance revisions is copied once, and the copy's reason keeps
// both entries' revisions and reasons.
func TestTLFV0884_EvidenceProvenanceSurvivesRepeatedDigests(t *testing.T) {
	id, root := splitParent(t)
	if x := atm(t, root, nil, "ticket", "refine", "--target", id, "--expected-revision", "2", "--request-id", "reword",
		"--issued-at", "2026-10-30T12:05:00Z", "--payload", `{"acceptanceCriteria":["first holds","second holds","third still holds"]}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("refine: %+v\n%s", x.res, x.stdout)
	}
	d := string(wire.Sum([]byte("focused log")))
	if ev := atm(t, root, nil, "ticket", "attach-evidence", "--target", id, "--expected-revision", "3",
		"--request-id", "ev-2", "--issued-at", "2026-10-30T12:06:00Z",
		"--payload", `{"evidence":["`+d+`"],"reason":"rerun log"}`); ev.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %+v\n%s", ev.res, ev.stdout)
	}
	x := atm(t, root, nil, "ticket", "split", id, "--criteria", "2,3", "--expected-revision", "4",
		"--issued-at", splitIssuedAt, "--request-id", "split-p")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("split: %+v\n%s", x.res, x.stdout)
	}
	_, shown := showRecord(t, root, field(x.res.Items[0], "child").Str)
	ev := field(shown, "attachedEvidence")
	if len(ev.Arr) != 1 {
		t.Fatalf("child evidence: %s", wire.Encode(ev))
	}
	reason := field(ev.Arr[0], "reason").Str
	for _, want := range []string{"[acceptanceRevision 1, ", "] focused log", "[acceptanceRevision 2, ", "] rerun log"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("child evidence reason %q lacks %q", reason, want)
		}
	}
}
