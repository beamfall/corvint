// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// skewReason is the refusal the main-built shadow printed on every shard of PR #163 (run
// 42616478225, V1-5216, V1-5219): its tested tree declared a .corvint/runtime-readers.json entry the
// trusted tool could not read, so `corvint test --dry-run` planned nothing and ran FULL.
const skewReason = "no plan: unsupported-affected-graph: .corvint/runtime-readers.json: unknown reader kind"

// fallbackShards rewrites every selection document of f to the no-plan FULL shape the shadow step
// writes from such a dry-run: scope FULL with the reason, the manifest digest, no plan or
// partition digest and no selected Go package.
func fallbackShards(f *fixture, reason string) {
	for _, d := range f.sel {
		d["scope"], d["reason"] = "FULL", reason
		d["planDigest"], d["partitionDigest"] = "", nil
		d["selected"] = []string{}
	}
}

// prRow classifies f as the pull-request row of its own run.
func prRow(t *testing.T, f *fixture) doc {
	t.Helper()
	row, _ := f.classify(t, map[string]string{"ROW_KIND": "pr", "ROW_SHA": "", "ROW_RUN": prRun, "ROW_OUTCOMES_RUN": prRun})
	return row
}

// CIN-V0-7921 (V1-5216, V1-5219): a pull-request run whose every shadow shard planned nothing
// (the trusted tool refused the tested tree's affected-graph inputs) is a complete-shaped FULL
// row typed skew, never INCOMPLETE; another no-plan reason types abstain, a planned whole-universe
// run types plan, and a no-plan shard beside planned shards leaves the row INCOMPLETE.
func TestCINV07921FallbackSelectionIsATypedFullRow_CINV07921(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/row.jq", "CIN-V0-7921")
	f := prFixture(t)
	fallbackShards(f, skewReason)
	row := prRow(t, f)
	if row["class"] != "FULL" || len(row["incomplete"].([]any)) != 0 {
		t.Fatalf("class %v incomplete %v, want FULL with none", row["class"], row["incomplete"])
	}
	full := sub(sub(row, "selection"), "full")
	if full["type"] != "skew" || num(full["unplanned"]) != shards || !strings.Contains(strs(full["reasons"])[0], "unsupported-affected-graph") {
		t.Fatalf("selection.full = %v", full)
	}
	if sub(row, "selection")["scope"] != "FULL" || num(sub(row, "selection")["selected"]) != 0 {
		t.Fatalf("selection = %v", row["selection"])
	}
	// A failing package under an all-FULL shadow is no miss: the shadow selected nothing to miss.
	f = prFixture(t)
	failed := strs(f.sel[3]["selected"])[0]
	fallbackShards(f, skewReason)
	fail(f.out[3], failed, "fail", "RAN", "fail")
	if row := prRow(t, f); row["class"] != "FULL" || num(sub(row, "counts")["failed"]) != 1 || num(sub(row, "counts")["missed"]) != 1 {
		t.Fatalf("failing FULL row = %v %v", row["class"], row["counts"])
	}
	// Another no-plan reason is an abstention.
	f = prFixture(t)
	fallbackShards(f, "no plan: the base commit is unreadable")
	if row := prRow(t, f); row["class"] != "FULL" || sub(sub(row, "selection"), "full")["type"] != "abstain" {
		t.Fatalf("abstained row = %v %v", row["class"], sub(row, "selection")["full"])
	}
	// A planned FULL run (every shard planned the whole universe, digests and all) is typed plan.
	f = prFixture(t)
	for _, d := range f.sel {
		d["scope"], d["reason"] = "FULL", "FALLBACK: a changed go.mod"
	}
	if row := prRow(t, f); row["class"] != "FULL" || sub(sub(row, "selection"), "full")["type"] != "plan" || num(sub(sub(row, "selection"), "full")["unplanned"]) != 0 {
		t.Fatalf("planned FULL row = %v %v", row["class"], sub(row, "selection")["full"])
	}
	// A FULL document with no reason is not a fallback document: it still fails the shape.
	f = prFixture(t)
	fallbackShards(f, "")
	wantIncomplete(t, prRow(t, f), "selection document(s) fail the corvint-pr-selection/0 shape")
	// One unplanned shard beside planned NARROWED shards cannot say what it would have selected.
	f = prFixture(t)
	f.sel[2]["scope"], f.sel[2]["reason"] = "FULL", skewReason
	f.sel[2]["planDigest"], f.sel[2]["partitionDigest"], f.sel[2]["selected"] = "", "", []string{}
	row = prRow(t, f)
	wantIncomplete(t, row, "selection shards 2 planned nothing (FULL: no plan: unsupported-affected-graph")
	if sub(row, "selection")["full"] != nil {
		t.Fatalf("a mixed row carries a full type: %v", sub(row, "selection")["full"])
	}
	// A planned FULL shard beside NARROWED shards is measured as before: its selection is whole.
	f = prFixture(t)
	f.sel[2]["scope"], f.sel[2]["reason"] = "FULL", "FALLBACK: a changed go.mod"
	if row := prRow(t, f); row["class"] != "CLEAN" {
		t.Fatalf("a planned FULL shard beside narrowed shards = %v %v", row["class"], row["incomplete"])
	}

	// Fault injection (V1-5218): without the FULL branch of row.jq, the all-abstained run (no
	// package selected anywhere) would be a CLEAN row, or a MISS once a package fails, and the bar
	// would read either as narrowing evidence. The branch is load-bearing.
	mutated := t.TempDir()
	for _, name := range []string{"row.jq", "classify.sh"} {
		b, err := os.ReadFile(script(t, name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "row.jq" {
			cut := `   elif $fullRun then "FULL"` + "\n"
			if !strings.Contains(string(b), cut) {
				t.Fatalf("row.jq lacks the FULL branch %q", cut)
			}
			b = []byte(strings.Replace(string(b), cut, "", 1))
		}
		writeFile(t, filepath.Join(mutated, name), b)
	}
	if err := os.Chmod(filepath.Join(mutated, "classify.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	classifyWith := func(t *testing.T, f *fixture) string {
		dir := t.TempDir()
		selDir, outDir := f.write(t, dir)
		out := filepath.Join(dir, "row.json")
		return strings.TrimSpace(run(t, []string{"ROW_KIND=pr", "ROW_PR=868", "ROW_SHA=", "ROW_HEAD=" + prHead, "ROW_RUN=" + prRun,
			"ROW_SELECTION_RUN=" + prRun, "ROW_OUTCOMES_RUN=" + prRun, "ROW_SHARDS=" + strconv.Itoa(shards), "ROW_EPOCH=1",
			"ROW_SELECTION_DIR=" + selDir, "ROW_OUTCOMES_DIR=" + outDir, "ROW_NARROWING_DIR=" + filepath.Join(dir, "narrowing")},
			"sh", filepath.Join(mutated, "classify.sh"), out))
	}
	f = prFixture(t)
	fallbackShards(f, skewReason)
	if class := classifyWith(t, f); class != "CLEAN" {
		t.Fatalf("mutated row.jq classified the abstained run %s, want CLEAN (the fault the FULL branch removes)", class)
	}
	fail(f.out[3], strs(prFixture(t).sel[3]["selected"])[0], "fail", "RAN", "fail")
	if class := classifyWith(t, f); class != "MISS" {
		t.Fatalf("mutated row.jq classified the failing abstained run %s, want MISS (the fault the FULL branch removes)", class)
	}
}

// collectPR runs collect.sh for the pull-request run of f against a gh stand-in and returns the
// row name it wrote.
func collectPR(t *testing.T, f *fixture, epoch string) (name, summary string) {
	t.Helper()
	dir := t.TempDir()
	f.stampEpoch(1)
	gh := fakeGH(t, dir)
	writeDoc(t, filepath.Join(dir, "commit.json"), doc{"parents": []any{doc{"sha": mainBase}, doc{"sha": prHead}}})
	writeDoc(t, filepath.Join(dir, "compare.json"), doc{"files": []any{doc{"filename": ".corvint/runtime-readers.json"}}, "status": "identical"})
	writeFile(t, filepath.Join(dir, "selection-run"), lines(prRun))
	selDir, outDir := f.write(t, filepath.Join(dir, "src"))
	var names []string
	for _, src := range []string{outDir, selDir, filepath.Join(dir, "src", "narrowing")} {
		entries, _ := os.ReadDir(src)
		for _, e := range entries {
			names = append(names, e.Name())
			if err := os.MkdirAll(filepath.Join(dir, "runs", prRun), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(filepath.Join(src, e.Name()), filepath.Join(dir, "runs", prRun, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeFile(t, filepath.Join(dir, "names-"+prRun), lines(names...))
	temp, output, sum := filepath.Join(dir, "temp"), filepath.Join(dir, "output"), filepath.Join(dir, "summary")
	writeFile(t, output, nil)
	cmd := exec.Command("sh", script(t, "collect.sh"))
	cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
		"GH_TOKEN=x", "REPO=beamfall/corvint", "RUN_ID=" + prRun, "EVENT=pull_request", "HEAD_SHA=" + prHead,
		"HEAD_BRANCH=claude/x", "PR_NUMBER=163", "PR_BASE_SHA=" + mainBase, "FORWARD_SHARDS=" + strconv.Itoa(shards), "FORWARD_EPOCH=" + epoch,
		"RUNNER_TEMP=" + temp, "GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + sum}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("collect.sh: %v\n%s", err, out)
	}
	o, _ := os.ReadFile(output)
	s, _ := os.ReadFile(sum)
	m := regexp.MustCompile(`(?m)^name=(.*)$`).FindStringSubmatch(string(o))
	if m == nil {
		return "", string(s)
	}
	return m[1], string(s)
}

// CIN-V0-7921 (V1-5216 AC1, V1-5219): the collector names the #163 shape
// ci-forward-FULL-pr163-<tested merge>-e1-f0-rskew, writes no INCOMPLETE row and no cost name
// (a FULL row prices nothing), and types a planned FULL run plan.
func TestCINV07921CollectorNamesTheFullRowWithItsType_CINV07921(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/collect.sh", "CIN-V0-7921")
	f := prFixture(t)
	fallbackShards(f, skewReason)
	name, summary := collectPR(t, f, "1")
	if name != "ci-forward-FULL-pr163-"+queueSHA+"-e1-f0-rskew" {
		t.Fatalf("row name %q\n%s", name, summary)
	}
	if strings.Contains(summary, "fail the corvint-pr-selection/0 shape") || strings.Contains(summary, "cost=") {
		t.Fatalf("summary:\n%s", summary)
	}
	f = prFixture(t)
	for _, d := range f.sel {
		d["scope"], d["reason"] = "FULL", "FALLBACK: a changed go.mod"
	}
	if name, summary := collectPR(t, f, "1"); name != "ci-forward-FULL-pr163-"+queueSHA+"-e1-f0-rplan" {
		t.Fatalf("planned row name %q\n%s", name, summary)
	}
	// The shape rejection the ticket observed still names a run whose dry-run printed nothing.
	f = prFixture(t)
	for _, d := range f.sel {
		d["scope"], d["reason"], d["planDigest"], d["partitionDigest"], d["selected"] = nil, nil, nil, nil, []string{}
	}
	if name, summary := collectPR(t, f, "1"); name != "ci-forward-INCOMPLETE-pr163-"+queueSHA+"-e1-f0" ||
		!strings.Contains(summary, "8 selection document(s) fail the corvint-pr-selection/0 shape") {
		t.Fatalf("no-document row name %q\n%s", name, summary)
	}
}

func fullName(pr int, sha string, why string) string {
	return fmt.Sprintf("ci-forward-FULL-pr%d-%s-e1-f0-r%s", pr, sha, why)
}

// CIN-V0-7921 (V1-5218 AC1, V1-5216 AC2): twenty all-abstained FULL rows meet nothing: the bar
// counts no complete row and the kill switch stays clear; beside eight complete rows they add
// nothing to the count and never replace a counted row. Four INCOMPLETE names of one pull
// request over three tested merges count three, not four (one row per pull request and commit).
func TestCINV07921FullRowsAreNotNarrowingEvidence_CINV07921(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/bar.jq", "CIN-V0-7921")
	sha := func(n int) string { return fmt.Sprintf("%040x", n) }
	var abstained []string
	for pr := 100; pr < 120; pr++ {
		abstained = append(abstained, fullName(pr, sha(pr), "skew"))
	}
	v := barJSON(t, "1", append(abstained, barNames("CLEAN", 90, "0", "0")))
	e, w, full := sub(v, "epochRows"), sub(v, "window"), sub(v, "full")
	if v["status"] != "NOT MET" || num(e["complete"]) != 0 || num(w["rows"]) != 0 || v["killSwitch"] != "clear" || num(full["rows"]) != 20 || num(full["skew"]) != 20 {
		t.Fatalf("20 abstained rows: status %v epoch %v window %v kill %v full %v", v["status"], e, w, v["killSwitch"], full)
	}
	// The markdown form says the same, and names them outside the bar and the window.
	md := run(t, nil, "sh", "-c", `printf '%s' "$1" | jq -R -s -r --arg epoch 1 -f "$2"`, "sh", strings.Join(abstained, "\n")+"\n", script(t, "bar.jq"))
	for _, want := range []string{"): NOT MET (0/8 complete, 0/1 failing)", "FULL shadow runs (CIN-V0-7921): 20 FULL row(s), outside the bar and the window: 20 skew ("} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown lacks %q:\n%s", want, md)
		}
	}
	// Eight complete rows meet the bar with or without the FULL rows, and the count is the same.
	met := metNames()[:8]
	met = append(met, barNames("CLEAN", 90, "0", "0"))
	with := barJSON(t, "1", append(append([]string{}, abstained...), met...))
	without := barJSON(t, "1", met)
	if with["status"] != "MET" || without["status"] != "MET" || num(sub(with, "epochRows")["complete"]) != 8 || num(sub(without, "epochRows")["complete"]) != 8 {
		t.Fatalf("with FULL rows %v %v; without %v %v", with["status"], sub(with, "epochRows"), without["status"], sub(without, "epochRows"))
	}
	// A newer FULL row of a counted pull request and commit does not replace its complete row.
	replaced := barJSON(t, "1", append([]string{fullName(1, sha(1), "abstain")}, met...))
	if replaced["status"] != "MET" || num(sub(replaced, "epochRows")["complete"]) != 8 || num(sub(replaced, "full")["abstain"]) != 1 {
		t.Fatalf("a FULL re-run row replaced a counted row: %v %v %v", replaced["status"], sub(replaced, "epochRows"), sub(replaced, "full"))
	}
	// Fault injection: the same twenty runs under the names the collector wrote before this
	// change (CLEAN rows) would have met the bar; a window of them under INCOMPLETE names would
	// have tripped the switch. Neither name shape is written for an all-FULL run now.
	var asClean, asIncomplete []string
	for pr := 100; pr < 120; pr++ {
		asClean = append(asClean, barNames("CLEAN", pr, "1", "0"))
		asIncomplete = append(asIncomplete, barNames("INCOMPLETE", pr, "1", "0"))
	}
	if v := barJSON(t, "1", append(append(asClean, barNames("CLEAN", 1, "1", "1")), barNames("CLEAN", 90, "0", "0"))); v["status"] != "MET" {
		t.Fatalf("the pre-change CLEAN names do not reproduce the defect: %v", v["status"])
	}
	if v := barJSON(t, "1", asIncomplete); v["killSwitch"] != "TRIPPED" {
		t.Fatalf("the pre-change INCOMPLETE names do not reproduce the defect: %v", v["killSwitch"])
	}
	// V1-5216 AC2: the #163 window shape, four INCOMPLETE names over three tested merges, one of
	// them twice, counts three rows; the same runs as FULL rows count none.
	incomplete := []string{barNames("INCOMPLETE", 163, "1", "0"), fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(2)),
		fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(3)), fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(2))}
	v = barJSON(t, "1", append(incomplete, met...))
	if num(sub(v, "window")["incomplete"]) != 3 || num(sub(v, "window")["rows"]) != 11 || v["killSwitch"] != "TRIPPED" {
		t.Fatalf("four INCOMPLETE names over three commits: %v %v", sub(v, "window"), v["killSwitch"])
	}
	asFull := []string{fullName(163, sha(163), "skew"), fullName(163, sha(2), "skew"), fullName(163, sha(3), "skew"), fullName(163, sha(2), "skew")}
	v = barJSON(t, "1", append(asFull, met...))
	if num(sub(v, "window")["incomplete"]) != 0 || v["killSwitch"] != "clear" || num(sub(v, "full")["rows"]) != 3 || v["status"] != "MET" {
		t.Fatalf("the #163 runs as FULL rows: %v %v full %v", sub(v, "window"), v["killSwitch"], sub(v, "full"))
	}
}

// CIN-V0-7921 (V1-5216 AC3): accept.sh over the #163 listing shape, a MET verdict, its twenty
// window rows and then one pull request's FULL fallback rows from four control-plane runs over
// three tested merges, says MET; the same runs under the old INCOMPLETE names said FULL with a
// tripped kill switch, which is what turned narrowing off for every pull request on 2026-11-09.
func TestCINV07921FallbackRerunsKeepTheVerdictMet_CINV07921(t *testing.T) {
	const run, repo, workflow = "4242", "beamfall/corvint", ".github/workflows/ci-forward-bar.yml"
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	sha := func(n int) string { return fmt.Sprintf("%040x", n) }
	accept := func(t *testing.T, fresh []string) (string, string) {
		dir := t.TempDir()
		gh := fakeGH(t, dir)
		artifact := doc{"id": 1, "name": "afp-shadow-bar-epoch-1", "expired": false, "created_at": now.Add(-24 * time.Hour).Format(time.RFC3339),
			"workflow_run": doc{"id": 4242, "head_branch": "main", "repository_id": 1, "head_repository_id": 1}}
		writeDoc(t, filepath.Join(dir, "verdict-artifacts.json"), doc{"artifacts": []any{artifact}})
		writeDoc(t, filepath.Join(dir, "run-4242.json"), doc{"path": workflow + "@refs/heads/main", "event": "schedule", "head_branch": "main",
			"head_repository": doc{"full_name": repo}, "repository": doc{"full_name": repo}})
		writeDoc(t, filepath.Join(dir, "runs", run, "afp-shadow-bar-epoch-1", "verdict.json"),
			doc{"profile": "afp-shadow-bar-verdict/0", "status": "MET", "epoch": 1, "killSwitch": "clear", "runId": 4242, "repository": repo, "mainSha": queueSHA})
		var all []any
		for _, n := range append(fresh, metNames()...) {
			all = append(all, doc{"name": n, "expired": false, "workflow_run": doc{"head_branch": "main", "repository_id": 1, "head_repository_id": 1}})
		}
		writeDoc(t, filepath.Join(dir, "all-artifacts.json"), doc{"artifacts": all})
		output := filepath.Join(dir, "output")
		writeFile(t, output, nil)
		cmd := exec.Command("sh", script(t, "accept.sh"))
		cmd.Env = []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GH_TOKEN=x", "REPO=" + repo, "FORWARD_EPOCH=1", "RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + output,
			"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary"), "VERDICT_WORKFLOW=" + workflow,
			"NOW=" + strconv.FormatInt(now.Unix(), 10)}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("accept.sh: %v\n%s", err, out)
		}
		o, _ := os.ReadFile(output)
		verdict := regexp.MustCompile(`(?m)^verdict=(.*)$`).FindStringSubmatch(string(o))
		reason := regexp.MustCompile(`(?m)^reason=(.*)$`).FindStringSubmatch(string(o))
		if verdict == nil || reason == nil {
			t.Fatalf("accept.sh wrote no verdict:\n%s", o)
		}
		return verdict[1], reason[1]
	}
	// Control-plane runs 42574479060, 42576912765, 42580922931 and 42619477099 of PR #163: the
	// newest first, the third tested merge named twice.
	reruns := []string{fullName(163, sha(0x163), "skew"), fullName(163, sha(0x263), "skew"), fullName(163, sha(0x363), "skew"), fullName(163, sha(0x263), "skew")}
	if verdict, reason := accept(t, reruns); verdict != "MET" {
		t.Fatalf("verdict %s (%s), want MET", verdict, reason)
	}
	old := []string{fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(0x163)), fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(0x263)),
		fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(0x363)), fmt.Sprintf("ci-forward-INCOMPLETE-pr163-%s-e1-f0", sha(0x263))}
	if verdict, reason := accept(t, old); verdict != "FULL" || !strings.Contains(reason, "tripped kill switch") {
		t.Fatalf("the old INCOMPLETE names do not reproduce the 2026-11-09 verdict: %s (%s)", verdict, reason)
	}
}
