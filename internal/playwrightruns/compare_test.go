package playwrightruns

import (
	"reflect"
	"testing"
)

func loaded(t *testing.T, raws ...[]byte) []Loaded {
	t.Helper()
	out := []Loaded{}
	for _, raw := range raws {
		out = append(out, Loaded{Run: mustParse(t, raw)})
	}
	return out
}

func onlyEntry(t *testing.T, r CompareReport) CompareEntry {
	t.Helper()
	if len(r.Tests) != 1 {
		t.Fatalf("entries = %d: %+v", len(r.Tests), r.Tests)
	}
	return r.Tests[0]
}

func TestCompareVerdicts(t *testing.T) {
	id := identity("app-7", "cfg-1", blobsA)
	candID := identity("app-8", "cfg-1", blobsA)
	testFileErr := "Error: expect(locator).toHaveText(expected) failed"
	t.Run("PWR-V0-006 PWR-V0-008 identical runs are PARITY", func(t *testing.T) {
		run := reportJSON(t, "1.63.0", id, cartTest(false, fail(testFileErr, fixtureRoot+"/cart.spec.ts", 9)), fxTest{file: "auth.spec.ts", title: "logs in", status: "expected", results: []fxResult{pass()}})
		cand := reportJSON(t, "1.63.0", candID, cartTest(false, fail(testFileErr, fixtureRoot+"/cart.spec.ts", 9)), fxTest{file: "auth.spec.ts", title: "logs in", status: "expected", results: []fxResult{pass()}})
		r := Compare(loaded(t, run, run), loaded(t, cand, cand))
		if r.Verdict != Parity || len(r.Reasons) != 0 || r.Summary != (CompareSummary{Same: 2}) || len(r.Unknowns) != 0 {
			t.Fatalf("report = %+v", r)
		}
		for _, e := range r.Tests {
			if e.Verdict != Same || len(e.Differences) != 0 || e.Baseline.Stability != "2/2" || e.Candidate.Reports != 2 {
				t.Fatalf("entry = %+v", e)
			}
		}
	})
	t.Run("PWR-V0-006 PWR-V0-007 a changed test-file assertion is DIVERGED", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, fail(testFileErr, fixtureRoot+"/cart.spec.ts", 9)))
		cand := reportJSON(t, "1.63.0", candID, cartTest(false, fail("Error: expect(page).toHaveURL(expected) failed", fixtureRoot+"/cart.spec.ts", 12)))
		r := Compare(loaded(t, base), loaded(t, cand))
		e := onlyEntry(t, r)
		if r.Verdict != Diverged || !reflect.DeepEqual(r.Reasons, []string{ReasonTestsChanged}) || e.Verdict != Changed ||
			!reflect.DeepEqual(e.Differences, []string{DifferenceTestFile}) || e.SupportDifference {
			t.Fatalf("report = %+v entry = %+v", r, e)
		}
		want := []Assertion{{Origin: OriginTestFile, Matcher: "toHaveText", File: "cart.spec.ts", Line: 9}}
		if !reflect.DeepEqual(e.Baseline.FailedAssertions, want) || e.Candidate.FailedAssertions[0].Matcher != "toHaveURL" {
			t.Fatalf("assertions = %+v / %+v", e.Baseline.FailedAssertions, e.Candidate.FailedAssertions)
		}
		passed := reportJSON(t, "1.63.0", candID, cartTest(false, pass()))
		if e := onlyEntry(t, Compare(loaded(t, base), loaded(t, passed))); e.Verdict != Changed || !reflect.DeepEqual(e.Differences, []string{DifferenceOutcome, DifferenceTestFile}) {
			t.Fatalf("outcome change = %+v", e)
		}
		noLocation := reportJSON(t, "1.63.0", candID, cartTest(false, fail(testFileErr, "", 0)))
		if e := onlyEntry(t, Compare(loaded(t, base), loaded(t, noLocation))); e.Verdict != Changed || !reflect.DeepEqual(e.Differences, []string{DifferenceTestFile, DifferenceUnknownOrigin}) {
			t.Fatalf("unknown origin = %+v", e)
		}
	})
	t.Run("PWR-V0-007 a fixture-only assertion difference is reported as a support difference", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, fail("Error: expect(locator).toBeVisible() failed", fixtureRoot+"/pages/cart.ts", 20)))
		cand := reportJSON(t, "1.63.0", candID, cartTest(false, fail("Error: expect(locator).toBeEnabled() failed", fixtureRoot+"/fixtures/login.ts", 31)))
		r := Compare(loaded(t, base), loaded(t, cand))
		e := onlyEntry(t, r)
		if r.Verdict != Parity || !reflect.DeepEqual(r.Reasons, []string{ReasonSupportDiffers}) || r.Summary != (CompareSummary{Same: 1, SupportDifferences: 1}) {
			t.Fatalf("report = %+v", r)
		}
		if e.Verdict != Same || !e.SupportDifference || !reflect.DeepEqual(e.Differences, []string{DifferenceSupport}) || e.Candidate.FailedAssertions[0].Origin != OriginSupport {
			t.Fatalf("entry = %+v", e)
		}
	})
	t.Run("PWR-V0-006 renamed tests are MISSING and NEW", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		renamed := cartTest(false, pass())
		renamed.title = "adds one item"
		r := Compare(loaded(t, base), loaded(t, reportJSON(t, "1.63.0", candID, renamed)))
		if r.Verdict != Diverged || !reflect.DeepEqual(r.Reasons, []string{ReasonTestsMissing, ReasonTestsNew}) || r.Summary != (CompareSummary{Missing: 1, New: 1}) {
			t.Fatalf("report = %+v", r)
		}
		for _, e := range r.Tests {
			if (e.Verdict == Missing) != (e.Candidate == nil) || (e.Verdict == New) != (e.Baseline == nil) {
				t.Fatalf("entry = %+v", e)
			}
		}
	})
	t.Run("PWR-V0-008 mismatched configs and missing inputs are INCOMPARABLE", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		cases := map[string]struct {
			baseline, candidate []Loaded
			want                []string
		}{
			"missing report":     {loaded(t, base), []Loaded{{Err: ErrUnreadable}}, []string{ReasonReportUnreadable}},
			"playwright version": {loaded(t, base), loaded(t, reportJSON(t, "1.64.0", candID, cartTest(false, pass()))), []string{ReasonVersionDiffers}},
			"config digest":      {loaded(t, base), loaded(t, reportJSON(t, "1.63.0", identity("app-8", "cfg-2", blobsA), cartTest(false, pass()))), []string{ReasonConfigDiffers}},
			"projects": {loaded(t, base), loaded(t, reportJSON(t, "1.63.0", candID, cartTest(false, pass()),
				fxTest{project: "firefox", file: "cart.spec.ts", title: "adds item", describe: []string{"cart"}, status: "expected", results: []fxResult{pass()}})), []string{ReasonProjectsDiffer}},
			"inputs within side": {loaded(t, base, reportJSON(t, "1.63.0", candID, cartTest(false, pass()))), loaded(t, base), []string{ReasonInputsDifferInSide}},
			"ambiguous and interrupted": {loaded(t, reportJSON(t, "1.63.0", id, cartTest(false, pass()), cartTest(false, pass()))),
				loaded(t, reportJSON(t, "1.63.0", candID, cartTest(false, fxResult{status: "interrupted"}))), []string{ReasonAmbiguous, ReasonInterrupted}},
			"no tests": {loaded(t, base), loaded(t, reportJSON(t, "1.63.0", candID)), []string{ReasonProjectsDiffer, ReasonNoTests}},
		}
		for name, c := range cases {
			r := Compare(c.baseline, c.candidate)
			if r.Verdict != Incomparable || !reflect.DeepEqual(r.Reasons, c.want) {
				t.Errorf("%s: verdict = %s reasons = %v, want %v", name, r.Verdict, r.Reasons, c.want)
			}
		}
		r := Compare(loaded(t, base), []Loaded{{Err: ErrShape}})
		if !reflect.DeepEqual(r.Problems, []Problem{{Side: SideCandidate, Report: 1, Reason: "REPORT_NOT_PLAYWRIGHT_JSON"}}) || len(r.Tests) != 0 {
			t.Fatalf("problems = %+v", r.Problems)
		}
		errored := loaded(t, base)
		errored[0].Run.ReportErrors = 1
		if r := Compare(errored, loaded(t, base)); r.Verdict != Incomparable || !reflect.DeepEqual(r.Reasons, []string{ReasonReportHasErrors}) {
			t.Fatalf("report errors = %+v", r)
		}
		undeclared := Compare(loaded(t, reportJSON(t, "1.63.0", nil, cartTest(false, pass()))), loaded(t, base))
		if undeclared.Verdict != Parity || !reflect.DeepEqual(undeclared.Unknowns, []string{UnknownInputsNotDeclared}) {
			t.Fatalf("undeclared = %+v", undeclared)
		}
	})
	t.Run("PWR-V0-009 resource facts and k/N stability over repeats", func(t *testing.T) {
		pass1 := cartTest(false, fxResult{status: "passed", worker: 3})
		failing := cartTest(false, fxResult{status: "failed", worker: 1, message: testFileErr, file: fixtureRoot + "/cart.spec.ts", line: 9},
			fxResult{status: "failed", retry: 1, worker: 3, message: testFileErr, file: fixtureRoot + "/cart.spec.ts", line: 9})
		r := Compare(loaded(t, reportJSON(t, "1.63.0", id, pass1), reportJSON(t, "1.63.0", id, failing), reportJSON(t, "1.63.0", id, pass1)),
			loaded(t, reportJSON(t, "1.63.0", candID, pass1)))
		f := r.Reports[SideBaseline][1]
		if f.Report != 2 || !reflect.DeepEqual(f.WorkerIndexes, []int{1, 3}) || f.ResultDurationMS != 24 || f.FixtureDurations != NotObserved || f.Cleanup != NotObserved || f.Workers != 2 {
			t.Fatalf("facts = %+v", f)
		}
		e := onlyEntry(t, r)
		if e.Baseline.Stability != "2/3" || e.Baseline.MaxRetry != 1 || e.Candidate.Stability != "1/1" ||
			!reflect.DeepEqual(e.Baseline.Outcomes, []OutcomeCount{{"failed", 1}, {"passed", 2}}) || e.Verdict != Changed ||
			!reflect.DeepEqual(e.Differences, []string{DifferenceOutcome, DifferenceRetries, DifferenceTestFile}) {
			t.Fatalf("entry = %+v %+v", e, e.Baseline)
		}
		same := Compare(loaded(t, reportJSON(t, "1.63.0", id, pass1), reportJSON(t, "1.63.0", id, pass1)), loaded(t, reportJSON(t, "1.63.0", candID, pass1)))
		if same.Verdict != Parity {
			t.Fatalf("2/2 versus 1/1 passed = %+v", same)
		}
	})
}
