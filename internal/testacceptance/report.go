package testacceptance

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"slices"
	"strings"
)

func classify(report *Report, r Request) {
	for i, t := range r.Tests {
		a := Assessment{ID: t.ID, Verdict: "blocked", Reasons: []string{}, Validity: testvalidity.Project(testvalidity.Input{})}
		reject := false
		passes, failures := 0, 0
		count := 0
		for _, run := range report.Runs {
			if cleanupState(run.Cleanup) == "survivors" {
				reject = true
				a.Reasons = append(a.Reasons, "observed-cleanup-survivor")
			} else if cleanupState(run.Cleanup) == "unknown" {
				a.Reasons = append(a.Reasons, "cleanup-unknown")
			}
			if run.Kind != "repeat" {
				continue
			}
			if len(run.Reasons) > 0 {
				a.Reasons = append(a.Reasons, run.Reasons...)
			}
			for _, row := range run.Rows {
				if row.ID != t.ID {
					continue
				}
				count++
				a.Validity = row.Validity
				if row.State == jstestprovider.StatePassed {
					passes++
				} else if row.State == jstestprovider.StateFailed {
					failures++
					reject = true
					a.Reasons = append(a.Reasons, "test-failed")
				} else if row.State == jstestprovider.StateFlaky || row.State == jstestprovider.StateSkipped {
					reject = true
					a.Reasons = append(a.Reasons, "flaky-or-skipped")
				} else {
					a.Reasons = append(a.Reasons, "test-incomplete")
				}
				if row.Retries != 0 || row.Attempts != 1 {
					reject = true
					a.Reasons = append(a.Reasons, "retry-or-attempt-count")
				}
				if len(row.IdentityUnknown) > 0 {
					a.Reasons = append(a.Reasons, "qualified-test-identity-unknown")
				}
			}
		}
		if passes > 0 && failures > 0 {
			reject = true
			a.Reasons = append(a.Reasons, "mixed-repeat-outcomes")
		}
		if count != r.Repeat {
			a.Reasons = append(a.Reasons, "repeat-evidence-incomplete")
		}
		c := report.Controls[i]
		if c.Status == "survived" {
			reject = true
			a.Reasons = append(a.Reasons, "negative-control-survived")
			a.Validity.Strength = testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: "verified-approved-control-survived"}
		} else if c.Status == "killed" {
			a.Reasons = append(a.Reasons, "control-killed-baseline-identity-unknown")
			a.Validity.Strength = testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: "qualified-baseline-identity-unknown"}
		} else {
			a.Reasons = append(a.Reasons, "negative-control-incomplete")
		}
		if cleanupState(c.Cleanup) == "survivors" {
			reject = true
			a.Reasons = append(a.Reasons, "control-cleanup-survivor")
		} else if cleanupState(c.Cleanup) == "unknown" {
			a.Reasons = append(a.Reasons, "control-cleanup-unknown")
		}
		// No accepted path is exposed until the native provider supplies qualified
		// positive per-test freshness. Requested Git pins cannot supply this axis.
		a.Reasons = append(a.Reasons, "provider-freshness-unknown")
		if reject {
			a.Verdict = "rejected"
		}
		slices.Sort(a.Reasons)
		a.Reasons = slices.Compact(a.Reasons)
		report.Assessments = append(report.Assessments, a)
	}
	report.Verdict = "blocked"
	report.Reasons = []string{}
	for _, a := range report.Assessments {
		if a.Verdict == "rejected" {
			report.Verdict = "rejected"
		}
		report.Reasons = append(report.Reasons, a.Reasons...)
	}
	slices.Sort(report.Reasons)
	report.Reasons = slices.Compact(report.Reasons)
	var body strings.Builder
	fmt.Fprintf(&body, "Experimental new E2E test assessment\n\nOverall: %s\nEnvironment: %s\nTests: %d\nRepeat count: %d\n", report.Verdict, r.Environment, len(r.Tests), r.Repeat)
	for _, a := range report.Assessments {
		fmt.Fprintf(&body, "- %s: %s\n", a.ID, a.Verdict)
	}
	body.WriteString("\nQualified per-test freshness: UNKNOWN. Accepted outcomes remain blocked (V1-0556).\nOrder probes retain requested filters; observed schedule may be UNKNOWN.\nLocal observation only; integration and native completion remain open.\n")
	report.Body = body.String()
}
