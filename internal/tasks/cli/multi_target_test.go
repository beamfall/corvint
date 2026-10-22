package cli_test

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mtRows maps each target argument to its multi-target row.
func mtRows(t *testing.T, x run) map[string]wire.Value {
	t.Helper()
	if len(x.res.Items) != 1 || field(x.res.Items[0], "profile").Str != "taskman-multi-target/0" {
		t.Fatalf("multi-target item: %s", x.stdout)
	}
	out := map[string]wire.Value{}
	for _, r := range field(x.res.Items[0], "targets").Arr {
		out[field(r, "target").Str] = r
	}
	return out
}

// TestTCDV0172_NoteSetSeveralTickets covers V1-2398: one note text on
// several tickets, one row per target; an unknown target is refused on its
// row without blocking the others; the same request id and issuedAt replay;
// a single target keeps its original item (TCD-V0-172).
func TestTCDV0172_NoteSetSeveralTickets(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	aID, a := tcdTicket(t, root, "alpha", "")
	_, b := tcdTicket(t, root, "bravo", "")
	args := []string{"ticket", "note", "set", a, b, "zz-none", "--request-id", "triage", "--issued-at", "2026-10-22T06:00:00Z", "--text", "blocked on #1540"}
	x := atm(t, root, nil, args...)
	rows := mtRows(t, x)
	if x.res.Outcome != wire.OutcomeRefused || field(rows[a], "outcome").Str != "OK" || field(rows[b], "outcome").Str != "OK" ||
		field(rows["zz-none"], "outcome").Str != "REFUSED" || field(rows[a], "requestId").Str != "triage-"+a || field(rows[a], "receipt").Str == "" {
		t.Fatalf("multi note set: %s", x.stdout)
	}
	for _, id := range []string{a, b} {
		n := atm(t, root, nil, "ticket", "note", "show", id)
		if field(n.res.Items[0], "state").Str != "CURRENT" || field(n.res.Items[0], "text").Str != "blocked on #1540" {
			t.Fatalf("note on %s: %s", id, n.stdout)
		}
	}
	again := mtRows(t, atm(t, root, nil, args...))
	if !field(again[a], "replayed").Bool || !field(again[b], "replayed").Bool {
		t.Fatal("rerun did not replay")
	}
	if x := atm(t, root, nil, "ticket", "note", "set", a, b, "--request-id", "x", "--supersedes", "1", "--text", "t"); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("multi-target --supersedes: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "note", "clear", a, b, "--request-id", "x"); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("multi-target clear: %s", x.stdout)
	}
	single := atm(t, root, nil, "ticket", "note", "set", aID, "--request-id", "solo", "--text", "only alpha")
	if single.res.Outcome != wire.OutcomeOK || field(single.res.Items[0], "profile").Str == "taskman-multi-target/0" ||
		field(single.res.Items[0], "resultingRevision").Str == "" {
		t.Fatalf("single target: %s", single.stdout)
	}
}

// TestTCDV0173_HoldSeveralTicketsReadsRevisions covers V1-2398: one holdId
// and reason on several --target tickets, each at the revision hold reads
// itself; a COMPLETED target is refused on its row without blocking the
// others; a single target without --expected-revision reads its revision;
// --expected-revision with several targets and several targets on another
// verb are refused (TCD-V0-173).
func TestTCDV0173_HoldSeveralTicketsReadsRevisions(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	aID, a := tcdTicket(t, root, "alpha", "")
	bID, b := tcdTicket(t, root, "bravo", "")
	cID, c := tcdTicket(t, root, "charlie", "")
	dID, _ := tcdTicket(t, root, "delta", "")
	if x := atm(t, root, nil, "ticket", "note", "set", bID, "--request-id", "bump", "--text", "revision 2"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("note: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "complete-manual", "--request-id", "done", "--target", cID, "--expected-revision", "1",
		"--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("complete-manual: %s", x.stdout)
	}
	payload := `{"holdId":"owner-wait","reason":"waits for owner"}`
	x := atm(t, root, nil, "ticket", "hold", "--target", a, "--target", b, "--target", c, "--request-id", "park", "--payload", payload)
	rows := mtRows(t, x)
	if x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeTicketState) ||
		field(rows[a], "outcome").Str != "OK" || field(rows[a], "expectedRevision").Str != "1" ||
		field(rows[b], "outcome").Str != "OK" || field(rows[b], "expectedRevision").Str != "2" ||
		field(rows[c], "outcome").Str != "REFUSED" {
		t.Fatalf("multi hold: %s", x.stdout)
	}
	for _, id := range []string{aID, bID} {
		if s := field(bcShow(t, root, id), "status").Str; s != "HELD" {
			t.Fatalf("%s status %s", id, s)
		}
	}
	if x := atm(t, root, nil, "ticket", "hold", "--target", dID, "--request-id", "solo", "--payload", payload); x.res.Outcome != wire.OutcomeOK ||
		field(bcShow(t, root, dID), "status").Str != "HELD" {
		t.Fatalf("single hold without --expected-revision: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "hold", "--target", a, "--target", b, "--expected-revision", "2", "--request-id", "r", "--payload", payload); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("several targets with --expected-revision: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "release-hold", "--target", a, "--target", b, "--expected-revision", "2", "--request-id", "r", "--payload", `{"holdId":"owner-wait"}`); x.res.Outcome != wire.OutcomeError {
		t.Fatalf("several targets on release-hold: %s", x.stdout)
	}
}
