package cli_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// bcRows maps local ticket token to batch-close disposition.
func bcRows(t *testing.T, x run) map[string]string {
	t.Helper()
	if len(x.res.Items) != 1 || field(x.res.Items[0], "profile").Str != "taskman-batch-close/0" {
		t.Fatalf("batch-close item: %s", x.stdout)
	}
	out := map[string]string{}
	for _, r := range field(x.res.Items[0], "tickets").Arr {
		out[field(r, "local").Str] = field(r, "disposition").Str
	}
	return out
}

func bcShow(t *testing.T, root, id string) wire.Value {
	t.Helper()
	x := atm(t, root, nil, "ticket", "show", id)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("ticket show %s: %s", id, x.stdout)
	}
	return x.res.Items[0]
}

// TestTCDV0169_BatchCloseClosesDeliveredOpenTickets covers V1-2396: a
// missing green run writes nothing; --dry-run plans without writing; the
// write closes each delivered OPEN ticket with one COMPLETE_MANUAL citing
// sha256 of the merge sha and run URL; a held, live-attempt and
// mentioned-only ticket is reported with a code and recovery and never
// closed, without stopping the others; a rerun skips COMPLETED tickets;
// --repo composes the run URL; the exit is 0 only when every delivered
// ticket ends COMPLETED (TCD-V0-169).
func TestTCDV0169_BatchCloseClosesDeliveredOpenTickets(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, claimed := leaseCLIStore(t, 1, time.Now().UTC().Add(-11*time.Minute))
	liveID := claimed[0].Ticket
	live := liveID[strings.LastIndex(liveID, ":")+1:]
	openID, open := tcdTicket(t, root, "open", "")
	heldID, held := tcdTicket(t, root, "held", "")
	_, mentioned := tcdTicket(t, root, "mentioned", "")
	eID, e := tcdTicket(t, root, "echo", "")
	if x := atm(t, root, nil, "ticket", "hold", "--target", heldID, "--expected-revision", "1", "--request-id", "hold-held",
		"--payload", `{"holdId":"owner-wait","reason":"waits for owner"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("hold: %s", x.stdout)
	}
	base := revParse(t, root, "HEAD")
	l20 := tdsCommit(t, root, "lane a", base)
	l21 := tdsCommit(t, root, "lane b", l20)
	l22 := tdsCommit(t, root, "lane e", base)
	m, h := tdsCommit(t, root, "batch x", base, l21), strings.Repeat("b", 40)
	m2 := tdsCommit(t, root, "batch y", base, l22)
	runURL := "https://github.com/beamfall/corvint/actions/runs/5"
	prs := tdsExport(t, "prs.json", `[
{"number":20,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+open+"\nRefs "+held+"\n\nResidual: "+mentioned+" blocked")+`","headRefOid":"`+l20+`","labels":[]},
{"number":21,"state":"MERGED","title":"lane b","body":"`+tdsJSON("Refs "+live)+`","headRefOid":"`+l21+`","labels":[]},
{"number":22,"state":"MERGED","title":"lane e","body":"`+tdsJSON("Refs "+e)+`","headRefOid":"`+l22+`","labels":[]},
{"number":30,"state":"MERGED","title":"Batch x: #20 #21","body":"- #20\n- #21","headRefOid":"`+h+`","mergeCommit":{"oid":"`+m+`"},"mergedAt":"2026-10-22T05:00:00Z","labels":[]},
{"number":31,"state":"MERGED","title":"Batch y: #22","body":"- #22","headRefOid":"`+h+`","mergeCommit":{"oid":"`+m2+`"},"mergedAt":"2026-10-22T06:00:00Z","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[
{"databaseId":5,"headSha":"`+m+`","conclusion":"success","url":"`+runURL+`"},
{"databaseId":9,"headSha":"`+h+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/9"},
{"databaseId":6,"headSha":"`+m2+`","conclusion":"success"}]`)
	red := tdsExport(t, "red.json", `[{"databaseId":5,"headSha":"`+m+`","conclusion":"failure"}]`)

	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	unchanged := func(what string) {
		t.Helper()
		if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
			t.Fatalf("%s changed the intent or state tree", what)
		}
	}
	if x := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "30", "--prs", prs, "--runs", red); !hasCode(x.res, wire.CodePrerequisiteUnsatisfied) {
		t.Fatalf("no green run: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "30", "--prs", prs); !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("no runs export: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "31", "--prs", prs, "--runs", runs); !hasCode(x.res, wire.CodePrerequisiteUnsatisfied) {
		t.Fatalf("run without url or --repo: %s", x.stdout)
	}
	dry := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "30", "--prs", prs, "--runs", runs, "--dry-run")
	unchanged("missing run or --dry-run")
	want := map[string]string{open: "CLOSE", held: "HELD", live: "ATTEMPT_LIVE", mentioned: "MENTIONED_ONLY"}
	if dry.code != 0 || !sameRows(bcRows(t, dry), want) || !field(dry.res.Items[0], "dryRun").Bool {
		t.Fatalf("dry run %v: %s", bcRows(t, dry), dry.stdout)
	}

	x := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "30", "--prs", prs, "--runs", runs)
	want[open] = "CLOSED"
	if x.code != 1 || x.res.Outcome != wire.OutcomeRefused || !hasCode(x.res, wire.CodeTicketState) || !hasCode(x.res, wire.CodeAttemptLive) || !sameRows(bcRows(t, x), want) {
		t.Fatalf("close %v: exit %d %s", bcRows(t, x), x.code, x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "runUrl").Str != runURL || field(item, "issuedAt").Str != "2026-10-22T05:00:00Z" {
		t.Fatalf("header: %s", x.stdout)
	}
	for _, r := range field(item, "tickets").Arr {
		switch field(r, "local").Str {
		case open:
			if field(r, "requestId").Str != "batch-close-30-"+open || field(r, "receipt").Str == "" {
				t.Fatalf("closed row: %s", wire.Encode(r))
			}
		case held, live, mentioned:
			if field(r, "recovery").Str == "" || field(r, "receipt").Kind != wire.KindNull {
				t.Fatalf("unclosed row: %s", wire.Encode(r))
			}
		}
	}
	shown := bcShow(t, root, openID)
	completion := field(field(shown, "record"), "completion")
	wantEvidence := []string{string(wire.Sum([]byte(m))), string(wire.Sum([]byte(runURL)))}
	slices.Sort(wantEvidence)
	var gotEvidence []string
	for _, d := range field(completion, "evidence").Arr {
		gotEvidence = append(gotEvidence, d.Str)
	}
	if field(shown, "status").Str != "COMPLETED" || !slices.Equal(gotEvidence, wantEvidence) ||
		field(completion, "reason").Str != "Delivered by lane PR #20; merged in batch PR #30 at "+m+"; CI green: "+runURL {
		t.Fatalf("closed ticket: %s", wire.Encode(shown))
	}
	if field(bcShow(t, root, heldID), "status").Str != "HELD" || field(bcShow(t, root, liveID), "status").Str != "OPEN" {
		t.Fatal("a held or live-attempt ticket was closed")
	}

	beforeIntent, beforeState = fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	again := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "30", "--prs", prs, "--runs", runs)
	unchanged("an idempotent rerun")
	want[open] = "ALREADY_COMPLETED"
	if again.code != 1 || !sameRows(bcRows(t, again), want) {
		t.Fatalf("rerun %v: %s", bcRows(t, again), again.stdout)
	}

	ok := atm(t, root, nil, "ticket", "batch-close", "--manual", "--pr", "31", "--prs", prs, "--runs", runs, "--repo", "beamfall/corvint")
	if ok.code != 0 || !sameRows(bcRows(t, ok), map[string]string{e: "CLOSED"}) || field(ok.res.Items[0], "runUrl").Str != "https://github.com/beamfall/corvint/actions/runs/6" {
		t.Fatalf("all closed: exit %d %s", ok.code, ok.stdout)
	}
	if field(bcShow(t, root, eID), "status").Str != "COMPLETED" {
		t.Fatal("batch 31 did not close its ticket")
	}
}

func sameRows(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}
