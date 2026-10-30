package cli_test

import (
	"bytes"
	"os"
	"os/exec"
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

// tdsCommit writes a commit object with HEAD's tree and the given parents
// without moving any ref, so a batch export can name real lane heads and a
// real merge commit (TCD-V0-179).
func tdsCommit(t *testing.T, root, msg string, parents ...string) string {
	t.Helper()
	args := []string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", "HEAD^{tree}", "-m", msg}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Fatalf("commit-tree %s: %v", msg, err)
	}
	return strings.TrimSpace(string(out))
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
// are DELIVERED, a "Not delivered" mention is only MENTIONED and a vetoed
// commit line only CITED (TCD-V0-167), an unreferenced ticket is CLEAR, and without a --prs export a
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
	want := map[string]string{inflight: "IN_FLIGHT", merged: "DELIVERED", committed: "DELIVERED", mentioned: "MENTIONED", vetoed: "CITED", clear: "MENTIONED"}
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
	_, e := tcdTicket(t, root, "echo", "")
	base := revParse(t, root, "HEAD")
	l20 := tdsCommit(t, root, "lane a", base)
	l21 := tdsCommit(t, root, "lane c", l20)
	deferred := tdsCommit(t, root, "deferred lane", base)
	m, h := tdsCommit(t, root, "batch merge", base, l21), strings.Repeat("b", 40)
	prs := tdsExport(t, "prs.json", `[
{"number":20,"state":"MERGED","title":"lane a","body":"`+tdsJSON("Refs "+a+"\n\nResidual: "+b+" blocked on owner")+`","headRefOid":"`+l20+`","labels":[]},
{"number":21,"state":"MERGED","title":"lane c","body":"`+tdsJSON("Closes #99\nRefs "+c)+`","headRefOid":"`+l21+`","labels":[]},
{"number":22,"state":"OPEN","title":"Batch next: #21","body":"","labels":[]},
{"number":23,"state":"OPEN","title":"lane pi","body":"`+tdsJSON("Refs "+e)+`","headRefOid":"`+deferred+`","labels":[]},
{"number":24,"state":"MERGED","title":"lane unexported","body":"`+tdsJSON("Refs "+d)+`","labels":[]},
{"number":25,"state":"MERGED","title":"Batch z: #20","body":"","headRefOid":"`+h+`","labels":[]},
{"number":30,"state":"MERGED","title":"Batch x: #20 #21","body":"`+tdsJSON("- #20: lane a ("+a+")\n- #21: lane c\n#23 (Pi) is deferred to the next batch\n#24 rides along")+`","headRefOid":"`+h+`","mergeCommit":{"oid":"`+m+`"},"labels":[]},
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
	// TCD-V0-179 (V1-2532): a deferred PR whose head is not in the merge's
	// ancestry, and a named PR whose head was not exported, are excluded
	// with their reason and deliver or mention nothing.
	var excluded []string
	for _, r := range field(item, "excludedPrs").Arr {
		excluded = append(excluded, field(r, "pr").Str+"/"+field(r, "reason").Str)
	}
	if strings.Join(excluded, " ") != "23/NOT_IN_MERGE_ANCESTRY 24/HEAD_NOT_OBSERVED" || strings.Contains(string(x.stdout), `"local":"`+e+`"`) ||
		!strings.Contains(strings.Join(x.res.Warnings, "\n"), "TCD-V0-179") {
		t.Fatalf("excluded PRs %v: %s", excluded, x.stdout)
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
	// TCD-V0-180 (V1-2532): a MERGED batch whose export lacks mergeCommit
	// names the missing field and the gh --json list, not "MERGED, not merged".
	if x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "25", "--prs", prs); x.code == 0 || !hasCode(x.res, wire.CodeMalformed) ||
		!strings.Contains(string(x.stdout), "no mergeCommit") || !strings.Contains(string(x.stdout), "number,state,title,body,headRefName,headRefOid,mergeCommit,labels,mergedAt") ||
		strings.Contains(string(x.stdout), "not merged") {
		t.Fatalf("missing mergeCommit: %s", x.stdout)
	}
}

// TestTCDV0167_DeliveryScreenCitedAndTruncated covers V1-2303: a commit that
// only mentions a ticket in prose is CITED, not DELIVERED, while a closing
// trailer, a subject led by the ID, a conventional-commit scope and a final
// subject parenthetical deliver (TCD-V0-167); a ticket created before the
// oldest commit of a scan cut at --limit is TRUNCATED rather than CLEAR or
// NOT_OBSERVED, and one created after it keeps its verdict (TCD-V0-168).
func TestTCDV0167_DeliveryScreenCitedAndTruncated(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, prose := tcdTicket(t, root, "prose", "")
	_, trailer := tcdTicket(t, root, "trailer", "")
	_, first := tcdTicket(t, root, "first", "")
	_, scope := tcdTicket(t, root, "scope", "")
	_, tail := tcdTicket(t, root, "tail", "")
	_, old := tcdTicket(t, root, "old", "")
	tcdCommit(t, root, "fix: old work ("+old+")")
	tcdCommit(t, root, "docs: release notes\n\nThe release still waits on "+prose+" and the RC.")
	tcdCommit(t, root, "fix: lease renewal\n\nRefs: "+trailer)
	tcdCommit(t, root, first+": delivery screen verdicts")
	tcdCommit(t, root, "feat("+scope+"): scope form")
	tcdCommit(t, root, "fix: subject tail ("+tail+", V1-0000).")
	prs := tdsExport(t, "prs.json", "[]")

	x := tdsRun(t, root, "ticket", "delivery-screen", prose, trailer, first, scope, tail, old, "--prs", prs)
	want := map[string]string{prose: "CITED", trailer: "DELIVERED", first: "DELIVERED", scope: "DELIVERED", tail: "DELIVERED", old: "DELIVERED"}
	if got := tdsVerdicts(t, x); x.code != 1 || !mapsEqual(got, want) {
		t.Fatalf("exit %d verdicts %v, want %v", x.code, got, want)
	}

	// --limit 1 reads only the newest commit, made after every ticket's
	// creation, so the undelivered tickets are TRUNCATED.
	x = tdsRun(t, root, "ticket", "delivery-screen", old, prose, tail, "--prs", prs, "--limit", "1")
	want = map[string]string{old: "TRUNCATED", prose: "TRUNCATED", tail: "DELIVERED"}
	if got := tdsVerdicts(t, x); x.code != 1 || !mapsEqual(got, want) || !field(x.res.Items[0], "scanTruncated").Bool {
		t.Fatalf("truncated: exit %d verdicts %v, want %v", x.code, got, want)
	}
	// A newest commit dated before the tickets were created (2026-10-17)
	// means the window reaches back past their creation: not TRUNCATED.
	c := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "-q", "-m", "chore: backdated")
	c.Dir, c.Env = root, append(os.Environ(), "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z", "GIT_AUTHOR_DATE=2026-10-01T00:00:00Z")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("backdated commit: %v: %s", err, out)
	}
	x = tdsRun(t, root, "ticket", "delivery-screen", old, "--prs", prs, "--limit", "1")
	if got := tdsVerdicts(t, x); x.code != 0 || got[old] != "CLEAR" {
		t.Fatalf("window covers creation: exit %d verdicts %v", x.code, got)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
