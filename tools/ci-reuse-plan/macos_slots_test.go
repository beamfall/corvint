// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// CIN-V0-5017: evaluate the actual scheduling and aggregator conditions. Only a
// successful classifier with its explicit skip decision may omit the four shards.
func TestCINV05017DarwinGatingFailsClosed(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "workflow conditions are evaluated with bash")
	w := readWorkflow(t, "go-macos.yml")
	blocks := []string{
		jobBlock(t, w, "go-macos-shard"),
		stepBlock(t, jobBlock(t, w, "go-macos"), "Check all four advisory shard outcomes"),
	}
	for _, tc := range []struct {
		cr, cadence, dr, mode string
		run                   bool
	}{
		{"success", "FULL", "success", "FULL", true},
		{"success", "DEFER", "skipped", "", false},
		{"success", "FULL", "success", "DOCS", false},
		{"failure", "DEFER", "failure", "DOCS", true},
		{"skipped", "DEFER", "skipped", "DOCS", true},
		{"success", "", "success", "", true},
		{"success", "UNKNOWN", "success", "UNKNOWN", true},
		{"failure", "", "success", "FULL", true},
		{"success", "FULL", "failure", "", true},
	} {
		for _, block := range blocks {
			m := regexp.MustCompile(`(?m)^\s+if: (.+)$`).FindStringSubmatch(block)
			if m == nil {
				t.Fatal("missing scheduling condition")
			}
			condition := strings.TrimSuffix(strings.TrimPrefix(m[1], "${{ "), " }}")
			condition = strings.NewReplacer(
				"always()", "true", "!cancelled()", "true",
				"github.event_name", "'pull_request'",
				"needs.cadence-plan.result", "'"+tc.cr+"'",
				"needs.cadence-plan.outputs.cadence", "'"+tc.cadence+"'",
				"needs.docs-plan.result", "'"+tc.dr+"'",
				"needs.docs-plan.outputs.mode", "'"+tc.mode+"'",
			).Replace(condition)
			// No shell expansion is admitted from the expression being tested.
			if strings.ContainsAny(condition, "$`\n") {
				t.Fatalf("unsupported condition %q", condition)
			}
			out, err := exec.Command("bash", "--noprofile", "--norc", "-c", "if [[ "+condition+" ]]; then echo run; else echo skip; fi").CombinedOutput()
			if err != nil {
				t.Fatalf("condition %q: %v: %s", condition, err, out)
			}
			if (strings.TrimSpace(string(out)) == "run") != tc.run {
				t.Errorf("%+v: %s => %s", tc, m[1], out)
			}
		}
	}
}

// CIN-V0-5018: no base-wide cancellation; only a successful, well-formed exact
// reference absence read permits a cancellation. Execute the workflow's script
// with an isolated gh spy: these tests perform no GitHub requests.
func TestCINV05018StaleQueueCleanup(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "cleanup uses bash, jq and a gh spy")
	if _, err := exec.LookPath("jq"); err != nil {
		t.Fatal(err)
	}
	w := readWorkflow(t, "merge-queue-cleanup.yml")
	for _, line := range []string{
		"    branches: [main]", "  delete:", "    - cron: '2-57/5 * * * *'",
		"  cancel-in-progress: false", "    runs-on: ubuntu-24.04", "      actions: write",
		"    if: github.event_name != 'delete' || (github.event.ref_type == 'branch' && startsWith(github.event.ref, 'gh-readonly-queue/'))",
	} {
		wantLine(t, "cleanup", w, line)
	}
	if strings.Contains(w, "actions/checkout") {
		t.Fatal("privileged cleanup must not execute candidate code")
	}
	for _, name := range []string{"ci.yml", "cem-release-gate.yml"} {
		original := readWorkflow(t, name)
		wantLine(t, name, original, "  group: ${{ github.workflow }}-${{ github.event.pull_request.number || (github.event_name == 'push' && github.sha) || github.ref }}")
	}
	script := runScript(t, jobBlock(t, w, "cancel-stale"))
	for _, tc := range []struct {
		name, workflow, event, state, branch, refs, failure string
		cancel, wantError                                   bool
	}{
		{"deleted", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "", true, false},
		{"queued CEM", "cem-release-gate.yml", "merge_group", "queued", "gh-readonly-queue/main/pr-1-a", `[]`, "", true, false},
		{"live", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[{"ref":"refs/heads/gh-readonly-queue/main/pr-1-a"}]`, "", false, false},
		{"live and newer head", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[{"ref":"refs/heads/gh-readonly-queue/main/pr-1-a"},{"ref":"refs/heads/gh-readonly-queue/main/pr-2-b"}]`, "", false, false},
		{"prefix is not exact", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[{"ref":"refs/heads/gh-readonly-queue/main/pr-1-ab"}]`, "", true, false},
		{"ref unreadable", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "refs", false, false},
		{"ref malformed", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `{}`, "", false, false},
		{"ref invalid JSON", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `oops`, "", false, false},
		{"ref invalid row", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[{}]`, "", false, false},
		{"run unreadable", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "run", false, false},
		{"listing unreadable", "ci.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "list", false, true},
		{"completed meanwhile", "ci.yml", "merge_group", "completed", "gh-readonly-queue/main/pr-1-a", `[]`, "", false, false},
		{"wrong event", "ci.yml", "push", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "", false, false},
		{"ordinary branch", "ci.yml", "merge_group", "in_progress", "main", `[]`, "", false, false},
		{"wrong workflow", "other.yml", "merge_group", "in_progress", "gh-readonly-queue/main/pr-1-a", `[]`, "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := t.TempDir()
			run := fmt.Sprintf(`{"id":123,"path":%q,"event":%q,"status":%q,"head_branch":%q}`, ".github/workflows/"+tc.workflow, tc.event, tc.state, tc.branch)
			// Candidate is on page two; actual run identity is re-read independently.
			files := map[string]string{
				"runs": `{"workflow_runs":[]} ` + `{"workflow_runs":[{"id":123,"event":"merge_group"}]}`,
				"run":  run, "refs": tc.refs,
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stub := `#!/bin/bash
set -eu
printf '%s\n' "$*" >> "$FIXTURE/calls"
[ "$1" = api ] || exit 90
shift
if [ "$1" = --method ]; then
  [ "$2" = POST ] && [ "$3" = repos/owner/repo/actions/runs/123/cancel ] || exit 91
  echo cancel >> "$FIXTURE/cancelled"
  exit 0
fi
if [ "$1" = --paginate ]; then
  [ "$FAILURE" != list ] || exit 92
  if [ "$2" = "repos/owner/repo/actions/workflows/$LIST_WORKFLOW/runs?event=merge_group&status=$LIST_STATUS&per_page=100" ]; then
    cat "$FIXTURE/runs"
  else
    echo '{"workflow_runs":[]}'
  fi
elif [ "$1" = repos/owner/repo/actions/runs/123 ]; then
  [ "$FAILURE" != run ] || exit 93
  cat "$FIXTURE/run"
elif [ "$1" = repos/owner/repo/git/matching-refs/heads%2Fgh-readonly-queue%2Fmain%2Fpr-1-a ]; then
  [ "$FAILURE" != refs ] || exit 94
  cat "$FIXTURE/refs"
else
  exit 95
fi
`
			if err := execfile.WriteFile(filepath.Join(d, "gh"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			wf, status := tc.workflow, tc.state
			if wf == "other.yml" {
				wf = "ci.yml"
			}
			if status == "completed" {
				status = "in_progress"
			}
			cmd := exec.Command("bash", "--noprofile", "--norc", "-c", script)
			cmd.Env = append(os.Environ(), "PATH="+d+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE="+d,
				"REPO=owner/repo", "GH_TOKEN=unused-spy-only", "FAILURE="+tc.failure,
				"LIST_WORKFLOW="+wf, "LIST_STATUS="+status, "GITHUB_STEP_SUMMARY="+filepath.Join(d, "summary"))
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantError {
				t.Fatalf("exit: %v; output: %s", err, out)
			}
			cancelled, _ := os.ReadFile(filepath.Join(d, "cancelled"))
			if (string(cancelled) == "cancel\n") != tc.cancel || strings.Count(string(cancelled), "cancel") > 1 {
				t.Fatalf("cancellation %q, want %v; output: %s", cancelled, tc.cancel, out)
			}
			calls, err := os.ReadFile(filepath.Join(d, "calls"))
			if err != nil {
				t.Fatal(err)
			}
			// Ensure each injected failure was reached, rather than hidden by setup failure.
			reached := map[string]string{"list": "--paginate", "run": "api repos/owner/repo/actions/runs/123", "refs": "api repos/owner/repo/git/matching-refs/"}
			if tc.failure != "" && !strings.Contains(string(calls), reached[tc.failure]) {
				t.Fatalf("injection not reached: %s", calls)
			}
		})
	}
}
