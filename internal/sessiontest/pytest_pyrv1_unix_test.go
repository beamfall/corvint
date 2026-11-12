// SPDX-License-Identifier: Apache-2.0

//go:build darwin || linux

package sessiontest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// fakePython stands in for a Python with pytest installed. `--version` is
// the interpreter's; `-m pytest --version` is pytest's, refused when
// pytest.missing exists. Any other `-m pytest` run logs its argv and
// working directory, copies report.xml to the --junitxml path when it
// exists, prints stdout.txt and exits with pytest.exit (default 0).
const fakePython = `#!/bin/sh
if [ "$1" = --version ]; then echo "Python 3.12.1"; exit 0; fi
if [ "$1" = -m ] && [ "$2" = pytest ] && [ "$3" = --version ]; then
  [ -f "$FAKE_DIR/pytest.missing" ] && { echo "No module named pytest" >&2; exit 1; }
  echo "pytest 8.3.2"; exit 0
fi
echo "$@" >> "$FAKE_DIR/pytest-argv"
pwd -P > "$FAKE_DIR/pytest-cwd"
for arg in "$@"; do
  case "$arg" in --junitxml=*) [ -f "$FAKE_DIR/report.xml" ] && cp "$FAKE_DIR/report.xml" "${arg#--junitxml=}";; esac
done
[ -f "$FAKE_DIR/stdout.txt" ] && cat "$FAKE_DIR/stdout.txt"
[ -f "$FAKE_DIR/pytest.exit" ] && exit "$(cat "$FAKE_DIR/pytest.exit")"
exit 0
`

// passingReport is a hand-written pytest JUnit XML report (xunit1, see
// pytestJUnitFixture in internal/testrunner/contract for the format source)
// for the two selected files of instance pytest@svc.
const passingReport = `<?xml version="1.0" encoding="utf-8"?><testsuites><testsuite name="pytest" tests="3">
<testcase classname="tests.test_a" name="test_one" file="tests/test_a.py" time="0.012"/>
<testcase classname="tests.test_a.TestK" name="test_two[x]" file="tests/test_a.py" time="0.003"/>
<testcase classname="tests.test_b" name="test_skip" file="tests/test_b.py" time="0"><skipped type="pytest.skip" message="later"/></testcase>
</testsuite></testsuites>`

// pytestHeader is the session header pytest prints first.
const pytestHeader = "============ test session starts ============\nplatform linux -- Python 3.12.7, pytest-8.3.4, pluggy-1.5.0\nrootdir: /x\ncollected 3 items\n\n"

var pytestFiles = []string{"pytest:svc/tests/test_a.py", "pytest:svc/tests/test_b.py"}

// pytestFixture is runnerFixture with pytest instance svc's two selected
// test files beside the Go unit, a fake python on PATH, the files the fake
// reads and svc/uv.lock. python is not installed when install is false.
func pytestFixture(t *testing.T, install bool, files map[string]string) (*Runner, Manifest, string, string) {
	t.Helper()
	runner, m, _, dir := runnerFixture(t, "pass")
	for _, file := range pytestFiles {
		m.Units = append(m.Units, Unit{Adapter: AdapterRunner, Instance: "pytest@svc", Reason: tierFileReason, ReasonCode: ReasonTierFile, Slice: SliceWhole, Tier: contract.TierFile, Unit: file})
	}
	if install {
		if err := execfile.WriteFile(filepath.Join(dir, "python"), []byte(fakePython), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
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

// runnerInvocation is the receipt's one runner invocation.
func runnerInvocation(t *testing.T, receipt Receipt) Invocation {
	t.Helper()
	for _, invocation := range receipt.Invocations {
		if invocation.Adapter == AdapterRunner {
			return invocation
		}
	}
	t.Fatalf("no runner invocation in %+v", receipt.Invocations)
	return Invocation{}
}

func outcomesOf(invocation Invocation) []string {
	var out []string
	for _, outcome := range invocation.Outcomes {
		out = append(out, outcome.TestID+" "+outcome.Outcome)
	}
	return out
}

// PYR-V1-003, PYR-V1-004, PYR-V1-008: a plan with Go and pytest selections
// runs both, and one receipt records both. pytest runs once from its
// rootdir with the selected files and a private report path; the receipt
// keeps the placeholder argv, the tier, each test's canonical ID, outcome
// and call-phase duration, each file's outcome, and the environment record:
// the Python and pytest versions from the session header and the lockfile's
// SHA-256.
func TestPYRV1003MixedPlanRunsGoAndPytest(t *testing.T) {
	lock := "version = 1\n"
	runner, m, digest, dir := pytestFixture(t, true, map[string]string{"report.xml": passingReport, "stdout.txt": pytestHeader + "..s\n3 passed in 0.02s\n", "svc/uv.lock": lock})
	var output bytes.Buffer
	runner.Output = &output
	receipt := runner.Execute(context.Background(), m, digest, 0)
	if receipt.Result != ResultPass || len(receipt.Incomplete) != 0 || len(receipt.Invocations) != 2 || receipt.Invocations[0].Adapter != AdapterGo {
		t.Fatalf("receipt %s incomplete %q invocations %+v", receipt.Result, receipt.Incomplete, receipt.Invocations)
	}
	pytest := runnerInvocation(t, receipt)
	wantArgv := []string{"python", "-m", "pytest", "--rootdir=.", "--junitxml=" + RunnerReportPlaceholder, "-o", "junit_family=xunit1", "-o", "junit_duration_report=call", "tests/test_a.py", "tests/test_b.py"}
	if !slices.Equal(pytest.Argv, wantArgv) || pytest.Dir != "svc" || pytest.Instance != "pytest@svc" || pytest.Tier != contract.TierFile ||
		pytest.AdapterManifest != AdapterManifestProfile || pytest.Version != "Python 3.12.1" || pytest.Exit != 0 || len(pytest.Reasons) != 0 {
		t.Fatalf("runner invocation %+v", pytest)
	}
	logged, _ := os.ReadFile(filepath.Join(dir, "pytest-argv"))
	if !strings.HasPrefix(string(logged), "-m pytest --rootdir=. --junitxml=/") || strings.Contains(string(logged), RunnerReportPlaceholder) ||
		!strings.HasSuffix(string(logged), " tests/test_a.py tests/test_b.py\n") {
		t.Fatalf("pytest argv %q", logged)
	}
	if cwd, _ := os.ReadFile(filepath.Join(dir, "pytest-cwd")); strings.TrimSpace(string(cwd)) != filepath.Join(dir, "svc") {
		t.Fatalf("pytest ran from %q", cwd)
	}
	wantTests := []contract.Result{
		{ID: "pytest:svc/tests/test_a.py::TestK::test_two[x]", Outcome: contract.OutcomePass, RunnerOutcome: contract.PytestPassed, DurationMillis: 3, Granularity: "test"},
		{ID: "pytest:svc/tests/test_a.py::test_one", Outcome: contract.OutcomePass, RunnerOutcome: contract.PytestPassed, DurationMillis: 12, Granularity: "test"},
		{ID: "pytest:svc/tests/test_b.py::test_skip", Outcome: contract.OutcomeSkip, RunnerOutcome: contract.PytestSkipped, Granularity: "test"},
	}
	if !slices.Equal(pytest.Tests, wantTests) {
		t.Fatalf("tests\n got %+v\nwant %+v", pytest.Tests, wantTests)
	}
	if got := outcomesOf(pytest); !slices.Equal(got, []string{"pytest:svc/tests/test_a.py PASS", "pytest:svc/tests/test_b.py NO_TESTS"}) {
		t.Fatalf("file outcomes %q", got)
	}
	sum := sha256.Sum256([]byte(lock))
	if env := pytest.Environment; env == nil || *env != (RunnerEnvironment{Python: "3.12.7", Pytest: "8.3.4", Lockfile: "svc/uv.lock", LockfileSHA256: hex.EncodeToString(sum[:])}) {
		t.Fatalf("environment %+v", pytest.Environment)
	}
	if !strings.Contains(output.String(), "3 passed in 0.02s") {
		t.Fatalf("pytest output not passed through: %q", output.String())
	}
	if result, reasons := Verify(m, digest, []Receipt{receipt}, nil); result != ResultPass {
		t.Fatalf("verify %s %q", result, reasons)
	}
	if encoded, err := Encode(receipt); err != nil || !bytes.Contains(encoded, []byte(`"tier":"file"`)) || !bytes.Contains(encoded, []byte(`"durationMillis":12`)) {
		t.Fatalf("encoded receipt %s %v", encoded, err)
	}
}

// PYR-V1-004: one failing test makes its file FAIL and the run FAIL; the
// receipt names its canonical ID as failed with its duration. PYR-V1-008:
// with no session header the versions come from the probes, and with no
// lockfile none is recorded.
func TestPYRV1004FailingTestFailsTheRun(t *testing.T) {
	report := strings.Replace(passingReport, `time="0.012"/>`, `time="0.250"><failure message="assert 1 == 2">E assert</failure></testcase>`, 1)
	runner, m, digest, _ := pytestFixture(t, true, map[string]string{"report.xml": report, "pytest.exit": "1"})
	receipt := runner.Execute(context.Background(), m, digest, 0)
	pytest := runnerInvocation(t, receipt)
	if receipt.Result != ResultFail || len(receipt.Incomplete) != 0 || pytest.Exit != 1 {
		t.Fatalf("receipt %s incomplete %q invocation %+v", receipt.Result, receipt.Incomplete, pytest)
	}
	if !slices.Contains(pytest.Tests, contract.Result{ID: "pytest:svc/tests/test_a.py::test_one", Outcome: contract.OutcomeFail, RunnerOutcome: contract.PytestFailed, DurationMillis: 250, Granularity: "test"}) {
		t.Fatalf("tests %+v", pytest.Tests)
	}
	if got := outcomesOf(pytest); !slices.Equal(got, []string{"pytest:svc/tests/test_a.py FAIL", "pytest:svc/tests/test_b.py NO_TESTS"}) {
		t.Fatalf("file outcomes %q", got)
	}
	if env := pytest.Environment; env == nil || *env != (RunnerEnvironment{Python: "3.12.1", Pytest: "8.3.2"}) {
		t.Fatalf("environment %+v", pytest.Environment)
	}
}

// PYR-V1-006: each kind of runner trouble is a non-PASS result with a typed
// reason, the selected files without a result are NOT_RUN, and the Go
// invocation still runs and reports.
func TestPYRV1006RunnerTroubleFailsClosed(t *testing.T) {
	notRun := []string{"pytest:svc/tests/test_a.py NOT_RUN", "pytest:svc/tests/test_b.py NOT_RUN"}
	for name, test := range map[string]struct {
		install  bool
		files    map[string]string
		reason   string
		outcomes []string
		launched bool
	}{
		"no interpreter":     {false, nil, "runner pytest@svc: runner-unavailable: runner python not found", notRun, false},
		"pytest not present": {true, map[string]string{"pytest.missing": ""}, "runner pytest@svc: runner-unavailable: python -m pytest --version failed, so pytest cannot be started or imported", notRun, false},
		"nothing collected": {true, map[string]string{"report.xml": `<testsuites><testsuite name="pytest" tests="0"></testsuite></testsuites>`, "pytest.exit": "5"},
			"runner pytest@svc: no-tests-collected: pytest exit status 5 (no tests collected)", notRun, true},
		"usage error, no report": {true, map[string]string{"pytest.exit": "4"}, "runner pytest@svc: runner-exit: pytest exit status 4 (usage error)", notRun, true},
		"internal error": {true, map[string]string{"report.xml": passingReport, "pytest.exit": "3"}, "runner pytest@svc: runner-exit: pytest exit status 3 (internal error)",
			[]string{"pytest:svc/tests/test_a.py PASS", "pytest:svc/tests/test_b.py NO_TESTS"}, true},
		"invalid report": {true, map[string]string{"report.xml": `<testsuites><testsuite><testcase classname="x" name="t" time="1"/></testsuite></testsuites>`},
			"runner pytest@svc: runner-report-invalid: pytest junit report: testcase \"t\" names no file (want junit_family=xunit1)", notRun, true},
		"plugin deselects a selected test": {true, map[string]string{"report.xml": passingReport, "stdout.txt": "collected 4 items / 1 deselected / 3 selected\n3 passed, 1 deselected in 0.02s\n"},
			"runner pytest@svc: tests-deselected: 1 collected tests were deselected by a plugin or option, so the selected files did not all run",
			[]string{"pytest:svc/tests/test_a.py PASS", "pytest:svc/tests/test_b.py NO_TESTS"}, true},
		"a selected file has no result": {true, map[string]string{"report.xml": strings.Replace(passingReport, `<testcase classname="tests.test_b" name="test_skip" file="tests/test_b.py" time="0"><skipped type="pytest.skip" message="later"/></testcase>`, "", 1)},
			"runner pytest@svc: 1 selected file(s) have no result and are NOT_RUN: pytest:svc/tests/test_b.py",
			[]string{"pytest:svc/tests/test_a.py PASS", "pytest:svc/tests/test_b.py NOT_RUN"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			runner, m, digest, dir := pytestFixture(t, test.install, test.files)
			receipt := runner.Execute(context.Background(), m, digest, 0)
			if receipt.Result != ResultIncomplete || !slices.Contains(receipt.Incomplete, test.reason) {
				t.Fatalf("receipt %s incomplete %q, want %q", receipt.Result, receipt.Incomplete, test.reason)
			}
			if !slices.ContainsFunc(receipt.Invocations, func(invocation Invocation) bool {
				return invocation.Adapter == AdapterGo && invocation.Outcomes[0].Outcome == OutcomePass
			}) {
				t.Fatalf("the Go invocation did not run: %+v", receipt.Invocations)
			}
			pytest := runnerInvocation(t, receipt)
			if got := outcomesOf(pytest); !slices.Equal(got, test.outcomes) {
				t.Fatalf("file outcomes %q, want %q", got, test.outcomes)
			}
			if len(pytest.Reasons) == 0 || pytest.Instance != "pytest@svc" || pytest.Dir != "svc" {
				t.Fatalf("runner invocation names no typed reason: %+v", pytest)
			}
			if _, err := os.Stat(filepath.Join(dir, "pytest-argv")); (err == nil) != test.launched {
				t.Fatalf("pytest launched %v, want %v", err == nil, test.launched)
			}
			if result, _ := Verify(m, digest, []Receipt{receipt}, nil); result == ResultPass {
				t.Fatal("verify passed a run with runner trouble")
			}
		})
	}
}
