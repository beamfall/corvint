package playwrightruns

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var blobsA = map[string]string{"cart.spec.ts": "1111111111111111111111111111111111111111"}

func cartTest(expectedFail bool, results ...fxResult) fxTest {
	status := "expected"
	for _, r := range results {
		if (r.status == "passed") == expectedFail {
			status = "unexpected"
		}
	}
	return fxTest{file: "cart.spec.ts", title: "adds item", describe: []string{"cart"}, status: status, expectedFail: expectedFail, results: results}
}

func runsOf(t *testing.T, raws ...[]byte) []*Run {
	out := []*Run{}
	for _, raw := range raws {
		out = append(out, mustParse(t, raw))
	}
	return out
}

func onlyEntries(t *testing.T, r StabilityReport, want int) []StabilityEntry {
	t.Helper()
	if len(r.Tests) != want {
		t.Fatalf("entries = %d, want %d: %+v", len(r.Tests), want, r.Tests)
	}
	return r.Tests
}

func TestReaderIdentityAndHygiene(t *testing.T) {
	t.Run("PWR-V0-001 stable identity, title path and screened first line", func(t *testing.T) {
		raw := reportJSON(t, "1.63.0", nil, cartTest(false, fail("\x1b[31mError: expect(locator).toHaveText(expected) failed\x1b[39m\nsecond line", fixtureRoot+"/cart.spec.ts", 9)),
			fxTest{project: "firefox", file: "auth/login.spec.ts", title: "logs in", status: "unexpected", results: []fxResult{fail("Error: token=abc123 rejected", "", 0)}})
		run := mustParse(t, raw)
		if len(run.Tests) != 2 || !reflect.DeepEqual(run.Projects, []string{"chromium", "firefox"}) {
			t.Fatalf("run = %+v", run)
		}
		cart := run.Tests[0]
		if !reflect.DeepEqual(cart.Identity, Identity{Project: "chromium", File: "cart.spec.ts", TitlePath: []string{"cart", "adds item"}}) {
			t.Fatalf("identity = %+v", cart.Identity)
		}
		if got := cart.Results[0].FirstError; got != "Error: expect(locator).toHaveText(expected) failed" {
			t.Fatalf("first error = %q", got)
		}
		if got := run.Tests[1].Results[0].FirstError; strings.Contains(got, "abc123") {
			t.Fatalf("sensitive line kept: %q", got)
		}
		if long := FirstLine(strings.Repeat("é", 400)); len(long) > maxLineBytes || !strings.HasPrefix(long, "é") {
			t.Fatalf("long line = %d bytes", len(long))
		}
	})
	t.Run("PWR-V0-001 refusals", func(t *testing.T) {
		for name, raw := range map[string]string{
			"not json":       "{",
			"two values":     `{"config":{"version":"1.63.0"},"suites":[]} {}`,
			"no version":     `{"config":{},"suites":[]}`,
			"no suites":      `{"config":{"version":"1.63.0"}}`,
			"unknown status": `{"config":{"version":"1.63.0"},"suites":[{"title":"a.spec.ts","file":"a.spec.ts","specs":[{"title":"t","file":"a.spec.ts","tests":[{"projectName":"p","status":"expected","results":[{"status":"weird"}]}]}]}]}`,
		} {
			if _, err := Parse([]byte(raw)); !errors.Is(err, ErrShape) {
				t.Errorf("%s: err = %v", name, err)
			}
		}
		if _, err := Parse(make([]byte, MaxReportBytes+1)); !errors.Is(err, ErrTooLarge) {
			t.Errorf("oversize: err = %v", err)
		}
		dir := t.TempDir()
		if _, err := ReadFile(filepath.Join(dir, "missing.json")); !errors.Is(err, ErrUnreadable) {
			t.Errorf("missing: err = %v", err)
		}
		if _, err := ReadFile(dir); !errors.Is(err, ErrUnreadable) {
			t.Errorf("directory: err = %v", err)
		}
		good := filepath.Join(dir, "report.json")
		if err := os.WriteFile(good, reportJSON(t, "1.63.0", nil, cartTest(false, pass())), 0o600); err != nil {
			t.Fatal(err)
		}
		if run, err := ReadFile(good); err != nil || len(run.Tests) != 1 || len(run.Sha256) != 64 {
			t.Fatalf("ReadFile = %+v, %v", run, err)
		}
	})
}

func TestStabilityVerdicts(t *testing.T) {
	id := identity("app-7", "cfg-1", blobsA)
	firstErr := "Error: expect(locator).toHaveText(expected) failed"
	otherErr := "TimeoutError: locator.click: Timeout 5000ms exceeded."
	t.Run("PWR-V0-003 fail pass fail at identical inputs is INTERMITTENT with both first errors", func(t *testing.T) {
		runs := runsOf(t,
			reportJSON(t, "1.63.0", id, cartTest(false, fail(firstErr, fixtureRoot+"/cart.spec.ts", 9))),
			reportJSON(t, "1.63.0", id, cartTest(false, pass())),
			reportJSON(t, "1.63.0", id, cartTest(false, fail(otherErr, fixtureRoot+"/pages/cart.ts", 20))))
		e := onlyEntries(t, Stability(runs, DefaultMinRuns), 1)[0]
		if e.Verdict != Intermittent || e.Reason != "" || !reflect.DeepEqual(e.Outcomes, []string{"failed", "passed", "failed"}) || !reflect.DeepEqual(e.Runs, []int{1, 2, 3}) {
			t.Fatalf("entry = %+v", e)
		}
		if !reflect.DeepEqual(e.FirstErrors, []string{firstErr, otherErr}) || len(e.Flags) != 0 {
			t.Fatalf("first errors/flags = %q %q", e.FirstErrors, e.Flags)
		}
	})
	t.Run("PWR-V0-002 changed test file blob is not pooled", func(t *testing.T) {
		changed := identity("app-7", "cfg-1", map[string]string{"cart.spec.ts": "2222222222222222222222222222222222222222"})
		runs := runsOf(t,
			reportJSON(t, "1.63.0", id, cartTest(false, pass())),
			reportJSON(t, "1.63.0", id, cartTest(false, pass())),
			reportJSON(t, "1.63.0", changed, cartTest(false, fail(firstErr, "", 0))))
		entries := onlyEntries(t, Stability(runs, 2), 2)
		byBlob := map[string]StabilityEntry{}
		for _, e := range entries {
			byBlob[e.Inputs.TestFileBlob] = e
		}
		if e := byBlob[blobsA["cart.spec.ts"]]; e.Verdict != Deterministic || !reflect.DeepEqual(e.Runs, []int{1, 2}) {
			t.Fatalf("original blob entry = %+v", e)
		}
		if e := byBlob["2222222222222222222222222222222222222222"]; e.Verdict != Insufficient || e.Reason != ReasonBelowThreshold || !reflect.DeepEqual(e.Runs, []int{3}) {
			t.Fatalf("changed blob entry = %+v", e)
		}
	})
	t.Run("PWR-V0-003 fewer runs than the threshold is INSUFFICIENT never DETERMINISTIC", func(t *testing.T) {
		two := runsOf(t, reportJSON(t, "1.63.0", id, cartTest(false, pass())), reportJSON(t, "1.63.0", id, cartTest(false, pass())))
		if e := onlyEntries(t, Stability(two, 3), 1)[0]; e.Verdict != Insufficient || e.Reason != ReasonBelowThreshold {
			t.Fatalf("entry = %+v", e)
		}
		if e := onlyEntries(t, Stability(two, 2), 1)[0]; e.Verdict != Deterministic {
			t.Fatalf("at threshold = %+v", e)
		}
		flaky := cartTest(false, fail(firstErr, "", 0), pass())
		flaky.status = "flaky"
		one := runsOf(t, reportJSON(t, "1.63.0", id, flaky))
		if e := onlyEntries(t, Stability(one, 3), 1)[0]; e.Verdict != Intermittent || e.Outcomes[0] != "flaky" {
			t.Fatalf("flaky run = %+v", e)
		}
	})
	t.Run("PWR-V0-004 intermittent expected-fail is flagged with suggestions", func(t *testing.T) {
		runs := runsOf(t,
			reportJSON(t, "1.63.0", id, cartTest(true, fail(firstErr, "", 0))),
			reportJSON(t, "1.63.0", id, cartTest(true, pass())),
			reportJSON(t, "1.63.0", id, cartTest(true, fail(firstErr, "", 0))))
		report := Stability(runs, DefaultMinRuns)
		e := onlyEntries(t, report, 1)[0]
		if e.Verdict != Intermittent || !reflect.DeepEqual(e.Flags, []string{FlagExpectedFailNotDeterministic}) || len(e.Suggestions) != 2 {
			t.Fatalf("entry = %+v", e)
		}
		if report.Summary != (StabilitySummary{Intermittent: 1, ExpectedFailNotDeterministic: 1, ServedDigestUnknownRuns: 3}) {
			t.Fatalf("summary = %+v", report.Summary)
		}
		steady := runsOf(t, reportJSON(t, "1.63.0", id, cartTest(true, fail(firstErr, "", 0))), reportJSON(t, "1.63.0", id, cartTest(true, fail(firstErr, "", 0))),
			reportJSON(t, "1.63.0", id, cartTest(true, fail(firstErr, "", 0))))
		if e := onlyEntries(t, Stability(steady, 3), 1)[0]; e.Verdict != Deterministic || len(e.Flags) != 0 {
			t.Fatalf("deterministic expected-fail = %+v", e)
		}
	})
	t.Run("PWR-V0-002 PWR-V0-003 missing inputs, ambiguity and interruption abstain", func(t *testing.T) {
		noBlob := identity("app-7", "cfg-1", map[string]string{})
		dup := reportJSON(t, "1.63.0", id, cartTest(false, pass()), cartTest(false, pass()))
		interrupted := reportJSON(t, "1.63.0", id, cartTest(false, fxResult{status: "interrupted"}))
		runs := runsOf(t, reportJSON(t, "1.63.0", nil, cartTest(false, pass())), reportJSON(t, "1.63.0", noBlob, cartTest(false, fail(firstErr, "", 0))), dup, interrupted)
		reasons := map[string][]int{}
		for _, e := range Stability(runs, 2).Tests {
			if e.Verdict != Insufficient || e.Inputs != nil {
				t.Fatalf("entry = %+v", e)
			}
			reasons[e.Reason] = e.Runs
		}
		want := map[string][]int{ReasonInputsMissing: {2}, ReasonAmbiguous: {3, 3}, ReasonInterrupted: {4}}
		if !reflect.DeepEqual(reasons, want) {
			t.Fatalf("reasons = %v, want %v", reasons, want)
		}
	})
	t.Run("PWR-V0-051 an undeclared report is named once, not per test", func(t *testing.T) {
		other := cartTest(false, pass())
		other.title = "removes an item"
		undeclared := reportJSON(t, "1.63.0", nil, cartTest(false, pass()), other)
		declared := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		report := Stability(runsOf(t, undeclared, declared, undeclared), 2)
		for _, e := range report.Tests {
			if e.Reason == ReasonInputsMissing || !reflect.DeepEqual(e.Runs, []int{2}) {
				t.Fatalf("entry = %+v", e)
			}
		}
		if len(report.Tests) != 1 || report.Summary.IdentityNotDeclaredRuns != 2 || report.Summary.Insufficient != 1 {
			t.Fatalf("tests = %+v summary = %+v", report.Tests, report.Summary)
		}
		for _, n := range []int{0, 2} {
			r := report.Runs[n]
			if r.IdentityInputs != "NOT_DECLARED" || r.UnpooledTests != 2 || r.Remedy != IdentityRemedy {
				t.Fatalf("run %d = %+v", n+1, r)
			}
		}
		if r := report.Runs[1]; r.UnpooledTests != 0 || r.Remedy != "" {
			t.Fatalf("declared run = %+v", r)
		}
	})
}

func TestStabilityInterruptedAndWindow(t *testing.T) {
	id := identity("app-7", "cfg-1", blobsA)
	t.Run("PWR-V0-058 an interrupted run is an excluded observation on the test's one row", func(t *testing.T) {
		skipped := cartTest(false, fxResult{status: "skipped"})
		skipped.status = "skipped"
		passing := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		aborted := reportJSON(t, "1.63.0", id, cartTest(false, fxResult{status: "interrupted"}))
		report := Stability(runsOf(t, passing, passing, aborted, passing, reportJSON(t, "1.63.0", id, skipped)), 3)
		e := onlyEntries(t, report, 1)[0]
		want := []ExcludedObservation{{Run: 3, Reason: ReasonInterrupted}, {Run: 5, Reason: ReasonNotRun}}
		if e.Verdict != Deterministic || !reflect.DeepEqual(e.Runs, []int{1, 2, 4}) || !reflect.DeepEqual(e.Excluded, want) {
			t.Fatalf("entry = %+v", e)
		}
		if s := report.Summary; s.Deterministic != 1 || s.Insufficient != 0 || s.InterruptedRuns != 1 || s.ExcludedObservations != 2 {
			t.Fatalf("summary = %+v", s)
		}
		if !report.Runs[2].Interrupted || report.Runs[0].Interrupted {
			t.Fatalf("runs = %+v", report.Runs)
		}
		// Excluded observations never count toward --min-runs.
		if e := onlyEntries(t, Stability(runsOf(t, passing, passing, aborted), 3), 1)[0]; e.Verdict != Insufficient || e.Reason != ReasonBelowThreshold || len(e.Excluded) != 1 {
			t.Fatalf("below threshold = %+v", e)
		}
		// Without a pooled entry at its inputs, the observation keeps its own INSUFFICIENT row.
		other := reportJSON(t, "1.63.0", identity("app-8", "cfg-1", blobsA), cartTest(false, pass()))
		r := Stability(runsOf(t, other, aborted), 2)
		if len(r.Tests) != 2 || r.Summary.ExcludedObservations != 0 {
			t.Fatalf("tests = %+v", r.Tests)
		}
	})
	t.Run("PWR-V0-063 --last keeps the latest logical runs by start time", func(t *testing.T) {
		failing := reportJSON(t, "1.63.0", id, cartTest(false, fail("Error: boom", "", 0)))
		passing := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		runs := runsOf(t, passing, failing, passing, passing)
		base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		for i, hour := range []int{3, 0, 1, 2} {
			runs[i].StartTime, runs[i].HasStartTime = base.Add(time.Duration(hour)*time.Hour), true
		}
		if e := onlyEntries(t, StabilityLast(runs, 3, 0), 1)[0]; e.Verdict != Intermittent {
			t.Fatalf("all runs = %+v", e)
		}
		report := StabilityLast(runs, 3, 3)
		if e := onlyEntries(t, report, 1)[0]; e.Verdict != Deterministic || !reflect.DeepEqual(e.Runs, []int{1, 3, 4}) {
			t.Fatalf("window = %+v", e)
		}
		if report.Last != 3 || !report.Runs[1].OutsideWindow || report.Runs[0].OutsideWindow {
			t.Fatalf("runs = %+v", report.Runs)
		}
	})
}

// PWR-V0-174 and PWR-V0-175 (GitHub #1918).
func TestStabilityClosureInterruptedAndCompletedWindow(t *testing.T) {
	id := identity("app-7", "cfg-1", blobsA)
	passing := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
	aborted := reportJSON(t, "1.63.0", id, cartTest(false, fxResult{status: "interrupted"}))
	skipped := cartTest(false, fxResult{status: "skipped"})
	skipped.status = "skipped"
	notRun := reportJSON(t, "1.63.0", id, skipped)
	t.Run("PWR-V0-174 a test that cannot pool keeps one row in closure mode", func(t *testing.T) {
		report := StabilityPooled(runsOf(t, passing, passing, aborted, notRun), 2, 0, PoolByClosure)
		e := onlyEntries(t, report, 1)[0]
		want := []ExcludedObservation{{Run: 3, Reason: ReasonInterrupted}, {Run: 4, Reason: ReasonNotRun}}
		if e.Verdict != Insufficient || e.Reason != ReasonInputsMissing || !reflect.DeepEqual(e.Excluded, want) {
			t.Fatalf("entry = %+v", e)
		}
		if s := report.Summary; s.Insufficient != 1 || s.ExcludedObservations != 2 {
			t.Fatalf("summary = %+v", s)
		}
	})
	window := func(t *testing.T, rule string) StabilityReport {
		runs := runsOf(t, passing, passing, aborted, passing)
		base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		for i := range runs {
			runs[i].StartTime, runs[i].HasStartTime = base.Add(time.Duration(i)*time.Hour), true
		}
		return StabilityWindowed(runs, 3, 3, PoolByInputs, rule)
	}
	t.Run("PWR-V0-175 the default window counts the interrupted run and names it", func(t *testing.T) {
		r := window(t, WindowAll)
		if e := onlyEntries(t, r, 1)[0]; e.Verdict != Insufficient || e.Reason != ReasonBelowThreshold {
			t.Fatalf("entry = %+v", e)
		}
		if r.Window != WindowAll || !reflect.DeepEqual(r.WindowInterruptedRuns, []int{3}) || !r.Runs[0].OutsideWindow {
			t.Fatalf("report = %+v", r)
		}
	})
	t.Run("PWR-V0-175 the completed window skips the interrupted run", func(t *testing.T) {
		r := window(t, WindowCompleted)
		if e := onlyEntries(t, r, 1)[0]; e.Verdict != Deterministic || !reflect.DeepEqual(e.Runs, []int{1, 2, 4}) {
			t.Fatalf("entry = %+v", e)
		}
		if r.Window != WindowCompleted || len(r.WindowInterruptedRuns) != 0 || !r.Runs[2].OutsideWindow {
			t.Fatalf("report = %+v", r)
		}
	})
}
