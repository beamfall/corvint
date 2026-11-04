// SPDX-License-Identifier: Apache-2.0
package cli_test

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"strings"
	"testing"
)

// TCL-V0-7404: two lane heads supply their own candidate without overrides.
func TestTCLV07404MultiLaneHeads(t *testing.T) {
	root, _, laneA, merge := gbRepo(t)
	laneB := revParse(t, root, "lane-b")
	a, _ := effectsTicket(t, root, "lanea", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["src/"]}`)
	b, _ := effectsTicket(t, root, "laneb", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/"]}`)
	local := func(id string) string { return id[strings.LastIndex(id, ":")+1:] }
	prs := tdsExport(t, "prs.json", `[{"number":40,"state":"MERGED","title":"a","body":"Refs `+local(a)+`","headRefOid":"`+laneA+`","labels":[]},{"number":41,"state":"MERGED","title":"b","body":"Refs `+local(b)+`","headRefOid":"`+laneB+`","labels":[]},{"number":50,"state":"MERGED","title":"Batch #40 #41","body":"- #40\n- #41","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-11-01T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`)
	x := atm(t, root, nil, "ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs, "--dry-run")
	rows := gbRows(t, x)
	for id, head := range map[string]string{local(a): laneA, local(b): laneB} {
		row := rows[id]
		if field(row, "disposition").Str != "CLOSE" || field(row, "candidateCommit").Str != head || field(row, "sealCommit").Str != head {
			t.Fatalf("%s: %s", id, wire.Encode(row))
		}
	}
}

// TCL-V0-7404: a retry precheck and its actual claim retain admitted widen
// paths from the previously submitted generation, instead of rejecting them.
func TestTCLV07404BatchRestoresSubmittedWiden(t *testing.T) {
	root, base, laneA, merge := bigBatchRepo(t)
	id, _ := effectsTicket(t, root, "restored", "P1", `{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["docs/"]}`)
	local := id[strings.LastIndex(id, ":")+1:]
	x := atm(t, root, nil, "claim", id, "--holder", "retry", "--base", base, "--request-id", "prior-claim")
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatal(string(x.stdout))
	}
	attempt, generation := field(x.res.Items[0], "attemptId").Str, field(x.res.Items[0], "generation").Str
	for _, args := range [][]string{
		{"widen", "--attempt", attempt, "--generation", generation, "--scope", "src/", "--request-id", "prior-widen"},
		{"submit", "--attempt", attempt, "--generation", generation, "--tree", revParse(t, root, laneA+"^{tree}"), "--request-id", "prior-submit"},
		{"release", "--attempt", attempt, "--generation", generation, "--request-id", "prior-release"},
	} {
		x = atm(t, root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
	prs := tdsExport(t, "prs.json", `[{"number":40,"state":"MERGED","title":"lane a","body":"Refs `+local+`","headRefOid":"`+laneA+`","labels":[]},{"number":50,"state":"MERGED","title":"Batch #40","body":"- #40","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-11-01T05:00:00Z","url":"https://github.com/beamfall/corvint/pull/50","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[{"databaseId":7,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/7"}]`)
	args := []string{"ticket", "batch-close", "--pr", "50", "--prs", prs, "--runs", runs}
	dry := atm(t, root, nil, append(append([]string{}, args...), "--dry-run")...)
	row := gbRows(t, dry)[local]
	if field(row, "disposition").Str != "CLOSE" {
		t.Fatal(string(dry.stdout))
	}
	x = atm(t, root, nil, args...)
	if field(gbRows(t, x)[local], "disposition").Str != "CLOSED" {
		t.Fatal(string(x.stdout))
	}
}
