// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testplatform"
)

const batchedJobCondition = "    if: ${{ !contains(github.event.pull_request.labels.*.name, 'ci:batched') }}"

// runBash runs SCRIPT under bash -e, as a GitHub-hosted run step does, in DIR with ENV added,
// and returns its exit code and combined output.
func runBash(t *testing.T, dir, script string, env ...string) (int, string) {
	t.Helper()
	cmd := exec.Command("bash", "-e", "-c", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		return exit.ExitCode(), string(out)
	}
	return 0, string(out)
}

// writeExecutable writes an executable shell script at PATH.
func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AFP-V0-432 (V1-2580): cem-release-gate runs `make cem-release-gate` on linux/amd64 and
// darwin/arm64 with the pinned Go toolchain and writes each host's run record and outcome. A host
// job fails on a suite failure (a nonzero exit without both files) and passes when only its own
// readiness view refused; it retains its output, record and outcome even when it fails. The
// cem-release-readiness job aggregates both hosts' records in the CI scope (V1-2736): it attests
// the independent-external-consumer row through --ci-unobservable, fails on any nonzero exit, and
// on exit 0 with a known-blocker line names that row and V1-0014 in a warning annotation and the
// step summary. Both live in their own workflow on ci.yml's triggers: a refusing job inside ci.yml
// would fail every CI run, and the AFP-V0-080 static reuse and the tested-tree reuse accept only
// successful ci.yml runs.
func TestAFPV0432CEMReleaseGateRunsOnBothPlatformsAndRetainsOutput(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the cem-release-gate steps run under bash with #!/bin/sh make and go spies")
	if strings.Contains(readWorkflow(t, "ci.yml"), "make cem-release-gate") {
		t.Error("ci.yml must not run the release gate: its refusal would fail every CI run and end CI reuse")
	}
	w := readWorkflow(t, "cem-release-gate.yml")
	for _, line := range []string{
		"  pull_request:",
		"  merge_group:",
		"    branches: [main]",
		"  contents: read",
	} {
		wantLine(t, "cem-release-gate.yml", w, line)
	}
	// Only the AFP-V0-672 reuse decision step may continue on error (an error there means FULL).
	for _, job := range []string{"cem-release-gate", "cem-release-readiness", "cem-release-record"} {
		if strings.Contains(jobBlock(t, w, job), "continue-on-error") {
			t.Errorf("%s must not mask a failed step with continue-on-error", job)
		}
	}
	if strings.Count(w, "continue-on-error") != 1 || !strings.Contains(jobBlock(t, w, "cem-release-reuse"), "continue-on-error: true") {
		t.Error("continue-on-error belongs only on the cem-release-reuse decision step")
	}
	const reuseAlways = "        if: ${{ always() && needs.cem-release-reuse.outputs.mode != 'REUSE' }}"
	b := jobBlock(t, w, "cem-release-gate")
	for _, line := range []string{
		"    needs: cem-release-reuse",
		"    if: ${{ !cancelled() && !contains(github.event.pull_request.labels.*.name, 'ci:batched') }}",
		"    runs-on: ${{ matrix.os }}",
		"        os: [ubuntu-24.04, macos-15]",
		"      fail-fast: false",
		`          go-version: "1.27.3"`,
		`          node-version: "24.11.1"`,
		`          make cem-release-gate CEM_RELEASE_RECORD="$out/record.json" CEM_RELEASE_OUTCOME="$out/outcome.json" > "$out/output.txt" 2>&1 || code=$?`,
		reuseAlways,
		"          name: cem-release-gate-${{ matrix.os }}",
		"          path: ${{ runner.temp }}/cem-release-gate/",
	} {
		wantLine(t, "cem-release-gate", b, line)
	}
	script := runScript(t, b)
	passed := `{"profile":"cem-release-run/1","runner":{"cem01-go:repository-envelope":"PASSED","corvint:tamper-matrix":"PASSED"},"platforms":{"linux/amd64":"PASSED"}}`
	unrun := `{"profile":"cem-release-run/1","runner":{"cem01-go:repository-envelope":"NOT_RUN","corvint:tamper-matrix":"PASSED"},"platforms":{"linux/amd64":"PASSED"}}`
	for _, tc := range []struct {
		name, exit, record string
		outcome            bool
		want               int
	}{
		{"ready", "0", passed, true, 0},
		{"host readiness refusal after the suites", "2", passed, true, 0},
		{"suite failure", "2", "", false, 2},
		{"record without outcome", "2", passed, false, 2},
		{"envelope suite recorded NOT_RUN", "2", unrun, true, 2},
		{"record without runner rows", "2", `{"runner":{}}`, true, 2},
	} {
		dir := t.TempDir()
		spy := "echo \"cem-release-suites: readiness: REFUSED (spy $1)\"\n"
		if tc.record != "" {
			spy += "for a; do case $a in CEM_RELEASE_RECORD=*) echo '" + tc.record + "' > \"${a#*=}\";; esac; done\n"
		}
		if tc.outcome {
			spy += "for a; do case $a in CEM_RELEASE_OUTCOME=*) echo '{}' > \"${a#*=}\";; esac; done\n"
		}
		writeExecutable(t, filepath.Join(dir, "bin", "make"), spy+"exit "+tc.exit+"\n")
		summary := filepath.Join(dir, "summary.md")
		code, out := runBash(t, dir, script,
			"PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"RUNNER_TEMP="+dir, "GITHUB_STEP_SUMMARY="+summary)
		if code != tc.want {
			t.Errorf("%s: step exited %d, want %d\n%s", tc.name, code, tc.want, out)
		}
		retained := readText(t, filepath.Join(dir, "cem-release-gate", "output.txt"))
		if !strings.Contains(retained, "readiness: REFUSED (spy cem-release-gate)") {
			t.Errorf("%s: retained output %q lacks the gate output", tc.name, retained)
		}
		if s := readText(t, summary); !strings.Contains(s, "make cem-release-gate exited "+tc.exit+";") {
			t.Errorf("%s: summary %q does not name the exit", tc.name, s)
		}
		if failed := strings.Contains(out, "::error::"); failed != (tc.want != 0) {
			t.Errorf("%s: error annotation present = %v\n%s", tc.name, failed, out)
		}
	}

	r := jobBlock(t, w, "cem-release-readiness")
	for _, line := range []string{
		"    if: ${{ !cancelled() && !contains(github.event.pull_request.labels.*.name, 'ci:batched') && needs.cem-release-gate.result == 'success' }}",
		"    needs: [cem-release-reuse, cem-release-gate]",
		"    runs-on: ubuntu-24.04",
		`          go-version: "1.27.3"`,
		"          pattern: cem-release-gate-*",
		`          go build -o "$RUNNER_TEMP/cem-release-suites" ./tools/cem-release-suites`,
		"          unobservable=independent-external-consumer",
		`          "$RUNNER_TEMP/cem-release-suites" --aggregate --readiness tools/cem-release-suites/READINESS.json --ci-unobservable "$unobservable" --outcome "$out/outcome.json" "$records/cem-release-gate-ubuntu-24.04/record.json" "$records/cem-release-gate-macos-15/record.json" > "$out/output.txt" 2>&1 || code=$?`,
		`          exit "$code"`,
		reuseAlways,
		"          name: cem-release-readiness",
		"          path: ${{ runner.temp }}/cem-release-readiness/",
	} {
		wantLine(t, "cem-release-readiness", r, line)
	}
	script = runScript(t, r)
	for _, tc := range []struct {
		exit    string
		blocker bool
	}{{"0", true}, {"0", false}, {"3", false}, {"1", false}} {
		exit := tc.exit
		dir := t.TempDir()
		// The go spy builds a tool spy that reports its arguments and exits with the tool's code.
		tool := "#!/bin/sh\necho \"cem-release-suites: readiness: REFUSED (spy $1 $4=$5 $(basename $(dirname $8)) $(basename $(dirname $9)))\"\n"
		if tc.blocker {
			tool += "echo \"cem-release-suites: readiness: known blocker: external $5 NOT_OBSERVED (caller-attested not CI-observable)\"\n"
		}
		tool += "exit " + exit + "\n"
		writeExecutable(t, filepath.Join(dir, "bin", "go"),
			"test \"$1 $2 $4\" = \"build -o ./tools/cem-release-suites\" || exit 9\nprintf '%s' '"+tool+"' > \"$3\" && chmod +x \"$3\"\n")
		summary := filepath.Join(dir, "summary.md")
		code, out := runBash(t, dir, script,
			"PATH="+filepath.Join(dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
			"RUNNER_TEMP="+dir, "GITHUB_STEP_SUMMARY="+summary)
		if want, _ := strconv.Atoi(exit); code != want {
			t.Errorf("aggregate exit %s: step exited %d\n%s", exit, code, out)
		}
		retained := readText(t, filepath.Join(dir, "cem-release-readiness", "output.txt"))
		if !strings.Contains(retained, "REFUSED (spy --aggregate --ci-unobservable=independent-external-consumer cem-release-gate-ubuntu-24.04 cem-release-gate-macos-15)") {
			t.Errorf("aggregate exit %s: retained output %q does not judge both hosts' records", exit, retained)
		}
		if s := readText(t, summary); !strings.Contains(s, "--aggregate exited "+exit+".") {
			t.Errorf("aggregate exit %s: summary %q does not name the exit", exit, s)
		}
		if failed := strings.Contains(out, "::error::"); failed != (exit != "0") {
			t.Errorf("aggregate exit %s: error annotation present = %v\n%s", exit, failed, out)
		}
		warned := strings.Contains(out, "::warning::") && strings.Contains(out, "independent-external-consumer is NOT_OBSERVED") && strings.Contains(out, "V1-0014")
		summarized := strings.Contains(readText(t, summary), "Known blocker: external row `independent-external-consumer` is NOT_OBSERVED (V1-0014)")
		if warned != tc.blocker || summarized != tc.blocker {
			t.Errorf("aggregate exit %s blocker %v: warning %v, summary %v\n%s", exit, tc.blocker, warned, summarized, out)
		}
	}
}

// AFP-V0-433 (V1-1687): cce-live runs `make cce-live` on macos-15 with the Playwright modules and
// node the live tests read, after installing exactly @playwright/test@1.64.0 and its headless
// shell into the runner's temporary directory; every step after the change check is gated on it.
func TestAFPV0433CCELiveRunsTheLiveTargetOnTheQualifiedHost(t *testing.T) {
	b := jobBlock(t, readWorkflow(t, "ci.yml"), "cce-live")
	for _, line := range []string{
		batchedJobCondition,
		"    runs-on: macos-15",
		`          go-version: "1.27.3"`,
		`          node-version: "24.11.1"`,
		`          mkdir -p "$RUNNER_TEMP/pw"`,
		`          cd "$RUNNER_TEMP/pw"`,
		"          npm install --no-save @playwright/test@1.64.0",
		"          npx playwright install chromium-headless-shell",
		"          CORVINT_PLAYWRIGHT_MODULES: ${{ runner.temp }}/pw/node_modules",
		"          CORVINT_E2E_NODE_BIN: node",
		`          make cce-live 2>&1 | tee "$RUNNER_TEMP/cce-live/output.txt"`,
		"          set -o pipefail",
	} {
		wantLine(t, "cce-live", b, line)
	}
	if strings.Contains(b, "continue-on-error") {
		t.Error("cce-live must not mask a failed live test with continue-on-error")
	}
	_, steps, _ := strings.Cut(b, "        id: changed\n")
	n := strings.Count(steps, "\n      - ")
	gated := strings.Count(steps, "        if: steps.changed.outputs.run == '1'\n") +
		strings.Count(steps, "        if: always() && steps.changed.outputs.run == '1'\n")
	if n == 0 || gated != n {
		t.Errorf("%d of the %d steps after the change check are gated on it", gated, n)
	}
	if !strings.Contains(readText(t, "../../Makefile"), "\tCORVINT_CCE_LIVE=required ") {
		t.Error("make cce-live no longer sets CORVINT_CCE_LIVE=required, so a missing toolchain could skip")
	}
}

// AFP-V0-434: the change check runs the live tests when the tested commit changes
// internal/changee2e or ci.yml against its first parent, or when no first parent is present,
// and otherwise leaves `run` unset with a summary line.
func TestAFPV0434CCELiveRunsOnlyWhenChangee2eChanges(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the change check runs under bash")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	script := stepScript(t, jobBlock(t, readWorkflow(t, "ci.yml"), "cce-live"), "changed")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(dir, rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, change string
		want         bool
	}{
		{"changee2e source", "internal/changee2e/prove.go", true},
		{"changee2e testdata", "internal/changee2e/testdata/x.json", true},
		{"this workflow", ".github/workflows/ci.yml", true},
		{"other package", "internal/other/x.go", false},
		{"sibling prefix", "internal/changee2e-other/x.go", false},
		{"no first parent", "", true},
	} {
		dir := t.TempDir()
		git(dir, "init", "-q")
		for _, rel := range []string{"internal/changee2e/prove.go", "internal/changee2e/testdata/x.json", ".github/workflows/ci.yml", "internal/other/x.go", "internal/changee2e-other/x.go"} {
			write(dir, rel, "base\n")
		}
		git(dir, "add", "-A")
		git(dir, "commit", "-q", "-m", "base")
		if tc.change != "" {
			write(dir, tc.change, "head\n")
			git(dir, "commit", "-q", "-am", "head")
		}
		output, summary := filepath.Join(dir, ".output"), filepath.Join(dir, ".summary")
		if code, out := runBash(t, dir, script, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+summary); code != 0 {
			t.Fatalf("%s: change check exited %d\n%s", tc.name, code, out)
		}
		got, _ := os.ReadFile(output)
		if ran := string(got) == "run=1\n"; ran != tc.want {
			t.Errorf("%s: output %q, want run=%v", tc.name, got, tc.want)
		}
		if s, _ := os.ReadFile(summary); !tc.want && !strings.Contains(string(s), "the live tests did not run") {
			t.Errorf("%s: summary %q does not say the tests did not run", tc.name, s)
		}
	}
}

// AFP-V0-435: before the live tests, the job fails with an error naming the expected tuple
// unless Playwright pins headless shell build 1248, that build's executable reports the
// qualified version, and node is the qualified version. The expected tuple equals the 1.64.0 row
// of qualifiedBundledBrowsers, which qualify checks receipts against.
func TestAFPV0435CCELiveRefusesAnUnqualifiedHeadlessShell(t *testing.T) {
	testplatform.SkipPOSIXShell(t, "the tuple check runs under bash with #!/bin/sh spies")
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is not available")
	}
	script := stepScript(t, jobBlock(t, readWorkflow(t, "ci.yml"), "cce-live"), "tuple")
	src := readText(t, "../../internal/jstestprovider/external.go")
	row := regexp.MustCompile(`"1\.64\.0": \{\s*revision:\s*"([0-9]+)",\s*observedVersion:\s*"([^"]+)",`).FindStringSubmatch(src)
	if row == nil {
		t.Fatal("no 1.64.0 row in qualifiedBundledBrowsers")
	}
	for _, line := range []string{"want_build=" + row[1], "want_version='" + row[2] + "'", "want_node=v24.11.1"} {
		if strings.Count(script, line+"\n") != 1 {
			t.Errorf("tuple check lacks %q; it must equal the qualified 1.64.0 tuple", line)
		}
	}
	for _, tc := range []struct {
		name, revision, version, node string
		noExe, ok                     bool
	}{
		{name: "qualified", revision: row[1], version: row[2], node: "v24.11.1", ok: true},
		{name: "older build", revision: "1243", version: row[2], node: "v24.11.1"},
		{name: "other executable version", revision: row[1], version: "Google Chrome for Testing 157.0.1.1", node: "v24.11.1"},
		{name: "no executable", revision: row[1], noExe: true, node: "v24.11.1"},
		{name: "other node", revision: row[1], version: row[2], node: "v22.23.3"},
	} {
		dir := t.TempDir()
		home, pw, bin := filepath.Join(dir, "home"), filepath.Join(dir, "pw"), filepath.Join(dir, "bin")
		if err := os.MkdirAll(filepath.Join(pw, "node_modules", "playwright-core"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, filepath.Join(pw, "node_modules", "playwright-core", "browsers.json"), map[string]any{
			"browsers": []map[string]string{{"name": "chromium", "revision": tc.revision}, {"name": "chromium-headless-shell", "revision": tc.revision}},
		})
		if !tc.noExe {
			writeExecutable(t, filepath.Join(home, "Library", "Caches", "ms-playwright", "chromium_headless_shell-"+row[1],
				"chrome-headless-shell-mac-arm64", "chrome-headless-shell"), "echo '"+tc.version+"'\n")
		}
		writeExecutable(t, filepath.Join(bin, "node"), "echo "+tc.node+"\n")
		summary := filepath.Join(dir, "summary.md")
		code, out := runBash(t, dir, script, "HOME="+home, "PW_DIR="+pw, "GITHUB_STEP_SUMMARY="+summary,
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		if tc.ok {
			if code != 0 || strings.Contains(out, "::error::") {
				t.Errorf("%s: exited %d, want 0\n%s", tc.name, code, out)
			}
			continue
		}
		if code == 0 || !strings.Contains(out, "::error::") || !strings.Contains(out, "build "+row[1]+" reporting '"+row[2]+"'") {
			t.Errorf("%s: exited %d without the tuple error\n%s", tc.name, code, out)
		}
		if s := readText(t, summary); !strings.Contains(s, "The job fails instead of passing.") {
			t.Errorf("%s: summary %q lacks the refusal", tc.name, s)
		}
	}
}
