package jstestprovider

import (
	"strings"
	"testing"
)

const cleanupReportSuites = `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[` +
	`{"title":"passes","ok":true,"file":"a.spec.ts","line":3,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":1}]}]},` +
	`{"title":"skips","ok":true,"file":"a.spec.ts","line":6,"tests":[{"status":"skipped","results":[{"status":"skipped","retry":0,"duration":0}]}]}` +
	`]}]`

// TestParsePlaywrightJSONPassedCleanupFailed: a report whose tests all pass or skip while it records
// a run-level error (a worker fixture teardown failure) is the passed-cleanup-failed infrastructure
// failure, not a clean pass (TSA-V0-168).
func TestParsePlaywrightJSONPassedCleanupFailed(t *testing.T) {
	report := cleanupReportSuites + `,"errors":[{"message":"Error: apiRequestContext.delete: Target page, context or browser has been closed\n    at cleanup (fixtures.ts:12)"}]}`
	outcomes, infra, err := ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 2 {
		t.Fatalf("outcomes %+v err %v", outcomes, err)
	}
	if infra == nil || infra.Reason != ReasonPassedCleanupFailed || !strings.Contains(infra.Detail, "has been closed") {
		t.Fatalf("infra %+v", infra)
	}
}

// TestParsePlaywrightJSONCleanupGateNegatives: a run without run-level errors stays clean, and a run
// with a failing test keeps its ordinary failed outcome, since it already fails.
func TestParsePlaywrightJSONCleanupGateNegatives(t *testing.T) {
	for name, report := range map[string]string{
		"no errors":     cleanupReportSuites + `,"errors":[]}`,
		"errors absent": cleanupReportSuites + `}`,
		"failing test":  `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"fails","ok":false,"file":"a.spec.ts","line":3,"tests":[{"status":"unexpected","results":[{"status":"failed","retry":0,"duration":1,"error":{"message":"boom"}}]}]}]}],"errors":[{"message":"teardown failed"}]}`,
	} {
		outcomes, infra, err := ParsePlaywrightJSON([]byte(report))
		if err != nil || infra != nil || len(outcomes) == 0 {
			t.Errorf("%s: outcomes %+v infra %+v err %v", name, outcomes, infra, err)
		}
	}
}
