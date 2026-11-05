// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

// TestAFPV0106RouteHealthWorkflowFailsOnADeadRoute runs the ci-route-health step against a stub
// gh serving the stale-qualification fixture and the tool built from this tree: the step fails,
// annotates the DEAD route and retains the report; with a DOCS hit in the window it passes. The
// job holds only read permissions and runs only from main.
func TestAFPV0106RouteHealthWorkflowFailsOnADeadRoute(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the report step runs under bash with a #!/bin/sh gh stub")
	for _, tool := range []string{"bash", "jq", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("the report step needs %s on PATH: %v", tool, err)
		}
	}
	w := readWorkflow(t, "ci-route-health.yml")
	b := jobBlock(t, w, "routes")
	for _, line := range []string{
		"    if: github.ref == 'refs/heads/main'",
		"      actions: read",
		"      contents: read",
		"          persist-credentials: false",
	} {
		wantLine(t, "ci-route-health", b, line)
	}
	if strings.Contains(w, ": write") {
		t.Error("ci-route-health must hold read permissions only")
	}
	script := filepath.Join(t.TempDir(), "step.sh")
	if err := os.WriteFile(script, []byte(runScript(t, stepBlock(t, b, "Report each fast route's hits over the newest eligible runs"))), 0o600); err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	tool := filepath.Join(tmp, "ci-reuse-plan")
	if out, err := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	report := func(docsHit bool) (int, string, string) {
		t.Helper()
		dir := t.TempDir()
		fixtures := filepath.Join(dir, "fixtures")
		if err := os.MkdirAll(filepath.Join(dir, "runner", "ci-route-health"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(fixtures, 0o700); err != nil {
			t.Fatal(err)
		}
		var listed []any
		for i := 1; i <= 10; i++ {
			listed = append(listed, map[string]any{"id": i, "head_sha": testHead, "event": "pull_request", "status": "completed",
				"conclusion": "success", "path": workflowPath, "repository": map[string]any{"full_name": testRepo}})
			jobs := prRunJobs(docsHit && i == 3)
			if docsHit && i == 4 {
				jobs[2].Conclusion = "success" // a narrowed record keeps NARROWED healthy too
			}
			writeJSON(t, filepath.Join(fixtures, "jobs-"+strconv.Itoa(i)+".json"), map[string]any{"jobs": jobs})
		}
		writeJSON(t, filepath.Join(fixtures, "runs.json"), map[string]any{"workflow_runs": listed})
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0o700); err != nil {
			t.Fatal(err)
		}
		writeExecutable(t, filepath.Join(bin, "gh"), `#!/bin/sh
[ "$1" = api ] || exit 2
case "$2" in
  "repos/$REPO/actions/workflows/ci.yml/runs?status=completed&per_page=100") f=runs.json ;;
  "repos/$REPO/actions/runs/"*"/jobs?filter=latest&per_page=100") id=${2#"repos/$REPO/actions/runs/"}; f=jobs-${id%%/*}.json ;;
  *) exit 1 ;;
esac
cat "$FIXTURES/$f"
`)
		runner := filepath.Join(dir, "runner")
		// The step runs the tool from RUNNER_TEMP, where the build step put it.
		if b, err := os.ReadFile(tool); err != nil {
			t.Fatal(err)
		} else if err := execfile.WriteFile(filepath.Join(runner, "ci-reuse-plan"), b, 0o700); err != nil {
			t.Fatal(err)
		}
		summary := filepath.Join(dir, "summary")
		cmd := exec.Command("bash", "--noprofile", "--norc", "-e", script)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "RUNNER_TEMP="+runner,
			"GITHUB_STEP_SUMMARY="+summary, "GH_TOKEN=t", "REPO="+testRepo, "FIXTURES="+fixtures)
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			code = cmd.ProcessState.ExitCode()
		}
		retained, _ := os.ReadFile(filepath.Join(runner, "ci-route-health", "report.json"))
		return code, string(out), string(retained)
	}
	code, out, retained := report(false)
	if code != 1 || !strings.Contains(out, "::error title=CI fast route DEAD (AFP-V0-105)::DOCS: 0 hit(s) in 10 eligible run(s)") ||
		!strings.Contains(retained, `"profile": "corvint-ci-route-health/0"`) {
		t.Fatalf("stale qualification: exit %d\n%s\nreport %s", code, out, retained)
	}
	if code, out, _ := report(true); code != 0 || strings.Contains(out, "::error") {
		t.Fatalf("a DOCS hit in the window: exit %d\n%s", code, out)
	}
}
