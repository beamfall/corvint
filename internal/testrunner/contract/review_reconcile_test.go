// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/testid"
)

// RNC-V1-006 (review P1-2, P2-1, P2-2, P2-3): the reconciled run records
// failure, incompleteness and duplicate results. A package that panicked
// fails the run and leaves it incomplete; a duplicated test makes the run
// not exact and names the test; a partial report keeps its results through
// ReconcileParsed, incomplete for unreadable lines and failed for a
// run-level failure; any other parse error leaves every selection NOT_RUN.
func TestRNCV1006RunRecordsFailureIncompletenessAndDuplicates(t *testing.T) {
	results, err := goParse(t,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestA"}`,
		`{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elapsed":0}`,
		`{"Action":"run","Package":"example.test/m/a","Test":"TestX"}`,
		`{"Action":"output","Package":"example.test/m/a","Test":"TestX","Output":"panic: boom\n"}`,
		`{"Action":"fail","Package":"example.test/m/a","Test":"TestX","Elapsed":0}`,
		`{"Action":"fail","Package":"example.test/m/a","Elapsed":0.005}`)
	if err != nil {
		t.Fatal(err)
	}
	if run := Reconcile(nil, results); !run.Failed || !run.Incomplete || !run.Exact {
		t.Fatalf("panicked package: %+v", run)
	}
	pass := Result{ID: "go:a::TestA", Outcome: OutcomePass, Granularity: testid.GranularityTest}
	if run := Reconcile(nil, []Result{pass}); run.Failed || run.Incomplete || !run.Exact || run.Duplicates != nil {
		t.Fatalf("clean run: %+v", run)
	}
	fail := pass
	fail.Outcome = OutcomeFail
	if run := Reconcile(nil, []Result{fail, pass}); run.Exact || !slices.Equal(run.Duplicates, []string{"go:a::TestA"}) || !run.Failed || run.Results[0].Outcome != OutcomeFail {
		t.Fatalf("fail then pass: %+v", run)
	}
	if run := ReconcileParsed(nil, []Result{pass}, &ReportError{Malformed: 1}); !run.Incomplete || run.Failed || len(run.Results) != 1 {
		t.Fatalf("malformed line: %+v", run)
	}
	if run := ReconcileParsed(nil, []Result{pass}, &ReportError{Failures: []string{"setup"}}); run.Incomplete || !run.Failed {
		t.Fatalf("run-level failure: %+v", run)
	}
	if run := ReconcileParsed([]string{"go:a::TestA"}, []Result{pass}, ErrNotImplemented); !run.Incomplete || !slices.Equal(run.NotRun, []string{"go:a::TestA"}) {
		t.Fatalf("unusable report: %+v", run)
	}
	_, err = goParse(t, `{"Action":"pass","Package":"example.test/m/a","Test":"TestA","Elap`)
	if run := ReconcileParsed(nil, nil, err); !run.Incomplete {
		t.Fatalf("truncated report: %+v", run)
	}
}
