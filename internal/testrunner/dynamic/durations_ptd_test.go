// SPDX-License-Identifier: Apache-2.0

package dynamic

import (
	"bytes"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/testduration"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// The report fixtures are each runner's documented report shape, written by hand: Vitest, Jest,
// Playwright, pytest, RSpec and Ruby are not installed on the delivering host.
const (
	// Jest's `--json` report (jestjs.io/docs/cli#--json, the AggregatedResult of
	// @jest/test-result: testResults[].assertionResults[].duration in milliseconds, absent when
	// not timed); Vitest's `--reporter=json` writes the same Jest-compatible shape
	// (vitest.dev/guide/reporters#json-reporter).
	ptdJestReport = `{"numTotalTests":2,"numPassedTests":2,"numFailedTests":0,"numPendingTests":0,"numTodoTests":0,"success":true,
"testResults":[{"name":"/r/math.test.ts","status":"passed","message":"","assertionResults":[
{"title":"adds","fullName":"math > adds","status":"passed","failureMessages":[],"duration":DURATION},
{"title":"subtracts","fullName":"math > subtracts","status":"passed","failureMessages":[]}]}]}`
	// Playwright's JSON reporter (playwright.dev/docs/test-reporters#json, the JSONReport type of
	// @playwright/test/reporter): suites[].specs[].tests[].results[], one result per retry, each
	// with its own duration in milliseconds.
	ptdPlaywrightReport = `{"suites":[{"title":"login.spec.ts","file":"login.spec.ts","specs":[{"id":"s1","title":"signs in","file":"login.spec.ts","tests":[
{"projectId":"chromium","projectName":"chromium","status":"flaky","expectedStatus":"passed","results":[
{"status":"failed","retry":0,"duration":900,"error":{"message":"boom"}},{"status":"passed","retry":1,"duration":700}]}]},
{"id":"s2","title":"signs out","file":"login.spec.ts","tests":[
{"projectId":"chromium","projectName":"chromium","status":"expected","expectedStatus":"passed","results":[{"status":"passed","retry":0,"duration":40}]}]}],"suites":[]}],
"errors":[],"stats":{"expected":1,"unexpected":0,"flaky":1,"skipped":0}}`
	// pytest's `--junitxml` report (docs.pytest.org/en/stable/how-to/output.html#creating-junitxml-format-files):
	// testcase elements with a time attribute in seconds, omitted here on one case.
	ptdPytestReport = `<?xml version="1.0" encoding="utf-8"?>
<testsuites><testsuite name="pytest" tests="2" errors="0" failures="0" skipped="0">
<testcase classname="tests.test_a" name="test_timed" file="tests/test_a.py" time="0.25"></testcase>
<testcase classname="tests.test_a" name="test_untimed" file="tests/test_a.py"></testcase>
</testsuite></testsuites>`
	// RSpec's `--format json` report (RSpec::Core::Formatters::JsonFormatter, rspec-core 3.13):
	// examples[].run_time in seconds.
	ptdRSpecReport = `{"version":"3.13.0","examples":[
{"id":"./spec/a_spec.rb[1:1]","description":"works","status":"passed","file_path":"./spec/a_spec.rb","run_time":0.012},
{"id":"./spec/a_spec.rb[1:2]","description":"lingers","status":"passed","file_path":"./spec/a_spec.rb"}],
"summary":{"example_count":2,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0}}`
	// Corvint's own Minitest reporter output, reporters/minitest_reporter.rb: JSON.generate of
	// Profile, Complete, Count, Tests and Problems, each Tests row {ID, Name, State} with DurationMs
	// from Minitest::Result#time (TestPTDV1002MinitestReporterOutput pins the reporter source).
	ptdMinitestReport = `{"Profile":"corvint-minitest/0","Complete":true,"Count":3,"Tests":[
{"ID":"CalcTest#test_add","Name":"test_add","State":"PASSED","DurationMs":7},
{"ID":"CalcTest#test_sub","Name":"test_sub","State":"PASSED"},
{"ID":"CalcTest#test_neg","Name":"test_neg","State":"PASSED","DurationMs":-2}],"Problems":[]}`
)

func ptdParse(t *testing.T, runner, report, body string) tr.Observation {
	t.Helper()
	o, err := Parse(tr.Input{Runner: runner, Reports: map[string][]byte{report: []byte(body)}})
	if err != nil {
		t.Fatalf("%s: %v", runner, err)
	}
	return o
}

func ptdTest(t *testing.T, o tr.Observation, id string) tr.Test {
	t.Helper()
	for _, test := range o.Tests {
		if test.ID == id {
			return test
		}
	}
	t.Fatalf("%s: no test %q in %+v", o.Runner, id, o.Tests)
	return tr.Test{}
}

func ptdWant(t *testing.T, test tr.Test, ms int64, source string) {
	t.Helper()
	if source == testduration.NotMeasured {
		if test.DurationMs != nil || test.DurationSource != testduration.NotMeasured {
			t.Fatalf("%s: want NOT_MEASURED, got %+v", test.ID, test)
		}
		return
	}
	if test.DurationMs == nil || *test.DurationMs != ms || test.DurationSource != source {
		t.Fatalf("%s: want %d ms from %s, got %+v", test.ID, ms, source, test)
	}
}

// PTD-V1-002, PTD-V1-003, PTD-V1-006, PTD-V1-007: each runner's time comes from its own report:
// Vitest and Jest assertion duration, the sum of Playwright attempt durations with each attempt
// kept, the pytest JUnit time attribute and the RSpec run_time; a test whose report has no time is
// NOT_MEASURED; a negative Jest duration is NOT_MEASURED and listed under durationInvalid.
func TestPTDV1002RunnerReportDurations(t *testing.T) {
	vitest := ptdParse(t, "vitest", "vitest.json", string(bytes.Replace([]byte(ptdJestReport), []byte("DURATION"), []byte("3"), 1)))
	ptdWant(t, ptdTest(t, vitest, "/r/math.test.ts::math > adds"), 3, testduration.SourceVitest)
	ptdWant(t, ptdTest(t, vitest, "/r/math.test.ts::math > subtracts"), 0, testduration.NotMeasured)
	if len(vitest.DurationInvalid) != 0 {
		t.Fatalf("vitest durationInvalid %v", vitest.DurationInvalid)
	}

	jest := ptdParse(t, "jest", "jest.json", string(bytes.Replace([]byte(ptdJestReport), []byte("DURATION"), []byte("-5"), 1)))
	ptdWant(t, ptdTest(t, jest, "/r/math.test.ts::math > adds"), 0, testduration.NotMeasured)
	if !slices.Equal(jest.DurationInvalid, []string{"/r/math.test.ts::math > adds"}) || !jest.Complete {
		t.Fatalf("jest durationInvalid %v complete %v problems %v", jest.DurationInvalid, jest.Complete, jest.Problems)
	}
	if fraction := ptdParse(t, "jest", "jest.json", string(bytes.Replace([]byte(ptdJestReport), []byte("DURATION"), []byte("12.4"), 1))); true {
		ptdWant(t, ptdTest(t, fraction, "/r/math.test.ts::math > adds"), 12, testduration.SourceJest)
	}

	playwright := ptdParse(t, "playwright", "playwright.json", ptdPlaywrightReport)
	retried := ptdTest(t, playwright, "s1::chromium")
	ptdWant(t, retried, 1600, testduration.SourcePlaywright)
	if !slices.Equal(retried.AttemptDurationsMs, []int64{900, 700}) || retried.State != tr.Flaky {
		t.Fatalf("playwright retry %+v", retried)
	}
	once := ptdTest(t, playwright, "s2::chromium")
	ptdWant(t, once, 40, testduration.SourcePlaywright)
	if once.AttemptDurationsMs != nil {
		t.Fatalf("a single attempt lists attempts: %+v", once)
	}

	pytest := ptdParse(t, "pytest", "pytest.xml", ptdPytestReport)
	ptdWant(t, ptdTest(t, pytest, "pytest::tests.test_a::test_timed"), 250, testduration.SourcePytest)
	ptdWant(t, ptdTest(t, pytest, "pytest::tests.test_a::test_untimed"), 0, testduration.NotMeasured)

	rspec := ptdParse(t, "rspec", "rspec.json", ptdRSpecReport)
	ptdWant(t, ptdTest(t, rspec, "./spec/a_spec.rb[1:1]"), 12, testduration.SourceRSpec)
	ptdWant(t, ptdTest(t, rspec, "./spec/a_spec.rb[1:2]"), 0, testduration.NotMeasured)

	// A runner outside the seven records no duration at all.
	bun := ptdParse(t, "bun-test", "bun.xml", ptdPytestReport)
	if test := ptdTest(t, bun, "pytest::tests.test_a::test_timed"); test.DurationMs != nil || test.DurationSource != "" {
		t.Fatalf("bun-test recorded a duration: %+v", test)
	}
}

// PTD-V1-002, PTD-V1-007: Corvint's Minitest reporter fills DurationMs from Minitest::Result#time;
// the reader records it as minitest-reporter, a test without it NOT_MEASURED, and a negative one
// NOT_MEASURED and listed under durationInvalid. Another owned reporter's duration is not read.
func TestPTDV1002MinitestReporterDuration(t *testing.T) {
	minitest := ptdParse(t, "minitest", "minitest.json", ptdMinitestReport)
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_add"), 7, testduration.SourceMinitest)
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_sub"), 0, testduration.NotMeasured)
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_neg"), 0, testduration.NotMeasured)
	if !slices.Equal(minitest.DurationInvalid, []string{"CalcTest#test_neg"}) {
		t.Fatalf("minitest durationInvalid %v", minitest.DurationInvalid)
	}
	rails := ptdParse(t, "rails-test", "minitest.json", ptdMinitestReport)
	ptdWant(t, ptdTest(t, rails, "CalcTest#test_add"), 7, testduration.SourceMinitest)
	unittest := ptdParse(t, "unittest", "unittest.json", bytesReplace(ptdMinitestReport, "corvint-minitest/0", "corvint-unittest/0"))
	if test := ptdTest(t, unittest, "CalcTest#test_add"); test.DurationMs != nil || test.DurationSource != "" {
		t.Fatalf("unittest kept a reporter duration: %+v", test)
	}
}

// PTD-V1-002 (Minitest receipt scenario, reporter half): the embedded reporter writes DurationMs
// from Minitest::Result#time on each row, and the Go reader takes a report in exactly the shape
// that reporter emits. Ruby's JSON.generate keeps the row hash's insertion order, so a skipped row
// is {ID, Name, State, Message, DurationMs} and a failed one adds FailureKind before Message.
func TestPTDV1002MinitestReporterOutput(t *testing.T) {
	reporter, err := reporters.ReadFile("reporters/minitest_reporter.rb")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"row = {ID: \"#{result.klass}##{result.name}\", Name: result.name, State: state}",
		"row[:DurationMs] = (result.time * 1000).round if result.time.is_a?(Numeric)",
		"f.write(JSON.generate(Profile: 'corvint-minitest/0', Complete: true,",
		"Count: @rows.length, Tests: @rows, Problems: []))",
	} {
		if !bytes.Contains(reporter, []byte(line)) {
			t.Fatalf("the embedded Minitest reporter no longer has %q", line)
		}
	}
	output := `{"Profile":"corvint-minitest/0","Complete":true,"Count":3,"Tests":[` +
		`{"ID":"CalcTest#test_add","Name":"test_add","State":"PASSED","DurationMs":7},` +
		`{"ID":"CalcTest#test_skip","Name":"test_skip","State":"SKIPPED","Message":"later","DurationMs":0},` +
		`{"ID":"CalcTest#test_boom","Name":"test_boom","State":"FAILED","FailureKind":"UNKNOWN","Message":"UnexpectedError: RuntimeError","DurationMs":1234}],"Problems":[]}`
	minitest, err := Parse(tr.Input{Runner: "minitest", ExitCode: 1, FailureExitCodes: []int{1}, Reports: map[string][]byte{"minitest.json": []byte(output)}})
	if err != nil {
		t.Fatal(err)
	}
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_add"), 7, testduration.SourceMinitest)
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_skip"), 0, testduration.SourceMinitest)
	ptdWant(t, ptdTest(t, minitest, "CalcTest#test_boom"), 1234, testduration.SourceMinitest)
	if len(minitest.DurationInvalid) != 0 || !minitest.Complete {
		t.Fatalf("minitest reporter output: invalid %v complete %v problems %v", minitest.DurationInvalid, minitest.Complete, minitest.Problems)
	}
}

func bytesReplace(s, old, new string) string {
	return string(bytes.Replace([]byte(s), []byte(old), []byte(new), 1))
}

// PTD-V1-007, PTD-V1-003: a time field that is not a number ("oops", an object, a value out of
// float64 range) no longer fails the whole Jest, Vitest, Playwright or RSpec report. Every outcome
// survives, the test is NOT_MEASURED and listed under durationInvalid, and the rest of the report
// still counts. A null time is a report without it: NOT_MEASURED and not invalid.
func TestPTDV1007NonNumericTimeKeepsTheReport(t *testing.T) {
	for _, value := range []string{`"oops"`, `{"ms":3}`, `1e400`, `true`} {
		for _, runner := range []string{"vitest", "jest"} {
			o := ptdParse(t, runner, runner+".json", string(bytes.Replace([]byte(ptdJestReport), []byte("DURATION"), []byte(value), 1)))
			if len(o.Tests) != 2 || !o.Complete {
				t.Fatalf("%s %s: tests %+v complete %v problems %v", runner, value, o.Tests, o.Complete, o.Problems)
			}
			adds := ptdTest(t, o, "/r/math.test.ts::math > adds")
			ptdWant(t, adds, 0, testduration.NotMeasured)
			if adds.State != tr.Passed || !slices.Equal(o.DurationInvalid, []string{adds.ID}) {
				t.Fatalf("%s %s: %+v durationInvalid %v", runner, value, adds, o.DurationInvalid)
			}
		}
		playwright := ptdParse(t, "playwright", "playwright.json", bytesReplace(ptdPlaywrightReport, `"duration":900`, `"duration":`+value))
		retried := ptdTest(t, playwright, "s1::chromium")
		ptdWant(t, retried, 0, testduration.NotMeasured)
		if retried.State != tr.Flaky || retried.AttemptDurationsMs != nil || !slices.Equal(playwright.DurationInvalid, []string{"s1::chromium"}) {
			t.Fatalf("playwright %s: %+v durationInvalid %v", value, retried, playwright.DurationInvalid)
		}
		ptdWant(t, ptdTest(t, playwright, "s2::chromium"), 40, testduration.SourcePlaywright)
		rspec := ptdParse(t, "rspec", "rspec.json", bytesReplace(ptdRSpecReport, `"run_time":0.012`, `"run_time":`+value))
		works := ptdTest(t, rspec, "./spec/a_spec.rb[1:1]")
		ptdWant(t, works, 0, testduration.NotMeasured)
		if works.State != tr.Passed || len(rspec.Tests) != 2 || !slices.Equal(rspec.DurationInvalid, []string{works.ID}) {
			t.Fatalf("rspec %s: %+v durationInvalid %v", value, works, rspec.DurationInvalid)
		}
	}
	null := ptdParse(t, "jest", "jest.json", string(bytes.Replace([]byte(ptdJestReport), []byte("DURATION"), []byte("null"), 1)))
	ptdWant(t, ptdTest(t, null, "/r/math.test.ts::math > adds"), 0, testduration.NotMeasured)
	if len(null.DurationInvalid) != 0 {
		t.Fatalf("a null time was listed invalid: %v", null.DurationInvalid)
	}
}
