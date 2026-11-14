// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// PTD-V1-002, PTD-V1-003, PTD-V1-007 (V1-5542): the Jest parser reads each
// assertion's duration in milliseconds; a null duration is not measured
// and the PTD-V1-007 scenario's -5 is invalid, each kept as a result. The
// report is hand-written in Jest's --json AggregatedResult shape.
func TestPTDV1007JestReportDurations(t *testing.T) {
	r, _ := For(testid.Jest)
	report := `{"success":true,"testResults":[{"name":"/repo/a.test.js","status":"passed","assertionResults":[` +
		`{"ancestorTitles":[],"title":"timed","status":"passed","duration":3},` +
		`{"ancestorTitles":[],"title":"untimed","status":"passed","duration":null},` +
		`{"ancestorTitles":[],"title":"negative","status":"passed","duration":-5}]}]}`
	results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.Jest, Root: "."}, Root: "/repo", Report: []byte(report)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "jest:a.test.js::timed", Outcome: OutcomePass, DurationMillis: 3, Granularity: testid.GranularityTest},
		{ID: "jest:a.test.js::untimed", Outcome: OutcomePass, Granularity: testid.GranularityTest, DurationMissing: true},
		{ID: "jest:a.test.js::negative", Outcome: OutcomePass, Granularity: testid.GranularityTest, DurationInvalid: true},
	}
	if !slices.Equal(results, want) {
		t.Fatalf("results\n got %+v\nwant %+v", results, want)
	}
}

// PTD-V1-002, PTD-V1-003 (V1-5542): the RSpec parser sums each example's
// run_time (seconds) into its file's result; a file with an example that
// has no run_time is not measured. The report is hand-written in the RSpec
// JSON formatter's shape.
func TestPTDV1002RSpecReportDurations(t *testing.T) {
	r, _ := For(testid.RSpec)
	report := `{"examples":[` +
		`{"id":"./spec/a_spec.rb[1:1]","file_path":"./spec/a_spec.rb","status":"passed","run_time":0.012},` +
		`{"id":"./spec/a_spec.rb[1:2]","file_path":"./spec/a_spec.rb","status":"passed","run_time":0.009},` +
		`{"id":"./spec/b_spec.rb[1:1]","file_path":"./spec/b_spec.rb","status":"passed","run_time":0.004},` +
		`{"id":"./spec/b_spec.rb[1:2]","file_path":"./spec/b_spec.rb","status":"pending"}],"summary":{"errors_outside_of_examples_count":0}}`
	results, err := r.Parse(ParseRequest{Instance: Instance{Runner: testid.RSpec, Root: "."}, Report: []byte(report)})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{
		{ID: "rspec:spec/a_spec.rb", Outcome: OutcomePass, DurationMillis: 21, Granularity: testid.GranularityFile},
		{ID: "rspec:spec/b_spec.rb", Outcome: OutcomePass, Granularity: testid.GranularityFile, DurationMissing: true},
	}
	if !slices.Equal(results, want) {
		t.Fatalf("results\n got %+v\nwant %+v", results, want)
	}
}
