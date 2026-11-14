// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testduration"
	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testrunner/contract"
)

// PTD-V1-002, PTD-V1-003, PTD-V1-004, PTD-V1-007 (V1-5542): a contract file
// unit's duration is the sum of the times the runner's report gave its
// tests, under the runner's own source: Jest's assertion duration, pytest's
// JUnit time and RSpec's run_time. A test with no time, or left NOT_RUN,
// makes the file NOT_MEASURED rather than a partial sum; an invalid time
// makes it invalid; another file's tests never count.
func TestPTDV1002ContractFileDurations(t *testing.T) {
	ms := func(measure testduration.Measure) int64 {
		if measure.Milliseconds == nil {
			return -1
		}
		return *measure.Milliseconds
	}
	for name, test := range map[string]struct {
		file, source string
		results      []contract.Result
		want         int64
		measured     bool
		invalid      bool
	}{
		"jest sum": {"jest:src/a.test.js", testduration.SourceJest, []contract.Result{
			{ID: "jest:src/a.test.js::adds", Outcome: contract.OutcomePass, DurationMillis: 4},
			{ID: "jest:src/a.test.js::cart::totals", Outcome: contract.OutcomeFail, DurationMillis: 3},
			{ID: "jest:src/b.test.js::other", Outcome: contract.OutcomePass, DurationMillis: 50},
		}, 7, true, false},
		"pytest sum": {"pytest:svc/tests/test_a.py", testduration.SourcePytest, []contract.Result{
			{ID: "pytest:svc/tests/test_a.py::test_one", Outcome: contract.OutcomePass, DurationMillis: 12},
			{ID: "pytest:svc/tests/test_a.py::TestK::test_two[x]", Outcome: contract.OutcomePass, DurationMillis: 3},
		}, 15, true, false},
		"rspec file": {"rspec:spec/a_spec.rb", testduration.SourceRSpec, []contract.Result{
			{ID: "rspec:spec/a_spec.rb", Outcome: contract.OutcomePass, DurationMillis: 21, Granularity: "file"},
		}, 21, true, false},
		"one test has no time": {"jest:src/a.test.js", testduration.SourceJest, []contract.Result{
			{ID: "jest:src/a.test.js::adds", Outcome: contract.OutcomePass, DurationMillis: 4},
			{ID: "jest:src/a.test.js::later", Outcome: contract.OutcomeSkip, DurationMissing: true},
		}, -1, false, false},
		"one test NOT_RUN": {"pytest:tests/test_calc.py", testduration.SourcePytest, []contract.Result{
			{ID: "pytest:tests/test_calc.py::test_add", Outcome: contract.OutcomePass, DurationMillis: 4},
			{ID: "pytest:tests/test_calc.py::test_sub", Outcome: contract.OutcomeNotRun},
		}, -1, false, false},
		"negative jest duration": {"jest:src/a.test.js", testduration.SourceJest, []contract.Result{
			{ID: "jest:src/a.test.js::adds", Outcome: contract.OutcomePass, DurationInvalid: true},
		}, -1, false, true},
		"no result": {"jest:src/a.test.js", testduration.SourceJest, nil, -1, false, false},
	} {
		got := contractMeasure(test.file, test.results, test.source)
		if ms(got) != test.want || (got.Source == test.source) != test.measured || got.Invalid != test.invalid || (!test.measured && got.Source != testduration.NotMeasured) {
			t.Errorf("%s: %+v (ms %d), want ms %d measured %v invalid %v", name, got, ms(got), test.want, test.measured, test.invalid)
		}
	}
}

// PTD-V1-003 (V1-5542, review P2-a): a file-level result carries no time of
// its own. A Jest file that fails to load, and a pytest collection error on
// both the JUnit and the pytest-json-report paths, leave the selected file
// NOT_MEASURED, never a measured 0 ms.
func TestPTDV1003FileLevelResultIsNotMeasured(t *testing.T) {
	root := t.TempDir()
	for name, test := range map[string]struct {
		runner, source, file string
		report               string
	}{
		"jest load failure": {testid.Jest, testduration.SourceJest, "jest:src/a.test.js",
			`{"success":false,"testResults":[{"name":"` + root + `/src/a.test.js","status":"failed","message":"Cannot find module './missing'","assertionResults":[]}]}`},
		"pytest junit collection error": {testid.Pytest, testduration.SourcePytest, "pytest:tests/test_a.py",
			`<testsuites><testsuite name="pytest"><testcase classname="" name="tests.test_a" file="tests/test_a.py"><error message="collection failure">ImportError</error></testcase></testsuite></testsuites>`},
		"pytest json collection error": {testid.Pytest, testduration.SourcePytest, "pytest:tests/test_a.py",
			`{"exitcode":2,"collectors":[{"nodeid":"tests/test_a.py","outcome":"failed"}],"tests":[]}`},
	} {
		runner, ok := contract.For(test.runner)
		if !ok {
			t.Fatalf("%s: no contract for %s", name, test.runner)
		}
		results, err := runner.Parse(contract.ParseRequest{Instance: contract.Instance{Runner: test.runner, Root: "."}, Report: []byte(test.report), Root: root})
		if err != nil {
			t.Fatalf("%s: parse: %v", name, err)
		}
		if len(results) != 1 || results[0].Granularity != testid.GranularityFile || !results[0].DurationMissing {
			t.Fatalf("%s: results %+v, want one file-level result with no time", name, results)
		}
		got := contractMeasure(test.file, results, test.source)
		if got.Milliseconds != nil || got.Source != testduration.NotMeasured || got.Invalid {
			t.Errorf("%s: %+v, want NOT_MEASURED", name, got)
		}
	}
}
