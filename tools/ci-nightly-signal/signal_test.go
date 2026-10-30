// SPDX-License-Identifier: Apache-2.0
package nightlysignal

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// The fixtures are `gh run view RUN --attempt 1 --json jobs` of two scheduled release-gates runs
// on main that failed (V1-1114): 37628218263 failed both full-gate legs, and 38052349384 failed
// both full-gate legs and companion-release.
const (
	runA = "37628218263"
	runB = "38052349384"
	sha  = "0123456789abcdef0123456789abcdef01234567"
)

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fakeGH writes a gh stand-in into DIR/bin. `run view` prints DIR/jobs-<run>.json (absent: exit 1),
// the oldest-first open nightly-triage issue listing runs the script's --jq filter over
// DIR/issues.json (absent: exit 1), and every
// write (`label create`, `issue comment`, `issue create`) appends its argv to DIR/writes and copies
// the body file to DIR/comment-<n>.md or DIR/created.md instead of posting anything.
func fakeGH(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	writeFile(t, filepath.Join(bin, "gh"), []byte(`#!/bin/sh
set -eu
d=`+dir+`
body() {
  while [ $# -gt 0 ]; do
    [ "$1" = --body-file ] && { cat "$2"; return; }
    shift
  done
  exit 1
}
case "$1 $2" in
"run view")
  [ "$4 $5 $6" = "--repo beamfall/corvint --attempt" ] && [ "$8 $9" = "--json jobs" ] || exit 1
  cat "$d/jobs-$3.json" ;;
"api repos/beamfall/corvint/issues?labels=nightly-triage&state=open&sort=created&direction=asc&per_page=100")
  [ "$3" = --jq ] || exit 1
  jq -r "$4" "$d/issues.json" ;;
"label create"|"issue comment"|"issue create")
  echo "$*" >> "$d/writes"
  case "$1 $2" in
  "issue comment") body "$@" > "$d/comment-$3.md" ;;
  "issue create") body "$@" > "$d/created.md"; echo "https://github.com/beamfall/corvint/issues/99" ;;
  esac ;;
*) exit 1 ;;
esac
`))
	if err := os.Chmod(filepath.Join(bin, "gh"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// signal runs signal.sh with only PATH, HOME and the job's environment against DIR, so a CI
// runner's GITHUB_* and GH_* variables never reach a fixture run.
func signal(t *testing.T, dir, run string) (string, error) {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "signal.sh runs under sh with a #!/bin/sh gh spy")
	for _, tool := range []string{"sh", "jq"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the nightly signal needs %s on PATH: %v", tool, err)
		}
	}
	script, err := filepath.Abs("../../.github/nightly-signal/signal.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Env = []string{"PATH=" + fakeGH(t, dir) + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + dir, "GH_TOKEN=unused", "REPO=beamfall/corvint", "RUN_ID=" + run, "RUN_ATTEMPT=1",
		"RUN_URL=https://github.com/beamfall/corvint/actions/runs/" + run + "/attempts/1", "SHA=" + sha,
		"GITHUB_STEP_SUMMARY=" + filepath.Join(dir, "summary.md")}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	return out.String(), err
}

// setup copies the recorded jobs of RUN, appending EXTRA job objects, and writes ISSUES, already
// in the API's oldest-first order, as the open issue listing (nil: the listing fails). A negative
// number stands for a pull request carrying the label, which the REST listing also returns.
func setup(t *testing.T, run string, issues []int, extra ...map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	var jobs struct {
		Jobs []any `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(read(t, filepath.Join("testdata", "run-"+run+".json"))), &jobs); err != nil {
		t.Fatal(err)
	}
	for _, e := range extra {
		jobs.Jobs = append(jobs.Jobs, e)
	}
	b, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "jobs-"+run+".json"), b)
	if issues != nil {
		list := make([]map[string]any, 0, len(issues))
		for _, n := range issues {
			if n < 0 {
				list = append(list, map[string]any{"number": -n, "pull_request": map[string]any{}})
				continue
			}
			list = append(list, map[string]any{"number": n})
		}
		b, err := json.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "issues.json"), b)
	}
	return dir
}

func contains(t *testing.T, what, text string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Fatalf("%s lacks %q:\n%s", what, w, text)
		}
	}
}

func lacks(t *testing.T, what, text string, unwanted ...string) {
	t.Helper()
	for _, w := range unwanted {
		if strings.Contains(text, w) {
			t.Fatalf("%s has %q:\n%s", what, w, text)
		}
	}
}

func TestSOPV0013OpensOneIssueWhenNoneIsOpen(t *testing.T) {
	dir := setup(t, runA, []int{})
	if out, err := signal(t, dir, runA); err != nil {
		t.Fatalf("signal: %v\n%s", err, out)
	}
	writes := read(t, filepath.Join(dir, "writes"))
	if want := "label create nightly-triage --repo beamfall/corvint --force --color B60205 " +
		"--description A scheduled release-gates run on main failed\n" +
		"issue create --repo beamfall/corvint --title Nightly release gates failing on main " +
		"--label nightly-triage --body-file "; !strings.HasPrefix(writes, want) || strings.Count(writes, "\n") != 2 {
		t.Fatalf("writes = %q, want the label and exactly one issue create", writes)
	}
	body := read(t, filepath.Join(dir, "created.md"))
	contains(t, "issue body", body, "https://github.com/beamfall/corvint/actions/runs/"+runA+"/attempts/1", sha,
		"- gate (full-gate, macos-15, 300, make -k gate) (failure): Run full-gate (https://github.com/beamfall/corvint/actions/runs/"+runA+"/job/",
		"- gate (full-gate, ubuntu-24.04, 300, make -k gate) (failure): Run full-gate (")
	lacks(t, "issue body", body, "focused-docs", "interop-gate", "linux-core", "companion-release")
	contains(t, "step summary", read(t, filepath.Join(dir, "summary.md")),
		"nightly signal: opened https://github.com/beamfall/corvint/issues/99", body)
}

func TestSOPV0013CommentsOnTheOldestOpenIssue(t *testing.T) {
	// The signal job is still running while it reads its own attempt, so its row has no
	// conclusion and is never listed; a cancelled or timed-out job is listed with its steps.
	running := map[string]any{"name": "nightly-failure-signal", "status": "in_progress", "conclusion": "",
		"url": "https://example.invalid/self", "steps": []any{}}
	timedOut := map[string]any{"name": "slow-job", "status": "completed", "conclusion": "timed_out",
		"url": "https://example.invalid/slow", "steps": []any{
			map[string]any{"name": "Set up job", "conclusion": "success"},
			map[string]any{"name": "Wait", "conclusion": "cancelled"}}}
	dir := setup(t, runB, []int{-5, 17, 23, 41}, running, timedOut)
	if out, err := signal(t, dir, runB); err != nil {
		t.Fatalf("signal: %v\n%s", err, out)
	}
	writes := read(t, filepath.Join(dir, "writes"))
	if lines := strings.Split(strings.TrimSuffix(writes, "\n"), "\n"); len(lines) != 2 ||
		!strings.HasPrefix(lines[1], "issue comment 17 --repo beamfall/corvint --body-file ") {
		t.Fatalf("writes = %q, want the label and one comment on #17", writes)
	}
	if _, err := os.Stat(filepath.Join(dir, "created.md")); !os.IsNotExist(err) {
		t.Fatalf("an issue was opened while #17 is open: %v", err)
	}
	body := read(t, filepath.Join(dir, "comment-17.md"))
	contains(t, "comment body", body, "https://github.com/beamfall/corvint/actions/runs/"+runB+"/attempts/1",
		"- gate (companion-release, macos-15, 120, make companion-release-gate) (failure): Run companion-release (",
		"- gate (full-gate, macos-15, 300, make -k gate) (failure): Run full-gate (",
		"- gate (full-gate, ubuntu-24.04, 300, make -k gate) (failure): Run full-gate (",
		"- slow-job (timed_out): Wait (https://example.invalid/slow)")
	lacks(t, "comment body", body, "nightly-failure-signal", "Set up job", "linux-core")
	contains(t, "step summary", read(t, filepath.Join(dir, "summary.md")), "nightly signal: commented on #17")
}

func TestSOPV0013FailsVisiblyWithoutWriting(t *testing.T) {
	for _, c := range []struct {
		name, run, want string
		issues          []int
		removeJobs      bool
	}{
		{"unreadable jobs", runA, "cannot read the jobs of run " + runA, []int{}, true},
		{"unlistable issues", runA, "cannot list open nightly-triage issues", nil, false},
		{"malformed run id", "37628218263;x", "is not a run id", []int{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := setup(t, runA, c.issues)
			if c.removeJobs {
				if err := os.Remove(filepath.Join(dir, "jobs-"+runA+".json")); err != nil {
					t.Fatal(err)
				}
			}
			out, err := signal(t, dir, c.run)
			if err == nil {
				t.Fatalf("signal passed:\n%s", out)
			}
			contains(t, "output", out, c.want)
			writes, _ := os.ReadFile(filepath.Join(dir, "writes"))
			if strings.Contains(string(writes), "issue ") {
				t.Fatalf("an issue was written after the failure: %q", writes)
			}
		})
	}
}

func TestSOPV0013WorkflowBinding(t *testing.T) {
	wf := read(t, "../../.github/workflows/release-gates.yml")
	one := func(text, pattern string) string {
		t.Helper()
		m := regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1)
		if len(m) != 1 {
			t.Fatalf("%d matches of %q, want 1", len(m), pattern)
		}
		return m[0][1]
	}
	if one(wf, `(?m)^permissions:\n((?:  .*\n)+)`) != "  contents: read\n" {
		t.Fatal("release-gates workflow-level permissions are not exactly contents: read")
	}
	if n := len(regexp.MustCompile(`(?m)^\s+issues:\s*write\s*$`).FindAllString(wf, -1)); n != 1 {
		t.Fatalf("%d grants of issues: write, want only the signal job's", n)
	}
	job := one(wf, `(?ms)^  nightly-failure-signal:\n(.*?)(?:\n  [a-z][^\n]*:\n|\z)`)
	if one(job, `(?m)^    permissions:\n((?:      .*\n)+)`) != "      actions: read\n      contents: read\n      issues: write\n" {
		t.Fatal("nightly-failure-signal permissions are not exactly actions: read, contents: read and issues: write")
	}
	cond := one(job, `(?ms)^    if: >-\n(.*?)\n    runs-on:`)
	for _, want := range []string{"always()", "github.event_name == 'schedule'", "github.ref == 'refs/heads/main'",
		"contains(needs.*.result, 'failure')", "contains(needs.*.result, 'cancelled')"} {
		contains(t, "nightly-failure-signal if", cond, want)
	}
	contains(t, "nightly-failure-signal job", job, "    needs: [gate, linux-core]\n", "ref: ${{ github.sha }}",
		"    concurrency:\n      group: nightly-failure-signal\n      cancel-in-progress: false\n",
		"sparse-checkout: .github/nightly-signal\n", "persist-credentials: false\n",
		"\n        run: sh trusted/.github/nightly-signal/signal.sh\n")
	if strings.Count(job, "uses:") != 1 || strings.Count(job, "run:") != 1 {
		t.Fatal("nightly-failure-signal runs more than the sparse checkout and signal.sh")
	}
}
