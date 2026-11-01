// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os/exec"
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
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("ATM_ACTOR", "tester")
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
