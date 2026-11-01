// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const (
	mainRunSHA  = "1111111111111111111111111111111111111111"
	mainGreen   = "2222222222222222222222222222222222222222"
	mergeA      = "3333333333333333333333333333333333333333"
	mergeB      = "4444444444444444444444444444444444444444"
	prHeadA     = "5555555555555555555555555555555555555555"
	prHeadB     = "6666666666666666666666666666666666666666"
	prHeadFull  = "7777777777777777777777777777777777777777"
	failedPkg   = "github.com/Beamfall/corvint/internal/specindex"
	passedPkg   = "github.com/Beamfall/corvint/internal/other"
	narrowedRun = 9001
)

func mainMissRecord(mode string, shard int, skipped ...string) doc {
	if skipped == nil {
		skipped = []string{}
	}
	return doc{"profile": "corvint-pr-narrowing/0", "mode": mode, "shard": shard, "shards": 8, "runId": narrowedRun,
		"executed": []string{"github.com/Beamfall/corvint/internal/a"}, "skipped": skipped}
}

// mainMissGH writes a gh stand-in for mainmiss.sh: each `api` path runs the script's own
// --jq filter over a canned API response in DIR, the run-artifacts path lists the directories of
// DIR/runs/ID (failing when DIR/fail-names-ID exists), and `run download ID --pattern P --dir OUT`
// copies the matching artifact directories of DIR/runs/ID (failing when DIR/fail-download-ID exists).
func mainMissGH(t *testing.T, dir string) string {
	bin := filepath.Join(dir, "bin")
	writeFile(t, filepath.Join(bin, "gh"), []byte(`#!/bin/sh
set -eu
d='`+dir+`'
if [ "$1" = api ]; then
  case "$2" in
  *workflows/ci.yml/runs\?branch=main*) f="$d/main-runs.json" ;;
  *workflows/ci.yml/runs\?event=pull_request*) h=${2#*head_sha=}; f="$d/pr-runs-${h%%&*}.json" ;;
  */compare/*) f="$d/compare.json" ;;
  */commits/*/pulls) c=${2#*/commits/}; f="$d/pulls-${c%%/*}.json" ;;
  */actions/runs/*/artifacts*) id=${2#*/actions/runs/}; id=${id%%/*}
    [ ! -f "$d/fail-names-$id" ] || exit 1
    [ ! -d "$d/runs/$id" ] || ls "$d/runs/$id"
    exit 0 ;;
  *) exit 1 ;;
  esac
  [ "$3" = --jq ] && [ -f "$f" ] && jq -r "$4" "$f"
  exit 0
fi
[ "$1 $2" = "run download" ] || exit 1
id=$3 pattern="" out=""
[ ! -f "$d/fail-download-$id" ] || exit 1
shift 3
while [ $# -gt 0 ]; do
  case "$1" in --pattern) pattern=$2; shift ;; --dir) out=$2; shift ;; esac
  shift
done
found=0
for a in "$d/runs/$id"/*; do
  [ -d "$a" ] || continue
  case "${a##*/}" in $pattern) mkdir -p "$out"; cp -R "$a" "$out/"; found=1 ;; esac
done
[ "$found" = 1 ]
`))
	if err := os.Chmod(filepath.Join(bin, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// AFP-V0-983 (V1-3750): a failed full push run of main whose failed package a narrowed pull
// request merged since the last green run skipped records a MAINMISS row naming that pull request.
// A narrowed pull request that skipped only passing packages, one whose shards all ran FULL, and
// one without records are no miss; a run that names no failed package writes no row.
func TestAFPV0983MainFailureNamesTheNarrowedPullRequest(t *testing.T) {
	dir := t.TempDir()
	gh := mainMissGH(t, dir)
	writeDoc(t, filepath.Join(dir, "runs", "500", "ci-failed-push-3", "failed.json"),
		doc{"reasons": []any{doc{"package": failedPkg, "reason": "test-failures", "tests": []string{"TestIndexImplementationPathsExist"}, "testCount": 1}}})
	writeDoc(t, filepath.Join(dir, "main-runs.json"), doc{"workflow_runs": []any{
		doc{"id": 500, "head_sha": mainRunSHA}, doc{"id": 450, "head_sha": mainGreen}, doc{"id": 400, "head_sha": strings.Repeat("9", 40)}}})
	writeDoc(t, filepath.Join(dir, "compare.json"), doc{"status": "ahead", "total_commits": 2,
		"commits": []any{doc{"sha": mergeA}, doc{"sha": mergeB}}})
	writeDoc(t, filepath.Join(dir, "pulls-"+mergeA+".json"), []any{
		doc{"number": 12, "merged_at": "2026-11-01T00:00:00Z", "base": doc{"ref": "main"}, "head": doc{"sha": prHeadA}}})
	writeDoc(t, filepath.Join(dir, "pulls-"+mergeB+".json"), []any{
		doc{"number": 13, "merged_at": "2026-11-01T00:00:00Z", "base": doc{"ref": "main"}, "head": doc{"sha": prHeadB}},
		doc{"number": 14, "merged_at": "2026-11-01T00:00:00Z", "base": doc{"ref": "main"}, "head": doc{"sha": prHeadFull}},
		doc{"number": 99, "merged_at": nil, "base": doc{"ref": "main"}, "head": doc{"sha": prHeadFull}}})
	for head, run := range map[string]int{prHeadA: 601, prHeadB: 602, prHeadFull: 603} {
		writeDoc(t, filepath.Join(dir, "pr-runs-"+head+".json"), doc{"workflow_runs": []any{doc{"id": run}}})
	}
	writeDoc(t, filepath.Join(dir, "runs", "601", "ci-narrowing-0", "narrowing.json"), mainMissRecord("NARROWED", 0, passedPkg))
	writeDoc(t, filepath.Join(dir, "runs", "601", "ci-narrowing-5", "narrowing.json"), mainMissRecord("NARROWED", 5, failedPkg))
	writeDoc(t, filepath.Join(dir, "runs", "602", "ci-narrowing-5", "narrowing.json"), mainMissRecord("NARROWED", 5, passedPkg))
	writeDoc(t, filepath.Join(dir, "runs", "603", "ci-narrowing-5", "narrowing.json"), mainMissRecord("FULL", 5, failedPkg))

	mainMiss := func(t *testing.T) (string, string) {
		temp := t.TempDir()
		output := filepath.Join(temp, "output")
		summary := filepath.Join(temp, "summary")
		writeFile(t, output, nil)
		run(t, []string{"PATH=" + gh + string(os.PathListSeparator) + os.Getenv("PATH"), "GH_TOKEN=x", "REPO=beamfall/corvint",
			"RUN_ID=500", "HEAD_SHA=" + mainRunSHA, "FORWARD_EPOCH=2", "RUNNER_TEMP=" + temp,
			"GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary}, "sh", script(t, "mainmiss.sh"))
		out, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		if strings.TrimSpace(string(out)) != "" {
			var miss doc
			b, err := os.ReadFile(filepath.Join(temp, "main-miss", "miss.json"))
			if err != nil || json.Unmarshal(b, &miss) != nil {
				t.Fatalf("miss.json unreadable: %v", err)
			}
			want := []any{doc{"pr": float64(12), "head": prHeadA, "runId": float64(narrowedRun), "packages": []any{failedPkg}}}
			if !reflect.DeepEqual(miss["misses"], want) || miss["profile"] != "corvint-main-miss/0" {
				t.Fatalf("misses = %v, want %v", miss["misses"], want)
			}
		}
		return strings.TrimSpace(string(out)), string(s)
	}
	name, summary := mainMiss(t)
	if want := "name=ci-forward-MAINMISS-r500-" + mainRunSHA + "-e2-n1-pr12"; name != want {
		t.Fatalf("output %q, want %q; summary:\n%s", name, want, summary)
	}
	if !strings.Contains(summary, "#12 skipped "+failedPkg) {
		t.Fatalf("summary:\n%s", summary)
	}

	// The bar reports the row's miss and pull request, outside the status and the kill switch.
	names := strings.TrimPrefix(name, "name=") + "\n" + "ci-forward-CLEAN-pr3-" + prHead + "-e2-f0\n"
	bar := run(t, nil, "sh", "-c", `printf '%s' "$1" | jq -R -s -r --arg epoch 2 --arg format json -f "$2"`, "sh", names, script(t, "bar.jq"))
	var verdict doc
	if err := json.Unmarshal([]byte(bar), &verdict); err != nil {
		t.Fatal(err)
	}
	if got := verdict["mainMiss"].(doc); got["misses"] != float64(1) || got["runs"] != float64(1) || !reflect.DeepEqual(got["pullRequests"], []any{float64(12)}) {
		t.Fatalf("bar mainMiss = %v", got)
	}
	if verdict["epochRows"].(doc)["miss"] != float64(0) {
		t.Fatalf("a MAINMISS name fed the bar's own MISS count: %v", verdict["epochRows"])
	}
	markdown := run(t, nil, "sh", "-c", `printf '%s' "$1" | jq -R -s -r --arg epoch 2 -f "$2"`, "sh", names, script(t, "bar.jq"))
	if !strings.Contains(markdown, "Main full-run misses (AFP-V0-983, advisory; never the bar or the kill switch): 1 narrowed pull request(s) skipped a package a later full run of main failed, over 1 failed run(s): #12.") {
		t.Fatalf("bar markdown:\n%s", markdown)
	}

	// A pull request whose run kept no narrowing record ran in full and is no gap.
	if err := os.RemoveAll(filepath.Join(dir, "runs", "603")); err != nil {
		t.Fatal(err)
	}
	if name, summary := mainMiss(t); name != "name=ci-forward-MAINMISS-r500-"+mainRunSHA+"-e2-n1-pr12" {
		t.Fatalf("a run without narrowing records changed the row: %q\n%s", name, summary)
	}
	// Codex P2 (AFP-V0-983): a failed download beside successful ones, an unreadable artifact
	// list, or a pull request with no readable run abstains, naming each gap, rather than
	// dropping that pull request and naming only the misses it could read.
	for _, gap := range []struct{ file, want string }{
		{"fail-download-602", "the ci-narrowing records of run 602 of #13 could not be downloaded"},
		{"fail-names-602", "the artifacts of run 602 of #13 are unreadable"},
		{"pr-runs-" + prHeadB + ".json", "no pull_request run of #13 at " + prHeadB + " is readable"},
	} {
		path := filepath.Join(dir, gap.file)
		saved, readErr := os.ReadFile(path)
		if strings.HasPrefix(gap.file, "pr-runs-") {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFile(t, path, nil)
		}
		name, summary := mainMiss(t)
		if name != "" || !strings.Contains(summary, "main miss: abstained for run 500, so no row: "+gap.want) {
			t.Fatalf("%s: output %q summary:\n%s", gap.file, name, summary)
		}
		if readErr == nil {
			writeFile(t, path, saved)
		} else if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}

	// The failing package skipped by no narrowed pull request is no miss.
	writeDoc(t, filepath.Join(dir, "runs", "601", "ci-narrowing-5", "narrowing.json"), mainMissRecord("NARROWED", 5, passedPkg))
	if name, summary := mainMiss(t); name != "" || !strings.Contains(summary, "no narrowed pull request merged since "+mainGreen+" skipped them; no row") {
		t.Fatalf("no-miss output %q summary:\n%s", name, summary)
	}
	// A run with no failed-package artifact writes no row and says why.
	if err := os.RemoveAll(filepath.Join(dir, "runs", "500")); err != nil {
		t.Fatal(err)
	}
	if name, summary := mainMiss(t); name != "" || !strings.Contains(summary, "retained no ci-failed-push artifact") {
		t.Fatalf("no-artifact output %q summary:\n%s", name, summary)
	}
}

// AFP-V0-983 (Codex P2): the ci.yml step that classifies a failed main shard runs in the job's
// checkout, source/, where failed.jq lives; run there in the workflow's own layout it writes the
// failed packages, and from the workspace root it writes no file rather than an empty one.
func TestAFPV0983FailedMainShardStepRunsInTheCheckout(t *testing.T) {
	body, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?ms)^      - name: Classify a failed main shard's packages for the main-miss link \(AFP-V0-983\)\n(.*?)\n      - name: `).FindSubmatch(body)
	if m == nil {
		t.Fatal("ci.yml has no main-miss classification step")
	}
	step := string(m[1])
	if !strings.Contains(step, "\n        working-directory: source\n") {
		t.Fatalf("the step does not run in the source/ checkout:\n%s", step)
	}
	run := regexp.MustCompile(`(?ms)^        run: \|\n(.*)\z`).FindStringSubmatch(step)
	if run == nil {
		t.Fatalf("the step has no run block:\n%s", step)
	}
	script := strings.ReplaceAll(regexp.MustCompile(`(?m)^          `).ReplaceAllString(run[1], ""), "${{ strategy.job-index }}", "3")
	workspace := t.TempDir()
	failedJQ, err := os.ReadFile("../../.github/forward-rows/failed.jq")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(workspace, "source", ".github", "forward-rows", "failed.jq"), failedJQ)
	temp := filepath.Join(workspace, "runner-temp")
	writeFile(t, filepath.Join(temp, "shard-costs", "shard-3.json"), lines(
		`{"Action":"run","Package":"`+failedPkg+`","Test":"TestIndexImplementationPathsExist"}`,
		`{"Action":"fail","Package":"`+failedPkg+`","Test":"TestIndexImplementationPathsExist"}`,
		`{"Action":"fail","Package":"`+failedPkg+`"}`))
	step3 := func(t *testing.T, dir string) (doc, bool) {
		out := filepath.Join(temp, "main-failed", "failed.json")
		_ = os.Remove(out)
		cmd := exec.Command("bash", "-e", "-c", script)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "RUNNER_TEMP=" + temp}
		_ = cmd.Run()
		b, err := os.ReadFile(out)
		if err != nil {
			return nil, false
		}
		var d doc
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("failed.json %q: %v", b, err)
		}
		return d, true
	}
	d, ok := step3(t, filepath.Join(workspace, "source"))
	if !ok || len(d["reasons"].([]any)) != 1 {
		t.Fatalf("in source/ the step wrote %v", d)
	}
	if got := d["reasons"].([]any)[0].(doc)["package"]; got != failedPkg {
		t.Fatalf("failed package = %v", got)
	}
	if _, ok := step3(t, workspace); ok {
		t.Fatal("from the workspace root, where failed.jq is absent, the step still wrote a document")
	}
}
