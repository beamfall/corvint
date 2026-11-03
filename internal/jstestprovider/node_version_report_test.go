package jstestprovider

import (
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// PWP-V0-961: the report-level process.version fills an unobserved receipt Node version only
// when every test that is not browser "none" names the same version; a disagreement or a missing
// per-test version stays unobserved, and without a report-level version the rule is unchanged.
func TestPWPV0961ReportedNodeVersionFallback(t *testing.T) {
	const browserless = `{"corvintBrowserless":"no-browser-use"}`
	withNode := func(v string) TestOutcome {
		return browserlessTest("page "+v, "chromium", `{"corvintBrowser":{"nodeVersion":"`+v+`"}}`, StatePassed)
	}
	none := browserlessTest("api", BrowserlessProject, browserless, StatePassed)
	missing := browserlessTest("page missing", "chromium", `{"corvintBrowser":{}}`, StatePassed)
	unknown := browserlessTest("page unknown", "", `null`, StatePassed)
	forged := browserlessTest("forged", BrowserlessProject, `{"corvintBrowserless":"no-browser-use","corvintBrowser":{"nodeVersion":"v24.11.1"}}`, StatePassed)
	attemptless := none
	attemptless.Attempts = nil
	reported := func(v string) *string { return &v }
	cases := []struct {
		name     string
		tests    []TestOutcome
		reported *string
		want     string
	}{
		{"all browserless, reported", []TestOutcome{none, none}, reported("v22.23.2"), "v22.23.2"},
		{"browserless and agreeing browser test", []TestOutcome{none, withNode("v22.23.2")}, reported("v22.23.2"), "v22.23.2"},
		{"no tests, reported", nil, reported("v22.23.2"), "v22.23.2"},
		{"browser tests agree with report", []TestOutcome{withNode("v24.11.1"), withNode("v24.11.1")}, reported("v24.11.1"), "v24.11.1"},
		{"browser test disagrees with report", []TestOutcome{none, withNode("v24.11.1")}, reported("v22.23.2"), ""},
		{"agreeing browser tests disagree with report", []TestOutcome{withNode("v24.11.1"), withNode("v24.11.1")}, reported("v22.23.2"), ""},
		{"browser tests disagree with each other", []TestOutcome{withNode("v22.23.2"), withNode("v24.11.1")}, reported("v22.23.2"), ""},
		{"browser test without a version", []TestOutcome{none, missing}, reported("v22.23.2"), ""},
		{"test without a recorded identity", []TestOutcome{none, unknown}, reported("v22.23.2"), ""},
		{"test without a project", []TestOutcome{none, {Name: "bare"}}, reported("v22.23.2"), ""},
		{"all browserless, empty report", []TestOutcome{none}, reported(""), ""},
		{"all browserless, no report member", []TestOutcome{none}, nil, ""},
		{"earlier rule without a report member", []TestOutcome{withNode("v22.23.2"), withNode("v22.23.2")}, nil, "v22.23.2"},
		{"no source", nil, nil, ""},
		// A browser "none" record outside the closed browserless shape is not skipped, so its own
		// conflicting or missing version keeps the version unobserved.
		{"forged browserless with a conflicting version", []TestOutcome{none, forged}, reported("v22.23.2"), ""},
		{"browserless with no attempts", []TestOutcome{none, attemptless}, reported("v22.23.2"), ""},
	}
	for _, c := range cases {
		if got := recordedNodeVersion("1.64.0", c.tests, c.reported); got != c.want {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
}

// PWP-V0-961: the report decoder admits the report-level nodeVersion member in every external
// profile and keeps it absent for a report an earlier reporter wrote.
func TestPWPV0961ReportDecodesNodeVersion(t *testing.T) {
	base := `{"version":"1.64.0","files":{},"configFiles":{},"status":"passed","tests":[],"errors":[]`
	for _, profile := range []string{ExternalProfile, AttestedExternalProfile, SensitiveExternalProfile, AttemptExternalProfile} {
		report, failure := decodeQualifiedReport([]byte(base+`,"nodeVersion":"v22.23.2"}`), profile)
		if failure != nil || report.NodeVersion == nil || *report.NodeVersion != "v22.23.2" {
			t.Fatalf("%s: report=%+v failure=%+v", profile, report.NodeVersion, failure)
		}
		earlier, failure := decodeQualifiedReport([]byte(base+`}`), profile)
		if failure != nil || earlier.NodeVersion != nil {
			t.Fatalf("%s: earlier report=%+v failure=%+v", profile, earlier.NodeVersion, failure)
		}
		if _, failure := decodeQualifiedReport([]byte(base+`,"nodeVersion":22}`), profile); failure == nil || failure.Reason != "report-unparseable" {
			t.Fatalf("%s: non-string nodeVersion failure=%+v", profile, failure)
		}
	}
}

// PWP-V0-961: a bare-npx run (no observed executable) whose tests are all browserless had an
// unobserved runtime tuple, so its no-browser-use test could not qualify (PWP-V3-015). With the
// report-level version the tuple is candidate on an admitted Node and stays unqualified on an
// unadmitted one: the tuple table decides, unchanged.
func TestPWPV0961BrowserlessNpxRunTupleCandidate(t *testing.T) {
	const noBrowserUse = `{"corvintBrowserless":"no-browser-use"}`
	run := func(reported *string) Receipt {
		r, report := browserlessReport(t, browserlessTest("api", BrowserlessProject, noBrowserUse, StatePassed),
			browserlessTest("skipped", BrowserlessProject, `{"corvintBrowserless":"not-run"}`, StateSkipped))
		r.Identity.NodeVersion = "" // bare npx: neither argv[0] nor npm_node_execpath named node
		report.NodeVersion = reported
		fillRecordedNodeVersion(&r, report)
		if err := bindQualifiedReport(&r, report); err != nil {
			t.Fatalf("browserless run refused: %v", err)
		}
		return r
	}
	before := run(nil)
	if tuple := ReceiptRuntimeTuple(before); tuple != RuntimeTupleUnobserved {
		t.Fatalf("without a report-level version tuple = %s, want %s", tuple, RuntimeTupleUnobserved)
	}
	if !qualifiedPrerequisitesUnknown(before, before.Tests[0]) {
		t.Fatal("no-browser-use test qualified on an unobserved Node")
	}
	admitted := "v22.23.2"
	after := run(&admitted)
	if after.Identity.NodeVersion != admitted || ReceiptRuntimeTuple(after) != RuntimeTupleCandidate {
		t.Fatalf("node=%q tuple=%s, want %s candidate", after.Identity.NodeVersion, ReceiptRuntimeTuple(after), admitted)
	}
	if qualifiedPrerequisitesUnknown(after, after.Tests[0]) {
		t.Fatal("no-browser-use test did not qualify on the candidate tuple")
	}
	if state := ReceiptTestProjection(after, after.Tests[0]).Execution.State; state != testvalidity.ExecutionPassed {
		t.Fatalf("no-browser-use test projected %s", state)
	}
	if !qualifiedPrerequisitesUnknown(after, after.Tests[1]) {
		t.Fatal("not-run test made an execution claim")
	}
	unadmitted := "v22.23.3"
	if r := run(&unadmitted); ReceiptRuntimeTuple(r) != RuntimeTupleUnqualified || !qualifiedPrerequisitesUnknown(r, r.Tests[0]) {
		t.Fatalf("unadmitted node tuple = %s", ReceiptRuntimeTuple(r))
	}
	// A runner outside 1.63.0 and 1.64.0 records no browserless test (PWP-V3-014), so none is skipped.
	if got := recordedNodeVersion("1.60.0", after.Tests, &admitted); got != "" {
		t.Fatalf("1.60.0 browserless run recorded %q", got)
	}
	// A version an executable source observed is never replaced by the reported one.
	observed, report := browserlessReport(t, browserlessTest("api", BrowserlessProject, noBrowserUse, StatePassed))
	observed.Identity.NodeVersion = "v24.11.1"
	report.NodeVersion = &admitted
	if fillRecordedNodeVersion(&observed, report); observed.Identity.NodeVersion != "v24.11.1" {
		t.Fatalf("observed version replaced by %q", observed.Identity.NodeVersion)
	}
	// The receipt round-trips the recorded version.
	data, err := json.Marshal(after.Identity)
	if err != nil {
		t.Fatal(err)
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil || identity.NodeVersion != admitted {
		t.Fatalf("identity round trip %+v %v", identity, err)
	}
}
