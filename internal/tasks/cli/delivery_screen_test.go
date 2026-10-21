package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// tdsExport writes a gh --json export file and returns its path.
func tdsExport(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// tdsRun runs a verb that exits 1 on an OK result (atm refuses that).
func tdsRun(t *testing.T, root string, args ...string) run {
	t.Helper()
	var out bytes.Buffer
	code := cli.Run(cli.Env{Cwd: root, Args: args, Stdout: &out})
	res, err := wire.DecodeResult(out.Bytes())
	if err != nil {
		t.Fatalf("envelope does not decode: %v\n%s", err, out.Bytes())
	}
	return run{code: code, stdout: out.Bytes(), res: res}
}

func tdsJSON(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\n", `\n`), `"`, `\"`)
}

// tdsVerdicts maps local ticket token to verdict.
func tdsVerdicts(t *testing.T, x run) map[string]string {
	t.Helper()
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || !x.res.Untrusted {
		t.Fatalf("delivery-screen: %s", x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "profile").Str != "taskman-delivery-screen/0" || field(item, "mutationAuthority").Bool {
		t.Fatalf("profile or authority: %s", x.stdout)
	}
	out := map[string]string{}
	for _, r := range field(item, "tickets").Arr {
		out[field(r, "local").Str] = field(r, "verdict").Str
	}
	return out
}

// TestTCDV0159_DeliveryScreenVerdicts covers V1-2251 acceptance 1 and 3: an
// open PR's Refs line is IN_FLIGHT, a merged PR's Refs line and a ref commit
// are DELIVERED, a "Not delivered" mention and a vetoed commit line are only
// MENTIONED, an unreferenced ticket is CLEAR, and without a --prs export a
// ticket nothing delivers is NOT_OBSERVED, never CLEAR. The store and
// journal trees are unchanged (TCD-V0-159..162).
func TestTCDV0159_DeliveryScreenVerdicts(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, inflight := tcdTicket(t, root, "inflight", "")
	_, merged := tcdTicket(t, root, "merged", "")
	_, committed := tcdTicket(t, root, "committed", "")
	_, mentioned := tcdTicket(t, root, "mentioned", "")
	_, vetoed := tcdTicket(t, root, "vetoed", "")
	_, clear := tcdTicket(t, root, "clear", "")
	tcdCommit(t, root, "fix: committed work ("+committed+")")
	tcdCommit(t, root, "chore: file residual follow-up for "+vetoed)
	prs := tdsExport(t, "prs.json", `[
{"number":10,"state":"OPEN","title":"lane","body":"`+tdsJSON("Refs "+inflight+"\n")+`","labels":[]},
{"number":11,"state":"MERGED","title":"lane two","body":"`+tdsJSON("Closes #4\nRefs "+merged+"\n\nNot delivered:\n- "+mentioned+" skipped (needs owner)\n- Refs "+clear+" already delivered")+`","labels":[]},
{"number":12,"state":"CLOSED","title":"abandoned","body":"`+tdsJSON("Refs "+mentioned)+`","labels":[]}]`)

	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	x := tdsRun(t, root, "ticket", "delivery-screen", inflight, merged, committed, mentioned, vetoed, clear, "--prs", prs)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("delivery-screen changed the intent or state tree")
	}
	if x.code != 1 {
		t.Fatalf("exit %d, want 1: %s", x.code, x.stdout)
	}
	want := map[string]string{inflight: "IN_FLIGHT", merged: "DELIVERED", committed: "DELIVERED", mentioned: "MENTIONED", vetoed: "MENTIONED", clear: "MENTIONED"}
	got := tdsVerdicts(t, x)
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("verdicts %v, want %v", got, want)
		}
	}

	_, fresh := tcdTicket(t, root, "fresh", "")
	if x := tdsRun(t, root, "ticket", "delivery-screen", fresh, "--prs", prs); x.code != 0 || tdsVerdicts(t, x)[fresh] != "CLEAR" {
		t.Fatalf("unreferenced ticket with an export: exit %d %s", x.code, x.stdout)
	}
	x = tdsRun(t, root, "ticket", "delivery-screen", fresh, committed)
	got = tdsVerdicts(t, x)
	if x.code != 1 || got[fresh] != "NOT_OBSERVED" || got[committed] != "DELIVERED" || field(x.res.Items[0], "github").Str != "NOT_OBSERVED" {
		t.Fatalf("no export: exit %d %v", x.code, got)
	}
	if x := tdsRun(t, root, "ticket", "delivery-screen", "--prs", prs); x.code != 2 || !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("no ticket: exit %d %s", x.code, x.stdout)
	}
	if x := tdsRun(t, root, "ticket", "delivery-screen", fresh, "--prs", tdsExport(t, "bad.json", "{")); x.code != 2 || !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("bad export: exit %d %s", x.code, x.stdout)
	}
}

// TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns covers V1-2254
// acceptance 1 (listing part) and 3: the tickets the batch's lane PRs
// deliver, each with lane PR, batch PR, merge commit and the successful runs
// at the merge commit or batch head; a mentioned ticket listed apart; an
// unnamed PR ignored; an unmerged batch refused (TCD-V0-163..164).
func TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, a := tcdTicket(t, root, "alpha", "")
	_, b := tcdTicket(t, root, "bravo", "")
	_, c := tcdTicket(t, root, "charlie", "")
	_, d := tcdTicket(t, root, "delta", "")
	m, h := strings.Repeat("a", 40), strings.Repeat("b", 40)
	prs := tdsExport(t, "prs.json", `[
{"number":20,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+a+"\n\nResidual: "+b+" blocked on owner")+`","labels":[]},
{"number":21,"state":"MERGED","title":"lane c","body":"`+tdsJSON("Closes #99\nRefs "+c)+`","labels":[]},
{"number":22,"state":"OPEN","title":"Batch next: #21","body":"","labels":[]},
{"number":30,"state":"MERGED","title":"Batch x: #20 #21","body":"`+tdsJSON("- #20: lane a ("+a+")\n- #21: lane c")+`","headRefOid":"`+h+`","mergeCommit":{"oid":"`+m+`"},"labels":[]},
{"number":40,"state":"MERGED","title":"other","body":"`+tdsJSON("Refs "+d)+`","labels":[]}]`)
	runs := tdsExport(t, "runs.json", `[
{"databaseId":5,"headSha":"`+m+`","conclusion":"success"},
{"databaseId":6,"headSha":"`+m+`","conclusion":"failure"},
{"databaseId":7,"headSha":"`+strings.Repeat("c", 40)+`","conclusion":"success"},
{"databaseId":8,"headSha":"`+h+`","conclusion":"success"}]`)

	x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "30", "--prs", prs, "--runs", runs)
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK || !x.res.Untrusted {
		t.Fatalf("batch-delivered: exit %d %s", x.code, x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "profile").Str != "taskman-batch-delivered/0" || field(item, "mutationAuthority").Bool || field(item, "mergeCommit").Str != m {
		t.Fatalf("header: %s", x.stdout)
	}
	strs := func(v wire.Value) string {
		var out []string
		for _, e := range v.Arr {
			out = append(out, e.Str)
		}
		return strings.Join(out, ",")
	}
	if strs(field(item, "lanePrs")) != "20,21" || strs(field(item, "greenRuns")) != "5,8" {
		t.Fatalf("lanes or runs: %s", x.stdout)
	}
	var rows []string
	for _, r := range field(item, "delivered").Arr {
		rows = append(rows, field(r, "local").Str+"@"+field(r, "lanePr").Str+"/"+field(r, "batchPr").Str+"/"+field(r, "status").Str+"/"+strs(field(r, "greenRuns")))
	}
	if strings.Join(rows, " ") != a+"@20/30/OPEN/5,8 "+c+"@21/30/OPEN/5,8" {
		t.Fatalf("delivered rows %v: %s", rows, x.stdout)
	}
	weak := field(item, "mentioned").Arr
	if len(weak) != 1 || field(weak[0], "local").Str != b || field(weak[0], "pr").Str != "20" {
		t.Fatalf("mentioned: %s", x.stdout)
	}

	x = atm(t, root, nil, "ticket", "batch-delivered", "--pr", "30", "--prs", prs)
	if x.code != 0 || field(x.res.Items[0], "greenRuns").Str != "NOT_OBSERVED" {
		t.Fatalf("no runs export: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "22", "--prs", prs); x.code == 0 || !hasCode(x.res, wire.CodePrerequisiteUnsatisfied) {
		t.Fatalf("open batch: %s", x.stdout)
	}
	if x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "77", "--prs", prs); x.code == 0 || !hasCode(x.res, wire.CodeMalformed) {
		t.Fatalf("absent batch: %s", x.stdout)
	}
}
