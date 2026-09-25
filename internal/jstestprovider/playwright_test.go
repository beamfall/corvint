package jstestprovider

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runhygiene"
)

func loadPW(t *testing.T, name string) []TestOutcome {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParsePlaywrightJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if infra != nil {
		t.Fatalf("unexpected run-level infrastructure failure: %+v", infra)
	}
	return outcomes
}

// TestParsePlaywrightJSON_MixedStates parses a real @playwright/test@1.63.0
// capture with one passing, one failing (with retry), one skipped, and one
// flaky (fails once then passes on retry) test - see evidence/ipr-08.md.
func TestParsePlaywrightJSON_MixedStates(t *testing.T) {
	outcomes := loadPW(t, "playwright-mixed.json")
	byTitle := map[string]TestOutcome{}
	for _, o := range outcomes {
		byTitle[o.Name] = o
	}
	if len(byTitle) != 4 {
		t.Fatalf("want 4 outcomes, got %d: %+v", len(byTitle), byTitle)
	}
	if o := byTitle["increments the counter"]; o.State != StatePassed {
		t.Fatalf("want passed, got %+v", o)
	}
	fail := byTitle["deliberately wrong workflow expectation"]
	if fail.State != StateFailed {
		t.Fatalf("want failed, got %+v", fail)
	}
	if fail.Anchor == nil || fail.Anchor.Line != 19 {
		t.Fatalf("want anchor at e2e.spec.js:19, got %+v", fail.Anchor)
	}
	if fail.Retries != 1 {
		t.Fatalf("want retries=1 (one retry attempted), got %d", fail.Retries)
	}
	if len(fail.Artifacts) == 0 {
		t.Fatalf("want bounded failure artifacts (trace/screenshot paths), got none")
	}
	if o := byTitle["skipped browser workflow"]; o.State != StateSkipped {
		t.Fatalf("want skipped, got %+v", o)
	}
	flaky := byTitle["flaky once then passes on retry"]
	if flaky.State != StateFlaky {
		t.Fatalf("want flaky, got %+v", flaky)
	}
	if flaky.Retries != 1 {
		t.Fatalf("want retries=1, got %d", flaky.Retries)
	}
}

func TestParsePlaywrightJSON_TimedOut(t *testing.T) {
	outcomes := loadPW(t, "playwright-timedout.json")
	if len(outcomes) != 1 || outcomes[0].State != StateTimedOut {
		t.Fatalf("want one timedOut outcome, got %+v", outcomes)
	}
}

// TestParsePlaywrightJSON_Interrupted confirms a SIGINT mid-run test - whose
// aggregate test.status Playwright itself reports as "skipped" even though
// the real result.status is "interrupted" - is classified as interrupted,
// not silently folded into an ordinary skip.
func TestParsePlaywrightJSON_Interrupted(t *testing.T) {
	outcomes := loadPW(t, "playwright-interrupted.json")
	byTitle := map[string]TestOutcome{}
	for _, o := range outcomes {
		byTitle[o.Name] = o
	}
	if o := byTitle["increments the counter"]; o.State != StatePassed {
		t.Fatalf("want the completed test still passed, got %+v", o)
	}
	interrupted := byTitle["deliberately wrong workflow expectation"]
	if interrupted.State != StateInterrupted {
		t.Fatalf("want interrupted (not skipped), got %+v", interrupted)
	}
	neverStarted := byTitle["skipped browser workflow"]
	if neverStarted.State != StateSkipped {
		t.Fatalf("want a test that never ran at all still skipped, got %+v", neverStarted)
	}
}

// TestParsePlaywrightJSON_MissingBrowser confirms a per-test "failed" result
// whose error message names a missing browser executable is classified as
// infrastructure, not an ordinary workflow assertion failure.
func TestParsePlaywrightJSON_MissingBrowser(t *testing.T) {
	outcomes := loadPW(t, "playwright-infra-missing-browser.json")
	for _, o := range outcomes {
		if o.State == StateSkipped {
			continue
		}
		if o.State != StateInfrastructure {
			t.Fatalf("want infrastructure for %q, got %+v", o.Name, o)
		}
	}
}

// TestParsePlaywrightJSON_NoSuitesCollected confirms a run that produced no
// suites at all (bad --project name) surfaces as a run-level
// InfrastructureFailure, never as zero silently-passing tests.
func TestParsePlaywrightJSON_NoSuitesCollected(t *testing.T) {
	data, err := os.ReadFile("testdata/playwright-infra-nosuites.json")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParsePlaywrightJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if infra == nil {
		t.Fatalf("want a run-level infrastructure failure, got outcomes=%+v", outcomes)
	}
	if infra.Reason != "no-suites-collected" {
		t.Fatalf("want reason no-suites-collected, got %q", infra.Reason)
	}
}

// TestParsePlaywrightJSON_EmptyReport confirms a report with no suites and no
// errors (e.g. "{}") still surfaces as a run-level InfrastructureFailure
// rather than a silently-passing empty receipt.
func TestParsePlaywrightJSON_EmptyReport(t *testing.T) {
	data, err := os.ReadFile("testdata/playwright-empty-report.json")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, infra, err := ParsePlaywrightJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if infra == nil {
		t.Fatalf("want a run-level infrastructure failure, got outcomes=%+v", outcomes)
	}
	if infra.Reason != "no-suites-collected" {
		t.Fatalf("want reason no-suites-collected, got %q", infra.Reason)
	}
}

// AFU-V1-012
func TestAFUV1PlaywrightProviderKeepsEveryAttempt(t *testing.T) {
	var flaky TestOutcome
	for _, o := range loadPW(t, "playwright-mixed.json") {
		if o.Name == "flaky once then passes on retry" {
			flaky = o
		}
	}
	if flaky.State != StateFlaky || len(flaky.AttemptDetails) != 2 {
		t.Fatalf("want a flaky test with two attempts, got %s with %d", flaky.State, len(flaky.AttemptDetails))
	}
	first, second := flaky.AttemptDetails[0], flaky.AttemptDetails[1]
	if first.State != StateFailed || first.Retry != 0 || first.DurationMS != 112 {
		t.Fatalf("first attempt lost its outcome or duration: %+v", first)
	}
	if !strings.Contains(first.FailureMessage, "fails on first attempt only") || first.Anchor == nil || first.Anchor.Line != 36 {
		t.Fatalf("first attempt lost its failure or anchor: %+v", first)
	}
	if len(first.Artifacts) != 3 || first.Artifacts[0].Name != "screenshot" || first.Artifacts[2].Name != "trace" {
		t.Fatalf("first attempt lost its attachments: %+v", first.Artifacts)
	}
	if second.State != StatePassed || second.Retry != 1 || second.DurationMS != 99 || second.FailureMessage != "" || len(second.Artifacts) != 0 {
		t.Fatalf("second attempt: %+v", second)
	}
	if flaky.DurationMS != 99 || flaky.FailureMessage != "" {
		t.Fatalf("last-attempt receipt fields changed: %+v", flaky)
	}
	data, err := json.Marshal(Receipt{Kind: "e2e", Tests: []TestOutcome{flaky}})
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Tests []struct {
			AttemptDetails []AttemptDetail `json:"attemptDetails"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &wire); err != nil || len(wire.Tests) != 1 || len(wire.Tests[0].AttemptDetails) != 2 {
		t.Fatalf("receipt wire lost attempts: %v %s", err, data)
	}
	if got := wire.Tests[0].AttemptDetails[0]; got.DurationMS != 112 || !strings.Contains(got.FailureMessage, "fails on first attempt only") || len(got.Artifacts) != 3 {
		t.Fatalf("receipt wire lost the first attempt's detail: %+v", got)
	}
	if _, err := EncodeQualified(Receipt{Profile: ExternalProfile, Kind: "e2e", Tests: []TestOutcome{flaky}}); err == nil || !strings.Contains(err.Error(), "attempt-details") {
		t.Fatalf("an external profile accepted attemptDetails: %v", err)
	}
	if _, failure := decodeQualifiedReport([]byte(`{"tests":[{"attemptDetails":[{"state":"passed","retry":0,"durationMs":1}]}]}`), ExternalProfile); failure == nil || failure.Reason != "report-unparseable" {
		t.Fatalf("the qualified reporter decoded attemptDetails: %+v", failure)
	}
}

// AFU-V1-038
func TestAFUV1PlaywrightProviderScrubsEveryAttempt(t *testing.T) {
	report := `{"suites":[{"title":"login.spec.ts","file":"login.spec.ts","specs":[{"title":"logs in","file":"login.spec.ts","line":3,"tests":[{"status":"flaky","results":[
 {"status":"failed","retry":0,"duration":5,"error":{"message":"login failed\nCookie: sid=abc123\nkey ghp_abcdefghijklmnopqrstuvwxyz0123456789"},
  "attachments":[{"name":"screenshot","path":"shot-0.png"},{"name":"cookies","path":"c.json"},{"name":"network","path":"run.har"},{"name":"inline"}]},
 {"status":"failed","retry":1,"duration":6,"error":{"message":"POST /login failed. Response body: {\"user\":\"jo\"}"},
  "attachments":[{"name":"screenshot","path":"shot-1.png"},{"name":"request","path":"req.json"}]}]}]}]}]}`
	outcomes, infra, err := ParsePlaywrightJSON([]byte(report))
	if err != nil || infra != nil || len(outcomes) != 1 || len(outcomes[0].AttemptDetails) != 2 {
		t.Fatalf("parse: %v %+v %+v", err, infra, outcomes)
	}
	got := outcomes[0]
	first, last := got.AttemptDetails[0], got.AttemptDetails[1]
	data, _ := json.Marshal(got)
	for _, leaked := range []string{"sid=abc123", "ghp_", "jo\\", "c.json", "run.har", "req.json", "inline"} {
		if strings.Contains(string(data), leaked) {
			t.Fatalf("receipt kept %q: %s", leaked, data)
		}
	}
	if first.FailureMessage != "login failed\n"+runhygiene.DroppedMarker+"\nkey [REDACTED]" || len(first.Artifacts) != 1 || first.Artifacts[0].Path != "shot-0.png" {
		t.Fatalf("first attempt not scrubbed: %+v", first)
	}
	if last.FailureMessage != "POST /login failed. "+runhygiene.DroppedMarker || len(last.Artifacts) != 1 || last.Artifacts[0].Path != "shot-1.png" {
		t.Fatalf("last attempt not scrubbed: %+v", last)
	}
	if got.FailureMessage != last.FailureMessage || len(got.Artifacts) != 1 || got.Artifacts[0].Path != "shot-1.png" {
		t.Fatalf("last-attempt receipt fields not scrubbed: %+v", got)
	}
}
