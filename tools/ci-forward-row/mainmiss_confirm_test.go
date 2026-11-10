// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// confirmRepo is a Git repository of main for mainmiss-confirm.sh: module example.org/m with the
// packages store, other, old and x at the base; a pull request merged with --no-ff (merge, whose
// first parent is base) that marks store failing and adds the failing package gone; a later commit
// of main (head, the failed full run); and a commit off main (stray) no merge reached.
type confirmRepo struct {
	t                               *testing.T
	dir                             string
	base, merge, head, stray, prTip string
}

func (r *confirmRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newConfirmRepo(t *testing.T) *confirmRepo {
	t.Helper()
	r := &confirmRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	writeFile(t, filepath.Join(r.dir, "go.mod"), []byte("module example.org/m\n\ngo 1.27.3\n"))
	for _, pkg := range []string{"store", "other", "old", "x"} {
		writeFile(t, filepath.Join(r.dir, pkg, pkg+".go"), []byte("package "+pkg+"\n"))
	}
	writeFile(t, filepath.Join(r.dir, "old", "FAIL"), nil)
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "base")
	r.base = r.git("rev-parse", "HEAD")
	r.git("checkout", "-q", "-b", "stray")
	writeFile(t, filepath.Join(r.dir, "x", "FAIL"), nil)
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "stray")
	r.stray = r.git("rev-parse", "HEAD")
	r.git("checkout", "-q", "-b", "feature", r.base)
	writeFile(t, filepath.Join(r.dir, "store", "FAIL"), nil)
	writeFile(t, filepath.Join(r.dir, "gone", "gone.go"), []byte("package gone\n"))
	writeFile(t, filepath.Join(r.dir, "gone", "FAIL"), nil)
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "feature")
	r.prTip = r.git("rev-parse", "HEAD")
	r.git("checkout", "-q", "main")
	r.git("merge", "-q", "--no-ff", "-m", "Merge pull request #12", r.prTip)
	r.merge = r.git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(r.dir, "README"), []byte("later\n"))
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "later")
	r.head = r.git("rev-parse", "HEAD")
	return r
}

// confirmFakeGo stands in for the Go toolchain: `go version` prints the pinned version; `go test ... ./DIR`
// fails with a package-level fail event when DIR/FAIL exists in the worktree, exits nonzero with no
// event when DIR/UNUSABLE exists, and passes otherwise. Every test invocation is appended to
// $FAKE_GO_LOG as "<worktree name> <dir>".
func confirmFakeGo(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "gobin")
	writeFile(t, filepath.Join(bin, "go"), []byte(`#!/bin/sh
set -u
if [ "$1" = version ]; then echo "go version go1.27.3 linux/amd64"; exit 0; fi
[ "$1" = test ] || exit 2
dir=""
for a in "$@"; do case "$a" in ./*) dir=${a#./} ;; esac; done
printf '%s %s %s\n' "${PWD##*/}" "$dir" "$*" >> "$FAKE_GO_LOG"
mod=$(sed -n 's/^module //p' go.mod)
pkg="$mod/$dir"
[ "$dir" != . ] || pkg=$mod
[ -d "$dir" ] || { echo "no such package ./$dir" >&2; exit 1; }
if [ -f "$dir/UNUSABLE" ]; then echo "go: build constraints exclude all Go files" >&2; exit 1; fi
printf '{"Action":"start","Package":"%s"}\n' "$pkg"
if [ -f "$dir/FAIL" ]; then
  printf '{"Action":"run","Package":"%s","Test":"TestIt"}\n{"Action":"fail","Package":"%s","Test":"TestIt"}\n{"Action":"fail","Package":"%s","Elapsed":0.1}\n' "$pkg" "$pkg" "$pkg"
  exit 1
fi
printf '{"Action":"pass","Package":"%s","Elapsed":0.1}\n' "$pkg"
`))
	if err := os.Chmod(filepath.Join(bin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

type confirmResult struct {
	outputs map[string]string
	summary string
	check   doc
	log     []string
	temp    string
}

func confirm(t *testing.T, r *confirmRepo, misses []any, extra ...string) confirmResult {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "mainmiss-confirm.sh runs under sh against a real Git repository")
	temp := t.TempDir()
	gobin := confirmFakeGo(t, temp)
	writeDoc(t, filepath.Join(temp, "miss.json"), doc{"profile": "corvint-main-miss/0", "runId": 500, "sha": r.head, "epoch": 2,
		"failed": []string{"example.org/m/store"}, "misses": misses})
	output, summary, log := filepath.Join(temp, "output"), filepath.Join(temp, "summary"), filepath.Join(temp, "go.log")
	writeFile(t, output, nil)
	writeFile(t, log, nil)
	env := []string{"PATH=" + gobin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + temp, "FAKE_GO_LOG=" + log,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"MISS=" + filepath.Join(temp, "miss.json"), "SOURCE=" + r.dir, "RUN_ID=500", "HEAD_SHA=" + r.head, "FORWARD_EPOCH=2",
		"RUNNER_TEMP=" + temp, "GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary, "GO_TEST_FLAGS=-race -exec /usr/bin/false"}
	cmd := exec.Command("sh", script(t, "mainmiss-confirm.sh"))
	cmd.Env = append(env, extra...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mainmiss-confirm.sh: %v\n%s", err, out)
	}
	res := confirmResult{outputs: map[string]string{}, temp: temp}
	o, _ := os.ReadFile(output)
	for _, l := range strings.Split(strings.TrimSpace(string(o)), "\n") {
		if k, v, ok := strings.Cut(l, "="); ok {
			res.outputs[k] = v
		}
	}
	s, _ := os.ReadFile(summary)
	res.summary = string(s)
	if _, err := os.Stat(filepath.Join(temp, "main-miss", "check.json")); err == nil {
		res.check = readDoc(t, filepath.Join(temp, "main-miss", "check.json"))
	}
	b, _ := os.ReadFile(log)
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l != "" {
			f := strings.Fields(l)
			res.log = append(res.log, f[0]+" "+f[1])
		}
	}
	// No worktree outlives the script.
	if wt := r.git("worktree", "list", "--porcelain"); strings.Contains(wt, "at-base") || strings.Contains(wt, "at-merge") {
		t.Fatalf("worktrees left behind:\n%s", wt)
	}
	return res
}

// CIN-V0-7922 (V1-5198): each suspected pull request's skipped package is rerun once at its merge
// commit's first parent and twice at the merge commit. Pass, fail, fail confirms the miss and names
// a MISS row (#12 store; #17's gone, absent at the base, confirms on its two merge failures); a
// pass at the merge is a flake (#15 other, rerun once there); a failure at the base is preexisting
// (#16 old, never rerun at the merge); a merge commit off main (#13) or none (#14) stays
// unconfirmed. The check names the counts and every pull request; the run's go test flags reach
// every rerun.
func TestCINV07922ConfirmationRerunsClassifyEachSuspectedPullRequest_CINV07922(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/mainmiss-confirm.sh", "")
	r := newConfirmRepo(t)
	miss := func(pr int, merge any, pkgs ...string) doc {
		return doc{"pr": pr, "head": r.prTip, "merge": merge, "runId": 9001, "packages": pkgs}
	}
	misses := []any{miss(12, r.merge, "example.org/m/store"), miss(13, r.stray, "example.org/m/x"), miss(14, nil, "example.org/m/x"),
		miss(15, r.merge, "example.org/m/other"), miss(16, r.merge, "example.org/m/old"), miss(17, r.merge, "example.org/m/gone")}
	res := confirm(t, r, misses)
	want := map[string]string{
		"check":     "ci-forward-MAINCHECK-r500-" + r.head + "-e2-c2-k1-u3-pr12_13_14_15_16_17",
		"confirmed": "1",
		"miss1":     "ci-forward-MISS-pr12-" + r.merge + "-e2-f1",
		"miss2":     "ci-forward-MISS-pr17-" + r.merge + "-e2-f1",
	}
	if fmt.Sprint(res.outputs) != fmt.Sprint(want) {
		t.Fatalf("outputs %v, want %v\n%s", res.outputs, want, res.summary)
	}
	verdicts := map[int]string{}
	whys := map[int]string{}
	for _, x := range res.check["results"].([]any) {
		d := x.(doc)
		verdicts[num(d["pr"])] = d["verdict"].(string)
		whys[num(d["pr"])] = d["packages"].([]any)[0].(doc)["verdict"].(string) + ": " + d["packages"].([]any)[0].(doc)["why"].(string)
	}
	if fmt.Sprint(verdicts) != fmt.Sprint(map[int]string{12: "CONFIRMED", 13: "UNCONFIRMED", 14: "UNCONFIRMED", 15: "FLAKE", 16: "UNCONFIRMED", 17: "CONFIRMED"}) {
		t.Fatalf("verdicts %v\n%v", verdicts, whys)
	}
	for pr, frag := range map[int]string{12: "confirmed: the package passes at the base " + r.base + " and failed twice at the merge commit " + r.merge,
		13: "unconfirmed: merge commit " + r.stray + " of #13 is not an ancestor of the failed commit", 14: "unconfirmed: no merge commit is named for #14",
		15: "flake: the package passed at the merge commit", 16: "preexisting: the package already fails at the base", 17: "the package is absent at the base, so the base counts as a pass"} {
		if !strings.Contains(whys[pr], frag) {
			t.Fatalf("#%d: %q lacks %q", pr, whys[pr], frag)
		}
	}
	// The reruns: store base once and merge twice, other base and merge once each, old base only,
	// gone merge twice, x never (its pull requests have no usable merge commit).
	if got := strings.Join(res.log, "\n"); got != strings.Join([]string{"at-base store", "at-merge store", "at-merge store",
		"at-base other", "at-merge other", "at-base old", "at-merge gone", "at-merge gone"}, "\n") {
		t.Fatalf("reruns:\n%s", got)
	}
	// Each rerun ran the shard's go test flags under the test-confine root.
	b, _ := os.ReadFile(filepath.Join(res.temp, "go.log"))
	if !strings.Contains(string(b), "test -race -exec /usr/bin/false -json -count=1 -timeout 30m ./store") {
		t.Fatalf("go test argv:\n%s", b)
	}
	// The MISS row documents name the confirmed packages, and the check its runner and the summary its verdicts.
	row := readDoc(t, filepath.Join(res.temp, "main-miss", "miss1", "row.json"))
	if row["profile"] != "corvint-main-miss-row/0" || row["class"] != "MISS" || num(row["pr"]) != 12 || row["sha"] != r.merge || fmt.Sprint(row["missed"]) != "[example.org/m/store]" {
		t.Fatalf("miss1 row = %v", row)
	}
	if sub(res.check, "runner")["go"] != "go version go1.27.3 linux/amd64" || fmt.Sprint(res.check["confirmed"]) != "[12 17]" || fmt.Sprint(res.check["flake"]) != "[15]" {
		t.Fatalf("check = %v", res.check)
	}
	for _, line := range []string{"CIN-V0-7922: #12 example.org/m/store: confirmed (", "CIN-V0-7922: #15 example.org/m/other: flake (",
		"main miss check `" + want["check"] + "`: 2 confirmed (#12, #17), 1 flake, 3 unconfirmed, over 6 suspected pull request(s)"} {
		if !strings.Contains(res.summary, line) {
			t.Fatalf("summary lacks %q:\n%s", line, res.summary)
		}
	}

	// A package budget leaves the later packages unconfirmed and runs nothing for them; an exhausted
	// time budget leaves every package unconfirmed. Neither confirms, neither names a MISS row.
	res = confirm(t, r, misses, "MAX_PACKAGES=1")
	if res.outputs["check"] != "ci-forward-MAINCHECK-r500-"+r.head+"-e2-c1-k0-u5-pr12_13_14_15_16_17" || res.outputs["miss2"] != "" || len(res.log) != 3 {
		t.Fatalf("MAX_PACKAGES=1: %v reruns %v", res.outputs, res.log)
	}
	res = confirm(t, r, misses, "BUDGET_SECONDS=-1")
	if res.outputs["check"] != "ci-forward-MAINCHECK-r500-"+r.head+"-e2-c0-k0-u6-pr12_13_14_15_16_17" || res.outputs["confirmed"] != "" || len(res.log) != 0 ||
		!strings.Contains(res.summary, "no time left for three reruns") {
		t.Fatalf("BUDGET_SECONDS=-1: %v reruns %v\n%s", res.outputs, res.log, res.summary)
	}
	// An unusable capture at the base (no package event, a nonzero exit) confirms nothing: a fresh
	// fixture whose base cannot run the package and whose merge fails it.
	fresh := &confirmRepo{t: t, dir: t.TempDir()}
	fresh.git("init", "-q", "-b", "main")
	writeFile(t, filepath.Join(fresh.dir, "go.mod"), []byte("module example.org/m\n\ngo 1.27.3\n"))
	writeFile(t, filepath.Join(fresh.dir, "store", "store.go"), []byte("package store\n"))
	writeFile(t, filepath.Join(fresh.dir, "store", "UNUSABLE"), nil)
	fresh.git("add", "-A")
	fresh.git("commit", "-q", "-m", "base")
	fresh.base = fresh.git("rev-parse", "HEAD")
	fresh.git("checkout", "-q", "-b", "feature")
	if err := os.Remove(filepath.Join(fresh.dir, "store", "UNUSABLE")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(fresh.dir, "store", "FAIL"), nil)
	fresh.git("add", "-A")
	fresh.git("commit", "-q", "-m", "feature")
	fresh.prTip = fresh.git("rev-parse", "HEAD")
	fresh.git("checkout", "-q", "main")
	fresh.git("merge", "-q", "--no-ff", "-m", "Merge pull request #12", fresh.prTip)
	fresh.merge = fresh.git("rev-parse", "HEAD")
	fresh.head = fresh.merge
	res = confirm(t, fresh, []any{doc{"pr": 12, "head": fresh.prTip, "merge": fresh.merge, "packages": []string{"example.org/m/store"}}})
	if res.outputs["check"] != "ci-forward-MAINCHECK-r500-"+fresh.head+"-e2-c0-k0-u1-pr12" || !strings.Contains(res.summary, "the base rerun at "+fresh.base+" is unusable") {
		t.Fatalf("unusable base: %v\n%s", res.outputs, res.summary)
	}
}

// CIN-V0-7922 (V1-5198 AC2): a confirmed main miss, uploaded as ci-forward-MISS-pr<N>-<merge>-e<E>-f1,
// is a MISS of the epoch: the bar is NOT MET and two of them trip the AFP-V0-030 kill switch; the
// suspicion (MAINMISS) and the check (MAINCHECK) names are reported beside it and feed neither.
func TestCINV07922ConfirmedMainMissIsAMissRow_CINV07922(t *testing.T) {
	requireGitHubChange(t, ".github/forward-rows/bar.jq", "CIN-V0-7922")
	sha := func(n int) string { return fmt.Sprintf("%040x", n) }
	suspect := "ci-forward-MAINMISS-r500-" + sha(500) + "-e1-n2-pr12_15"
	check := "ci-forward-MAINCHECK-r500-" + sha(500) + "-e1-c1-k1-u0-pr12_15"
	missRow := "ci-forward-MISS-pr12-" + sha(12) + "-e1-f1"
	v := barJSON(t, "1", append([]string{missRow, check, suspect}, metNames()...))
	if v["status"] != "NOT MET" || num(sub(v, "epochRows")["miss"]) != 1 || num(sub(v, "window")["miss"]) != 1 || v["killSwitch"] != "clear" {
		t.Fatalf("one confirmed miss: %v %v %v", v["status"], sub(v, "epochRows"), v["killSwitch"])
	}
	checks := sub(sub(v, "mainMiss"), "checks")
	if num(checks["runs"]) != 1 || num(checks["confirmed"]) != 1 || num(checks["flake"]) != 1 || num(checks["unconfirmed"]) != 0 || fmt.Sprint(checks["pullRequests"]) != "[12 15]" {
		t.Fatalf("mainMiss.checks = %v", checks)
	}
	if m := sub(v, "mainMiss"); num(m["misses"]) != 2 || num(m["runs"]) != 1 {
		t.Fatalf("mainMiss = %v", m)
	}
	// The suspicion and the check alone change nothing: MET, clear.
	if v := barJSON(t, "1", append([]string{check, suspect}, metNames()...)); v["status"] != "MET" || v["killSwitch"] != "clear" {
		t.Fatalf("suspicion and check without a MISS row: %v %v", v["status"], v["killSwitch"])
	}
	// A second confirmed miss in the window trips the switch; a re-run's check of the same run counts once.
	second := "ci-forward-MISS-pr15-" + sha(15) + "-e1-f1"
	newer := "ci-forward-MAINCHECK-r500-" + sha(500) + "-e1-c2-k0-u0-pr12_15"
	v = barJSON(t, "1", append([]string{newer, second, missRow, check, suspect}, metNames()...))
	if v["killSwitch"] != "TRIPPED" || num(sub(v, "window")["miss"]) != 2 || num(sub(sub(v, "mainMiss"), "checks")["confirmed"]) != 2 || num(sub(sub(v, "mainMiss"), "checks")["runs"]) != 1 {
		t.Fatalf("two confirmed misses: %v %v %v", v["killSwitch"], sub(v, "window"), sub(v, "mainMiss"))
	}
	md := run(t, nil, "sh", "-c", `printf '%s\n%s\n%s\n' "$1" "$2" "$3" | jq -R -s -r --arg epoch 1 -f "$4"`, "sh", missRow, check, suspect, script(t, "bar.jq"))
	if !strings.Contains(md, "Main full-run misses (AFP-V0-983, suspected; a miss confirmed by reruns is uploaded as a MISS row, CIN-V0-7922): 2 narrowed pull request(s) skipped a package a later full run of main failed, over 1 failed run(s): #12, #15; confirmation checks over 1 run(s): 1 confirmed, 1 flake, 0 unconfirmed.") {
		t.Fatalf("markdown:\n%s", md)
	}
}

// CIN-V0-7922 (V1-5198): main-miss.yml runs for this repository's failed full push runs of main
// only, links with the trusted mainmiss.sh, confirms with mainmiss-confirm.sh under the shards'
// -race build, test-confine wrapper and 50m deadline, retains the MAINMISS, MAINCHECK and up to
// three MISS names, files an issue and fails on a confirmed miss, with exactly actions: read,
// contents: read and issues: write; it names the live forward epoch, and the control plane no
// longer carries the job.
func TestCINV07922MainMissWorkflowBinding_CINV07922(t *testing.T) {
	requireGitHubChange(t, ".github/workflows/main-miss.yml", "")
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	wf, ci, cp := read("../../.github/workflows/main-miss.yml"), read("../../.github/workflows/ci.yml"), read("../../.github/workflows/ci-control-plane.yml")
	if strings.Contains(cp, "\n  main-miss:\n") || strings.Contains(cp, "mainmiss.sh") {
		t.Fatal("ci-control-plane.yml still carries the main-miss job")
	}
	one := func(text, pattern string) string {
		t.Helper()
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%d matches of %q, want 1", len(m), pattern)
		}
		return m[0][1]
	}
	if e, f := one(ci, `(?m)^      CORVINT_PR_FORWARD_EPOCH: "([0-9]+)"$`), one(wf, `(?m)^      FORWARD_EPOCH: "([0-9]+)"$`); e != f {
		t.Fatalf("main-miss.yml FORWARD_EPOCH %s differs from CORVINT_PR_FORWARD_EPOCH %s", f, e)
	}
	if one(wf, `(?m)^    permissions:\n((?:      .*\n)+)`) != "      actions: read\n      contents: read\n      issues: write\n" {
		t.Fatal("the miss job's permissions are not exactly actions: read, contents: read and issues: write")
	}
	for _, want := range []string{
		"    if: github.event.workflow_run.event == 'push' && github.event.workflow_run.head_branch == 'main' && github.event.workflow_run.head_repository.full_name == github.repository && github.event.workflow_run.conclusion == 'failure'\n",
		"          ref: ${{ github.sha }}\n          path: trusted\n          sparse-checkout: .github/forward-rows\n          persist-credentials: false\n",
		"        run: sh trusted/.github/forward-rows/mainmiss.sh\n",
		"          name: ${{ steps.miss.outputs.name }}\n          path: ${{ runner.temp }}/main-miss/miss.json\n",
		"          ref: ${{ github.event.workflow_run.head_sha }}\n          path: source\n          fetch-depth: 0\n          persist-credentials: false\n",
		`          GO_TEST_FLAGS="-race -exec $RUNNER_TEMP/test-confine" sh "$GITHUB_WORKSPACE/trusted/.github/forward-rows/mainmiss-confirm.sh"` + "\n",
		"          MISS: ${{ runner.temp }}/main-miss/miss.json\n          SOURCE: ${{ github.workspace }}/source\n",
		`          TEST_TIMEOUT: "50m"` + "\n",
		"          name: ${{ steps.confirm.outputs.check }}\n          path: ${{ runner.temp }}/main-miss/check.json\n",
		"          name: ${{ steps.confirm.outputs.miss1 }}\n          path: ${{ runner.temp }}/main-miss/miss1/row.json\n",
		"          name: ${{ steps.confirm.outputs.miss2 }}\n          path: ${{ runner.temp }}/main-miss/miss2/row.json\n",
		"          name: ${{ steps.confirm.outputs.miss3 }}\n          path: ${{ runner.temp }}/main-miss/miss3/row.json\n",
		"        if: always() && steps.confirm.outputs.confirmed == '1'\n",
		`            gh issue create -R "$REPO" --title "$title" --body "$body"` + "\n",
		"      - name: Fail on a confirmed miss\n        if: steps.confirm.outputs.confirmed == '1'\n",
		`          go build -trimpath -buildvcs=false -o "$RUNNER_TEMP/test-confine" ./cmd` + "\n",
	} {
		if !strings.Contains(wf, want) {
			t.Fatalf("main-miss.yml lacks %q", want)
		}
	}
	// The rerun deadline is the shards' (ci.yml run_slice); every step after the link runs only for
	// a named suspicion; the workflow never checks out or runs code of anything but main.
	if !strings.Contains(ci, `-count=1 -race -timeout 50m "$1" "$2" "$3"`) {
		t.Fatal("ci.yml's run_slice no longer runs with -timeout 50m; TEST_TIMEOUT in main-miss.yml must follow")
	}
	steps := regexp.MustCompile(`(?m)^      - (?:name: .*|uses: .*)\n(?:        .*\n)*`).FindAllString(wf, -1)
	if len(steps) < 12 {
		t.Fatalf("%d steps parsed", len(steps))
	}
	for i, s := range steps[3:] {
		if !strings.Contains(s, "steps.miss.outputs.name != ''") && !strings.Contains(s, "steps.confirm.outputs.") {
			t.Fatalf("step %d after the link is not gated on a named suspicion or the confirmation:\n%s", i+3, s)
		}
	}
	if strings.Contains(wf, "pull_request") || strings.Contains(wf, "head_repository.full_name != ") {
		t.Fatal("main-miss.yml reaches pull-request code")
	}
}
