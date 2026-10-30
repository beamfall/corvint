// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/testplatform"
)

// fakeGH writes a gh stand-in into DIR/bin. Artifact listings print DIR/pages/<name>.json (absent:
// an empty page), archive downloads print DIR/zips/<id>.zip, the open-issue query runs the
// script's --jq filter over DIR/issues.json, and every write is recorded in DIR/writes, with
// each body file copied to DIR/body-<command>.md. FAKE_PIN_REFUSED makes `issue pin` fail.
func fakeGH(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
set -eu
dir=$FAKE_DIR
body() { while [ $# -gt 0 ]; do [ "$1" != --body-file ] || cp "$2" "$dir/body-$cmd.md"; shift; done; }
cmd=$2
case "$1 $2" in
  "api --paginate")
    name=${3##*name=}; name=${name%%&*}
    if [ -f "$dir/pages/$name.json" ]; then cat "$dir/pages/$name.json"; else echo '{"total_count":0,"artifacts":[]}'; fi ;;
  "api repos/beamfall/corvint/actions/artifacts/"*"/zip")
    id=${2%/zip}; id=${id##*/}; cat "$dir/zips/$id.zip" ;;
  "api repos/beamfall/corvint/issues?labels=ci-flake-report&state=open&sort=created&direction=asc&per_page=100")
    [ "$3" = --jq ]; jq -r "$4" "$dir/issues.json" ;;
  "label create") echo "label create $*" >> "$dir/writes" ;;
  "issue edit") echo "issue edit $3 $4 $5" >> "$dir/writes"; shift 2; body "$@" ;;
  "issue create") echo "issue create $*" >> "$dir/writes"; shift 2; body "$@"; echo https://github.com/beamfall/corvint/issues/99 ;;
  "issue pin") echo "issue pin $3 $4 $5" >> "$dir/writes"; [ -z "${FAKE_PIN_REFUSED:-}" ] || { echo "pin refused" >&2; exit 1; } ;;
  *) echo "unexpected gh $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// buildTool compiles the report tool once per test into DIR.
func buildTool(t *testing.T, dir string) string {
	t.Helper()
	tool := filepath.Join(dir, testplatform.Executable("ci-flake-report"))
	cmd := exec.Command("go", "build", "-o", tool, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return tool
}

func runReport(t *testing.T, dir, bin, tool string, extra ...string) (string, string, int) {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "report.sh is a POSIX shell script run by sh")
	for _, need := range []string{"sh", "jq", "go"} {
		if _, err := exec.LookPath(need); err != nil {
			t.Skipf("report.sh needs %s on PATH: %v", need, err)
		}
	}
	script, err := filepath.Abs("../../.github/flake-report/report.sh")
	if err != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(dir, "summary.md")
	cmd := exec.Command("sh", script)
	cmd.Env = append([]string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "HOME=" + dir,
		"FAKE_DIR=" + dir, "GH_TOKEN=fake", "REPO=beamfall/corvint", "RUN_URL=https://example.invalid/run/7",
		"TOOL=" + tool, "FLAKE_REPORT_NOW=" + now.Format(time.RFC3339), "FLAKE_REPORT_SHARDS=2",
		"GITHUB_STEP_SUMMARY=" + summary}, extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	rc := 0
	if exit, ok := err.(*exec.ExitError); ok {
		rc = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(summary)
	return stdout.String() + stderr.String(), string(b), rc
}

func TestAFPV0530ReportScriptUpdatesOneIssueInPlace(t *testing.T) {
	dir := t.TempDir()
	bin := fakeGH(t, dir)
	tool := buildTool(t, dir)
	day := 24 * time.Hour
	for _, d := range []string{"pages", "zips"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Shard name 1 of each event has a record in the window, one outside it; shard 0 has none.
	pages := map[string]string{
		"ci-flaky-pull_request-1": page(art(11, "ci-flaky-pull_request-1", now.Add(-3*day), 1100),
			art(12, "ci-flaky-pull_request-1", now.Add(-30*day), 1200)),
		"ci-flaky-merge_group-1": page(art(21, "ci-flaky-merge_group-1", now.Add(-day), 2100)),
	}
	for name, p := range pages {
		if err := os.WriteFile(filepath.Join(dir, "pages", name+".json"), []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeZip(t, filepath.Join(dir, "zips"), 11, flakyRecord("pull_request", 1100, 1, "example.com/m/a:TestOne"))
	writeZip(t, filepath.Join(dir, "zips"), 21, flakyRecord("merge_group", 2100, 1, "example.com/m/a:TestOne"))
	issues := `[{"number": 40, "pull_request": {"url": "x"}}, {"number": 41}, {"number": 42}]`
	if err := os.WriteFile(filepath.Join(dir, "issues.json"), []byte(issues), 0o644); err != nil {
		t.Fatal(err)
	}

	out, summary, rc := runReport(t, dir, bin, tool)
	if rc != 0 {
		t.Fatalf("report.sh exited %d:\n%s", rc, out)
	}
	writes, _ := os.ReadFile(filepath.Join(dir, "writes"))
	// The label is created only when missing, never rewritten with --force.
	if !strings.Contains(string(writes), "label create ci-flake-report --repo beamfall/corvint --color 5319E7") ||
		strings.Contains(string(writes), "--force") ||
		!strings.Contains(string(writes), "issue edit 41 --repo beamfall/corvint") ||
		strings.Contains(string(writes), "issue create") || strings.Contains(string(writes), "issue pin") {
		t.Fatalf("writes:\n%s", writes)
	}
	body, err := os.ReadFile(filepath.Join(dir, "body-edit.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"2 FLAKY shard pass(es) in 2 CI run(s): merge_group 1, pull_request 1.",
		"| `example.com/m/a.TestOne` | 2 | 2 | 1 | 1 | 2026-10-23 | 2026-10-25 |",
		"(https://example.invalid/run/7)",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("issue body lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(string(body), "Not read") {
		t.Errorf("every record was downloadable, yet the body names unread records:\n%s", body)
	}
	if !strings.Contains(summary, string(body)) || !strings.Contains(summary, "flake report: updated issue #41 in place") {
		t.Errorf("job summary:\n%s", summary)
	}

	// A lost download is named, not dropped.
	if err := os.Remove(filepath.Join(dir, "zips", "21.zip")); err != nil {
		t.Fatal(err)
	}
	out, _, rc = runReport(t, dir, bin, tool)
	if rc != 0 {
		t.Fatalf("report.sh with a lost download exited %d:\n%s", rc, out)
	}
	body, _ = os.ReadFile(filepath.Join(dir, "body-edit.md"))
	if !strings.Contains(string(body), "Not read (1)") || !strings.Contains(string(body), "artifact 21 `ci-flaky-merge_group-1` (2026-10-25): archive missing: not downloaded") ||
		!strings.Contains(out, "::warning title=flake report::artifact 21 (ci-flaky-merge_group-1, run 2100,") {
		t.Errorf("lost download:\n%s\n%s", body, out)
	}
}

func TestAFPV0530ReportScriptOpensThePinnedIssueOnce(t *testing.T) {
	dir := t.TempDir()
	bin := fakeGH(t, dir)
	tool := buildTool(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "issues.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, summary, rc := runReport(t, dir, bin, tool)
	if rc != 0 {
		t.Fatalf("report.sh exited %d:\n%s", rc, out)
	}
	writes, _ := os.ReadFile(filepath.Join(dir, "writes"))
	if !strings.Contains(string(writes), "issue create --repo beamfall/corvint --title CI flake report (weekly, AFP-V0-530) --label ci-flake-report --body-file") ||
		!strings.Contains(string(writes), "issue pin 99 --repo beamfall/corvint") || strings.Contains(string(writes), "issue edit") {
		t.Fatalf("writes:\n%s", writes)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "body-create.md"))
	if !strings.Contains(string(body), "No FLAKY record in the window.") || !strings.Contains(summary, "flake report: opened and pinned issue #99") {
		t.Errorf("empty window:\n%s\n%s", body, summary)
	}

	// A refused pin (three issues are already pinned) is reported, not fatal.
	out, summary, rc = runReport(t, dir, bin, tool, "FAKE_PIN_REFUSED=1")
	if rc != 0 || !strings.Contains(summary, "pinning refused (at most three issues can be pinned), pin it by hand") {
		t.Fatalf("refused pin: exit %d\n%s\n%s", rc, out, summary)
	}
}

func TestAFPV0530WorkflowBinding(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/flake-report.yml")
	if err != nil {
		t.Fatal(err)
	}
	wf := string(b)
	for _, want := range []string{
		`(?m)^on:\n  schedule:\n    - cron: "[0-9]+ [0-9]+ \* \* [0-6]"\n  workflow_dispatch:\n`,
		`(?m)^permissions:\n  contents: read\n`,
		`(?m)^concurrency:\n  group: flake-report\n  cancel-in-progress: false\n`,
		`(?m)^  report:\n    if: github\.ref == 'refs/heads/main'\n`,
		`(?m)^    permissions:\n      actions: read\n      contents: read\n      issues: write\n`,
		`(?m)^      - uses: actions/checkout@[0-9a-f]{40} # v[0-9.]+\n        with:\n          ref: \$\{\{ github\.sha \}\}\n          persist-credentials: false\n`,
		`(?m)^      - uses: actions/setup-go@[0-9a-f]{40} # v[0-9.]+\n        with:\n          cache: false\n          go-version: "1\.27\.3"\n`,
		`go build -trimpath -buildvcs=false -o "\$RUNNER_TEMP/ci-flake-report" \./tools/ci-flake-report\n`,
		`(?m)^          GH_TOKEN: \$\{\{ github\.token \}\}\n          REPO: \$\{\{ github\.repository \}\}\n`,
		`(?m)^        run: sh \.github/flake-report/report\.sh\n`,
	} {
		if !regexp.MustCompile(want).MatchString(wf) {
			t.Errorf("flake-report.yml lacks %s", want)
		}
	}
	if strings.Count(wf, "permissions:") != 2 || strings.Count(wf, "issues: write") != 1 || strings.Contains(wf, "pull_request") {
		t.Errorf("flake-report.yml grants more than the report job's issues: write, or runs on pull requests")
	}
	if strings.Count(wf, "uses: ") != 2 {
		t.Errorf("flake-report.yml uses %d actions, want checkout and setup-go only", strings.Count(wf, "uses: "))
	}
}
