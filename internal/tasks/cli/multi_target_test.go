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

// TestCALV0967_InterruptedMultiTargetWriteResumes covers V1-3646: a
// two-ticket change interrupted after its first target landed resumes by
// rerunning the same write; the landed target replays its receipt, the other
// applies once, and a further rerun writes nothing: a replay reports the same
// resultingRevision and no new receipt. The base request id is
// long enough that every derived id takes the 64-byte digest form
// (CAL-V0-967).
func TestCALV0967_InterruptedMultiTargetWriteResumes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, a := tcdTicket(t, root, "alpha", "")
	_, b := tcdTicket(t, root, "bravo", "")
	base := "lane-fric-b-split-v1-3646-0123456789abcdef0123456789abcdef-0123"
	write := func(targets ...string) map[string]wire.Value {
		args := append([]string{"ticket", "note", "set"}, targets...)
		return mtRows(t, atm(t, root, nil, append(args, "--request-id", base, "--issued-at", "2026-11-01T06:00:00Z", "--text", "split from V1-3646")...))
	}
	first := write(a, "zz-none") // the second target fails: the change is half applied
	if field(first[a], "outcome").Str != "OK" || field(first["zz-none"], "outcome").Str != "REFUSED" || len(field(first[a], "requestId").Str) != wire.MaxRequestIDBytes {
		t.Fatalf("interrupted write: %v", first)
	}
	resumed := write(a, b)
	if !field(resumed[a], "replayed").Bool || field(resumed[a], "receipt").Str != "" || field(resumed[a], "resultingRevision").Str != field(first[a], "resultingRevision").Str ||
		field(resumed[b], "outcome").Str != "OK" || field(resumed[b], "replayed").Bool {
		t.Fatalf("resume: a=%s b=%s first=%s", wire.EncodeFile(resumed[a]), wire.EncodeFile(resumed[b]), wire.EncodeFile(first[a]))
	}
	again := write(a, b)
	for _, id := range []string{a, b} {
		if !field(again[id], "replayed").Bool || field(again[id], "receipt").Str != "" || field(again[id], "resultingRevision").Str != "2" {
			t.Fatalf("converged rerun wrote %s again: %v", id, again)
		}
	}
}
