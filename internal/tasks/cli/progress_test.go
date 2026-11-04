// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func progressPayload(phase, branch, head, pr, reason string) string {
	q := func(s string) string {
		if s == "" {
			return "null"
		}
		return `"` + s + `"`
	}
	return `{"branch":` + q(branch) + `,"head":` + q(head) + `,"host":"mac","lane":"w4","phase":"` + phase + `","pr":` + q(pr) + `,"reason":` + q(reason) + `}`
}

func onlyTicket(t *testing.T, root string) string {
	t.Helper()
	x := atm(t, root, nil, "ticket", "list")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("ticket list: %+v", x.res)
	}
	return field(x.res.Items[0], "ticketId").Str
}

// TestTLFV0881_BarrierAdmitsProgressAndStillRefusesClaims: under an
// OPERATOR/ADMISSION barrier an OPERATOR records progress through the default
// role matrix, the record keeps its acceptance and status, and the same
// barrier still refuses a claim after the write.
func TestTLFV0881_BarrierAdmitsProgressAndStillRefusesClaims(t *testing.T) {
	r := receiptFixture(t)
	id := onlyTicket(t, r.Root)
	if x := atm(t, r.Root, nil, "pause", "--request-id", "pause", "--role", "OPERATOR"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("pause: %+v", x.res)
	}
	before := atm(t, r.Root, nil, "ticket", "show", id)
	if before.res.Outcome != wire.OutcomeOK {
		t.Fatalf("show: %+v", before.res)
	}
	x := atm(t, r.Root, nil, "ticket", "progress", "--target", id, "--expected-revision", "1", "--role", "OPERATOR",
		"--request-id", "p1", "--issued-at", "2026-10-31T12:00:00Z", "--payload", progressPayload("IN_PROGRESS", "", "", "", ""))
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "resultingRevision").Str != "2" ||
		field(x.res.Items[0], "resultingAcceptanceRevision").Str != "1" {
		t.Fatalf("progress under the barrier: %+v\n%s", x.res, x.stdout)
	}
	after := atm(t, r.Root, nil, "ticket", "show", id)
	pre, post := field(before.res.Items[0], "record"), field(after.res.Items[0], "record")
	if field(pre, "status").Str != field(post, "status").Str || field(post, "acceptanceRevision").Str != "1" {
		t.Fatalf("status or acceptance moved: %s", wire.Encode(post))
	}
	if p := field(after.res.Items[0], "progress"); field(p, "phase").Str != "IN_PROGRESS" || field(p, "actor").Str == "" {
		t.Fatalf("show progress: %s", wire.Encode(p))
	}
	if entries := field(post, "progress"); len(entries.Arr) != 1 {
		t.Fatalf("record progress entries: %s", wire.Encode(entries))
	}
	claim := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", "c1")
	if claim.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a claim was admitted under the barrier after a progress write: %+v", claim.res)
	}
	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" {
		t.Fatalf("receipt audit: %+v", audit.res)
	}
}

// TestTLFV0882_SearchAndRoadmapByPhase: search --phase matches exactly the
// effective phase, NONE matches tickets without one, roadmap and search rows
// carry the effective entry, and CLEARED ends it.
func TestTLFV0882_SearchAndRoadmapByPhase(t *testing.T) {
	r := receiptFixture(t)
	id := onlyTicket(t, r.Root)
	ids := func(args ...string) []string {
		t.Helper()
		x := atm(t, r.Root, nil, append([]string{"ticket", "search"}, args...)...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("search %v: %+v", args, x.res)
		}
		var out []string
		for _, it := range x.res.Items {
			out = append(out, field(it, "ticketId").Str)
		}
		return out
	}
	if got := ids("--phase", "NONE"); len(got) != 1 || got[0] != id {
		t.Fatalf("--phase NONE before any write: %v", got)
	}
	head := strings.Repeat("a", 40)
	x := atm(t, r.Root, nil, "ticket", "progress", "--target", id, "--expected-revision", "1",
		"--request-id", "p1", "--issued-at", "2026-10-31T12:00:00Z", "--payload", progressPayload("DELIVERED_ON_BRANCH", "claude/lane-w4", head, "", ""))
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("progress: %+v\n%s", x.res, x.stdout)
	}
	if got := ids("--phase", "DELIVERED_ON_BRANCH"); len(got) != 1 {
		t.Fatalf("--phase DELIVERED_ON_BRANCH: %v", got)
	}
	if got := ids("--phase", "NONE"); len(got) != 0 {
		t.Fatalf("--phase NONE after a write: %v", got)
	}
	if got := ids("--phase", "IN_REVIEW"); len(got) != 0 {
		t.Fatalf("--phase IN_REVIEW: %v", got)
	}
	s := atm(t, r.Root, nil, "ticket", "search", "--phase", "DELIVERED_ON_BRANCH")
	if p := field(s.res.Items[0], "progress"); field(p, "head").Str != head {
		t.Fatalf("search row progress: %s", wire.Encode(s.res.Items[0]))
	}
	rm := atm(t, r.Root, nil, "roadmap")
	if rm.res.Outcome != wire.OutcomeOK || field(field(rm.res.Items[0], "progress"), "phase").Str != "DELIVERED_ON_BRANCH" {
		t.Fatalf("roadmap row: %+v", rm.res)
	}
	if bad := atm(t, r.Root, nil, "ticket", "search", "--phase", "CLEARED"); bad.res.Outcome == wire.OutcomeOK {
		t.Fatal("--phase CLEARED was accepted")
	}
	x = atm(t, r.Root, nil, "ticket", "progress", "--target", id, "--expected-revision", "2",
		"--request-id", "p2", "--issued-at", "2026-10-31T12:05:00Z", "--payload", `{"branch":null,"head":null,"host":null,"lane":null,"phase":"CLEARED","pr":null,"reason":"superseded"}`)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("clear: %+v", x.res)
	}
	if got := ids("--phase", "NONE"); len(got) != 1 {
		t.Fatalf("--phase NONE after CLEARED: %v", got)
	}
	help := atm(t, r.Root, nil, "ticket", "progress", "--help", "--verbose")
	for _, want := range []string{"DELIVERED_ON_BRANCH", "OPERATOR/ADMISSION", "Example:"} {
		if !strings.Contains(string(help.stdout), want) {
			t.Errorf("ticket progress help lacks %q", want)
		}
	}
}

// TestTLFV0883_CloseoutDebtReportsMergedProgressHead: an OPEN ticket whose
// effective progress names a scanned commit is a progress-head candidate
// although no commit message cites it; a head outside the scan, or a
// cleared marker, reports nothing.
func TestTLFV0883_CloseoutDebtReportsMergedProgressHead(t *testing.T) {
	setActor(t, "tester")
	root, _ := ergStore(t)
	merged, _ := tcdTicket(t, root, "merged", "")
	unmerged, _ := tcdTicket(t, root, "unmerged", "")
	tcdCommit(t, root, "lane work with no ticket citation")
	raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(raw))
	mark := func(id, req, rev, payload string) {
		t.Helper()
		x := atm(t, root, nil, "ticket", "progress", "--target", id, "--expected-revision", rev,
			"--request-id", req, "--issued-at", "2026-10-31T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("progress %s: %s", id, x.stdout)
		}
	}
	mark(merged, "p-merged", "1", progressPayload("MERGED_AWAITING_CLOSEOUT", "", head, "", ""))
	mark(unmerged, "p-unmerged", "1", progressPayload("MERGED_AWAITING_CLOSEOUT", "", strings.Repeat("c", 40), "", ""))
	code, res := tcdRun(t, root)
	_, rows := tcdRows(t, res)
	if code != 1 || len(rows) != 1 || rows[0].ticket != merged || rows[0].citation != "progress-head" ||
		rows[0].subject != "lane work with no ticket citation" {
		t.Fatalf("closeout-debt rows (exit %d): %+v", code, rows)
	}
	mark(merged, "p-clear", "2", `{"branch":null,"head":null,"host":null,"lane":null,"phase":"CLEARED","pr":null,"reason":null}`)
	if code, res = tcdRun(t, root); code != 0 {
		_, rows = tcdRows(t, res)
		t.Fatalf("cleared marker still reported (exit %d): %+v", code, rows)
	}
}

// TestTCDV02400_InProgressHeadIsExcludedNotDebt covers V1-4448: only a
// delivery-phase progress head cites. A lane's IN_PROGRESS dispatch base and
// a BLOCKED head reachable in the ref are excluded with a named reason and
// are no debt; a DELIVERED_ON_BRANCH head at a later merged commit is the
// only head candidate; a message citation of an IN_PROGRESS ticket keeps its
// row (negative control).
func TestTCDV02400_InProgressHeadIsExcludedNotDebt(t *testing.T) {
	setActor(t, "tester")
	root, _ := ergStore(t)
	parked, parkedLocal := tcdTicket(t, root, "parked", "")
	moved, _ := tcdTicket(t, root, "moved", "")
	blocked, blockedLocal := tcdTicket(t, root, "blocked", "")
	tcdCommit(t, root, "dispatch base with no ticket citation")
	base := revParse(t, root, "HEAD")
	mark := func(id, req, rev, payload string) {
		t.Helper()
		x := atm(t, root, nil, "ticket", "progress", "--target", id, "--expected-revision", rev,
			"--request-id", req, "--issued-at", "2026-11-03T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("progress %s: %s", id, x.stdout)
		}
	}
	mark(parked, "p-parked", "1", progressPayload("IN_PROGRESS", "lane/x", base, "", ""))
	mark(moved, "p-moved", "1", progressPayload("IN_PROGRESS", "lane/x", base, "", ""))
	mark(blocked, "p-blocked", "1", progressPayload("BLOCKED", "lane/x", base, "", "waits on the owner"))

	// Only IN_PROGRESS and BLOCKED heads at the ref commit: no debt, each
	// excluded with its reason and the head as its commit.
	code, res := tcdRun(t, root)
	item, rows := tcdRows(t, res)
	if code != 0 || len(rows) != 0 {
		t.Fatalf("IN_PROGRESS and BLOCKED heads are debt (exit %d): %+v", code, rows)
	}
	got := map[string]string{}
	for _, e := range field(item, "excludedCandidates").Arr {
		var commits []string
		for _, c := range field(e, "commits").Arr {
			commits = append(commits, c.Str)
		}
		got[field(e, "ticketId").Str] = field(e, "reason").Str + " " + strings.Join(commits, ",")
	}
	want := map[string]string{parked: "PROGRESS_IN_PROGRESS " + base, moved: "PROGRESS_IN_PROGRESS " + base, blocked: "PROGRESS_BLOCKED " + base}
	if len(got) != len(want) || got[parked] != want[parked] || got[moved] != want[moved] || got[blocked] != want[blocked] {
		t.Fatalf("excludedCandidates %v, want %v", got, want)
	}
	human := wire.Encode(field(item, "human"))
	for _, line := range []string{parkedLocal + " excluded PROGRESS_IN_PROGRESS via 1 commit(s)", blockedLocal + " excluded PROGRESS_BLOCKED via 1 commit(s)"} {
		if !strings.Contains(string(human), line) {
			t.Fatalf("human lacks %q: %s", line, human)
		}
	}

	// The lane merges later and marks DELIVERED_ON_BRANCH at its head: that
	// head is the one candidate; the parked dispatch base stays excluded.
	git(t, root, "checkout", "-q", "-b", "lane")
	tcdCommit(t, root, "lane work with no ticket citation")
	laneHead := revParse(t, root, "HEAD")
	git(t, root, "checkout", "-q", "main")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Merge lane", "lane")
	mark(moved, "p-moved-delivered", "2", progressPayload("DELIVERED_ON_BRANCH", "lane/x", laneHead, "", ""))
	code, res = tcdRun(t, root)
	item, rows = tcdRows(t, res)
	if code != 1 || len(rows) != 1 || rows[0].ticket != moved || rows[0].citation != "progress-head" || rows[0].subject != "lane work with no ticket citation" {
		t.Fatalf("closeout-debt rows (exit %d): %+v", code, rows)
	}
	if c := field(item, "candidates").Arr[0]; field(c, "commit").Str != laneHead {
		t.Fatalf("the candidate cites %s, want the delivered head %s", field(c, "commit").Str, laneHead)
	}
	if ex := wire.Encode(field(item, "excludedCandidates")); strings.Contains(string(ex), moved) || !strings.Contains(string(ex), parked) {
		t.Fatalf("excludedCandidates after delivery: %s", ex)
	}

	// Negative control: a message citation of the IN_PROGRESS ticket is
	// still a candidate, and the ticket is then not excluded.
	tcdCommit(t, root, "fix: finish the parked work ("+parkedLocal+")")
	code, res = tcdRun(t, root)
	item, rows = tcdRows(t, res)
	cited := false
	for _, r := range rows {
		if r.ticket == parked {
			cited = cited || r.citation == "direct-id"
			if r.citation == "progress-head" {
				t.Fatalf("the IN_PROGRESS head cited: %+v", rows)
			}
		}
	}
	if code != 1 || !cited || strings.Contains(string(wire.Encode(field(item, "excludedCandidates"))), parked) {
		t.Fatalf("message-cited IN_PROGRESS ticket (exit %d): %+v excluded %s", code, rows, wire.Encode(field(item, "excludedCandidates")))
	}
}

// TestTCDV02400_MixedExclusionCausesStayVisible: a ticket excluded both by a
// build-log-only citation and by an IN_PROGRESS head keeps each cause with
// its own commit in causes, and the human line names both; a single-cause
// ticket has one cause.
func TestTCDV02400_MixedExclusionCausesStayVisible(t *testing.T) {
	setActor(t, "tester")
	root, _ := ergStore(t)
	mixed, mixedLocal := tcdTicket(t, root, "mixed", "")
	single, _ := tcdTicket(t, root, "single", "")
	tcdCommit(t, root, "dispatch base with no ticket citation")
	base := revParse(t, root, "HEAD")
	for _, id := range []string{mixed, single} {
		x := atm(t, root, nil, "ticket", "progress", "--target", id, "--expected-revision", "1",
			"--request-id", "p-"+id, "--issued-at", "2026-11-03T12:00:00Z", "--payload", progressPayload("IN_PROGRESS", "lane/x", base, "", ""))
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("progress %s: %s", id, x.stdout)
		}
	}
	logPath := filepath.Join(root, "docs", "build-log", "2026-11-03-lane.md")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("# lane\n\n- "+mixedLocal+" reviewed.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "--", "docs/build-log/2026-11-03-lane.md")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "docs(build-log): review notes "+mixedLocal)
	logCommit := revParse(t, root, "HEAD")

	code, res := tcdRun(t, root)
	item, rows := tcdRows(t, res)
	if code != 0 || len(rows) != 0 {
		t.Fatalf("excluded tickets are debt (exit %d): %+v", code, rows)
	}
	got := map[string]string{}
	for _, e := range field(item, "excludedCandidates").Arr {
		var causes []string
		for _, c := range field(e, "causes").Arr {
			var commits []string
			for _, x := range field(c, "commits").Arr {
				commits = append(commits, x.Str)
			}
			causes = append(causes, field(c, "reason").Str+"="+strings.Join(commits, ","))
		}
		got[field(e, "ticketId").Str] = field(e, "reason").Str + " " + strings.Join(causes, " ")
	}
	want := map[string]string{
		mixed:  "BUILD_LOG_ONLY BUILD_LOG_ONLY=" + logCommit + " PROGRESS_IN_PROGRESS=" + base,
		single: "PROGRESS_IN_PROGRESS PROGRESS_IN_PROGRESS=" + base,
	}
	if len(got) != len(want) || got[mixed] != want[mixed] || got[single] != want[single] {
		t.Fatalf("excludedCandidates %v, want %v", got, want)
	}
	line := mixedLocal + " excluded BUILD_LOG_ONLY via 2 commit(s) (causes BUILD_LOG_ONLY, PROGRESS_IN_PROGRESS)"
	if human := wire.Encode(field(item, "human")); !strings.Contains(string(human), line) {
		t.Fatalf("human lacks %q: %s", line, human)
	}
}
