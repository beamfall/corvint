// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// RNC-V1-003, RNC-V1-014: Jest runs the selected files by literal path,
// exact for files and whole-file for a test-level ID, from the instance
// root; the full command adds only the report flags.
func TestRNCV1003JestSelection(t *testing.T) {
	instance := Instance{Runner: testid.Jest, Root: "web", Config: "web/jest.config.js"}
	base := Request{Instance: instance, Full: []string{"npx", "jest"}, Report: "/r.json", Tier: TierFile}
	with := func(tests ...string) Request {
		request := base
		request.Tests = tests
		return request
	}
	got := command(t, with("jest:web/src/b.test.js", "jest:web/src/a.test.js", "jest:web/-dash.test.js"))
	if want := []string{"npx", "jest", "--json", "--outputFile=/r.json", "--runTestsByPath", "./-dash.test.js", "src/a.test.js", "src/b.test.js"}; !slices.Equal(got.Argv, want) || !got.Exact || got.Dir != "web" {
		t.Fatalf("files: %+v, want %q", got, want)
	}
	if got := command(t, with("jest:web/src/a.test.js::math::adds")); got.Exact || !strings.Contains(got.Reason, "whole file") {
		t.Fatalf("test-level: %+v", got)
	}
	if got := command(t, base); !slices.Equal(got.Argv, []string{"npx", "jest", "--json", "--outputFile=/r.json"}) || got.Tier != TierFull || !got.Exact {
		t.Fatalf("full: %+v", got)
	}
}

// RNC-V1-005, RNC-V1-010 (report half): the Jest --json report gives one
// outcome and duration per test title under its canonical ID.
func TestRNCV1005JestReport(t *testing.T) {
	r, _ := For(testid.Jest)
	got, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "."}, Root: "/repo", Report: []byte(`{"success":false,"testResults":[{"name":"/repo/src/a.test.js","status":"failed","assertionResults":[
		{"ancestorTitles":["math"],"title":"adds","status":"passed","duration":7},
		{"ancestorTitles":["math"],"title":"divides","status":"failed","duration":2}]}]}`)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "jest:src/a.test.js::math::adds", Outcome: OutcomePass, DurationMillis: 7, Granularity: testid.GranularityTest},
		{ID: "jest:src/a.test.js::math::divides", Outcome: OutcomeFail, DurationMillis: 2, Granularity: testid.GranularityTest},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
}

// RNC-V1-003: pytest names each selection as an exact node ID relative to
// the instance root (the rootdir), parameter parts verbatim.
func TestRNCV1003PytestSelection(t *testing.T) {
	instance := Instance{Runner: testid.Pytest, Root: "svc", Config: "svc/pytest.ini"}
	got := command(t, Request{Instance: instance, Full: []string{"python", "-m", "pytest"}, Report: "/r.json", Tests: []string{
		"pytest:svc/tests/test_a.py::TestA::test_x[1-2]", "pytest:svc/tests/test_b.py", "pytest:svc/-x_test.py::test_y",
	}})
	want := []string{"python", "-m", "pytest", "--json-report", "--json-report-file=/r.json", "./-x_test.py::test_y", "tests/test_a.py::TestA::test_x[1-2]", "tests/test_b.py"}
	if !slices.Equal(got.Argv, want) || !got.Exact || got.Dir != "svc" || got.Tier != TierFile {
		t.Fatalf("pytest: %+v, want %q", got, want)
	}
}

// RNC-V1-005: the pytest-json-report report gives each test's outcome and
// summed phase durations; a collector that failed is a file-level fail, and
// an internal error with nothing failing is a run-level failure.
func TestRNCV1005PytestReport(t *testing.T) {
	r, _ := For(testid.Pytest)
	instance := Instance{Runner: testid.Pytest, Root: "svc"}
	got, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(`{"exitcode":1,"collectors":[{"nodeid":"","outcome":"passed"},{"nodeid":"tests/test_broken.py","outcome":"failed"}],"tests":[
		{"nodeid":"tests/test_a.py::TestA::test_x[1-2]","outcome":"passed","setup":{"duration":0.001},"call":{"duration":0.0105},"teardown":{"duration":0.0005}},
		{"nodeid":"tests/test_a.py::test_y","outcome":"failed","call":{"duration":0.002}},
		{"nodeid":"tests/test_a.py::test_z","outcome":"skipped"},
		{"nodeid":"tests/test_a.py::test_e","outcome":"error","setup":{"duration":0.003}}]}`)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "pytest:svc/tests/test_a.py::TestA::test_x[1-2]", Outcome: OutcomePass, DurationMillis: 12, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_y", Outcome: OutcomeFail, DurationMillis: 2, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_z", Outcome: OutcomeSkip, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_a.py::test_e", Outcome: OutcomeError, DurationMillis: 3, Granularity: testid.GranularityTest},
		{ID: "pytest:svc/tests/test_broken.py", Outcome: OutcomeFail, Granularity: testid.GranularityFile},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("results\n got %+v\nwant %+v", got, want)
	}
	var partial *ReportError
	if _, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(`{"exitcode":3,"tests":[]}`)}); !errors.As(err, &partial) || len(partial.Failures) != 1 {
		t.Fatalf("internal error: %v", err)
	}
}

// RNC-V1-003, RNC-V1-005: RSpec runs the selected spec files and reads one
// file-level result per file, worst outcome and summed run time; an error
// outside examples is a run-level failure.
func TestRNCV1005RSpecSelectionAndReport(t *testing.T) {
	instance := Instance{Runner: testid.RSpec, Root: ".", Config: ".rspec"}
	got := command(t, Request{Instance: instance, Full: []string{"bundle", "exec", "rspec"}, Report: "/r.json", Tests: []string{"rspec:spec/a_spec.rb", "rspec:spec/b_spec.rb::B::works"}})
	if want := []string{"bundle", "exec", "rspec", "--format", "progress", "--format", "json", "--out", "/r.json", "spec/a_spec.rb", "spec/b_spec.rb"}; !slices.Equal(got.Argv, want) || got.Exact || !strings.Contains(got.Reason, "whole file") {
		t.Fatalf("rspec: %+v, want %q", got, want)
	}
	r, _ := For(testid.RSpec)
	results, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(`{"examples":[
		{"id":"./spec/a_spec.rb[1:1]","file_path":"./spec/a_spec.rb","status":"passed","run_time":0.004},
		{"id":"./spec/a_spec.rb[1:2]","file_path":"./spec/a_spec.rb","status":"failed","run_time":0.002},
		{"id":"./spec/shared_spec.rb[1:1]","file_path":"./spec/support/shared.rb","status":"pending","run_time":0}],
		"summary":{"errors_outside_of_examples_count":0}}`)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "rspec:spec/a_spec.rb", Outcome: OutcomeFail, DurationMillis: 6, Granularity: testid.GranularityFile},
		{ID: "rspec:spec/shared_spec.rb", Outcome: OutcomeSkip, Granularity: testid.GranularityFile},
	}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results\n got %+v\nwant %+v", results, want)
	}
	if _, err := r.Parse(ParseRequest{Instance: instance, Report: []byte(`{"examples":[],"summary":{"errors_outside_of_examples_count":1}}`)}); err == nil {
		t.Fatal("a spec file that did not load read as a clean run")
	}
}

// RNC-V1-007: Playwright reports kind e2e from the runner; the others know
// no kind.
func TestRNCV1007RunnerKinds(t *testing.T) {
	for _, runner := range testid.Runners {
		want := ""
		if runner == testid.Playwright {
			want = KindE2E
		}
		if got := Kind(runner); got != want {
			t.Errorf("%s kind %q, want %q", runner, got, want)
		}
	}
}
