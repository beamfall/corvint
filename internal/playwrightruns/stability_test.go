package playwrightruns

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
		if report.Summary != (StabilitySummary{Intermittent: 1, ExpectedFailNotDeterministic: 1}) {
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
		want := map[string][]int{ReasonInputsMissing: {1, 2}, ReasonAmbiguous: {3, 3}, ReasonInterrupted: {4}}
		if !reflect.DeepEqual(reasons, want) {
			t.Fatalf("reasons = %v, want %v", reasons, want)
		}
	})
}
