package cli_test

import (
	"bytes"
	"errors"
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

// reconcileRepo builds a store whose main branch merges a direct PR #35 the
// way beamfall/corvint#35 did: the merge message names the PR title, whose
// final parenthetical names a and b, and the PR body lists them as
// `- **ID**:` lines, delivers c on a Delivers line, names d only in prose
// and h on a vetoed line. A scoped commit delivers e and an umbrella commit
// mentions f and g. It returns the root, the locals and the PR export path.
func reconcileRepo(t *testing.T) (root string, l map[string]string, prs, runs string) {
	t.Helper()
	root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	l = map[string]string{}
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		_, l[n] = tcdTicket(t, root, "rc"+n, "")
	}
	git(t, root, "checkout", "-q", "-b", "side")
	tcdCommit(t, root, "lane work")
	git(t, root, "checkout", "-q", "main")
	title := "Tasks: OPEN reads skip archived tickets (" + l["a"] + ", " + l["b"] + ")"
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "Merge pull request #35 from x/side\n\n"+title, "side")
	merge := revParse(t, root, "HEAD")
	tcdCommit(t, root, "fix: scoped work ("+l["e"]+")")
	tcdCommit(t, root, "docs: mention "+l["f"]+" and "+l["g"])
	body := "Archive-speed lane.\n\n- **" + l["a"] + "**: OPEN reads skip archived tickets.\n- **" + l["b"] + "**: fixture genesis dirs.\n- **" + l["h"] + "**: blocked on owner.\nDelivers " + l["c"] + "\n\nFollow-up friction: " + l["d"] + "."
	prs = tdsExport(t, "prs.json", `[
{"number":35,"state":"MERGED","title":"`+tdsJSON(title)+`","body":"`+tdsJSON(body)+`","headRefOid":"`+revParse(t, root, "side")+`","mergeCommit":{"oid":"`+merge+`"},"mergedAt":"2026-10-31T05:00:00Z","labels":[]},
{"number":36,"state":"MERGED","title":"unscanned","body":"`+tdsJSON("Delivers "+l["g"])+`","mergeCommit":{"oid":"`+strings.Repeat("c", 40)+`"},"labels":[]}]`)
	runs = tdsExport(t, "runs.json", `[{"databaseId":5,"headSha":"`+merge+`","conclusion":"success","url":"https://github.com/beamfall/corvint/actions/runs/5"}]`)
	return root, l, prs, runs
}

// TestTCDV0870_DirectPRDeliversItsOwnTickets covers V1-3496: batch-delivered
// on a direct PR (no lane PRs) lists the tickets its title parenthetical,
// `- **ID**:` lines and Delivers line declare as delivered; a prose mention
// and a vetoed line stay mentioned (TCD-V0-870).
func TestTCDV0870_DirectPRDeliversItsOwnTickets(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, l, prs, runs := reconcileRepo(t)
	x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "35", "--prs", prs, "--runs", runs)
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("batch-delivered: exit %d %s", x.code, x.stdout)
	}
	item := x.res.Items[0]
	var delivered, mentioned []string
	for _, r := range field(item, "delivered").Arr {
		delivered = append(delivered, field(r, "local").Str)
	}
	for _, r := range field(item, "mentioned").Arr {
		mentioned = append(mentioned, field(r, "local").Str)
	}
	if got, want := strings.Join(delivered, " "), strings.Join(sortedLocals(l["a"], l["b"], l["c"]), " "); got != want {
		t.Fatalf("delivered %q, want %q: %s", got, want, x.stdout)
	}
	if got, want := strings.Join(mentioned, " "), strings.Join(sortedLocals(l["d"], l["h"]), " "); got != want {
		t.Fatalf("mentioned %q, want %q: %s", got, want, x.stdout)
	}
}

// TestTCDV0871_CloseoutDebtProposesBatchClose covers V1-3496 acceptance 1-3:
// with --prs, closeout-debt proposes BATCH_CLOSE for the merged PR whose
// merge commit cites a and b (and lists c, which only its body delivers),
// COMMIT_DELIVERED for a scoped direct commit, and REVIEW for an umbrella
// mention; it writes nothing, and without --prs its bytes are unchanged
// (TCD-V0-871). The proposed batch-close then completes the PR's tickets
// and a rerun reports no candidate or proposal for them (TCD-V0-873).
func TestTCDV0871_CloseoutDebtProposesBatchClose(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, l, prs, runs := reconcileRepo(t)
	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)

	_, plain := tcdRun(t, root)
	code, res := tcdRun(t, root, "--prs", prs)
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("closeout-debt --prs changed the intent or state tree")
	}
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	item, rows := tcdRows(t, res)
	if _, has := plain.Items[0].Obj.Get("proposals"); has {
		t.Fatalf("closeout-debt without --prs carries proposals: %+v", plain.Items[0])
	}
	if len(rows) == 0 {
		t.Fatal("no candidate rows")
	}
	proposal := map[string]string{}
	for i, c := range field(item, "candidates").Arr {
		p := field(c, "proposal").Str
		if p == "BATCH_CLOSE" {
			p += "#" + field(c, "pr").Str
		} else if field(c, "pr").Kind != wire.KindNull {
			t.Fatalf("row %d has a pr without BATCH_CLOSE: %+v", i, c)
		}
		proposal[field(c, "local").Str] = p
	}
	want := map[string]string{l["a"]: "BATCH_CLOSE#35", l["b"]: "BATCH_CLOSE#35", l["e"]: "COMMIT_DELIVERED", l["f"]: "REVIEW", l["g"]: "REVIEW"}
	if len(proposal) != len(want) {
		t.Fatalf("proposals %v, want %v", proposal, want)
	}
	for k, v := range want {
		if proposal[k] != v {
			t.Fatalf("proposals %v, want %v", proposal, want)
		}
	}
	ps := field(item, "proposals").Arr
	if len(ps) != 1 || field(ps[0], "pr").Str != "35" || field(ps[0], "mergeCommit").Str != revParse(t, root, "HEAD~2") {
		t.Fatalf("proposals: %+v", ps)
	}
	var tickets []string
	for _, v := range field(ps[0], "tickets").Arr {
		tickets = append(tickets, v.Str)
	}
	if strings.Join(tickets, " ") != strings.Join(sortedLocals(l["a"], l["b"], l["c"]), " ") {
		t.Fatalf("proposal tickets %v", tickets)
	}
	command := field(ps[0], "command").Str
	if command != "corvint-tasks ticket batch-close --pr 35 --prs "+prs+" --runs RUNS" {
		t.Fatalf("command %q", command)
	}
	var reviews []string
	for _, v := range field(item, "reviewTickets").Arr {
		reviews = append(reviews, v.Str)
	}
	if strings.Join(reviews, " ") != strings.Join(sortedLocals(l["e"], l["f"], l["g"]), " ") {
		t.Fatalf("reviewTickets %v", reviews)
	}
	if _, again := tcdRun(t, root, "--prs", prs); !bytes.Equal(wire.EncodeFile(again.Items[0]), wire.EncodeFile(res.Items[0])) {
		t.Fatal("closeout-debt --prs output is not deterministic")
	}
	if c, bad := tcdRun(t, root, "--prs", tdsExport(t, "bad.json", "{")); c != 2 || !hasCode(bad, wire.CodeMalformed) {
		t.Fatalf("undecodable export: exit %d %+v", c, bad)
	}

	// TCD-V0-873: run the proposed close; nothing the PR delivered stays OPEN.
	x := atm(t, root, nil, strings.Fields(strings.Replace(strings.TrimPrefix(command, "corvint-tasks "), "RUNS", runs, 1)+" --manual")...)
	if x.code != 0 || x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("proposed batch-close: exit %d %s", x.code, x.stdout)
	}
	_, after := tcdRun(t, root, "--prs", prs)
	afterItem, afterRows := tcdRows(t, after)
	for _, r := range afterRows {
		for _, n := range []string{"a", "b", "c"} {
			if strings.HasSuffix(r.ticket, ":"+l[n]) {
				t.Fatalf("%s still a candidate after the reconcile close: %+v", l[n], afterRows)
			}
		}
	}
	if len(field(afterItem, "proposals").Arr) != 0 {
		t.Fatalf("proposals after the close: %+v", field(afterItem, "proposals"))
	}
	for _, n := range []string{"a", "b", "c"} {
		if st := field(bcShow(t, root, l[n]), "status").Str; st != "COMPLETED" {
			t.Fatalf("%s is %s after the close", l[n], st)
		}
	}
}

const reconcileScript = "../../../script/tasks-reconcile.sh"

// TestTCDV0872_ReconcileScriptRetainsReportAndExitStatus covers V1-3496
// acceptance 1: make tasks-reconcile's script, with gh stubbed to print the
// exports, retains the exports and report under
// <git-common-dir>/corvint/reconcile/<refCommit>/, prints the proposal, exits
// 1 on debt and writes nothing by default; RECONCILE_APPLY=dry-run runs the
// proposal's batch-close as a dry run; a failing export or a missing binary
// exits 2 and is never a silent skip (TCD-V0-872).
func TestTCDV0872_ReconcileScriptRetainsReportAndExitStatus(t *testing.T) {
	script, err := filepath.Abs(reconcileScript)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_TASKS", cli.CompiledTasksBinary(t))
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	t.Setenv("RECONCILE_REF", "main")
	root, l, prs, runs := reconcileRepo(t)
	stub := filepath.Join(t.TempDir(), "gh")
	fixture.Write(t, stub, []byte("#!/bin/sh\ncase \"$1\" in pr) cat '"+prs+"' ;; run) cat '"+runs+"' ;; *) exit 9 ;; esac\n"))
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	reconcile := func(env ...string) (int, string) {
		t.Helper()
		c := exec.Command(script)
		c.Dir = root
		c.Env = append(os.Environ(), env...)
		out, err := c.CombinedOutput()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Fatalf("reconcile: %v %s", err, out)
		}
		return c.ProcessState.ExitCode(), string(out)
	}
	intentDir, stateDir := filepath.Join(root, ".taskman"), filepath.Join(root, ".git", "taskman")
	beforeIntent, beforeState := fixture.TreeSnapshot(t, intentDir), fixture.TreeSnapshot(t, stateDir)
	head := revParse(t, root, "main")
	dir := filepath.Join(root, ".git", "corvint", "reconcile", head)

	code, out := reconcile("GH=" + stub)
	if code != 1 || !strings.Contains(out, "proposal BATCH_CLOSE #35: ") || !strings.Contains(out, "batch-close --pr 35 --prs "+filepath.Join(dir, "prs.json")) {
		t.Fatalf("default reconcile: exit %d %s", code, out)
	}
	for _, f := range []string{"prs.json", "runs.json", "report.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("retained %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "apply-35.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the default reconcile applied a proposal: %v", err)
	}
	code, out = reconcile("GH="+stub, "RECONCILE_APPLY=dry-run")
	if code != 1 || !strings.Contains(out, "apply: ") {
		t.Fatalf("dry-run reconcile: exit %d %s", code, out)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "apply-35.json"))
	if err != nil || !strings.Contains(string(raw), `"dryRun":true`) || !strings.Contains(string(raw), l["a"]) {
		t.Fatalf("dry-run apply output: %v %s", err, raw)
	}
	if !fixture.SameTree(beforeIntent, fixture.TreeSnapshot(t, intentDir)) || !fixture.SameTree(beforeState, fixture.TreeSnapshot(t, stateDir)) {
		t.Fatal("the reconcile changed the intent or state tree")
	}
	if code, out = reconcile("GH=false"); code != 2 || !strings.Contains(out, "gh pr list failed") {
		t.Fatalf("failed export: exit %d %s", code, out)
	}
	if code, out = reconcile("GH="+stub, "CORVINT_TASKS=/nonexistent/corvint-tasks"); code != 2 || !strings.Contains(out, "corvint-tasks not found") {
		t.Fatalf("missing binary: exit %d %s", code, out)
	}
	if code, _ = reconcile("GH="+stub, "RECONCILE_APPLY=yes"); code != 2 {
		t.Fatalf("bad RECONCILE_APPLY: exit %d", code)
	}
}

func sortedLocals(ls ...string) []string {
	out := append([]string(nil), ls...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
