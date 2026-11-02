// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTLFV01101_BatchReadsRecordedProgress covers V1-4268: batch-delivered
// and batch-close read each ticket's effective progress, so a ticket marked
// IN_REVIEW with a lane or batch pr, or MERGED_AWAITING_CLOSEOUT with a head
// the merge brought in, is delivered without a body mention (TLF-V0-1101).
// Progress naming an excluded PR, a pre-merge head, or an IN_PROGRESS phase
// delivers nothing; a text delivery keeps its source.
func TestTLFV01101_BatchReadsRecordedProgress(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	textID, text := tcdTicket(t, root, "text", "")
	reviewID, review := tcdTicket(t, root, "review", "")
	mergedID, merged := tcdTicket(t, root, "merged", "")
	promotedID, promoted := tcdTicket(t, root, "promoted", "")
	busyID, busy := tcdTicket(t, root, "busy", "")
	deferredID, deferredT := tcdTicket(t, root, "deferred", "")
	staleID, stale := tcdTicket(t, root, "stale", "")
	base := revParse(t, root, "HEAD")
	l20 := tdsCommit(t, root, "lane a", base)
	deferred := tdsCommit(t, root, "deferred lane", base)
	m := tdsCommit(t, root, "batch merge", base, l20)
	mark := func(id, payload string) {
		t.Helper()
		x := atm(t, root, nil, "ticket", "progress", "--target", id, "--expected-revision", "current",
			"--request-id", "p-"+id[strings.LastIndex(id, ":")+1:], "--issued-at", "2026-11-01T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("progress %s: %s", id, x.stdout)
		}
	}
	mark(textID, progressPayload("IN_REVIEW", "", "", "20", ""))
	mark(reviewID, progressPayload("IN_REVIEW", "", "", "20", ""))
	mark(mergedID, progressPayload("MERGED_AWAITING_CLOSEOUT", "", l20, "", ""))
	mark(promotedID, progressPayload("IN_MERGE_QUEUE", "", "", "30", ""))
	mark(busyID, progressPayload("IN_PROGRESS", "", "", "", ""))
	mark(deferredID, progressPayload("IN_REVIEW", "", "", "23", ""))
	mark(staleID, progressPayload("MERGED_AWAITING_CLOSEOUT", "", base, "", ""))
	prs := tdsExport(t, "prs.json", `[
{"number":20,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+text+"\n\nResidual: "+promoted+" blocked on owner")+`","headRefOid":"`+l20+`","labels":[]},
{"number":23,"state":"OPEN","title":"lane pi","body":"","headRefOid":"`+deferred+`","labels":[]},
{"number":30,"state":"MERGED","title":"Batch x: #20","body":"`+tdsJSON("- #20: lane a\n#23 is deferred")+`","headRefOid":"`+l20+`","mergeCommit":{"oid":"`+m+`"},"mergedAt":"2026-11-01T05:00:00Z","url":"https://github.com/o/r/pull/30","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":5,"headSha":"`+m+`","conclusion":"success","url":"https://github.com/o/r/actions/runs/5"}]`)

	x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "30", "--prs", prs, "--runs", runs)
	if x.code != 0 {
		t.Fatalf("batch-delivered: exit %d %s", x.code, x.stdout)
	}
	item := x.res.Items[0]
	got := map[string]string{}
	for _, r := range field(item, "delivered").Arr {
		got[field(r, "local").Str] = field(r, "source").Str + "@" + field(r, "lanePr").Str
	}
	want := map[string]string{text: "PR_TEXT@20", review: "PROGRESS@20", merged: "PROGRESS@"}
	if !sameRows(got, want) {
		t.Fatalf("delivered %v: %s", got, x.stdout)
	}
	// A text mention, here a "blocked" veto, outranks progress naming the
	// batch PR: the ticket stays mentioned and is never closed.
	if weak := field(item, "mentioned").Arr; len(weak) != 1 || field(weak[0], "local").Str != promoted {
		t.Fatalf("mentioned: %s", x.stdout)
	}

	dry := atm(t, root, nil, "ticket", "batch-close", "--pr", "30", "--prs", prs, "--runs", runs, "--dry-run")
	rows := gbRows(t, dry)
	if len(rows) != 4 || field(rows[promoted], "disposition").Str != "MENTIONED_ONLY" || field(rows[review], "source").Str != "PROGRESS" ||
		// The fixture merge keeps its parent's tree: an empty diff proves no
		// claim scope and is refused before any write (TCD-V0-1101).
		field(rows[review], "disposition").Str != "UNRUNNABLE" || field(rows[review], "code").Str != wire.CodeMalformed ||
		!strings.Contains(field(rows[review], "recovery").Str, "--manual") ||
		field(rows[text], "source").Str != "PR_TEXT" {
		t.Fatalf("gated dry run: %s", dry.stdout)
	}
	manual := atm(t, root, nil, "ticket", "batch-close", "--pr", "30", "--prs", prs, "--runs", runs, "--manual")
	if got := bcRows(t, manual); !sameRows(got, map[string]string{text: "CLOSED", review: "CLOSED", merged: "CLOSED", promoted: "MENTIONED_ONLY"}) {
		t.Fatalf("--manual %v: %s", got, manual.stdout)
	}
	rec := field(bcShow(t, root, reviewID), "record")
	if reason := field(field(rec, "completion"), "reason").Str; !strings.Contains(reason, "Delivered by lane PR #20") || !strings.Contains(reason, "recorded progress IN_REVIEW") {
		t.Fatalf("progress close reason %q: %s", reason, wire.Encode(rec))
	}
	for _, id := range []string{promotedID, busyID, deferredID, staleID} {
		if st := field(bcShow(t, root, id), "status").Str; st != "OPEN" {
			t.Fatalf("%s (%s %s %s) is %s", id, busy, deferredT, stale, st)
		}
	}
}

// TestTLFV01102_BulkProgressWritesManyTargets covers V1-4268: one `ticket
// progress --batch` invocation marks many tickets, each an ordinary
// PROGRESS_SET with its own CAS and receipt and a per-entry outcome; a
// retry replays, a stale entry is refused alone, and a bad entry refuses the
// whole batch before any write (TLF-V0-1102).
func TestTLFV01102_BulkProgressWritesManyTargets(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	aID, a := tcdTicket(t, root, "alpha", "")
	bID, b := tcdTicket(t, root, "bravo", "")
	cID, c := tcdTicket(t, root, "charlie", "")
	file := filepath.Join(t.TempDir(), "marks.json")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(`[{"target":"` + a + `","expectedRevision":"current","payload":{"phase":"IN_REVIEW","pr":"20","lane":"w4"}},
{"target":"` + b + `","expectedRevision":"1","payload":{"phase":"BLOCKED","reason":"waits on owner"}},
{"target":"` + cID + `","expectedRevision":"current","payload":` + progressPayload("IN_PROGRESS", "", "", "", "") + `}]`)
	args := []string{"ticket", "progress", "--batch", "--request-id", "bulk", "--targets-file", file, "--issued-at", "2026-11-01T12:00:00Z", "--role", "OPERATOR"}
	entries := func(x run) []string {
		var out []string
		for _, e := range field(x.res.Items[0], "entries").Arr {
			out = append(out, field(e, "requestId").Str+"="+field(e, "outcome").Str+"/"+field(e, "resultingRevision").Str+"/"+strings.Join(strs(field(e, "codes")), ","))
		}
		return out
	}
	x := atm(t, root, nil, args...)
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "completed").Str != "3" ||
		strings.Join(entries(x), " ") != "bulk/0=COMPLETED/2/ bulk/1=COMPLETED/2/ bulk/2=COMPLETED/2/" {
		t.Fatalf("bulk progress: %v %s", entries(x), x.stdout)
	}
	for _, e := range field(x.res.Items[0], "entries").Arr {
		if field(e, "receipt").Str == "" {
			t.Fatalf("entry without a receipt: %s", wire.Encode(e))
		}
	}
	for id, phase := range map[string]string{aID: "IN_REVIEW", bID: "BLOCKED", cID: "IN_PROGRESS"} {
		if p := field(bcShow(t, root, id), "progress"); field(p, "phase").Str != phase {
			t.Fatalf("%s progress: %s", id, wire.Encode(p))
		}
	}
	if again := atm(t, root, nil, args...); again.res.Outcome != wire.OutcomeOK || field(again.res.Items[0], "replayed").Str != "3" {
		t.Fatalf("retry does not replay: %v %s", entries(again), again.stdout)
	}

	write(`[{"target":"` + a + `","expectedRevision":"1","payload":{"phase":"MERGED_AWAITING_CLOSEOUT","head":"` + strings.Repeat("c", 40) + `"}},
{"target":"` + b + `","expectedRevision":"current","payload":{"phase":"CLEARED"}}]`)
	stale := atm(t, root, nil, "ticket", "progress", "--batch", "--request-id", "bulk2", "--targets-file", file, "--role", "OPERATOR")
	if got := entries(stale); stale.res.Outcome != wire.OutcomeRefused || len(got) != 2 || strings.HasPrefix(got[0], "bulk2/0=COMPLETED") || got[1] != "bulk2/1=COMPLETED/3/" {
		t.Fatalf("stale entry: %v %s", got, stale.stdout)
	}

	before := field(bcShow(t, root, cID), "record")
	write(`[{"target":"` + c + `","expectedRevision":"current","payload":{"phase":"CLEARED"}},{"target":"NOPE-1","expectedRevision":"1","payload":{"phase":"CLEARED"}}]`)
	if bad := tdsRun(t, root, "ticket", "progress", "--batch", "--request-id", "bulk3", "--targets-file", file, "--role", "OPERATOR"); bad.res.Outcome == wire.OutcomeOK ||
		!strings.Contains(string(bad.stdout), "/1/target") {
		t.Fatalf("bad entry: %s", bad.stdout)
	}
	if after := field(bcShow(t, root, cID), "record"); string(wire.Encode(after)) != string(wire.Encode(before)) {
		t.Fatal("a refused batch wrote an entry")
	}
	for _, bad := range [][]string{
		{"ticket", "progress", "--batch", "--request-id", "b4", "--targets-file", file, "--target", a},
		{"ticket", "progress", "--batch", "--request-id", "b4", "--targets-file", file, "--payload", "[]"},
		{"ticket", "refine", "--request-id", "b4", "--targets-file", file},
	} {
		if r := tdsRun(t, root, bad...); !hasCode(r.res, wire.CodeMalformed) {
			t.Fatalf("%v: %s", bad, r.stdout)
		}
	}
	if h := atm(t, root, nil, "ticket", "progress", "--help", "--verbose"); !strings.Contains(string(h.stdout), "--targets-file FILE") || !strings.Contains(string(h.stdout), "TLF-V0-1102") {
		t.Fatalf("progress help: %s", h.stdout)
	}
}
