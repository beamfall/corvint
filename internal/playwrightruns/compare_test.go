package playwrightruns

import (
	"encoding/json"
	"reflect"
	"strings"
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
		if r.Verdict != Parity || len(r.Reasons) != 0 || r.Summary != (CompareSummary{Same: 2}) || !reflect.DeepEqual(r.Unknowns, []string{UnknownServedDigest}) {
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
			absent := []OutcomeCount{{"absent", 1}}
			if (e.Verdict == Missing) != reflect.DeepEqual(e.Candidate.Outcomes, absent) || (e.Verdict == New) != reflect.DeepEqual(e.Baseline.Outcomes, absent) ||
				e.Candidate.Stability != "1/1" || e.Baseline.Stability != "1/1" {
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
		if undeclared.Verdict != Parity || !reflect.DeepEqual(undeclared.Unknowns, []string{UnknownInputsNotDeclared, UnknownServedDigest}) {
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

// mutate rewrites one generated report document through its decoded JSON form.
func mutate(t *testing.T, raw []byte, edit func(doc map[string]any)) []byte {
	t.Helper()
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReaderAndCompareEdgeCases(t *testing.T) {
	id := identity("app-7", "cfg-1", blobsA)
	t.Run("PWR-V0-001 a test without projectName is refused", func(t *testing.T) {
		raw := mutate(t, reportJSON(t, "1.63.0", id, cartTest(false, pass())), func(doc map[string]any) {
			test := doc["suites"].([]any)[0].(map[string]any)["suites"].([]any)[0].(map[string]any)["specs"].([]any)[0].(map[string]any)["tests"].([]any)[0].(map[string]any)
			delete(test, "projectName")
		})
		if _, err := Parse(raw); err != ErrShape {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("PWR-V0-001 PWR-V0-007 report-syntax paths and assertion modifiers", func(t *testing.T) {
		raw := mutate(t, reportJSON(t, "1.63.0", id,
			cartTest(false, fail("Error: expect(page.locator('#a')).resolves.not.toHaveText(expected) failed", `C:\work\e2e\cart.spec.ts`, 9))), func(doc map[string]any) {
			doc["config"].(map[string]any)["rootDir"] = `C:\work\e2e`
		})
		got := mustParse(t, raw).Tests[0].Results[0].Failures
		want := []Assertion{{Origin: OriginTestFile, Matcher: "resolves.not.toHaveText", File: "cart.spec.ts", Line: 9}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("failures = %+v", got)
		}
		custom := func(name string) []byte {
			return reportJSON(t, "1.63.0", id, cartTest(false, fail("Error: expect(response)."+name+"() failed", fixtureRoot+"/cart.spec.ts", 9)))
		}
		r := Compare(loaded(t, custom("toHaveHTTPStatus2")), loaded(t, custom("toHaveHTTPStatus3")))
		if e := onlyEntry(t, r); e.Verdict != Changed || e.Baseline.FailedAssertions[0].Matcher != "toHaveHTTPStatus2" {
			t.Fatalf("custom matcher = %+v", e)
		}
		for _, pair := range [][2]string{{"toHaveRésumé", "toHaveRôle"}, {"toHave·Résumé", "toHave·Rôle"}, {"toHave$a", "toHave$b"}} {
			r = Compare(loaded(t, custom(pair[0])), loaded(t, custom(pair[1])))
			if e := onlyEntry(t, r); e.Verdict != Changed || e.Baseline.FailedAssertions[0].Matcher != pair[0] {
				t.Fatalf("matcher %q = %+v", pair[0], e)
			}
		}
	})
	t.Run("PWR-V0-002 conflicting blob spellings are not resolved", func(t *testing.T) {
		conflicting := identity("app-7", "cfg-1", map[string]string{"cart.spec.ts": "1111", "./cart.spec.ts": "2222"})
		runs := []*Run{mustParse(t, reportJSON(t, "1.63.0", conflicting, cartTest(false, pass()))), mustParse(t, reportJSON(t, "1.63.0", conflicting, cartTest(false, pass())))}
		e := Stability(runs, 2).Tests
		if len(e) != 1 || e[0].Verdict != Insufficient || e[0].Reason != ReasonInputsMissing {
			t.Fatalf("entries = %+v", e)
		}
		other := reportJSON(t, "1.63.0", identity("app-7", "cfg-1", map[string]string{"cart.spec.ts": "3333", "./cart.spec.ts": "4444"}), cartTest(false, pass()))
		same := reportJSON(t, "1.63.0", conflicting, cartTest(false, pass()))
		if r := Compare(loaded(t, same, other), loaded(t, same)); !reflect.DeepEqual(r.Reasons, []string{ReasonInputsDifferInSide}) {
			t.Fatalf("different conflicts: reasons = %v", r.Reasons)
		}
		if r := Compare(loaded(t, same, same), loaded(t, same)); r.Verdict != Parity {
			t.Fatalf("same conflicts = %+v", r)
		}
	})
	t.Run("PWR-V0-001 identities are screened on output and never merged", func(t *testing.T) {
		a, b := cartTest(false, pass()), cartTest(false, pass())
		a.title, b.title = "login token=abc123", "login token=def456"
		run := mustParse(t, reportJSON(t, "1.63.0", id, a, b))
		entries := Stability([]*Run{run, run}, 2).Tests
		if len(entries) != 2 {
			t.Fatalf("entries = %+v", entries)
		}
		raw, _ := json.Marshal(entries)
		cmp, _ := json.Marshal(Compare(loaded(t, reportJSON(t, "1.63.0", id, a, b)), loaded(t, reportJSON(t, "1.63.0", id, a, b))))
		if strings.Contains(string(raw), "abc123") || strings.Contains(string(cmp), "def456") || strings.Count(string(cmp), `"verdict":"SAME"`) != 2 {
			t.Fatalf("stability = %s compare = %s", raw, cmp)
		}
	})
	t.Run("PWR-V0-008 every applicable reason is kept", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, pass()))
		r := Compare(loaded(t, base), append(loaded(t, reportJSON(t, "1.64.0", id, cartTest(false, pass()))), Loaded{Err: ErrUnreadable}))
		if !reflect.DeepEqual(r.Reasons, []string{ReasonReportUnreadable, ReasonVersionDiffers}) || len(r.Tests) != 0 {
			t.Fatalf("unreadable plus version = %+v", r)
		}
		otherCfg := identity("app-8", "cfg-1", blobsA)
		undeclared := reportJSON(t, "1.63.0", nil, cartTest(false, pass()))
		for _, order := range [][][]byte{{undeclared, base, reportJSON(t, "1.63.0", otherCfg, cartTest(false, pass()))}, {base, undeclared, reportJSON(t, "1.63.0", otherCfg, cartTest(false, pass()))}} {
			if r := Compare(loaded(t, order...), loaded(t, base)); !reflect.DeepEqual(r.Reasons, []string{ReasonInputsDifferInSide}) {
				t.Fatalf("within-side order: reasons = %v", r.Reasons)
			}
		}
		configured := mutate(t, base, func(doc map[string]any) {
			doc["config"].(map[string]any)["projects"] = []any{map[string]any{"name": "chromium"}, map[string]any{"name": "webkit"}}
		})
		r = Compare(loaded(t, base), loaded(t, configured))
		if !reflect.DeepEqual(r.Reasons, []string{ReasonProjectsDiffer}) || !reflect.DeepEqual(r.Reports[SideCandidate][0].ConfiguredProjects, []string{"chromium", "webkit"}) {
			t.Fatalf("configured projects = %+v", r)
		}
	})
	t.Run("PWR-V0-003 a skip the test did not expect is NOT_RUN, an expected skip is an outcome", func(t *testing.T) {
		skipped := cartTest(false, fxResult{status: "skipped"})
		skipped.status = "skipped"
		notRun := reportJSON(t, "1.63.0", id, skipped)
		intended := mutate(t, notRun, func(doc map[string]any) {
			test := doc["suites"].([]any)[0].(map[string]any)["suites"].([]any)[0].(map[string]any)["specs"].([]any)[0].(map[string]any)["tests"].([]any)[0].(map[string]any)
			test["expectedStatus"] = "skipped"
		})
		for _, c := range []struct {
			raw             []byte
			verdict, reason string
		}{{notRun, Insufficient, ReasonNotRun}, {intended, Deterministic, ""}} {
			e := Stability([]*Run{mustParse(t, c.raw), mustParse(t, c.raw), mustParse(t, c.raw)}, 3).Tests
			if len(e) != 1 || e[0].Verdict != c.verdict || e[0].Reason != c.reason {
				t.Fatalf("entries = %+v", e)
			}
		}
		if e := onlyEntry(t, Compare(loaded(t, notRun), loaded(t, intended))); e.Verdict != Changed || e.Baseline.Outcomes[0].Outcome != "notRun" {
			t.Fatalf("compare = %+v", e)
		}
	})
	t.Run("PWR-V0-006 an ambiguous identity gets no verdict in either order", func(t *testing.T) {
		failing := cartTest(false, fail("Error: expect(locator).toBeVisible() failed", fixtureRoot+"/cart.spec.ts", 9))
		cand := loaded(t, reportJSON(t, "1.63.0", id, cartTest(false, pass())))
		for _, order := range [][]fxTest{{cartTest(false, pass()), failing}, {failing, cartTest(false, pass())}} {
			r := Compare(loaded(t, reportJSON(t, "1.63.0", id, order...)), cand)
			e := onlyEntry(t, r)
			if e.Verdict != AmbiguousTest || e.Baseline != nil || e.Candidate != nil || r.Summary != (CompareSummary{Ambiguous: 1}) ||
				!reflect.DeepEqual(r.Reasons, []string{ReasonAmbiguous}) {
				t.Fatalf("report = %+v", r)
			}
		}
	})
	t.Run("PWR-V0-001 PWR-V0-007 screening never hides a support difference or merges pools", func(t *testing.T) {
		base := reportJSON(t, "1.63.0", id, cartTest(false, fail("Error: expect(locator).toBeVisible() failed", fixtureRoot+"/pages/token=abc123.ts", 20)))
		cand := reportJSON(t, "1.63.0", id, cartTest(false, fail("Error: expect(locator).toBeVisible() failed", fixtureRoot+"/pages/token=def456.ts", 20)))
		r := Compare(loaded(t, base), loaded(t, cand))
		out, _ := json.Marshal(r)
		if e := onlyEntry(t, r); !e.SupportDifference || !reflect.DeepEqual(r.Reasons, []string{ReasonSupportDiffers}) || strings.Contains(string(out), "abc123") || strings.Contains(string(out), "def456") {
			t.Fatalf("compare = %s", out)
		}
		secret := cartTest(false, pass())
		secret.project = "token=eee555"
		runs := []*Run{}
		for _, app := range []string{"token=aaa111", "token=ddd444"} {
			declared := identity(app, "token=bbb222", map[string]string{"cart.spec.ts": "token=ccc333"})
			runs = append(runs, mustParse(t, mutate(t, reportJSON(t, "1.63.0", declared, secret), func(doc map[string]any) {
				doc["config"].(map[string]any)["version"] = "1.63.0 token=fff666"
			})))
		}
		st := Stability(runs, 2)
		raw, _ := json.Marshal(st)
		if len(st.Tests) != 2 {
			t.Fatalf("pools merged: %s", raw)
		}
		for _, s := range []string{"aaa111", "bbb222", "ccc333", "ddd444", "eee555", "fff666"} {
			if strings.Contains(string(raw), s) {
				t.Fatalf("%s kept: %s", s, raw)
			}
		}
	})
}
