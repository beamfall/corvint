// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// V1-1094: the reuse step, the advisory drift report, the qualification campaign, the
// ci-test-slices default (AFP-V0-041) and the forward-rows job (AFP-V0-047) name the
// go-product-shard count separately. A count that drifts from the matrix makes every main push
// FULL (no record name matches), every drift report abstain, a qualification bind another
// partition, a slice regeneration refuse the run's logs, and every forward row INCOMPLETE, all
// silently; so each must equal the matrix length.
func TestAFPV0024WorkflowShardCountsAgree(t *testing.T) {
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ci := read("../../.github/workflows/ci.yml")
	qualification := read("../../.github/workflows/pr-tests-qualification.yml")
	controlPlane := read("../../.github/workflows/ci-control-plane.yml")
	slices := read("../ci-test-slices/main.go")
	one := func(name, text, pattern string) string {
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%s: %d matches of %q, want 1", name, len(m), pattern)
		}
		return m[0][1]
	}
	matrix := strings.Split(one("matrix", ci, `(?m)^        shard: \[([0-9, ]+)\]$`), ", ")
	for i, s := range matrix {
		if s != strconv.Itoa(i) {
			t.Fatalf("matrix = %v, want 0..N-1", matrix)
		}
	}
	want := strconv.Itoa(len(matrix))
	for name, got := range map[string]string{
		"REUSE_SHARDS":            one("REUSE_SHARDS", ci, `(?m)^          REUSE_SHARDS: "([0-9]+)"$`),
		"ci-shard-cost-drift":     one("SHARDS", jobBlock(t, ci, "ci-shard-cost-drift"), `(?m)^      SHARDS: "([0-9]+)"$`),
		"ci-tested-tree-record":   one("SHARDS", jobBlock(t, ci, "ci-tested-tree-record"), `(?m)^      SHARDS: "([0-9]+)"$`),
		"qualification default":   one("qualification", qualification, `(?m)^      shards:\n(?:        .*\n)*?        default: "([0-9]+)"$`),
		"qualification CI remark": one("qualification", qualification, `CI uses ([0-9]+);`),
		"ci-test-slices default":  one("ci-test-slices", slices, `fs\.Int\("shards", ([0-9]+),`),
		"forward-rows shards":     one("FORWARD_SHARDS", controlPlane, `(?m)^      FORWARD_SHARDS: "([0-9]+)"$`),
	} {
		if got != want {
			t.Errorf("%s = %s, want the go-product-shard matrix length %s", name, got, want)
		}
	}
}

// V1-0760 and V1-1360: the reuse step decides for both the main push and the merge-queue
// entry, names its event to the tool, and merge-queue runs keep the tested-tree record (since
// AFP-V0-170 the record job's log) that a main push reuses. Losing any of these silently
// restores FULL on that path.
func TestAFPV0046WorkflowWiresQueueReuse(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)
	for _, want := range []string{
		"        id: reuse\n        if: github.event_name == 'push' || github.event_name == 'merge_group'\n",
		"      - name: Retain reuse decision\n        if: github.event_name == 'push' || github.event_name == 'merge_group'\n",
		`--event "$GITHUB_EVENT_NAME"`,
		`--target "$GITHUB_SHA"`,
		`list "head_sha=$GITHUB_SHA&event=merge_group" queue-runs.json`,
		`--jobs "$d/jobs" --logs "$d/logs"`,
		`if: ${{ !cancelled() && (github.event_name == 'pull_request' || github.event_name == 'merge_group') && needs.go-product-shard.result == 'success'`,
		`git -C "$src" merge-base --is-ancestor "$base" refs/remotes/origin/main`,
	} {
		if n := strings.Count(ci, want); n != 1 {
			t.Errorf("ci.yml has %d copies of %q, want 1", n, want)
		}
	}
}

// readWorkflow returns a workflow file of this repository.
func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../.github/workflows/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// jobBlock returns job JOB of workflow text W, from its two-space header to the next one.
func jobBlock(t *testing.T, w, job string) string {
	t.Helper()
	head := "\n  " + job + ":\n"
	i := strings.Index(w, head)
	if i < 0 || strings.Count(w, head) != 1 {
		t.Fatalf("want exactly one job %q", job)
	}
	rest := w[i+len(head):]
	if m := regexp.MustCompile(`\n  [a-z][a-z0-9-]*:\n`).FindStringIndex(rest); m != nil {
		rest = rest[:m[0]+1]
	}
	return rest
}

// wantLine fails unless BLOCK holds LINE exactly once as a whole line.
func wantLine(t *testing.T, name, block, line string) {
	t.Helper()
	if n := strings.Count("\n"+block, "\n"+line+"\n"); n != 1 {
		t.Errorf("%s has %d copies of line %q, want 1", name, n, line)
	}
}

// statusJobRuns evaluates the if of status job JOB in ci-control-plane.yml for a completed CI
// run of EVENT and CONCLUSION. It knows only &&-joined equality tests of those two payload
// fields, so any other condition shape fails the test rather than being guessed at.
func statusJobRuns(t *testing.T, w, job, event, conclusion string) bool {
	t.Helper()
	b := jobBlock(t, w, job)
	m := regexp.MustCompile(`(?m)^    if: (.+)$`).FindStringSubmatch(b)
	if m == nil {
		t.Fatalf("%s has no single-line if", job)
	}
	clause := regexp.MustCompile(`^github\.event\.workflow_run\.(event|conclusion) (==|!=) '([a-z_]+)'$`)
	payload := map[string]string{"event": event, "conclusion": conclusion}
	runs := true
	for _, c := range strings.Split(m[1], "&&") {
		p := clause.FindStringSubmatch(strings.TrimSpace(c))
		if p == nil {
			t.Fatalf("%s: unrecognised if clause %q; extend statusJobRuns", job, c)
		}
		runs = runs && (payload[p[1]] == p[3]) == (p[2] == "==")
	}
	return runs
}

// V1-1778 (AFP-V0-079): each status job runs for every completed CI run of its event, cancelled
// ones included, because GitHub can cancel a run after go-product passed; with no earlier
// ci-control-plane status on that head, a skipped job would leave its merge blocked for good.
// A newer completion for the same event and head replaces a queued or running job; losing the
// event in the group would let a pull-request job cancel a merge-queue post.
func TestAFPV0079ControlPlaneReportsEveryCompletedRun(t *testing.T) {
	w := readWorkflow(t, "ci-control-plane.yml")
	jobs := map[string]string{"pull_request": "control-plane", "merge_group": "merge-group"}
	for event, job := range jobs {
		b := jobBlock(t, w, job)
		wantLine(t, job, b, "      group: ci-control-plane-"+event+"-${{ github.event.workflow_run.head_sha }}")
		wantLine(t, job, b, "      cancel-in-progress: true")
		for _, conclusion := range []string{"success", "failure", "cancelled", "timed_out"} {
			for other, otherJob := range jobs {
				if got, want := statusJobRuns(t, w, otherJob, event, conclusion), other == event; got != want {
					t.Errorf("%s for a %s run that concluded %s: runs=%v, want %v", otherJob, event, conclusion, got, want)
				}
			}
		}
		if statusJobRuns(t, w, job, "push", "success") {
			t.Errorf("%s must not run for a push run", job)
		}
	}
	// forward-rows keeps one run per completed run: no group, success or failure only.
	fr := jobBlock(t, w, "forward-rows")
	if strings.Contains(fr, "concurrency:") {
		t.Error("forward-rows must keep one run per completed CI run")
	}
	wantLine(t, "forward-rows", fr, "      (github.event.workflow_run.conclusion == 'success' || github.event.workflow_run.conclusion == 'failure')")
}

// The cancelled-after-go-product case end to end: the run concluded cancelled, its head has no
// ci-control-plane status yet, and each status step, run with a stub gh, posts one. Neither step
// reads the conclusion, so the job condition above is the only thing that could suppress it.
func TestAFPV0079CancelledRunStillPostsTheStatus(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatalf("the status steps need bash on PATH: %v", err)
	}
	const head = "59cc560b9d98f746bba0282d18c0442e65b85a98"
	w := readWorkflow(t, "ci-control-plane.yml")
	for _, tc := range []struct{ job, event, branch, want string }{
		{"control-plane", "pull_request", "claude/x", "state=success"},
		{"merge-group", "merge_group", "gh-readonly-queue/main/pr-1144-" + head, "state=success"},
	} {
		if !statusJobRuns(t, w, tc.job, tc.event, "cancelled") {
			t.Errorf("%s does not run for a cancelled %s run", tc.job, tc.event)
			continue
		}
		dir := t.TempDir()
		script := filepath.Join(dir, "step.sh")
		if err := os.WriteFile(script, []byte(runScript(t, jobBlock(t, w, tc.job))), 0o600); err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		// A stub gh: one open PR (#1144) at HEAD that changes two files outside .github, printed
		// as the step's --jq filters would; a status POST is appended to posts.log.
		stub := `#!/bin/sh
[ "$1" = api ] || exit 2
shift
case " $* " in
  *" -X POST "*) echo "$*" >>"$STUB_DIR/posts.log"; exit 0 ;;
  *"repos/$REPO/pulls?state=open&per_page=100"*) echo 1144 ;;
  *"repos/$REPO/pulls/1144/files?per_page=100"*) printf 'README.md\ndocs/x.md\n' ;;
  *"repos/$REPO/pulls/1144 "*) echo 2 ;;
  *) exit 22 ;;
esac
`
		if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"STUB_DIR="+dir, "REPO=beamfall/corvint", "HEAD="+head, "BRANCH="+tc.branch,
			"RUN_URL=https://github.com/beamfall/corvint/actions/runs/1", "GH_TOKEN=stub")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s step: %v\n%s", tc.job, err, out)
		}
		posts, err := os.ReadFile(filepath.Join(dir, "posts.log"))
		if err != nil {
			t.Fatalf("%s posted no status for a cancelled run: %v", tc.job, err)
		}
		if p := string(posts); strings.Count(p, "\n") != 1 || !strings.Contains(p, "repos/beamfall/corvint/statuses/"+head) ||
			!strings.Contains(p, tc.want) || !strings.Contains(p, "context=ci-control-plane") {
			t.Errorf("%s posted %q, want one ci-control-plane %s on %s", tc.job, p, tc.want, head)
		}
	}
}

// V1-1642: the static trio of a main push is skipped only through the static-reuse decision,
// and go-product accepts a skipped go-static only on a push that decided REUSE.
func TestAFPV0080WorkflowWiresPushStaticReuse(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	sr := jobBlock(t, ci, "static-reuse")
	wantLine(t, "static-reuse", sr, "    if: github.event_name == 'push' && github.ref == 'refs/heads/main'")
	wantLine(t, "static-reuse", sr, "        continue-on-error: true")
	if strings.Contains(sr, "permissions:") || strings.Contains(sr, "actions/checkout") {
		t.Error("static-reuse must read listings only, with the workflow's permissions")
	}
	cond := "    if: ${{ !cancelled() && !contains(github.event.pull_request.labels.*.name, 'ci:batched') && needs.static-reuse.outputs.mode != 'REUSE' }}"
	for _, job := range []string{"go-static", "go-interop", "artifact-integrity"} {
		b := jobBlock(t, ci, job)
		wantLine(t, job, b, "    needs: static-reuse")
		wantLine(t, job, b, cond)
	}
	gp := jobBlock(t, ci, "go-product")
	wantLine(t, "go-product", gp, "    needs: [go-product-shard, go-static, docs-plan, static-reuse]")
	wantLine(t, "go-product", gp, `          if [ "$GITHUB_EVENT_NAME" = push ] && [ "$STATIC_REUSE" = REUSE ] && [ "$STATIC_RESULT" = skipped ]; then`)
	wantLine(t, "go-product", gp, `            test "$STATIC_RESULT" = success`)
	wantLine(t, "go-product", gp, `          test "$SHARDS_RESULT" = success`)
}

// staticReuseScript extracts the run script of the static-reuse step.
func staticReuseScript(t *testing.T) string {
	t.Helper()
	return runScript(t, jobBlock(t, readWorkflow(t, "ci.yml"), "static-reuse"))
}

// runScript extracts the run script of the single run step of job block B.
func runScript(t *testing.T, b string) string {
	t.Helper()
	if strings.Count(b, "        run: |\n") != 1 {
		t.Fatal("want exactly one run script in the job")
	}
	_, body, _ := strings.Cut(b, "        run: |\n")
	var out []string
	for _, line := range strings.Split(body, "\n") {
		if line != "" && !strings.HasPrefix(line, "          ") {
			break
		}
		out = append(out, strings.TrimPrefix(line, "          "))
	}
	return strings.Join(out, "\n")
}

// V1-1642 fail-closed: the step decides REUSE only for a successful merge-queue run of this exact
// commit, branch and repository whose latest go-static, artifact-integrity and go-interop jobs all
// passed on that commit; every other listing, and every unreadable one, leaves mode empty.
func TestAFPV0080StaticReuseDecidesOnlyForAPassedQueueRunOfThisCommit(t *testing.T) {
	for _, tool := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the static-reuse step needs %s on PATH: %v", tool, err)
		}
	}
	const sha = "59cc560b9d98f746bba0282d18c0442e65b85a98"
	const repo = "beamfall/corvint"
	script := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(script, []byte(staticReuseScript(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(edit func(r map[string]any), jobs []map[string]any, env ...string) (string, string) {
		dir := t.TempDir()
		r := map[string]any{"id": 39162222036, "head_sha": sha, "event": "merge_group", "conclusion": "success",
			"path": ".github/workflows/ci.yml", "head_branch": "gh-readonly-queue/main/pr-1139-" + sha,
			"head_repository": map[string]any{"full_name": repo}}
		if edit != nil {
			edit(r)
		}
		writeJSON(t, filepath.Join(dir, "runs.json"), map[string]any{"workflow_runs": []any{r}})
		if jobs != nil {
			writeJSON(t, filepath.Join(dir, "jobs-39162222036.json"), map[string]any{"jobs": jobs})
		}
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		// A stub curl: the runs listing, or the jobs listing of one run, from the fixture dir;
		// anything else, or an authenticated call when STUB_DENY_AUTH is set, fails as curl -f does.
		stub := `#!/bin/sh
out= url= auth=
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out=$2; shift ;;
    -H) case "$2" in Authorization:*) auth=1 ;; esac; shift ;;
    -*) ;;
    *) url=$1 ;;
  esac
  shift
done
[ -n "$auth" ] && [ -n "${STUB_DENY_AUTH:-}" ] && exit 22
case "$url" in
  "$STUB_API/workflows/ci.yml/runs?head_sha=$STUB_SHA&event=merge_group&status=success&per_page=20") f=runs.json ;;
  "$STUB_API/runs/"*"/jobs?filter=latest&per_page=100") id=${url#"$STUB_API/runs/"}; f=jobs-${id%%/*}.json ;;
  *) exit 22 ;;
esac
[ -f "$STUB_DIR/$f" ] || exit 22
cat "$STUB_DIR/$f" > "$out"
`
		if err := os.WriteFile(filepath.Join(bin, "curl"), []byte(stub), 0o700); err != nil {
			t.Fatal(err)
		}
		output, summary := filepath.Join(dir, "output"), filepath.Join(dir, "summary")
		cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", script)
		cmd.Env = append([]string{
			"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
			"GITHUB_API_URL=https://api.github.test", "GITHUB_REPOSITORY=" + repo, "GITHUB_SHA=" + sha,
			"RUNNER_TEMP=" + dir, "GITHUB_OUTPUT=" + output, "GITHUB_STEP_SUMMARY=" + summary,
			"GH_TOKEN=t", "STUB_DIR=" + dir, "STUB_SHA=" + sha,
			"STUB_API=https://api.github.test/repos/" + repo + "/actions",
		}, env...)
		_ = cmd.Run() // a nonzero exit is continue-on-error in the workflow; only the output binds
		o, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		return string(o), string(s)
	}
	job := func(name, conclusion, headSHA string) map[string]any {
		return map[string]any{"name": name, "conclusion": conclusion, "head_sha": headSHA}
	}
	passed := func() []map[string]any {
		return []map[string]any{job("docs-plan", "success", sha), job("go-static", "success", sha),
			job("artifact-integrity", "success", sha), job("go-interop", "success", sha)}
	}
	out, summary := run(nil, passed())
	if out != "mode=REUSE\n" || !strings.Contains(summary, "merge-queue run 39162222036; not rerun") {
		t.Fatalf("a passed queue run of this commit: output %q summary %q, want REUSE", out, summary)
	}
	if out, _ := run(nil, passed(), "STUB_DENY_AUTH=1"); out != "mode=REUSE\n" {
		t.Errorf("a refused token must retry the public listing anonymously: output %q", out)
	}
	for name, c := range map[string]struct {
		edit func(map[string]any)
		jobs func() []map[string]any
	}{
		"other commit":            {func(r map[string]any) { r["head_sha"] = strings.Repeat("0", 40) }, passed},
		"pull_request run":        {func(r map[string]any) { r["event"] = "pull_request" }, passed},
		"failed run":              {func(r map[string]any) { r["conclusion"] = "failure" }, passed},
		"other workflow":          {func(r map[string]any) { r["path"] = ".github/workflows/other.yml" }, passed},
		"fork repository":         {func(r map[string]any) { r["head_repository"] = map[string]any{"full_name": "fork/corvint"} }, passed},
		"not a queue branch":      {func(r map[string]any) { r["head_branch"] = "main" }, passed},
		"string id":               {func(r map[string]any) { r["id"] = "39162222036" }, passed},
		"jobs unreadable":         {nil, nil},
		"go-interop failed":       {nil, func() []map[string]any { j := passed(); j[3]["conclusion"] = "failure"; return j }},
		"go-static skipped":       {nil, func() []map[string]any { j := passed(); j[1]["conclusion"] = "skipped"; return j }},
		"artifact-integrity gone": {nil, func() []map[string]any { return passed()[:2] }},
		"duplicate job":           {nil, func() []map[string]any { return append(passed(), job("go-static", "success", sha)) }},
		"job on another commit":   {nil, func() []map[string]any { j := passed(); j[2]["head_sha"] = strings.Repeat("1", 40); return j }},
	} {
		var jobs []map[string]any
		if c.jobs != nil {
			jobs = c.jobs()
		}
		if out, _ := run(c.edit, jobs); out != "" {
			t.Errorf("%s: output %q, want no mode (the jobs run in full)", name, out)
		}
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// AFP-V0-081: a ci:batched constituent starts none of the jobs its batch pull request and queue
// entry rerun in full, but keeps the cheap documentation gates and the required go-product
// failure that keeps it from merging alone.
func TestAFPV0081BatchedConstituentSkipsOnlyRerunJobs(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	const batched = "!contains(github.event.pull_request.labels.*.name, 'ci:batched')"
	for _, job := range []string{"go-product-shard", "go-static", "go-interop", "artifact-integrity", "ci-shard-cost-drift"} {
		if !strings.Contains(jobBlock(t, ci, job), "    if: ${{ ") || !strings.Contains(jobBlock(t, ci, job), batched) {
			t.Errorf("%s must not start for a ci:batched constituent", job)
		}
	}
	for _, job := range []string{"doc-gates", "docs-plan", "go-product"} {
		if strings.Contains(jobBlock(t, ci, job), "'ci:batched'") {
			t.Errorf("%s must still run for a ci:batched constituent", job)
		}
	}
	wantLine(t, "go-product", jobBlock(t, ci, "go-product"), "    if: always()")
	wantLine(t, "containment", jobBlock(t, readWorkflow(t, "intake-containment.yml"), "containment"), "    if: ${{ "+batched+" }}")
}

// V1-0728: a main push run is grouped by its own commit, so a later merge never cancels it,
// while a pull request still cancels its superseded run.
func TestAFPV0082PushConcurrencyGroupsByCommit(t *testing.T) {
	wantLine(t, "ci.yml", readWorkflow(t, "ci.yml"),
		"  group: ${{ github.workflow }}-${{ github.event.pull_request.number || (github.event_name == 'push' && github.sha) || github.ref }}")
}
