package dynamic

import (
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"testing"
)

func TestIndependentReviewRegressions(t *testing.T) {
	cases := []struct{ Name, Runner, Body string }{
		{"duplicate_json_failure_overwritten", "jest", `{"numTotalTests":1,"success":true,"testResults":[{"name":"a","status":"passed","assertionResults":[{"title":"t","fullName":"t","status":"failed","status":"passed"}]}]}`},
		{"unknown_xml_outcome_becomes_pass", "pytest", `<testsuites><testsuite name="pytest" tests="1" failures="0" errors="0" skipped="0"><testcase classname="C" name="t"><fatalError message="crashed"/></testcase></testsuite></testsuites>`},
		{"trailing_xml_failure_ignored", "pytest", `<testsuite name="pytest" tests="1" failures="0" errors="0" skipped="0"><testcase classname="C" name="t"/></testsuite><testsuite tests="1" errors="1"><testcase name="failed"><error/></testcase></testsuite>`},
		{"playwright_failed_attempt_becomes_pass", "playwright", `{"suites":[{"title":"s","specs":[{"id":"id","title":"t","tests":[{"projectId":"p","status":"expected","expectedStatus":"passed","results":[{"status":"failed","retry":0,"error":{"message":"assertion failed"}}]}]}]}],"stats":{"expected":1,"unexpected":0,"flaky":0,"skipped":0}}`},
		{"rspec_failure_counter_ignored", "rspec", `{"version":"3.13.6","examples":[{"id":"t","description":"t","status":"passed"}],"summary":{"example_count":1,"failure_count":1,"errors_outside_of_examples_count":0}}`},
		{"node_suite_failure_and_summary_counts_ignored", "node-test", "{\"type\":\"test:pass\",\"data\":{\"name\":\"t\",\"file\":\"a.js\",\"line\":1,\"column\":1,\"testNumber\":1,\"details\":{\"type\":\"test\"}}}\n{\"type\":\"test:fail\",\"data\":{\"name\":\"suite\",\"details\":{\"type\":\"suite\",\"error\":{\"failureType\":\"hookFailed\",\"message\":\"suite teardown failed\"}}}}\n{\"type\":\"test:summary\",\"data\":{\"success\":true,\"counts\":{\"tests\":1,\"failed\":1,\"passed\":0}}}\n"},
		{"wdio_pass_with_error", "webdriverio", `{"end":"2026-10-01","framework":"mocha","state":{"passed":1,"failed":0,"skipped":0},"suites":[{"name":"s","sessionId":"session","tests":[{"name":"t","state":"passed","error":{"message":"socket lost"}}],"hooks":[]}]}`},
		{"nightwatch_last_error_ignored", "nightwatch", `{"name":"s","report":{"testsCount":1,"errorsCount":0,"completed":{"t":{"status":"pass"}},"lastError":{"message":"session died"}}}`},
		{"owned_report_final_pass_failed_attempt", "unittest", `{"Profile":"corvint-unittest/0","Complete":true,"Count":1,"Tests":[{"ID":"t","Name":"t","State":"PASSED","Attempts":[{"State":"FAILED","FailureKind":"ASSERTION"}]}],"Problems":[]}`},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			o, e := Parse(tr.Input{Runner: c.Runner, Reports: map[string][]byte{"r": []byte(c.Body)}})
			if e == nil && o.Complete {
				t.Fatalf("contradictory report accepted: %+v", o)
			}
		})
	}
}

func TestClosedReportAmbiguity(t *testing.T) {
	for _, body := range []string{`{"x":1,"\u0078":2}`, `{"status":"failed","STATUS":"passed"}`, `{"nested":{"x":1,"x":2}}`} {
		if uniqueJSON([]byte(body)) == nil {
			t.Fatalf("ambiguous keys accepted: %s", body)
		}
	}
	for _, body := range []string{`<testsuite tests="1"><testcase name="t" status="failed"/></testsuite>`, `<testsuite tests="1"><testcase name="t"><x:failure xmlns:x="urn:x"/></testcase></testsuite>`, `<testsuite tests="0" tests="1"/>`} {
		if closedXML([]byte(body)) == nil {
			t.Fatalf("ambiguous XML accepted: %s", body)
		}
	}
}
func TestLiteralSelectorAndNativeCLIArguments(t *testing.T) {
	req := tr.Request{Runner: "playwright", Root: "/repo", Executable: "/bin/runner", ReportDir: "/report", Selectors: []string{"tests/a.spec.ts|."}}
	inv, e := Build(req)
	if e != nil {
		t.Fatal(e)
	}
	if inv.Argv[len(inv.Argv)-1] != `^/repo/tests/a\.spec\.ts\|\.$` {
		t.Fatal(inv.Argv)
	}
	req.Runner = "vitest"
	if _, e = Build(req); e == nil {
		t.Fatal("ambiguous Vitest filter accepted")
	}
	req.Runner = "bun-test"
	req.Selectors = []string{"a.test.ts"}
	inv, e = Build(req)
	if e != nil || inv.Argv[len(inv.Argv)-1] != "./a.test.ts" || req.Selectors[0] != "a.test.ts" {
		t.Fatalf("%v %v", inv, e)
	}
	req.Runner = "storybook-test-runner"
	req.Selectors = nil
	inv, e = Build(req)
	if e != nil || len(inv.Argv) != 3 || inv.Argv[1] != "--outputFile" {
		t.Fatalf("%v %v", inv, e)
	}
	req.ReportDir = "/report space"
	if _, e = Build(req); e == nil {
		t.Fatal("legacy shell forwarded path accepted")
	}
	req.Runner = "cypress"
	req.ReportDir = "/report,option"
	if _, e = Build(req); e == nil {
		t.Fatal("ambiguous reporter options accepted")
	}
}

func TestCypressCommandPrecedesNativeOptions(t *testing.T) {
	v, e := Build(tr.Request{Runner: "cypress", Executable: "/bin/cypress", ReportDir: "/report"})
	if e != nil || len(v.Argv) < 2 || v.Argv[0] != "run" || v.Argv[1] != "--posix-exit-codes" {
		t.Fatalf("Cypress subcommand must precede options: %v %v", v.Argv, e)
	}
}
