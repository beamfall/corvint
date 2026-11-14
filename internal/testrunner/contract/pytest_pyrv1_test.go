// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// pytestJUnitFixture is a hand-written pytest JUnit XML report in the xunit1
// family, the shape pytest's own writer produces (src/_pytest/junitxml.py:
// mangle_test_address joins the node ID's file, with `/` as `.` and `.py`
// dropped, and its class parts into classname, keeps the last part and its
// parameter text as name, and xunit1 keeps the file attribute; a failed
// collector is a testcase with an empty classname and an error; an xfail is
// <skipped type="pytest.xfail">; a strict XPASS is a failure whose message
// starts "[XPASS(strict)]"). It was not produced by a live pytest here
// (NOT_RUN: pytest is not installed on this host).
const pytestJUnitFixture = `<?xml version="1.0" encoding="utf-8"?>
<testsuites name="pytest tests"><testsuite name="pytest" errors="2" failures="2" skipped="2" tests="8" time="0.120" timestamp="2026-11-12T10:00:00" hostname="h">
<testcase classname="tests.test_a.TestA" name="test_x[1-2]" file="tests/test_a.py" line="3" time="0.012" />
<testcase classname="tests.test_a" name="test_y" file="tests/test_a.py" line="9" time="0.002"><failure message="assert 1 == 2">tests/test_a.py:10: AssertionError</failure></testcase>
<testcase classname="tests.test_a" name="test_z" file="tests/test_a.py" line="12" time="0.000"><skipped type="pytest.skip" message="later">tests/test_a.py:12: later</skipped></testcase>
<testcase classname="tests.test_a" name="test_e" file="tests/test_a.py" line="15" time="0.003"><error message="failed on setup with &quot;KeyError&quot;">E KeyError</error></testcase>
<testcase classname="tests.test_a" name="test_xf" file="tests/test_a.py" line="18" time="0.001"><skipped type="pytest.xfail" message="known bug" /></testcase>
<testcase classname="tests.test_a" name="test_xp" file="tests/test_a.py" line="21" time="0.001"><failure message="[XPASS(strict)] fixed">[XPASS(strict)] fixed</failure></testcase>
<testcase classname="pkg.v1.0.test_dots" name="test_d[a.b::c]" file="pkg/v1.0/test_dots.py" line="1" time="0.0045" />
<testcase classname="" name="tests.test_broken" file="tests/test_broken.py" time="0.000"><error message="collection failure">ImportError</error></testcase>
</testsuite></testsuites>`

// PYR-V1-004: the JUnit XML report gives every test its canonical ID, its
// outcome with pytest's own word (xfailed and xpassed included) and its
// call-phase duration; a file that failed to collect is a file-level error.
func TestPYRV1004PytestJUnitReport(t *testing.T) {
	r, _ := For(testid.Pytest)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Pytest, Root: "svc"}, Report: []byte(pytestJUnitFixture)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "pytest:svc/tests/test_a.py::TestA::test_x[1-2]", Outcome: OutcomePass, RunnerOutcome: PytestPassed, DurationMillis: 12, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_y", Outcome: OutcomeFail, RunnerOutcome: PytestFailed, DurationMillis: 2, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_z", Outcome: OutcomeSkip, RunnerOutcome: PytestSkipped, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_e", Outcome: OutcomeError, RunnerOutcome: PytestError, DurationMillis: 3, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_xf", Outcome: OutcomeSkip, RunnerOutcome: PytestXfailed, DurationMillis: 1, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_xp", Outcome: OutcomeFail, RunnerOutcome: PytestXpassed, DurationMillis: 1, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/pkg/v1.0/test_dots.py::test_d[a.b%3A%3Ac]", Outcome: OutcomePass, RunnerOutcome: PytestPassed, DurationMillis: 5, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_broken.py", Outcome: OutcomeError, RunnerOutcome: PytestError, Granularity: testid.GranularityFile},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
	run := Reconcile([]string{"pytest:svc/tests/test_a.py", "pytest:svc/tests/test_broken.py"}, got)
	if !run.Failed || !run.Incomplete {
		t.Fatalf("a failing test and a file that did not collect: %+v", run)
	}
}

// PYR-V1-006: a report pytest did not write in the xunit1 family (no file
// attribute), one renamed by --junitprefix, and one that is not JUnit XML
// are refused, so the caller records every selected test NOT_RUN.
func TestPYRV1006PytestJUnitRefusals(t *testing.T) {
	r, _ := For(testid.Pytest)
	instance := Instance{Runner: testid.Pytest, Root: "."}
	for name, report := range map[string]string{
		"xunit2, no file":  `<testsuites><testsuite><testcase classname="tests.test_a" name="test_y" time="0.1"/></testsuite></testsuites>`,
		"junitprefix":      `<testsuite><testcase classname="pre.tests.test_a" name="test_y" file="tests/test_a.py" time="0.1"/></testsuite>`,
		"other root":       `<report><testcase classname="tests.test_a" name="test_y" file="tests/test_a.py"/></report>`,
		"truncated":        `<testsuites><testsuite><testcase classname="tests.test_a" name="test_y" file="tests/test_a.py"`,
		"escaping file":    `<testsuite><testcase classname="...x" name="test_y" file="../x.py" time="0"/></testsuite>`,
		"not a json shape": `{"tests": 3}`,
	} {
		results, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(report)})
		if err == nil {
			t.Errorf("%s: parsed %+v, want a refusal", name, results)
			continue
		}
		if run := ReconcileParsed([]string{"pytest:tests/test_a.py"}, results, err); !run.Incomplete || !slices.Equal(run.NotRun, []string{"pytest:tests/test_a.py"}) {
			t.Errorf("%s: %+v", name, run)
		}
	}
}

// PYR-V1-006 (xdist): pytest-xdist workers report through the controller,
// which writes one JUnit report merged by node ID; a selected test no
// worker reported is NOT_RUN and the run INCOMPLETE, and a test a crashed
// worker was running is reported as an error (xdist's "worker crashed"
// report), never a pass. The fixture follows xdist's crash report text
// (xdist/dsession.py handle_crashitem), not run here (NOT_RUN).
func TestPYRV1006XdistMergedReport(t *testing.T) {
	r, _ := For(testid.Pytest)
	instance := Instance{Runner: testid.Pytest, Root: "."}
	report := `<testsuites><testsuite name="pytest" tests="3">
<testcase classname="tests.test_a" name="test_one" file="tests/test_a.py" time="0.01"/>
<testcase classname="tests.test_b" name="test_two" file="tests/test_b.py" time="0.02"/>
<testcase classname="tests.test_b" name="test_crash" file="tests/test_b.py" time="0"><failure message="worker 'gw1' crashed while running 'tests/test_b.py::test_crash'">worker 'gw1' crashed</failure></testcase>
</testsuite></testsuites>`
	results, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(report)})
	if err != nil {
		t.Fatal(err)
	}
	selected := []string{"pytest:tests/test_a.py::test_one", "pytest:tests/test_a.py::test_three", "pytest:tests/test_b.py::test_crash", "pytest:tests/test_b.py::test_two"}
	run := Reconcile(selected, results)
	if !run.Incomplete || !run.Failed || !slices.Equal(run.NotRun, []string{"pytest:tests/test_a.py::test_three"}) {
		t.Fatalf("xdist run %+v", run)
	}
}

// PYR-V1-006, PYR-V1-008: the exit statuses that are not test outcomes
// carry typed reasons, and the session header, `pytest --version` and the
// deselected count are read from pytest's own output lines.
func TestPYRV1006PytestOutputSignals(t *testing.T) {
	for code, want := range map[int]string{0: "", 1: "", 2: ReasonRunnerExit, 3: ReasonRunnerExit, 4: ReasonRunnerExit, 5: ReasonNoTestsCollected, 137: ReasonRunnerExit} {
		if got := PytestExitReason(code); !strings.HasPrefix(got, want) || (want == "") != (got == "") {
			t.Errorf("exit %d: %q, want prefix %q", code, got, want)
		}
	}
	if python, pytest, ok := PytestHeader("platform linux -- Python 3.12.1, pytest-8.3.2, pluggy-1.5.0"); !ok || python != "3.12.1" || pytest != "8.3.2" {
		t.Errorf("header: %q %q %v", python, pytest, ok)
	}
	if _, _, ok := PytestHeader("rootdir: /x"); ok {
		t.Error("a rootdir line is not the header")
	}
	if got := PytestVersion([]byte("pytest 8.3.2\n")); got != "8.3.2" {
		t.Errorf("pytest 8 version %q", got)
	}
	if got := PytestVersion([]byte("This is pytest version 6.2.5, imported from /x/pytest/__init__.py\n")); got != "6.2.5" {
		t.Errorf("pytest 6 version %q", got)
	}
	for line, want := range map[string]int{
		"collected 4 items / 1 deselected / 3 selected":             1,
		"================ 3 passed, 2 deselected in 0.05s ========": 2,
		"3 passed in 0.05s": 0,
		"0 deselected":      0,
	} {
		if got, ok := PytestDeselected(line); got != want || ok != (want != 0) {
			t.Errorf("%q: %d %v, want %d", line, got, ok, want)
		}
	}
}

// PTD-V1-002, PTD-V1-003, PTD-V1-007: a JUnit testcase time is read in
// seconds; an absent one is not measured and one that is negative or not a
// number is invalid, each kept as a result rather than refusing the report.
func TestPTDV1002PytestJUnitTimes(t *testing.T) {
	r, _ := For(testid.Pytest)
	report := `<testsuites><testsuite name="pytest">
<testcase classname="tests.test_a" name="test_timed" file="tests/test_a.py" time="0.250"/>
<testcase classname="tests.test_a" name="test_untimed" file="tests/test_a.py"/>
<testcase classname="tests.test_a" name="test_negative" file="tests/test_a.py" time="-1"/>
<testcase classname="tests.test_a" name="test_text" file="tests/test_a.py" time="soon"/>
</testsuite></testsuites>`
	results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Pytest, Root: "."}, Report: []byte(report)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "pytest:tests/test_a.py::test_timed", Outcome: OutcomePass, RunnerOutcome: PytestPassed, DurationMillis: 250, Granularity: testid.GranularityTest},
		{ID: "pytest:tests/test_a.py::test_untimed", Outcome: OutcomePass, RunnerOutcome: PytestPassed, Granularity: testid.GranularityTest, DurationMissing: true},
		{ID: "pytest:tests/test_a.py::test_negative", Outcome: OutcomePass, RunnerOutcome: PytestPassed, Granularity: testid.GranularityTest, DurationInvalid: true},
		{ID: "pytest:tests/test_a.py::test_text", Outcome: OutcomePass, RunnerOutcome: PytestPassed, Granularity: testid.GranularityTest, DurationInvalid: true},
	}
	if !slices.Equal(results, want) {
		t.Fatalf("results\n got %+v\nwant %+v", results, want)
	}
}
