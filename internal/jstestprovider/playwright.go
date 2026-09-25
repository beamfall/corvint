package jstestprovider

import (
	"encoding/json"
	"strings"

	"github.com/Beamfall/corvint/internal/runhygiene"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tcq"
)

// Playwright's JSON reporter schema, confirmed empirically against
// @playwright/test@1.63.0 (see evidence/ipr-08-runner-selection.md section 4
// open question, resolved by testdata/playwright-mixed.json et al.):
// suites nest (suite.suites[]), each suite carries specs[], each spec
// carries tests[] (one per project), each test carries results[] (one per
// attempt/retry). test.status is Playwright's own aggregate classification:
// "expected", "unexpected", "flaky", or "skipped" - flaky is never a raw
// result.status, only this aggregate (memo section 4).
type playwrightReport struct {
	Suites []playwrightSuite `json:"suites"`
	Errors []playwrightError `json:"errors"`
}

type playwrightSuite struct {
	Title  string            `json:"title"`
	File   string            `json:"file"`
	Specs  []playwrightSpec  `json:"specs"`
	Suites []playwrightSuite `json:"suites"`
}

type playwrightSpec struct {
	Title string           `json:"title"`
	File  string           `json:"file"`
	Line  int              `json:"line"`
	Tests []playwrightTest `json:"tests"`
}

type playwrightTest struct {
	Status  string             `json:"status"` // expected | unexpected | flaky | skipped
	Results []playwrightResult `json:"results"`
}

type playwrightResult struct {
	Status      string                 `json:"status"` // passed | failed | timedOut | skipped | interrupted
	Retry       int                    `json:"retry"`
	Duration    float64                `json:"duration"`
	Error       *playwrightError       `json:"error"`
	Errors      []playwrightError      `json:"errors"`
	Attachments []playwrightAttachment `json:"attachments"`
}

type playwrightError struct {
	Message  string              `json:"message"`
	Location *playwrightLocation `json:"location"`
}

type playwrightLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type playwrightAttachment struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// infrastructureMessagePatterns are Playwright error-message substrings that
// indicate the environment failed to run the test at all - a missing
// browser binary - rather than the test itself failing. Distinguishing
// these from a real workflow assertion failure is the "infrastructure vs
// test failure" requirement: without this, a missing-browser install would
// read as an ordinary failing test.
var infrastructureMessagePatterns = []string{
	"browserType.launch: Executable doesn't exist",
	"Looks like Playwright Test or Playwright was just installed",
}

// ParsePlaywrightJSON parses one Playwright `--reporter=json` output into
// TestOutcomes. A run that produced no suites at all (e.g. "No tests
// found", an unrecognized --project) is reported as a run-level
// InfrastructureFailure via the returned bool/failure, never as zero
// silently-passing tests.
func ParsePlaywrightJSON(data []byte) ([]TestOutcome, *InfrastructureFailure, error) {
	var report playwrightReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, nil, err
	}
	if len(report.Suites) == 0 {
		detail := "playwright reported no suites"
		if len(report.Errors) > 0 {
			detail = report.Errors[0].Message
		}
		return nil, &InfrastructureFailure{Reason: "no-suites-collected", Detail: detail}, nil
	}
	var out []TestOutcome
	for _, s := range report.Suites {
		walkPlaywrightSuite(s, &out)
	}
	return out, nil, nil
}

func walkPlaywrightSuite(s playwrightSuite, out *[]TestOutcome) {
	for _, spec := range s.Specs {
		for _, t := range spec.Tests {
			*out = append(*out, playwrightOutcome(spec, t))
		}
	}
	for _, sub := range s.Suites {
		walkPlaywrightSuite(sub, out)
	}
}

func playwrightOutcome(spec playwrightSpec, t playwrightTest) TestOutcome {
	outcome := TestOutcome{Name: spec.Title, FullName: spec.Title, Anchor: &Anchor{File: spec.File, Line: spec.Line}}
	last := lastResult(t.Results)
	if last != nil {
		outcome.DurationMS = last.Duration
		outcome.Retries = last.Retry
		if msg, loc := lastFailureDetail(last); msg != "" {
			outcome.FailureMessage = msg
			if loc != nil {
				outcome.Anchor = &Anchor{File: loc.File, Line: loc.Line}
			}
		}
		for _, a := range last.Attachments {
			outcome.Artifacts = append(outcome.Artifacts, FailureArtifact{Name: a.Name, Path: a.Path})
		}
	}
	for i := range t.Results {
		outcome.AttemptDetails = append(outcome.AttemptDetails, playwrightAttemptDetail(&t.Results[i]))
	}
	outcome.State = playwrightState(t, last, outcome.FailureMessage)
	scrubPlaywrightOutcome(&outcome)
	return outcome
}

// scrubPlaywrightOutcome applies the run-evidence hygiene and the product secret screen (AFU-V1-038)
// to the last-attempt fields and to every attempt, after classification has read the raw message.
func scrubPlaywrightOutcome(outcome *TestOutcome) {
	outcome.FailureMessage, outcome.Artifacts = scrubAttempt(outcome.FailureMessage, outcome.Artifacts)
	for i := range outcome.AttemptDetails {
		detail := &outcome.AttemptDetails[i]
		detail.FailureMessage, detail.Artifacts = scrubAttempt(detail.FailureMessage, detail.Artifacts)
	}
}

func scrubAttempt(message string, artifacts []FailureArtifact) (string, []FailureArtifact) {
	var kept []FailureArtifact
	for _, a := range artifacts {
		if runhygiene.KeepAttachment(a.Name, a.Path) && !secretscreen.MatchString(a.Name+" "+a.Path) {
			kept = append(kept, a)
		}
	}
	screened, _ := secretscreen.Screen(runhygiene.ScrubFailure(message))
	return screened, kept
}

// playwrightAttemptDetail keeps one attempt as Playwright reported it, so a retry does not erase
// the earlier attempt's duration, failure, anchor or attachments (AFU-V1-012).
func playwrightAttemptDetail(r *playwrightResult) AttemptDetail {
	detail := AttemptDetail{State: ExecutionState(r.Status), Retry: r.Retry, DurationMS: r.Duration}
	msg, loc := lastFailureDetail(r)
	detail.FailureMessage = msg
	if loc != nil {
		detail.Anchor = &Anchor{File: loc.File, Line: loc.Line}
	}
	for _, a := range r.Attachments {
		detail.Artifacts = append(detail.Artifacts, FailureArtifact{Name: a.Name, Path: a.Path})
	}
	return detail
}

// playwrightState classifies from the last attempt's own result.status
// first (passed/failed/timedOut/skipped/interrupted are Playwright's real
// per-attempt states), not from the test-level aggregate alone: a test
// interrupted mid-run reports its own aggregate status as "skipped" even
// though the underlying result.status is "interrupted" (confirmed against
// testdata/playwright-interrupted.json), so reading only the aggregate would
// misclassify a cancellation as an ordinary skip.
// resultStatuses maps Playwright attempt statuses onto the TCQ report vocabulary
// so the shared TCQ-V0-049 rule, not the reporter's `flaky` label, decides that
// a test which passed on retry is flaky.
func resultStatuses(results []playwrightResult) []string {
	statuses := make([]string, 0, len(results))
	for _, result := range results {
		statuses = append(statuses, tcqStatus[ExecutionState(result.Status)])
	}
	return statuses
}

func playwrightState(t playwrightTest, last *playwrightResult, message string) ExecutionState {
	if last == nil {
		return StateSkipped
	}
	switch last.Status {
	case "passed":
		if tcq.Flaky(resultStatuses(t.Results)) {
			return StateFlaky
		}
		return StatePassed
	case "timedOut":
		return StateTimedOut
	case "interrupted":
		return StateInterrupted
	case "skipped":
		return StateSkipped
	case "failed":
		for _, pattern := range infrastructureMessagePatterns {
			if strings.Contains(message, pattern) {
				return StateInfrastructure
			}
		}
		return StateFailed
	default:
		return StateInfrastructure
	}
}

func lastResult(results []playwrightResult) *playwrightResult {
	if len(results) == 0 {
		return nil
	}
	return &results[len(results)-1]
}

func lastFailureDetail(r *playwrightResult) (string, *playwrightLocation) {
	if r.Error != nil && r.Error.Message != "" {
		return r.Error.Message, r.Error.Location
	}
	if len(r.Errors) > 0 {
		return r.Errors[0].Message, r.Errors[0].Location
	}
	return "", nil
}
