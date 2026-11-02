// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTLFV01101_GatedCloseOfProgressDelivery closes progress-delivered
// tickets through the gated lifecycle over a batch merge with a real diff
// (lane A adds src/a.go, lane B docs/b.md), so a broken gated close of a
// PROGRESS row fails here. It also pins the evidence rules (TLF-V0-1101): a
// matching pr never overrides a head the batch did not merge, and a mark
// naming an excluded PR stays excluded even when its head is merged.
func TestTLFV01101_GatedCloseOfProgressDelivery(t *testing.T) {
	root, base, laneA, merge := gbRepo(t)
	undeclared := `{"coverage":"INCOMPLETE","externalUnbounded":false,"resources":[],"touchPaths":["elsewhere/"]}`
	byPRID, byPR := progressTicket(t, root, "bypr", undeclared)
	byHeadID, byHead := progressTicket(t, root, "byhead", undeclared)
	staleHeadID, staleHead := progressTicket(t, root, "stalehead", undeclared)
	excludedID, excluded := progressTicket(t, root, "excluded", undeclared)
	deferred := tdsCommit(t, root, "deferred lane", base)
	mark := func(id, payload string) {
		t.Helper()
		x := atm(t, root, nil, "ticket", "progress", "--target", id, "--expected-revision", "current",
			"--request-id", "p-"+id[strings.LastIndex(id, ":")+1:], "--issued-at", "2026-11-01T12:00:00Z", "--payload", payload)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("progress %s: %s", id, x.stdout)
		}
	}
	mark(byPRID, progressPayload("IN_REVIEW", "", "", "40", ""))
	mark(byHeadID, progressPayload("MERGED_AWAITING_CLOSEOUT", "", laneA, "", ""))
	// Finding 1: the batch PR matches, but the head is the batch's first
	// parent, which the batch did not merge.
	mark(staleHeadID, progressPayload("MERGED_AWAITING_CLOSEOUT", "", base, "50", ""))
	// Finding 2: the PR is excluded (its head is not in the merge's
	// ancestry), though the mark's head is a merged lane commit.
	mark(excludedID, progressPayload("MERGED_AWAITING_CLOSEOUT", "", laneA, "45", ""))
	prs := tdsExport(t, "prs.json", `[
{"number":40,"state":"MERGED","title":"lane a","body":"","headRefOid":"`+laneA+`","labels":[]},
{"number":45,"state":"OPEN","title":"lane deferred","body":"","headRefOid":"`+deferred+`","labels":[]},
{"number":50,"state":"MERGED","title":"Batch z: #40 #45","body":"- #40\n- #45 deferred","headRefOid":"`+laneA+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-10-28T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`)

	d := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "50", "--prs", prs, "--runs", runs)
	got := map[string]string{}
	for _, r := range field(d.res.Items[0], "delivered").Arr {
		got[field(r, "local").Str] = field(r, "source").Str + "@" + field(r, "lanePr").Str
	}
	if !sameRows(got, map[string]string{byPR: "PROGRESS@40", byHead: "PROGRESS@"}) {
		t.Fatalf("delivered %v: %s", got, d.stdout)
	}
	if w := strings.Join(d.res.Warnings, " "); !strings.Contains(w, staleHead) || !strings.Contains(w, "not merged by batch PR #50") {
		t.Fatalf("no warning names the contradicted mark: %s", d.stdout)
	}

	x := atm(t, root, nil, "ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs)
	rows := gbRows(t, x)
	if got := gbDispositions(rows); x.code != 0 || !sameRows(got, map[string]string{byPR: "CLOSED", byHead: "CLOSED"}) {
		t.Fatalf("gated close %v: exit %d %s", got, x.code, x.stdout)
	}
	for _, r := range []wire.Value{rows[byPR], rows[byHead]} {
		if field(r, "source").Str != "PROGRESS" || field(r, "completionKind").Str != "VERIFIED" || field(field(r, "claimScope"), "changedPaths").Str != "2" {
			t.Fatalf("gated progress row: %s", wire.Encode(r))
		}
	}
	for _, id := range []string{byPRID, byHeadID} {
		if rec := field(bcShow(t, root, id), "record"); field(field(rec, "completion"), "kind").Str != "VERIFIED" {
			t.Fatalf("%s not VERIFIED: %s", id, wire.Encode(rec))
		}
	}
	for _, id := range []string{staleHeadID, excludedID} {
		if st := field(bcShow(t, root, id), "status").Str; st != "OPEN" {
			t.Fatalf("%s (%s) is %s", id, excluded, st)
		}
	}
}

// progressTicket creates a ticket with the given effects and returns its id
// and local token.
func progressTicket(t *testing.T, root, title, effects string) (string, string) {
	t.Helper()
	id, _ := effectsTicket(t, root, title, "P1", effects)
	return id, id[strings.LastIndex(id, ":")+1:]
}
