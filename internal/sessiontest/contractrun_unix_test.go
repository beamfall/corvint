// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package sessiontest

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testduration"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// fakeJest stands in for Jest: it answers --version, logs its argv and
// writes a Jest --json report for the files after --runTestsByPath to the
// --outputFile path, one test "works" per file, as FAKE_MODE says: pass,
// fail (a failing test, exit 1), noreport, partial (only the first file),
// runfail (success false with every test passing) or exit1 (every test
// passes, exit 1); fifo leaves a FIFO at the report path and exits 0.
const fakeJest = `#!/bin/sh
[ "$1" = --version ] && { echo 29.7.0; exit 0; }
echo "$@" >> "$FAKE_DIR/jest-argv"
here=$(pwd -P)
out="" files="" bypath=0
for a in "$@"; do
  case "$a" in
  --outputFile=*) out="${a#--outputFile=}" ;;
  --runTestsByPath) bypath=1 ;;
  -*) ;;
  *) [ "$bypath" = 1 ] && files="$files $a" ;;
  esac
done
[ "$FAKE_MODE" = noreport ] && exit 0
[ "$FAKE_MODE" = fifo ] && { mkfifo "$out"; exit 0; }
status=passed success=true code=0
case "$FAKE_MODE" in
fail) status=failed success=false code=1 ;;
runfail) success=false ;;
exit1) code=1 ;;
esac
sep="" body=""
for f in $files; do
  body="$body$sep{\"name\":\"$here/$f\",\"status\":\"$status\",\"assertionResults\":[{\"ancestorTitles\":[\"suite\"],\"title\":\"works\",\"status\":\"$status\",\"duration\":7}]}"
  sep=","
  [ "$FAKE_MODE" = partial ] && break
done
printf '{"success":%s,"testResults":[%s]}\n' "$success" "$body" > "$out"
exit $code
`

// contractFixture is a local narrowed manifest of two selected Jest files
// in instance jest@., whose declared runner command is the fake Jest.
func contractFixture(t *testing.T, mode string) (*Runner, Manifest, string, string) {
	t.Helper()
	runner, m, _, dir := runnerFixture(t, "pass")
	if err := execfile.WriteFile(filepath.Join(dir, "fakejest"), []byte(fakeJest), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Environ = []string{"PATH=" + os.Getenv("PATH"), "FAKE_MODE=" + mode, "FAKE_DIR=" + dir}
	runner.Declared = BuiltInDeclared()
	runner.Declared.Runners = []RunnerCommand{{Key: "jest", Argv: []string{"fakejest"}}}
	writeJestInstall(t, dir, "29.7.0")
	m.Units = []Unit{
		{Adapter: AdapterContract, Shard: 0, Slice: "jest@.", Unit: "jest:src/a.test.js"},
		{Adapter: AdapterContract, Shard: 0, Slice: "jest@.", Unit: "jest:src/b.test.js"},
	}
	sourceDigest, err := Digest(m.Source)
	if err != nil {
		t.Fatal(err)
	}
	m.SourceDigest = sourceDigest
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	return runner, m, digest, dir
}

// RNC-V1-019, RNC-V1-005, RNC-V1-006, RNC-V1-010: a contract invocation
// runs exactly the selected files with the report written to a private
// file, and the receipt records the instance, tier and exact flag, each
// file's outcome under its canonical test ID, and one outcome and duration
// per test read from the report; the report directory is removed.
func TestRNCV1019ContractRunRecordsPerTestResults(t *testing.T) {
	runner, m, digest, dir := contractFixture(t, "pass")
	var output bytes.Buffer
	runner.Output = &output
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Incomplete) != 0 || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	run := receipt.Invocations[0]
	if run.Adapter != AdapterContract || run.AdapterManifest != AdapterManifestProfile || run.Instance != "jest@." || run.Tier != contract.TierFile ||
		run.Exact == nil || !*run.Exact || run.Exit != 0 || run.Runner != filepath.Join(dir, "fakejest") || run.Version != "29.7.0" {
		t.Fatalf("invocation %+v", run)
	}
	// PTD-V1-002, PTD-V1-004 (V1-5542): each file outcome records the sum of its tests' report
	// times, at file granularity, from the Jest report's assertion duration.
	wantTests := []contract.Result{
		{ID: "jest:src/a.test.js::suite::works", Outcome: contract.OutcomePass, DurationMillis: 7, Granularity: "test"},
		{ID: "jest:src/b.test.js::suite::works", Outcome: contract.OutcomePass, DurationMillis: 7, Granularity: "test"},
	}
	if len(run.Outcomes) != 2 || !slices.Equal(run.Tests, wantTests) {
		t.Fatalf("outcomes %+v tests %+v", run.Outcomes, run.Tests)
	}
	for index, file := range []string{"jest:src/a.test.js", "jest:src/b.test.js"} {
		outcome := run.Outcomes[index]
		if outcome.Outcome != OutcomePass || outcome.Slice != "jest@." || outcome.TestID != file || outcome.Unit != file || outcome.DurationMs == nil || *outcome.DurationMs != 7 ||
			outcome.DurationSource != testduration.SourceJest || outcome.DurationGranularity != testduration.GranularityFile {
			t.Fatalf("outcome %d %+v", index, outcome)
		}
	}
	report := ""
	for _, word := range run.Argv {
		if value, ok := strings.CutPrefix(word, "--outputFile="); ok {
			report = value
		}
	}
	if report == "" || report == ContractReportPlaceholder || !slices.Equal(run.Argv[len(run.Argv)-3:], []string{"--runTestsByPath", "src/a.test.js", "src/b.test.js"}) {
		t.Fatalf("argv %q", run.Argv)
	}
	if _, err := os.Stat(filepath.Dir(report)); !os.IsNotExist(err) {
		t.Fatalf("the report directory %s was left behind: %v", filepath.Dir(report), err)
	}
	logged, _ := os.ReadFile(filepath.Join(dir, "jest-argv"))
	if string(logged) != strings.Join(run.Argv[1:], " ")+"\n" {
		t.Fatalf("logged argv %q", logged)
	}
	if !strings.Contains(output.String(), "contract jest@.: 2 results: 2 passed, 0 failed, 0 errored, 0 skipped, 0 NOT_RUN\n") {
		t.Fatalf("output %q", output.String())
	}
	if result, reasons := Verify(m, digest, []Receipt{receipt}, nil); result != ResultPass {
		t.Fatalf("verify %s %q", result, reasons)
	}
	encoded, err := Encode(receipt)
	if err != nil || !bytes.Contains(encoded, []byte(`"instance":"jest@.","tier":"file","exact":true,"tests":[{"testId":"jest:src/a.test.js::suite::works","outcome":"pass","durationMillis":7,"granularity":"test"}`)) {
		t.Fatalf("encoded receipt %v %s", err, encoded)
	}
}

// RNC-V1-019, RNC-V1-006: a failing test fails its file; a missing report
// or a test the report does not answer is NOT_RUN and the run INCOMPLETE; a
// run-level failure no test carries, or a nonzero exit with every test
// passing, is never a pass; and the exit status is kept in the receipt.
func TestRNCV1019ContractRunFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		mode, result, reason string
		outcomes             []string
		exit                 int
	}{
		{mode: "fail", result: ResultFail, outcomes: []string{OutcomeFail, OutcomeFail}, exit: 1},
		{mode: "noreport", result: ResultIncomplete, reason: "contract jest@.: the machine report cannot be read (the runner wrote no machine report); its 2 selected test files are NOT_RUN", outcomes: []string{OutcomeNotRun, OutcomeNotRun}},
		{mode: "partial", result: ResultIncomplete, reason: "contract jest@.: 1 selected tests have no result in the report, so they are NOT_RUN: [jest:src/b.test.js]", outcomes: []string{OutcomePass, OutcomeNotRun}},
		{mode: "runfail", result: ResultFail, outcomes: []string{OutcomeFail, OutcomeFail}},
		{mode: "exit1", result: ResultFail, outcomes: []string{OutcomePass, OutcomePass}, exit: 1},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			runner, m, digest, _ := contractFixture(t, tc.mode)
			receipt := runner.Execute(context.Background(), m, digest, 0)
			if receipt.Result != tc.result || len(receipt.Invocations) != 1 || receipt.Invocations[0].Exit != tc.exit {
				t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
			}
			if tc.reason != "" && !slices.Contains(receipt.Incomplete, tc.reason) {
				t.Fatalf("incomplete %q, want %q", receipt.Incomplete, tc.reason)
			}
			var got []string
			for _, outcome := range receipt.Invocations[0].Outcomes {
				got = append(got, outcome.Outcome)
			}
			if !slices.Equal(got, tc.outcomes) {
				t.Fatalf("outcomes %q, want %q", got, tc.outcomes)
			}
			if tc.mode == "partial" && !slices.Contains(receipt.Invocations[0].Tests, contract.Result{ID: "jest:src/b.test.js", Outcome: contract.OutcomeNotRun, Granularity: "file"}) {
				t.Fatalf("tests %+v", receipt.Invocations[0].Tests)
			}
		})
	}
}

// RNC-V1-019: a runner that cannot be discovered, or a selection the
// contract refuses, launches nothing: each selected file is NOT_RUN beside
// the reason and the run is INCOMPLETE.
func TestRNCV1019ContractRunnerUnavailable(t *testing.T) {
	runner, m, digest, dir := contractFixture(t, "pass")
	if err := os.Remove(filepath.Join(dir, "fakejest")); err != nil {
		t.Fatal(err)
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultIncomplete || !slices.Contains(receipt.Incomplete, "contract fakejest: runner fakejest not found") ||
		len(receipt.Invocations) != 1 || receipt.Invocations[0].Instance != "jest@." || receipt.Invocations[0].Outcomes[0].Outcome != OutcomeNotRun {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	runner, m, _, _ = contractFixture(t, "pass")
	m.Units = []Unit{{Adapter: AdapterContract, Shard: 0, Slice: "minitest@.", Unit: "minitest:test/a_test.rb"}}
	digest, _ = Digest(m)
	receipt = runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultIncomplete || !slices.Contains(receipt.Incomplete, "contract minitest@.: no contract selection or full command for this runner instance") ||
		len(receipt.Invocations) != 1 || receipt.Invocations[0].Exit != -1 || receipt.Invocations[0].Outcomes[0].Outcome != OutcomeNotRun {
		t.Fatalf("refused: receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
}

// fakePlaywright writes a Playwright JSON report for one passing test to
// the path PLAYWRIGHT_JSON_OUTPUT_NAME names, and fails without a report
// when that path is still the plan's placeholder.
const fakePlaywright = `#!/bin/sh
[ "$1" = --version ] && { echo 1.48.0; exit 0; }
echo "$PLAYWRIGHT_JSON_OUTPUT_NAME" > "$FAKE_DIR/playwright-env"
case "$PLAYWRIGHT_JSON_OUTPUT_NAME" in ""|*"{report}"*) exit 7 ;; esac
printf '{"config":{"rootDir":"%s"},"suites":[{"title":"a.spec.ts","file":"e2e/a.spec.ts","specs":[{"title":"works","tests":[{"projectName":"chromium","status":"expected","results":[{"duration":5}]}]}]}]}\n' "$(pwd -P)" > "$PLAYWRIGHT_JSON_OUTPUT_NAME"
`

// RNC-V1-019, RNC-V1-007: a Playwright contract run gets the real report
// path through PLAYWRIGHT_JSON_OUTPUT_NAME, never the placeholder, and the
// receipt records that one environment entry and the per-test result.
func TestRNCV1019PlaywrightReportPathReachesTheRunner(t *testing.T) {
	runner, m, _, dir := contractFixture(t, "pass")
	if err := execfile.WriteFile(filepath.Join(dir, "fakeplaywright"), []byte(fakePlaywright), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared.Runners = []RunnerCommand{{Key: "playwright", Argv: []string{"fakeplaywright", "test"}}}
	m.Units = []Unit{{Adapter: AdapterContract, Kind: contract.KindE2E, KindSource: contract.KindSourceRunner, Shard: 0, Slice: "playwright@.", Unit: "playwright:e2e/a.spec.ts"}}
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	run := receipt.Invocations[0]
	if len(run.Env) != 1 || !strings.HasPrefix(run.Env[0], "PLAYWRIGHT_JSON_OUTPUT_NAME=") || strings.Contains(run.Env[0], ContractReportPlaceholder) {
		t.Fatalf("recorded env %q", run.Env)
	}
	logged, _ := os.ReadFile(filepath.Join(dir, "playwright-env"))
	if strings.TrimSpace(string(logged)) != strings.TrimPrefix(run.Env[0], "PLAYWRIGHT_JSON_OUTPUT_NAME=") {
		t.Fatalf("runner saw %q, receipt %q", logged, run.Env)
	}
	if !slices.Equal(run.Tests, []contract.Result{{ID: "playwright:e2e/a.spec.ts::chromium::works", Outcome: contract.OutcomePass, DurationMillis: 5, Granularity: "test"}}) {
		t.Fatalf("tests %+v", run.Tests)
	}
}

// fakePytest stands in for pytest: it answers --version and writes, to
// the --junitxml path, a JUnit XML report (xunit1) in which only test_add
// of tests/test_calc.py ran and passed, as when a plugin, -k or a
// collection skip deselects its sibling test_sub.
const fakePytest = `#!/bin/sh
[ "$1" = --version ] && { echo "pytest 8.3.3"; exit 0; }
out=""
for a in "$@"; do
  case "$a" in --junitxml=*) out="${a#--junitxml=}" ;; esac
done
printf '<testsuites><testsuite name="pytest"><testcase classname="tests.test_calc" name="test_add" file="tests/test_calc.py" time="0.004"/></testsuite></testsuites>\n' > "$out"
`

// RNC-V1-006: a test the inventory lists in a selected file that the
// runner omits is NOT_RUN, so a passing sibling in the same file cannot
// make the run PASS: the file is NOT_RUN and the run INCOMPLETE.
func TestRNCV1006OmittedInventoriedTestIsNotRun(t *testing.T) {
	runner, m, _, dir := contractFixture(t, "pass")
	if err := execfile.WriteFile(filepath.Join(dir, "fakepytest"), []byte(fakePytest), 0o755); err != nil {
		t.Fatal(err)
	}
	runner.Declared.Runners = []RunnerCommand{{Key: "pytest", Argv: []string{"fakepytest"}}}
	m.Units = []Unit{{Adapter: AdapterContract, Shard: 0, Slice: "pytest@.", Unit: "pytest:tests/test_calc.py"}}
	m.Listed = []ListedTests{{Slice: "pytest@.", Unit: "pytest:tests/test_calc.py", Tests: []string{"pytest:tests/test_calc.py::test_add", "pytest:tests/test_calc.py::test_sub"}}}
	digest, err := Digest(m)
	if err != nil {
		t.Fatal(err)
	}
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result == ResultPass || receipt.Result != ResultIncomplete || len(receipt.Invocations) != 1 {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	run := receipt.Invocations[0]
	if len(run.Outcomes) != 1 || run.Outcomes[0].Outcome != OutcomeNotRun {
		t.Fatalf("outcomes %+v", run.Outcomes)
	}
	want := []contract.Result{
		{ID: "pytest:tests/test_calc.py::test_add", Outcome: contract.OutcomePass, RunnerOutcome: contract.PytestPassed, DurationMillis: 4, Granularity: "test"},
		{ID: "pytest:tests/test_calc.py::test_sub", Outcome: contract.OutcomeNotRun, Granularity: "test"},
	}
	if !slices.Equal(run.Tests, want) {
		t.Fatalf("tests %+v", run.Tests)
	}
	if !slices.Contains(receipt.Incomplete, "contract pytest@.: 1 selected tests have no result in the report, so they are NOT_RUN: [pytest:tests/test_calc.py::test_sub]") {
		t.Fatalf("incomplete %q", receipt.Incomplete)
	}
	if result, _ := Verify(m, digest, []Receipt{receipt}, nil); result == ResultPass {
		t.Fatalf("verify %s", result)
	}
}

// RNC-V1-019: a FIFO the runner leaves at the report path is refused
// without blocking: the receipt is produced within the bound, every
// selected file is NOT_RUN and the run INCOMPLETE.
func TestRNCV1019FIFOReportIsIncomplete(t *testing.T) {
	runner, m, digest, _ := contractFixture(t, "fifo")
	done := make(chan Receipt, 1)
	go func() { done <- runner.Execute(context.Background(), m, digest, 0) }()
	var receipt Receipt
	select {
	case receipt = <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the contract run blocked on a FIFO report")
	}
	if receipt.Result != ResultIncomplete || len(receipt.Invocations) != 1 ||
		!slices.ContainsFunc(receipt.Incomplete, func(reason string) bool {
			return strings.HasPrefix(reason, "contract jest@.: the machine report cannot be read (") && strings.Contains(reason, "not a regular file")
		}) {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	for _, outcome := range receipt.Invocations[0].Outcomes {
		if outcome.Outcome != OutcomeNotRun {
			t.Fatalf("outcomes %+v", receipt.Invocations[0].Outcomes)
		}
	}
}
