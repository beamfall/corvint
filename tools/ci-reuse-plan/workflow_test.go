// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
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
		"go-product coverage":     one("SHARDS", jobBlock(t, ci, "go-product"), `(?m)^          SHARDS: "([0-9]+)"$`),
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
// run of EVENT and CONCLUSION; LISTED says whether the run's head is the head of a pull request
// the payload lists (AFP-V0-855). It knows only equality tests of those two payload fields, that
// one listed-head test, parentheses, && and ||, so any other condition shape fails the test
// rather than being guessed at.
func statusJobRuns(t *testing.T, w, job, event, conclusion string, listed bool) bool {
	t.Helper()
	b := jobBlock(t, w, job)
	m := regexp.MustCompile(`(?m)^    if: (.+)$`).FindStringSubmatch(b)
	if m == nil {
		t.Fatalf("%s has no single-line if", job)
	}
	clause := regexp.MustCompile(`^github\.event\.workflow_run\.(event|conclusion) (==|!=) '([a-z_]+)'$`)
	const listedHead = "contains(github.event.workflow_run.pull_requests.*.head.sha, github.event.workflow_run.head_sha)"
	payload := map[string]string{"event": event, "conclusion": conclusion}
	var eval func(expr string) bool
	// split cuts EXPR at top-level occurrences of OP, outside parentheses.
	split := func(expr, op string) []string {
		var parts []string
		depth, last := 0, 0
		for i := 0; i < len(expr); i++ {
			switch {
			case expr[i] == '(':
				depth++
			case expr[i] == ')':
				depth--
			case depth == 0 && strings.HasPrefix(expr[i:], op):
				parts = append(parts, expr[last:i])
				last = i + len(op)
			}
		}
		return append(parts, expr[last:])
	}
	eval = func(expr string) bool {
		expr = strings.TrimSpace(expr)
		if parts := split(expr, "||"); len(parts) > 1 {
			anyTrue := false
			for _, p := range parts {
				anyTrue = eval(p) || anyTrue
			}
			return anyTrue
		}
		if parts := split(expr, "&&"); len(parts) > 1 {
			allTrue := true
			for _, p := range parts {
				allTrue = eval(p) && allTrue
			}
			return allTrue
		}
		if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
			return eval(expr[1 : len(expr)-1])
		}
		if expr == listedHead {
			return listed
		}
		p := clause.FindStringSubmatch(expr)
		if p == nil {
			t.Fatalf("%s: unrecognised if clause %q; extend statusJobRuns", job, expr)
		}
		return (payload[p[1]] == p[3]) == (p[2] == "==")
	}
	return eval(m[1])
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
			for _, listed := range []bool{true, false} {
				for other, otherJob := range jobs {
					// AFP-V0-855: only a cancelled pull-request run whose head no listed pull
					// request has is left without a control-plane job.
					want := other == event && !(otherJob == "control-plane" && conclusion == "cancelled" && !listed)
					if got := statusJobRuns(t, w, otherJob, event, conclusion, listed); got != want {
						t.Errorf("%s for a %s run that concluded %s (head listed %v): runs=%v, want %v", otherJob, event, conclusion, listed, got, want)
					}
				}
			}
		}
		if statusJobRuns(t, w, job, "push", "success", true) {
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
	testplatform.SkipPOSIXShell(t, "the status step runs under bash with a #!/bin/sh gh spy")
	if _, err := exec.LookPath("bash"); err != nil {
		t.Fatalf("the status steps need bash on PATH: %v", err)
	}
	const head = "59cc560b9d98f746bba0282d18c0442e65b85a98"
	w := readWorkflow(t, "ci-control-plane.yml")
	for _, tc := range []struct{ job, event, branch, want string }{
		{"control-plane", "pull_request", "claude/x", "state=success"},
		{"merge-group", "merge_group", "gh-readonly-queue/main/pr-1144-" + head, "state=success"},
	} {
		if !statusJobRuns(t, w, tc.job, tc.event, "cancelled", true) {
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

// V1-1778 (AFP-V0-855): a pull-request run cancelled because a newer push superseded its head
// starts no control-plane job, whichever way GitHub lists the pull request (by head SHA, so none
// is listed, or by branch, so its newer head is). The clause names the run's own head, so a
// listed pull request at that head still starts the job, and merge-group is untouched.
func TestAFPV0855SupersededCancelledRunStartsNoJob(t *testing.T) {
	w := readWorkflow(t, "ci-control-plane.yml")
	if statusJobRuns(t, w, "control-plane", "pull_request", "cancelled", false) {
		t.Error("control-plane starts for a superseded cancelled pull-request run")
	}
	if !statusJobRuns(t, w, "control-plane", "pull_request", "cancelled", true) {
		t.Error("control-plane must still start for a cancelled run whose head is a listed pull request's head")
	}
	if !statusJobRuns(t, w, "merge-group", "merge_group", "cancelled", false) {
		t.Error("merge-group must keep starting for a cancelled queue run")
	}
	b := jobBlock(t, w, "control-plane")
	if !strings.Contains(b, "contains(github.event.workflow_run.pull_requests.*.head.sha, github.event.workflow_run.head_sha)") {
		t.Error("control-plane must compare the listed pull requests' heads with the run's own head")
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
	wantLine(t, "go-product", gp, `            test "$SHARDS_RESULT" = success`)
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
	testplatform.SkipPOSIXShell(t, "the static-reuse step runs under bash with a #!/bin/sh gh spy")
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
// check, which the merge queue's full run keeps from landing it untested (AFP-V0-027).
func TestAFPV0081BatchedConstituentSkipsOnlyRerunJobs(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	const batched = "!contains(github.event.pull_request.labels.*.name, 'ci:batched')"
	for _, job := range []string{"go-product-shard", "go-static", "go-interop", "artifact-integrity", "ci-shard-cost-drift"} {
		if !strings.Contains(jobBlock(t, ci, job), "    if: ${{ ") || !strings.Contains(jobBlock(t, ci, job), batched) {
			t.Errorf("%s must not start for a ci:batched constituent", job)
		}
	}
	for _, job := range []string{"doc-gates", "docs-plan", "go-product"} {
		// Only the job condition decides whether it runs; go-product reads the label in its step.
		if cond := regexp.MustCompile(`(?m)^    if: .*$`).FindString(jobBlock(t, ci, job)); strings.Contains(cond, "'ci:batched'") {
			t.Errorf("%s must still run for a ci:batched constituent", job)
		}
	}
	wantLine(t, "go-product", jobBlock(t, ci, "go-product"), "    if: always()")
	wantLine(t, "containment", jobBlock(t, readWorkflow(t, "intake-containment.yml"), "containment"), "    if: ${{ "+batched+" }}")
}

// AFP-V0-027 (V1-2265): go-product passes, with a notice, only for a pull_request labelled
// ci:batched whose shards and go-static were both skipped; every other skipped or failed shard or
// static result still fails it, and the label never stands in for a failed docs-plan.
func TestAFPV0027BatchedSkipPassesGoProductWithANotice(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the go-product step runs under bash")
	ci := readWorkflow(t, "ci.yml")
	b := jobBlock(t, ci, "go-product")
	wantLine(t, "go-product", b,
		"          BATCHED: ${{ github.event_name == 'pull_request' && contains(github.event.pull_request.labels.*.name, 'ci:batched') }}")
	script := runScript(t, b)
	run := func(batched, shards, static, docs string) (bool, string) {
		t.Helper()
		dir := t.TempDir()
		// Every shard's AFP-V0-591 coverage marker of a full run (TestAFPV0591CoverageGuard).
		for i := 0; i < 8; i++ {
			marker := fmt.Sprintf(`{"profile":"corvint-pr-coverage/0","runId":"7","runAttempt":"1","shard":%d,"shards":8,"placement":"PARTITION","selected":[]}`, i)
			path := filepath.Join(dir, "ci-coverage", fmt.Sprintf("ci-coverage-%d", i), "coverage.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(marker), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("bash", "-eo", "pipefail", "-c", script)
		cmd.Env = append(os.Environ(), "BATCHED="+batched, "SHARDS_RESULT="+shards, "STATIC_RESULT="+static,
			"STATIC_REUSE=", "DOCS_RESULT="+docs, "DOCS_MODE=FULL", "SHARDS=8", "RUNNER_TEMP="+dir, "GITHUB_RUN_ID=7",
			"GITHUB_RUN_ATTEMPT=1", "GITHUB_EVENT_NAME=pull_request", "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
		out, err := cmd.CombinedOutput()
		return err == nil, string(out)
	}
	if ok, out := run("true", "skipped", "skipped", "success"); !ok || !strings.Contains(out, "::notice::") || !strings.Contains(out, "ci:batched") {
		t.Errorf("a ci:batched skip must pass with a notice naming the label; ok=%v\n%s", ok, out)
	}
	for name, c := range map[string][4]string{
		"unlabelled skipped shards":       {"false", "skipped", "skipped", "success"},
		"batched with a failed docs-plan": {"true", "skipped", "skipped", "failure"},
		"batched but go-static ran":       {"true", "skipped", "success", "success"},
		"batched but shards failed":       {"true", "failure", "skipped", "success"},
		"batched but shards cancelled":    {"true", "cancelled", "skipped", "success"},
		"unlabelled failed shards":        {"false", "failure", "success", "success"},
	} {
		if ok, out := run(c[0], c[1], c[2], c[3]); ok {
			t.Errorf("%s: go-product passed\n%s", name, out)
		}
	}
	if ok, out := run("false", "success", "success", "success"); !ok {
		t.Errorf("a full pass must pass go-product\n%s", out)
	}
	// The shards of the queue run that must test the constituent: the label is read from the
	// pull_request payload only, so it is off for merge_group.
	if !strings.Contains(jobBlock(t, ci, "go-product-shard"), "!contains(github.event.pull_request.labels.*.name, 'ci:batched')") {
		t.Error("go-product-shard must read the label from the pull_request payload")
	}
	// A skipped shard job never prints the AFP-V0-170 tree record the queue's REUSE needs.
	if !strings.Contains(jobBlock(t, ci, "ci-tested-tree-record"), "needs.go-product-shard.result == 'success'") {
		t.Error("ci-tested-tree-record must require passed shards")
	}
}

// AFP-V0-380 (V1-1103): interop/cem01-go's tests and vet run exactly once per CI run, in
// go-static, which the required go-product check needs; go-interop keeps only the formatting and
// Windows build checks that go-static does not run for that module.
func TestAFPV0380InteropTestsRunOnceInGoStatic(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	static := jobBlock(t, ci, "go-static")
	_, step, ok := strings.Cut(static, "        working-directory: source/interop/cem01-go\n")
	if !ok {
		t.Fatal("go-static has no interop/cem01-go step")
	}
	script := runScript(t, step)
	// No literal here ends in a package pattern, which would make this package an unbounded
	// reader (AFP-V0-012 rule (d)).
	if lines := strings.Split(strings.TrimSpace(script), "\n"); len(lines) != 2 ||
		!strings.HasPrefix(lines[0], "go test -count=1 -timeout 30m ") || !strings.HasPrefix(lines[1], "go vet ") {
		t.Errorf("go-static interop step = %q, want its tests and vet", script)
	}
	interop := runScript(t, jobBlock(t, ci, "go-interop"))
	if strings.Contains(interop, "go test") || strings.Contains(interop, "go vet") {
		t.Errorf("go-interop reruns interop/cem01-go tests or vet:\n%s", interop)
	}
	for _, line := range []string{`test -z "$(gofmt -l .)"`, "GOOS=windows GOARCH=amd64 go build "} {
		if !strings.Contains(interop, line) {
			t.Errorf("go-interop lost %q", line)
		}
	}
	if !strings.Contains(regexp.MustCompile(`(?m)^    needs: .*$`).FindString(jobBlock(t, ci, "go-product")), "go-static") {
		t.Error("go-product must need go-static, which owns the interop tests")
	}
}

// TestCINV0008LocalAuthorityModuleTestsRunInGoStatic (V1-3329): go-static, which starts on every
// path that starts the shards and which go-product needs, is the one CI run of the nested
// tools/local-authority module's tests and vet. Its external modules are fetched once, verified
// against the module's go.sum under -mod=readonly; the tests and vet then run under the job's
// GOPROXY=off. No other job runs that module.
func TestCINV0008LocalAuthorityModuleTestsRunInGoStatic(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	static := jobBlock(t, ci, "go-static")
	step := stepBlock(t, static, "Nested module tools/local-authority tests and vet (CIN-V0-008)")
	wantLine(t, "go-static", step, "        working-directory: source/tools/local-authority")
	for _, env := range []string{`      GOPROXY: "off"`, `      GOSUMDB: "off"`} {
		wantLine(t, "go-static", static, env)
	}
	// No literal here ends in a package pattern, which would make this package an unbounded
	// reader (AFP-V0-012 rule (d)).
	lines := strings.Split(strings.TrimSpace(runScript(t, step)), "\n")
	if len(lines) != 3 ||
		lines[0] != "GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOFLAGS=-mod=readonly go mod download" ||
		!strings.HasPrefix(lines[1], "go test -count=1 -timeout 30m ") || !strings.HasPrefix(lines[2], "go vet ") ||
		strings.Contains(lines[1], "GOPROXY") || strings.Contains(lines[2], "GOPROXY") {
		t.Errorf("go-static local-authority step = %q, want a verified download, then its tests and vet offline", lines)
	}
	if n := strings.Count(ci, "working-directory: source/tools/local-authority\n"); n != 1 {
		t.Errorf("tools/local-authority runs in %d steps, want only go-static's", n)
	}
	if !strings.Contains(regexp.MustCompile(`(?m)^    needs: .*$`).FindString(jobBlock(t, ci, "go-product")), "go-static") {
		t.Error("go-product must need go-static, which owns the tools/local-authority tests")
	}
}

// V1-0728: a main push run is grouped by its own commit, so a later merge never cancels it,
// while a pull request still cancels its superseded run.
func TestAFPV0082PushConcurrencyGroupsByCommit(t *testing.T) {
	wantLine(t, "ci.yml", readWorkflow(t, "ci.yml"),
		"  group: ${{ github.workflow }}-${{ github.event.pull_request.number || (github.event_name == 'push' && github.sha) || github.ref }}")
}

// AFP-V0-163 (V1-2113): a merge-queue entry decides integrity reuse before the gate and skips
// only the gate's steps on REUSE (so the job still concludes success for the AFP-V0-080 push
// reuse). The decision reads only the jobs listing and one job log, never an artifact, because
// an artifact name does not authenticate the job that uploaded it.
func TestAFPV0163WorkflowWiresIntegrityTreeReuse(t *testing.T) {
	w := readWorkflow(t, "ci.yml")
	ai := jobBlock(t, w, "artifact-integrity")
	for _, line := range []string{
		"    permissions:",
		"      actions: read",
		"      contents: read",
		"          fetch-depth: 2",
		"        id: reuse",
		"        if: github.event_name == 'merge_group'",
		"        if: steps.reuse.outputs.mode == 'REUSE'",
		"          name: ci-integrity-reuse",
		"        run: make go-archive-gate",
	} {
		wantLine(t, "artifact-integrity", ai, line)
	}
	if n := strings.Count(ai, "        if: steps.reuse.outputs.mode != 'REUSE'\n"); n != 2 {
		t.Errorf("setup-go and the archive gate must both be skipped only on REUSE: %d conditions", n)
	}
	if r, gate := strings.Index(ai, "id: reuse"), strings.Index(ai, "run: make go-archive-gate"); r < 0 || r > gate {
		t.Error("the reuse decision must precede the gate")
	}
	reuse := stepScript(t, ai, "reuse")
	for _, want := range []string{
		`get "$api/runs/$id/jobs?filter=latest&per_page=100" "$d/jobs-$id.json" || continue`,
		`select(.name == "artifact-integrity" or .name == "artifact-integrity-record")`,
		`get "$api/jobs/$job/logs" "$d/log-$job.txt" || continue`,
		`test "$n" = 1 || continue`,
	} {
		if !strings.Contains(reuse, want) {
			t.Errorf("the reuse step must hold %q", want)
		}
	}
	// No artifact is ever evidence: the step lists none, and no job names one after a tree.
	if strings.Contains(reuse, "artifacts") || strings.Contains(w, "ci-tested-tree-$tree-artifact-integrity") ||
		strings.Contains(w, "-artifact-integrity\n") {
		t.Error("integrity reuse must not read or name a tree-named artifact")
	}
	if n := strings.Count(reuse, "$api/"); n != 3 {
		t.Errorf("the reuse step reads %d API paths, want the run listing, the jobs listing and the record job log", n)
	}
}

// AFP-V0-163: the record job, the only evidence the queue accepts, runs only in a pull-request run
// after artifact-integrity passed there, persists no credentials, and runs no repository code: a checkout
// without credentials, then one script of shell builtins, git and printf.
func TestAFPV0163RecordJobRunsNoRepositoryCode(t *testing.T) {
	rec := jobBlock(t, readWorkflow(t, "ci.yml"), "artifact-integrity-record")
	for _, line := range []string{
		"    needs: artifact-integrity",
		"    if: ${{ !cancelled() && github.event_name == 'pull_request' && needs.artifact-integrity.result == 'success' }}",
		"          persist-credentials: false",
		"          PR_HEAD: ${{ github.event.pull_request.head.sha }}",
	} {
		wantLine(t, "artifact-integrity-record", rec, line)
	}
	if n := strings.Count(rec, "\n      - "); n != 2 || !strings.Contains(rec, "\n      - uses: actions/checkout@") || strings.Count(rec, "uses:") != 1 {
		t.Errorf("the record job must be exactly a checkout and one run step: %d steps", n)
	}
	for _, banned := range []string{"github.token", "secrets.", "GH_TOKEN", "permissions:", "services:", "container:", "shell:", "working-directory:"} {
		if strings.Contains(rec, banned) {
			t.Errorf("the record job must not use %q", banned)
		}
	}
	allowed := regexp.MustCompile(`^(set -eu|test "\$\(git rev-parse [^;&|<>]*\)" = "\$(GITHUB_SHA|PR_HEAD)"|tree=\$\(git rev-parse --verify -q "\$GITHUB_SHA\^\{tree\}"\)|printf '[^']*' "\$tree" "\$GITHUB_SHA" "\$PR_HEAD")$`)
	script := stepScript(t, rec, "record")
	for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
		if !allowed.MatchString(line) {
			t.Errorf("the record script may run only builtins, git and printf: %q", line)
		}
	}
}

// V1-2113 fail-closed: REUSE only for a successful pull-request run of the queue commit's second
// parent whose latest artifact-integrity and artifact-integrity-record jobs both passed on that
// head, and whose record job log holds exactly one record line naming this tree and head. The log
// served here is the record script's own output, so the two steps agree on the line. Every other
// listing or log, an unreadable one, or a non-merge commit leaves mode empty and the gate runs.
func TestAFPV0163IntegrityReuseDecidesOnlyForAPassedPullRequestRunOfThisTree(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the record and reuse steps run under bash with a #!/bin/sh gh spy")
	for _, tool := range []string{"bash", "jq", "git", "grep", "sed", "tr"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the integrity reuse step needs %s on PATH: %v", tool, err)
		}
	}
	const repo = "beamfall/corvint"
	const runID, jobID = 39472511845, 112233445566
	w := readWorkflow(t, "ci.yml")
	scripts := t.TempDir()
	script, recordScript := filepath.Join(scripts, "step.sh"), filepath.Join(scripts, "record.sh")
	if err := os.WriteFile(script, []byte(stepScript(t, jobBlock(t, w, "artifact-integrity"), "reuse")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordScript, []byte(stepScript(t, jobBlock(t, w, "artifact-integrity-record"), "record")), 0o600); err != nil {
		t.Fatal(err)
	}
	// A two-parent queue commit (main + pull-request head) and a single-parent commit.
	work := t.TempDir()
	gitEnv := []string{"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test",
		"PATH=" + os.Getenv("PATH"), "HOME=" + work}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir, cmd.Env = work, gitEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	git("commit", "-q", "--allow-empty", "-m", "base")
	git("checkout", "-q", "-b", "pr")
	if err := os.WriteFile(filepath.Join(work, "f"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "f")
	git("commit", "-q", "-m", "pr")
	head := git("rev-parse", "HEAD")
	git("checkout", "-q", "main")
	git("commit", "-q", "--allow-empty", "-m", "main moved")
	git("merge", "-q", "--no-ff", "-m", "queue", "pr")
	merge, tree := git("rev-parse", "HEAD"), git("rev-parse", "HEAD^{tree}")

	// The pull-request run's record job prints its line; GitHub prefixes each log line with a
	// timestamp and ends it with CRLF.
	cmd := exec.Command("bash", "--noprofile", "--norc", "-eo", "pipefail", recordScript)
	cmd.Dir, cmd.Env = work, append(append([]string{}, gitEnv...), "GITHUB_SHA="+merge, "PR_HEAD="+head)
	printed, err := cmd.Output()
	if err != nil {
		t.Fatalf("the record script: %v", err)
	}
	line := strings.TrimSuffix(string(printed), "\n")
	if strings.Count(line, "\n") != 0 || !strings.Contains(line, " tree="+tree+" ") || !strings.HasSuffix(line, " head="+head) {
		t.Fatalf("the record script printed %q, want one line naming tree %s and head %s", printed, tree, head)
	}
	ts := "2026-10-21T10:00:00.1234567Z "
	logOf := func(lines ...string) string {
		var b strings.Builder
		b.WriteString(ts + "##[group]Run set -eu\r\n" + ts + "printf 'corvint-ci-integrity-tree/0 tree=%s target=%s head=%s\\n' \"$tree\" \"$GITHUB_SHA\" \"$PR_HEAD\"\r\n" + ts + "##[endgroup]\r\n")
		for _, l := range lines {
			b.WriteString(ts + l + "\r\n")
		}
		return b.String()
	}
	other := func(c string) string { return strings.Repeat(c, 40) }

	run := func(target string, edit func(r, ai, rec map[string]any, log *string), env ...string) (string, string) {
		git("checkout", "-q", target)
		dir := t.TempDir()
		r := map[string]any{"id": runID, "head_sha": head, "event": "pull_request", "conclusion": "success",
			"path": ".github/workflows/ci.yml", "repository": map[string]any{"full_name": repo}}
		ai := map[string]any{"id": 1, "run_id": runID, "name": "artifact-integrity", "conclusion": "success", "head_sha": head}
		rec := map[string]any{"id": jobID, "run_id": runID, "name": "artifact-integrity-record", "conclusion": "success", "head_sha": head}
		log := logOf(line)
		if edit != nil {
			edit(r, ai, rec, &log)
		}
		jobs := []any{map[string]any{"id": 2, "run_id": runID, "name": "go-static", "conclusion": "success", "head_sha": head}}
		for _, j := range []map[string]any{ai, rec} {
			if j["drop"] != true {
				jobs = append(jobs, j)
			}
			if j["dup"] == true {
				jobs = append(jobs, j)
			}
		}
		writeJSON(t, filepath.Join(dir, "runs.json"), map[string]any{"workflow_runs": []any{r}})
		if rec["unreadable"] != true {
			writeJSON(t, filepath.Join(dir, "jobs.json"), map[string]any{"jobs": jobs})
		}
		if log != "unreadable" {
			if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte(log), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		// A forged tree-named artifact is on offer too; the step must never ask for it.
		writeJSON(t, filepath.Join(dir, "artifacts.json"), map[string]any{"artifacts": []any{map[string]any{
			"name": "ci-tested-tree-" + tree + "-artifact-integrity", "expired": false,
			"workflow_run": map[string]any{"id": runID, "head_sha": head}}}})
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		// A stub curl answering only the exact listings and the one job log the step may read.
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
  "$STUB_API/workflows/ci.yml/runs?head_sha=$STUB_HEAD&event=pull_request&status=success&per_page=20") f=runs.json ;;
  "$STUB_API/runs/$STUB_RUN/jobs?filter=latest&per_page=100") f=jobs.json ;;
  "$STUB_API/jobs/$STUB_JOB/logs") f=log.txt ;;
  "$STUB_API/runs/$STUB_RUN/artifacts"*) f=artifacts.json ;;
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
		cmd.Dir = work
		cmd.Env = append(append([]string{}, gitEnv[:len(gitEnv)-2]...),
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+dir,
			"GITHUB_API_URL=https://api.github.test", "GITHUB_REPOSITORY="+repo, "GITHUB_SHA="+target,
			"RUNNER_TEMP="+dir, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+summary,
			"GH_TOKEN=t", "STUB_DIR="+dir, "STUB_HEAD="+head, "STUB_RUN="+strconv.Itoa(runID), "STUB_JOB="+strconv.Itoa(jobID),
			"STUB_API=https://api.github.test/repos/"+repo+"/actions")
		cmd.Env = append(cmd.Env, env...)
		_ = cmd.Run() // a nonzero exit is continue-on-error in the workflow; only the output binds
		o, _ := os.ReadFile(output)
		s, _ := os.ReadFile(summary)
		return string(o), string(s)
	}
	out, summary := run(merge, nil)
	if out != "mode=REUSE\n" || !strings.Contains(summary, `"profile":"corvint-ci-integrity-reuse/0","mode":"REUSE","tree":"`+tree+`","head":"`+head+`"`) ||
		!strings.Contains(summary, `"runId":`+strconv.Itoa(runID)+`,"recordJobId":`+strconv.Itoa(jobID)+`}`) {
		t.Fatalf("a passed pull-request run of this tree: output %q summary %q, want REUSE naming the run, record job and tree", out, summary)
	}
	if out, _ := run(merge, nil, "STUB_DENY_AUTH=1"); out != "mode=REUSE\n" {
		t.Errorf("a refused token must retry the public listing anonymously: output %q", out)
	}
	if out, _ := run(head, nil); out != "" {
		t.Errorf("a single-parent commit: output %q, want no mode", out)
	}
	type edit = func(r, ai, rec map[string]any, log *string)
	for name, e := range map[string]edit{
		"other head":      func(r, ai, rec map[string]any, log *string) { r["head_sha"] = other("0") },
		"merge_group run": func(r, ai, rec map[string]any, log *string) { r["event"] = "merge_group" },
		"failed run":      func(r, ai, rec map[string]any, log *string) { r["conclusion"] = "failure" },
		"other workflow":  func(r, ai, rec map[string]any, log *string) { r["path"] = ".github/workflows/other.yml" },
		"other repository": func(r, ai, rec map[string]any, log *string) {
			r["repository"] = map[string]any{"full_name": "fork/corvint"}
		},
		"string run id":             func(r, ai, rec map[string]any, log *string) { r["id"] = strconv.Itoa(runID) },
		"gate job failed":           func(r, ai, rec map[string]any, log *string) { ai["conclusion"] = "failure" },
		"gate job missing":          func(r, ai, rec map[string]any, log *string) { ai["drop"] = true },
		"record job failed":         func(r, ai, rec map[string]any, log *string) { rec["conclusion"] = "failure" },
		"record job skipped":        func(r, ai, rec map[string]any, log *string) { rec["conclusion"] = "skipped" },
		"record job missing":        func(r, ai, rec map[string]any, log *string) { rec["drop"] = true },
		"record job duplicated":     func(r, ai, rec map[string]any, log *string) { rec["dup"] = true },
		"record job on other head":  func(r, ai, rec map[string]any, log *string) { rec["head_sha"] = other("1") },
		"record job of another run": func(r, ai, rec map[string]any, log *string) { rec["run_id"] = 1 },
		"record job string id":      func(r, ai, rec map[string]any, log *string) { rec["id"] = strconv.Itoa(jobID) },
		"jobs unreadable":           func(r, ai, rec map[string]any, log *string) { rec["unreadable"] = true },
		"log unreadable":            func(r, ai, rec map[string]any, log *string) { *log = "unreadable" },
		"log without a record":      func(r, ai, rec map[string]any, log *string) { *log = logOf() },
		"log with two records":      func(r, ai, rec map[string]any, log *string) { *log = logOf(line, line) },
		"record of another tree": func(r, ai, rec map[string]any, log *string) {
			*log = logOf(strings.Replace(line, " tree="+tree+" ", " tree="+other("2")+" ", 1))
		},
		"record of another head": func(r, ai, rec map[string]any, log *string) {
			*log = logOf(strings.Replace(line, " head="+head, " head="+other("3"), 1))
		},
		"record with trailing text": func(r, ai, rec map[string]any, log *string) { *log = logOf(line + " x") },
		"record inside other text":  func(r, ai, rec map[string]any, log *string) { *log = logOf("echo " + line) },
	} {
		if out, _ := run(merge, e); out != "" {
			t.Errorf("%s: output %q, want no mode (the gate runs in full)", name, out)
		}
	}
}

// TestCINV0006DocGatesRefusesAICoauthorTrailersOnPullRequests (V1-3227): the doc-gates step runs
// the Makefile's ai-coauthor-trailers-check, which runs the repository's trailer check over the
// pull request's commits and passes its exit status through, fails a pull request without its
// base, skips on any other event, and skips, saying so, in a tree without the script.
func TestCINV0006DocGatesRefusesAICoauthorTrailersOnPullRequests(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the make target runs under sh with a stub check script")
	b := stepBlock(t, jobBlock(t, readWorkflow(t, "ci.yml"), "doc-gates"), "Refuse AI co-author trailers in the pull request's commits (CIN-V0-006)")
	wantLine(t, "doc-gates", b, "          BASE_SHA: ${{ github.event.pull_request.base.sha }}")
	// A make target, which batch-precheck's sync-check can mirror; an inline script it cannot.
	wantLine(t, "doc-gates", b, "        run: make -s ai-coauthor-trailers-check")
	makefile, err := filepath.Abs("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	run := func(event, base, stub string) (string, error) {
		dir := t.TempDir()
		if stub != "" {
			if err := os.MkdirAll(filepath.Join(dir, "script"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "script", "check-ai-coauthor-trailers.sh"), []byte(stub), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("make", "-s", "-f", makefile, "ai-coauthor-trailers-check")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GITHUB_EVENT_NAME="+event, "BASE_SHA="+base)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	const base = "1111111111111111111111111111111111111111"
	spy := "echo \"args: $*\"\nexit \"${STUB_EXIT:-0}\"\n"
	if out, err := run("pull_request", base, spy); err != nil || !strings.Contains(out, "args: "+base+" HEAD") {
		t.Fatalf("pull request: %v\n%s", err, out)
	}
	if out, err := run("pull_request", base, "echo refused\nexit 1\n"); err == nil || !strings.Contains(out, "refused") {
		t.Fatalf("a refusal must fail the step: %v\n%s", err, out)
	}
	if out, err := run("pull_request", base, ""); err != nil || !strings.Contains(out, "no script/check-ai-coauthor-trailers.sh") {
		t.Fatalf("missing script: %v\n%s", err, out)
	}
	if out, err := run("pull_request", "", "echo ran\nexit 0\n"); err == nil || strings.Contains(out, "ran") || !strings.Contains(out, "has no BASE_SHA") {
		t.Fatalf("a pull request without BASE_SHA must fail: %v\n%s", err, out)
	}
	// Outside a pull request the event's base is empty, so the check is skipped.
	for _, event := range []string{"merge_group", "push", ""} {
		if out, err := run(event, "", "echo ran\nexit 1\n"); err != nil || strings.Contains(out, "ran") || !strings.Contains(out, "skipped") {
			t.Fatalf("%q: %v\n%s", event, err, out)
		}
	}
}

// TestCINV0007ShardsSkipAtJobLevelOnlyForADocsOrReuseDecision (V1-2902): no shard job starts on
// a passed DOCS or REUSE decision, and go-product accepts skipped shards only then; a skip under
// any other decision, or a failed docs-plan, still fails the required check.
func TestCINV0007ShardsSkipAtJobLevelOnlyForADocsOrReuseDecision(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the go-product step runs under bash")
	ci := readWorkflow(t, "ci.yml")
	wantLine(t, "go-product-shard", jobBlock(t, ci, "go-product-shard"),
		"    if: ${{ always() && !cancelled() && !contains(github.event.pull_request.labels.*.name, 'ci:batched') && !(needs.docs-plan.result == 'success' && (needs.docs-plan.outputs.mode == 'DOCS' || needs.docs-plan.outputs.mode == 'REUSE')) }}")
	script := runScript(t, jobBlock(t, ci, "go-product"))
	run := func(event, shards, docs, mode string) (bool, string) {
		t.Helper()
		dir := t.TempDir()
		cmd := exec.Command("bash", "-eo", "pipefail", "-c", script)
		cmd.Env = append(os.Environ(), "BATCHED=false", "SHARDS_RESULT="+shards, "STATIC_RESULT=success", "STATIC_REUSE=",
			"DOCS_RESULT="+docs, "DOCS_MODE="+mode, "SHARDS=8", "RUNNER_TEMP="+dir, "GITHUB_RUN_ID=7", "GITHUB_RUN_ATTEMPT=1",
			"GITHUB_EVENT_NAME="+event, "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
		out, err := cmd.CombinedOutput()
		return err == nil, string(out)
	}
	for _, event := range []string{"pull_request", "merge_group", "push"} {
		for _, mode := range []string{"DOCS", "REUSE"} {
			if ok, out := run(event, "skipped", "success", mode); !ok || !strings.Contains(out, "CIN-V0-007: no go-product shard started (docs-plan "+mode+")") {
				t.Errorf("%s %s: skipped shards must pass; ok=%v\n%s", event, mode, ok, out)
			}
		}
		for name, c := range map[string][3]string{
			"skipped under FULL":           {"skipped", "success", "FULL"},
			"skipped under no decision":    {"skipped", "success", ""},
			"skipped with failed docs":     {"skipped", "failure", "DOCS"},
			"skipped with skipped docs":    {"skipped", "skipped", "REUSE"},
			"failed shards under DOCS":     {"failure", "success", "DOCS"},
			"cancelled shards under REUSE": {"cancelled", "success", "REUSE"},
		} {
			if ok, out := run(event, c[0], c[1], c[2]); ok {
				t.Errorf("%s, %s: go-product passed\n%s", event, name, out)
			}
		}
	}
}

// DCI-V0-856 (V1-0470): a pull-request run whose docs-plan chose DOCS leaves no record that lets
// a queue or push run skip its root race invocation, and only a pull-request event plans DOCS.
// DCI-V0-855 binds the docs-only qualification to its classifier and policy alone on the strength
// of this backstop, so losing any clause here would let a README reader added after review reach
// main untested.
func TestDCIV0856DocsRunLeavesNoQueueReuseEvidence(t *testing.T) {
	w := readWorkflow(t, "ci.yml")
	ifLine := func(job string) string {
		m := regexp.MustCompile(`(?m)^    if: (.+)$`).FindStringSubmatch(jobBlock(t, w, job))
		if m == nil {
			t.Fatalf("%s has no single-line if", job)
		}
		return m[1]
	}
	for job, clause := range map[string]string{
		"ci-tested-tree-record":   " && (needs.docs-plan.result != 'success' || (needs.docs-plan.outputs.mode != 'DOCS' && needs.docs-plan.outputs.mode != 'REUSE')) }}",
		"ci-narrowed-tree-record": " && needs.docs-plan.result == 'success' && needs.docs-plan.outputs.mode != 'DOCS' && needs.docs-plan.outputs.mode != 'REUSE' }}",
	} {
		// The clause must close a top-level && chain: the text before it balances its parentheses.
		got := ifLine(job)
		rest := strings.TrimSuffix(got, clause)
		if !strings.HasPrefix(got, "${{ !cancelled() && ") || rest == got || strings.Count(rest, "(") != strings.Count(rest, ")") {
			t.Errorf("%s if = %q, want a top-level && ending %q", job, got, clause)
		}
	}
	plan := jobBlock(t, w, "docs-plan")
	wantLine(t, "docs-plan", plan, `          if [ "$GITHUB_EVENT_NAME" != pull_request ] || [ ! -f tools/docs-ci-plan/main.go ]; then`)
}
