package testacceptance

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"regexp"
	"slices"
	"strings"
)

func classify(report *Report, r Request) {
	if r.Schema == FreshRequestSchema {
		classifyFreshness(report, r)
		return
	}
	for i, t := range r.Tests {
		a := Assessment{ID: t.ID, Verdict: "blocked", Reasons: []string{}, Validity: testvalidity.Project(testvalidity.Input{})}
		reject := false
		passes, failures := 0, 0
		count := 0
		cleanupStates := []string{}
		for _, run := range report.Runs {
			if run.Kind == "probe-isolated" && run.TestID != t.ID {
				continue
			}
			cleanupStates = append(cleanupStates, cleanupState(run.Cleanup))
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
				observeDuration(&a.Repeats, row.DurationMS)
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
		a.Repeats.Passed, a.Repeats.Failed, a.Repeats.Other = passes, failures, count-passes-failures
		a.Order = orderEvidence(report.Runs, r, t.ID)
		if passes > 0 && failures > 0 {
			reject = true
			a.Reasons = append(a.Reasons, "mixed-repeat-outcomes")
		}
		if count != r.Repeat {
			a.Reasons = append(a.Reasons, "repeat-evidence-incomplete")
		}
		c := report.Controls[i]
		cleanupStates = append(cleanupStates, cleanupState(c.Cleanup))
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
		a.Cleanup = worstCleanup(cleanupStates)
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
	report.Body = renderBody(*report, r)
}

func observeDuration(s *RepeatSummary, ms float64) {
	if s.MinDurationMS == nil || ms < *s.MinDurationMS {
		v := ms
		s.MinDurationMS = &v
	}
	if s.MaxDurationMS == nil || ms > *s.MaxDurationMS {
		v := ms
		s.MaxDurationMS = &v
	}
}

// worstCleanup orders cleanup facts by severity: a known survivor outranks an
// unknown observation, which outranks an observed-absent result.
func worstCleanup(states []string) string {
	worst := "unknown"
	if len(states) > 0 {
		worst = "observed-absent"
	}
	for _, s := range states {
		if s == "survivors" {
			return "survivors"
		}
		if s == "unknown" {
			worst = "unknown"
		}
	}
	return worst
}

// probeState is the row state from one probe run, or an explicit gap when the
// run carries reasons or did not report this test.
func probeState(run Run, id string) string {
	for _, row := range run.Rows {
		if row.ID == id {
			if len(run.Reasons) > 0 {
				return "incomplete"
			}
			return string(row.State)
		}
	}
	return "missing"
}

// orderEvidence summarizes the probes for one test. Only passed/failed probe
// states are evidence; anything else leaves the classification incomplete.
func orderEvidence(runs []Run, r Request, id string) OrderEvidence {
	o := OrderEvidence{Status: "not-probed", RequestedFileOrder: "not-probed", OriginalState: "not-run", ReversedState: "not-run", IsolatedStates: []string{}}
	if !mixedOutcomes(runs, id) {
		return o
	}
	passed, failed := string(jstestprovider.StatePassed), string(jstestprovider.StateFailed)
	for _, run := range runs {
		switch {
		case run.Kind == "probe-original":
			o.OriginalState = probeState(run, id)
		case run.Kind == "probe-reversed":
			o.ReversedState = probeState(run, id)
		case run.Kind == "probe-isolated" && run.TestID == id:
			o.IsolatedStates = append(o.IsolatedStates, probeState(run, id))
		}
	}
	decided := func(s string) bool { return s == passed || s == failed }
	switch {
	case len(files(r)) < 2:
		o.RequestedFileOrder = "not-varied"
	case !decided(o.OriginalState) || !decided(o.ReversedState):
		o.RequestedFileOrder = "incomplete"
	case o.OriginalState == o.ReversedState:
		o.RequestedFileOrder = "same-outcome"
	default:
		o.RequestedFileOrder = "outcome-differs"
	}
	isolatedPassed, isolatedFailed := 0, 0
	for _, s := range o.IsolatedStates {
		if s == passed {
			isolatedPassed++
		} else if s == failed {
			isolatedFailed++
		}
	}
	switch {
	case len(o.IsolatedStates) != IsolationRepeats || isolatedPassed+isolatedFailed != IsolationRepeats:
		o.Status = "isolation-incomplete"
	case isolatedFailed == 0:
		o.Status = "failures-not-reproduced-in-isolation"
	case isolatedPassed == 0:
		o.Status = "fails-in-isolation"
	default:
		o.Status = "nondeterministic-in-isolation"
	}
	return o
}

var codePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var buildPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._:/=+-]{0,199}$`)

// safe admits only the closed identifier/enum shapes into the PR body.
func safe(p *regexp.Regexp, s string) string {
	if p.MatchString(s) {
		return s
	}
	return "UNVALIDATED"
}

func milliseconds(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *v)
}

// renderBody produces a fixed Markdown body from validated identifiers, enums,
// counts and digests. No source text, author prose or caller template enters it.
func renderBody(report Report, r Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Experimental new E2E test assessment\n\n**Overall verdict: %s**\n\n", safe(codePattern, report.Verdict))
	fmt.Fprintf(&b, "| Binding | Value |\n|---|---|\n")
	fmt.Fprintf(&b, "| Product revision | `%s` (tree `%s`) |\n", safe(oidPattern, report.Product.Commit), safe(oidPattern, report.Product.Tree))
	fmt.Fprintf(&b, "| Test-repository revision | `%s` (tree `%s`) |\n", safe(oidPattern, report.TestRepository.Commit), safe(oidPattern, report.TestRepository.Tree))
	fmt.Fprintf(&b, "| Environment | `%s` |\n", safe(idPattern, report.Environment))
	fmt.Fprintf(&b, "| Corvint build | `%s` |\n", safe(buildPattern, report.Build))
	fmt.Fprintf(&b, "| Companion executable | `%s` |\n", safe(hashPattern, report.ExecutableSHA256))
	fmt.Fprintf(&b, "| Request digest | `%s` |\n", safe(hashPattern, report.RequestDigest))
	fmt.Fprintf(&b, "| Repeats | %d per test, retries 0, workers 1 |\n\n", r.Repeat)
	fmt.Fprintf(&b, "| Test | Verdict | Passed/failed/other | Duration ms (min-max) | Strength | Cleanup | Order probe | File order | Isolated runs |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, a := range report.Assessments {
		isolated := "-"
		if len(a.Order.IsolatedStates) > 0 {
			states := make([]string, len(a.Order.IsolatedStates))
			for i, s := range a.Order.IsolatedStates {
				states[i] = safe(codePattern, s)
			}
			isolated = strings.Join(states, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | %s | %d/%d/%d | %s-%s | %s | %s | %s | %s | %s |\n", safe(idPattern, a.ID), safe(codePattern, a.Verdict), a.Repeats.Passed, a.Repeats.Failed, a.Repeats.Other, milliseconds(a.Repeats.MinDurationMS), milliseconds(a.Repeats.MaxDurationMS), safe(idPattern, string(a.Validity.Strength.State)), safe(codePattern, a.Cleanup), safe(codePattern, a.Order.Status), safe(codePattern, a.Order.RequestedFileOrder), isolated)
	}
	b.WriteString("\nReasons:\n\n")
	for _, a := range report.Assessments {
		reasons := make([]string, len(a.Reasons))
		for i, reason := range a.Reasons {
			reasons[i] = safe(codePattern, reason)
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", safe(idPattern, a.ID), strings.Join(reasons, ", "))
	}
	unknowns := make([]string, len(report.Unknowns))
	for i, u := range report.Unknowns {
		unknowns[i] = safe(codePattern, u)
	}
	fmt.Fprintf(&b, "\nUnknowns: %s\n", strings.Join(unknowns, ", "))
	if r.Schema == FreshRequestSchema {
		b.WriteString("\nPer-test freshness and control strength are rederived from every retained native baseline and control receipt.\n")
	} else {
		b.WriteString("\nQualified per-test freshness: UNKNOWN. Accepted outcomes remain blocked (V1-0556).\nOrder probes and isolation runs retain requested filters; observed schedule may be UNKNOWN.\nLocal observation only; integration and native completion remain open.\n")
	}

	return b.String()
}
