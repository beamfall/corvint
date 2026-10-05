package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestIssue502_EscalationCLIWritesAndReads drives `ticket escalate`,
// `ticket answer` and `ticket escalation list|show|history` against real
// claim receipts (ESC-V0-001, ESC-V0-004, ESC-V0-009): the origin comes from
// the claim receipt, a receipt that does not admit the attempt abstains,
// ambiguous shorthand names the open requests, exact and shorthand answers
// resolve one question, and the reads page, filter and walk the chain
// without writing anything.
func TestIssue502_EscalationCLIWritesAndReads(t *testing.T) {
	root, claimed := leaseCLIStore(t, 2, time.Now().UTC().Truncate(time.Second).Add(-11*time.Minute))
	t.Setenv("CORVINT_TASKS_ACTOR", "holder")
	a, b := claimed[0], claimed[1]

	ok := func(x run) wire.Value {
		t.Helper()
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) == 0 {
			t.Fatalf("%v: %+v", x.res.Command, x.res)
		}
		return x.res.Items[0]
	}
	event := func(item wire.Value) wire.Value {
		t.Helper()
		events := field(item, "events").Arr
		if len(events) != 1 {
			t.Fatalf("events = %+v", events)
		}
		return events[0]
	}

	open := ok(atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", a.Receipt,
		"--kind", "decision", "--question", "Which store?", "--options", "x,y", "--request-id", "q-1"))
	if ev := event(open); field(ev, "operation").Str != "OPEN" || field(ev, "escalationId").Str != "q-1" || field(ev, "revision").Str != "1" {
		t.Fatalf("open event = %+v", ev)
	}
	if field(open, "ticketId").Str != a.Ticket || field(open, "escalationCode").Kind != wire.KindNull {
		t.Fatalf("open = %+v", open)
	}
	replay := ok(atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", a.Receipt,
		"--kind", "decision", "--question", "Which store?", "--options", "x,y", "--request-id", "q-1"))
	if !field(replay, "replayed").Bool {
		t.Fatalf("identical retry did not replay: %+v", replay)
	}

	// B's attempt never appears in A's ADMIT receipt: the CLI abstains
	// before writing anything.
	stateDir := filepath.Join(root, ".git", "taskman")
	if repo, err := intent.Resolve(root); err == nil {
		stateDir = repo.StateDir
	}
	before := fixture.TreeSnapshot(t, stateDir)
	wrong := atm(t, root, nil, "ticket", "escalate", "--attempt", b.AttemptID, "--claim-receipt", string(*a.Outcome.ReceiptSeq),
		"--kind", "decision", "--question", "?", "--request-id", "q-bad")
	if wrong.res.Outcome != wire.OutcomeRefused || !hasCode(wrong.res, wire.CodeMissingEvidence) ||
		field(wrong.res.Items[0], "escalationCode").Str != "MISSING_ADMISSION_CONTEXT" {
		t.Fatalf("mismatched receipt = %+v", wrong.res)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("the state dir changed")
	}

	ok(atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", string(*a.Outcome.ReceiptSeq),
		"--kind", "scope", "--question", "Widen?", "--request-id", "q-2"))

	big := atm(t, root, nil, "ticket", "escalate", "--attempt", a.AttemptID, "--claim-receipt", a.Receipt,
		"--kind", "decision", "--question", strings.Repeat("q", 4097), "--request-id", "q-big")
	if big.res.Outcome == wire.OutcomeOK || !hasCode(big.res, wire.CodeLimitExceeded) {
		t.Fatalf("oversized question = %+v", big.res)
	}

	ambiguous := atm(t, root, nil, "ticket", "answer", "--target", a.Ticket, "--text", "yes", "--request-id", "ans-0")
	if ambiguous.res.Outcome != wire.OutcomeRefused || field(ambiguous.res.Items[0], "escalationCode").Str != "AMBIGUOUS_OPEN_QUESTIONS" {
		t.Fatalf("ambiguous answer = %+v", ambiguous.res)
	}
	var ids []string
	for _, v := range field(ambiguous.res.Items[0], "requestIds").Arr {
		ids = append(ids, v.Str)
	}
	if !slices.Equal(ids, []string{"q-1", "q-2"}) {
		t.Fatalf("ambiguous answer names %v", ids)
	}
	stale := atm(t, root, nil, "ticket", "answer", "--target", a.Ticket, "--text", "x", "--request", "q-1",
		"--expected-request-revision", "1", "--expected-revision", "1", "--request-id", "ans-stale")
	if stale.res.Outcome != wire.OutcomeRefused || !hasCode(stale.res, wire.CodeStaleTicket) {
		t.Fatalf("stale ticket CAS = %+v", stale.res)
	}

	exact := ok(atm(t, root, nil, "ticket", "answer", "--target", a.Ticket, "--text", "x", "--request", "q-1",
		"--expected-request-revision", "1", "--request-id", "ans-1"))
	if ev := event(exact); field(ev, "resolvedRequestId").Str != "q-1" || field(ev, "revision").Str != "2" {
		t.Fatalf("exact answer event = %+v", ev)
	}
	short := ok(atm(t, root, nil, "ticket", "answer", "--target", a.Ticket, "--text", "no", "--request-id", "ans-2"))
	if ev := event(short); field(ev, "resolvedRequestId").Str != "q-2" || field(ev, "resolvedPreviousRevision").Str != "1" {
		t.Fatalf("shorthand answer event = %+v", ev)
	}
	ok(atm(t, root, nil, "ticket", "escalate", "--attempt", b.AttemptID, "--claim-receipt", b.Receipt,
		"--kind", "blocked", "--question", "Wait for A?", "--blocked-by", a.Ticket, "--request-id", "q-3"))

	before = fixture.TreeSnapshot(t, stateDir)
	list := func(args ...string) run {
		t.Helper()
		x := atm(t, root, nil, append([]string{"ticket", "escalation", "list"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK || !x.res.Untrusted || x.res.Page == nil {
			t.Fatalf("list %v = %+v", args, x.res)
		}
		return x
	}
	keys := func(x run) []string {
		var out []string
		for _, it := range x.res.Items {
			out = append(out, field(it, "requestId").Str+"/"+field(it, "state").Str)
		}
		return out
	}
	all := list()
	if got := keys(all); !slices.Equal(got, []string{"q-1/ANSWERED", "q-2/ANSWERED", "q-3/OPEN"}) {
		t.Fatalf("list = %v", got)
	}
	first := all.res.Items[0]
	if field(first, "applicability").Str != "CURRENT" || field(first, "program").Str != "UNKNOWN" || field(first, "retryState").Str != "NOT_OBSERVED" ||
		field(first, "question").Str != "Which store?" || field(first, "clockUncertain").Bool ||
		field(field(first, "source"), "holder").Str != "holder" || field(field(first, "source"), "attemptId").Str != a.AttemptID {
		t.Fatalf("list entry = %+v", first)
	}
	page := list("--limit", "2")
	if len(page.res.Items) != 2 || !page.res.Page.Truncated {
		t.Fatalf("page 1 = %+v", page.res)
	}
	next := list("--limit", "2", "--cursor", field(page.res.Items[1], "originSha256").Str)
	if got := keys(next); !slices.Equal(got, []string{"q-3/OPEN"}) || next.res.Page.Offset != "2" || next.res.Page.Truncated {
		t.Fatalf("page 2 = %v %+v", got, next.res.Page)
	}
	if got := keys(list("--state", "ANSWERED")); !slices.Equal(got, []string{"q-1/ANSWERED", "q-2/ANSWERED"}) {
		t.Fatalf("state filter = %v", got)
	}
	if got := keys(list("--kind", "blocked")); !slices.Equal(got, []string{"q-3/OPEN"}) {
		t.Fatalf("kind filter = %v", got)
	}
	if got := keys(list("--target", b.Ticket)); !slices.Equal(got, []string{"q-3/OPEN"}) {
		t.Fatalf("target filter = %v", got)
	}
	if x := list("--program", "night-shift"); len(x.res.Items) != 0 || !warningsContain(x.res.Warnings, "UNKNOWN") {
		t.Fatalf("program filter = %+v", x.res)
	}
	if x := atm(t, root, nil, "ticket", "escalation", "list", "--limit", "51"); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("limit 51 accepted: %+v", x.res)
	}

	show := ok(atm(t, root, nil, "ticket", "escalation", "show", a.Ticket, "q-1"))
	if answer := field(show, "answer"); field(answer, "text").Str != "x" || field(answer, "requestId").Str != "ans-1" {
		t.Fatalf("show answer = %+v", show)
	}
	if blocked := ok(atm(t, root, nil, "ticket", "escalation", "show", b.Ticket, "q-3")); field(field(blocked, "blockedBy"), "ticketId").Str != a.Ticket {
		t.Fatalf("show blockedBy = %+v", blocked)
	}
	if x := atm(t, root, nil, "ticket", "escalation", "show", a.Ticket, "q-9"); x.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("show missing = %+v", x.res)
	}

	history := atm(t, root, nil, "ticket", "escalation", "history", a.Ticket, "q-1")
	if len(history.res.Items) != 2 || field(history.res.Items[0], "operation").Str != "ANSWER" || field(history.res.Items[1], "operation").Str != "OPEN" ||
		field(history.res.Items[0], "previousSha256").Str != field(history.res.Items[1], "sha256").Str {
		t.Fatalf("history = %+v", history.res)
	}
	for _, it := range history.res.Items {
		if p := field(it, "escalation"); field(p, "recordedAt").Str != field(history.res.Items[1], "recordedAt").Str ||
			field(field(p, "source"), "holder").Str != "holder" || field(p, "applicability").Str != "CURRENT" {
			t.Fatalf("history provenance = %+v", it)
		}
	}
	tail := atm(t, root, nil, "ticket", "escalation", "history", a.Ticket, "q-1", "--limit", "1", "--cursor", field(history.res.Items[0], "previousSha256").Str)
	if len(tail.res.Items) != 1 || field(tail.res.Items[0], "text").Str != "Which store?" || tail.res.Page.Offset != "1" {
		t.Fatalf("history page = %+v", tail.res)
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("the state dir changed")
	}

	// A deleted origin is missing evidence: list marks the ticket's entries
	// UNAVAILABLE and show refuses, never inventing the question.
	origin := field(list("--target", b.Ticket).res.Items[0], "originSha256").Str
	if err := os.Remove(filepath.Join(stateDir, "evidence", origin)); err != nil {
		t.Fatal(err)
	}
	damaged := list("--target", b.Ticket)
	if it := damaged.res.Items[0]; field(it, "material").Str != "UNAVAILABLE" || field(it, "code").Str != wire.CodeMissingEvidence ||
		!warningsContain(damaged.res.Warnings, "unavailable") {
		t.Fatalf("damaged list = %+v", damaged.res)
	}
	if x := atm(t, root, nil, "ticket", "escalation", "show", b.Ticket, "q-3"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeMissingEvidence) {
		t.Fatalf("damaged show = %+v", x.res)
	}

	// A rewritten answer event is journal damage, not malformed input.
	head := field(show, "headSha256").Str
	if err := os.WriteFile(filepath.Join(stateDir, "evidence", head), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"show", "history"} {
		if x := atm(t, root, nil, "ticket", "escalation", verb, a.Ticket, "q-1"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeJournalForked) {
			t.Fatalf("rewritten event %s = %+v", verb, x.res)
		}
	}
	if !strings.HasPrefix(a.Receipt, "0") {
		t.Fatalf("claim receipt name %q is not the padded form", a.Receipt)
	}
}
