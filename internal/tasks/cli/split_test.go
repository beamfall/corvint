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
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
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
