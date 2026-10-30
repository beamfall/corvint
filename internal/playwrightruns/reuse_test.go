package playwrightruns

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// reuseDecl is a corvintRunIdentity declaration with a test-suite revision.
func reuseDecl(app, testRev, config, blob string) map[string]any {
	d := identity(app, config, map[string]string{"cart.spec.ts": blob})
	if testRev != "" {
		d["testRevision"] = testRev
	}
	return d
}

func cartRun(t *testing.T, version string, decl map[string]any, status string, results ...fxResult) []byte {
	t.Helper()
	return reportJSON(t, version, decl, fxTest{file: "cart.spec.ts", title: "adds item", status: status, results: results})
}

func base(t *testing.T) []byte {
	return cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "expected", pass())
}

func onlyReuse(t *testing.T, r ReuseReport) ReuseEntry {
	t.Helper()
	if len(r.Tests) != 1 {
		t.Fatalf("want one row, got %+v", r.Tests)
	}
	return r.Tests[0]
}

func wantRow(t *testing.T, e ReuseEntry, decision, reason string) {
	t.Helper()
	if e.Decision != decision || e.Reason != reason {
		t.Fatalf("got %s/%s (%+v), want %s/%s", e.Decision, e.Reason, e, decision, reason)
	}
}

func reusable(t *testing.T, selection []byte, fresh FreshnessCheck, priors ...[]byte) ReuseEntry {
	t.Helper()
	return onlyReuse(t, Reusable(mustParse(t, selection), runsOf(t, priors...), fresh))
}

func TestReusableDecisions(t *testing.T) {
	t.Run("PWR-V0-033 PWR-V0-034 a qualified pass at the same fingerprint is EXACT_PASS", func(t *testing.T) {
		e := reusable(t, base(t), nil, cartRun(t, "1.63.0", reuseDecl("app-0", "tests-1", "cfg-1", "blob-1"), "expected", pass()), base(t))
		wantRow(t, e, DecisionReusable, ReuseExactPass)
		eq(t, "priorRuns", e.PriorRuns, []int{2})
	})
	t.Run("PWR-V0-035 a change to any fingerprint input is MUST_RUN naming it", func(t *testing.T) {
		cases := map[string][]byte{
			FieldApplicationRevision: cartRun(t, "1.63.0", reuseDecl("app-0", "tests-1", "cfg-1", "blob-1"), "expected", pass()),
			FieldTestRevision:        cartRun(t, "1.63.0", reuseDecl("app-1", "tests-0", "cfg-1", "blob-1"), "expected", pass()),
			FieldTestFileBlob:        cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-0"), "expected", pass()),
			FieldConfigDigest:        cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-0", "blob-1"), "expected", pass()),
			FieldPlaywrightVersion:   cartRun(t, "1.62.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "expected", pass()),
		}
		for field, prior := range cases {
			e := reusable(t, base(t), nil, prior)
			wantRow(t, e, DecisionMustRun, ReasonFingerprintChanged)
			eq(t, field, e.Fields, []string{field})
		}
		// The project is part of the test identity: another project's pass is no prior result.
		other := reportJSON(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), fxTest{project: "firefox", file: "cart.spec.ts", title: "adds item", status: "expected", results: []fxResult{pass()}})
		wantRow(t, reusable(t, base(t), nil, other), DecisionMustRun, ReasonNoPriorResult)
	})
	t.Run("PWR-V0-034 missing or unqualified evidence fails closed", func(t *testing.T) {
		undeclared := cartRun(t, "1.63.0", nil, "expected", pass())
		e := reusable(t, undeclared, nil, base(t))
		wantRow(t, e, DecisionMustRun, ReasonInputsMissing)
		eq(t, "fields", e.Fields, []string{FieldApplicationRevision, FieldTestRevision, FieldTestFileBlob, FieldConfigDigest})
		e = reusable(t, cartRun(t, "1.63.0", reuseDecl("app-1", "", "cfg-1", "blob-1"), "expected", pass()), nil, base(t))
		wantRow(t, e, DecisionMustRun, ReasonInputsMissing)
		eq(t, "fields", e.Fields, []string{FieldTestRevision})
		e = reusable(t, base(t), nil, undeclared)
		wantRow(t, e, DecisionMustRun, ReasonInputsMissing)
		eq(t, "detail", e.Detail, "prior")
		wantRow(t, reusable(t, base(t), nil), DecisionMustRun, ReasonNoPriorResult)

		failed := cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "unexpected", fail("Error: boom", fixtureRoot+"/cart.spec.ts", 4))
		e = reusable(t, base(t), nil, failed)
		wantRow(t, e, DecisionMustRun, ReasonPriorNotPassed)
		eq(t, "detail", e.Detail, "failed")
		e = reusable(t, base(t), nil, base(t), failed)
		wantRow(t, e, DecisionMustRun, ReasonPriorNotPassed)
		eq(t, "priorRuns", e.PriorRuns, []int{2})
		flaky := cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "flaky", fail("Error: once", "", 0), fxResult{status: "passed", retry: 1})
		eq(t, "flaky", reusable(t, base(t), nil, flaky).Detail, "flaky")
		unexpectedPass := cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "unexpected", pass())
		wantRow(t, reusable(t, base(t), nil, unexpectedPass), DecisionMustRun, ReasonPriorNotPassed)

		interrupted := cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "unexpected", fxResult{status: "interrupted"})
		e = reusable(t, base(t), nil, interrupted)
		wantRow(t, e, DecisionMustRun, ReasonPriorNotQualified)
		eq(t, "detail", e.Detail, ReasonInterrupted)
		withErrors := mutate(t, base(t), func(doc map[string]any) { doc["errors"] = []any{map[string]any{"message": "load failed"}} })
		eq(t, "report errors", reusable(t, base(t), nil, withErrors).Detail, ReasonReportHasErrors)
		shard := mutate(t, base(t), func(doc map[string]any) {
			doc["config"].(map[string]any)["shard"] = map[string]any{"current": 1, "total": 2}
		})
		eq(t, "refused shard set", reusable(t, base(t), nil, shard).Detail, ReasonShardSetIncomplete)
	})
	t.Run("PWR-V0-034 an ambiguous selected identity is MUST_RUN", func(t *testing.T) {
		twice := reportJSON(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"),
			fxTest{file: "cart.spec.ts", title: "adds item", status: "expected", results: []fxResult{pass()}},
			fxTest{file: "cart.spec.ts", title: "adds item", status: "expected", results: []fxResult{pass()}})
		twice = mutate(t, twice, func(doc map[string]any) {
			top := doc["suites"].([]any)[0].(map[string]any)
			specs := top["specs"].([]any)
			specs[1].(map[string]any)["line"] = 9
		})
		wantRow(t, reusable(t, twice, nil, base(t)), DecisionMustRun, ReasonAmbiguous)
	})
}

// stubFresh answers every pair with one verdict and records the pairs asked.
type stubFresh struct {
	verdict FreshVerdict
	asked   []string
}

func (s *stubFresh) check(oldRev, newRev, file string) FreshVerdict {
	s.asked = append(s.asked, fmt.Sprintf("%s>%s:%s", oldRev, newRev, file))
	return s.verdict
}

func TestReusableFreshness(t *testing.T) {
	appMoved := cartRun(t, "1.63.0", reuseDecl("app-0", "tests-1", "cfg-1", "blob-1"), "expected", pass())
	t.Run("PWR-V0-036 a FRESH revision pair admits FRESH_PASS", func(t *testing.T) {
		s := &stubFresh{verdict: FreshVerdict{Verdict: "FRESH"}}
		e := reusable(t, base(t), s.check, appMoved)
		wantRow(t, e, DecisionReusable, ReuseFreshPass)
		eq(t, "fields", e.Fields, []string{FieldApplicationRevision})
		eq(t, "asked", s.asked, []string{"app-0>app-1:cart.spec.ts"})
	})
	t.Run("PWR-V0-036 STALE names the deciding paths, capped", func(t *testing.T) {
		paths := []string{}
		for i := 0; i < MaxRowPaths+3; i++ {
			paths = append(paths, fmt.Sprintf("src/p%02d.ts", i))
		}
		s := &stubFresh{verdict: FreshVerdict{Verdict: "STALE", Paths: paths, Truncated: 2}}
		e := reusable(t, base(t), s.check, appMoved)
		wantRow(t, e, DecisionMustRun, ReasonStale)
		eq(t, "paths", e.Paths, paths[:MaxRowPaths])
		eq(t, "truncated", e.PathsTruncated, 5)
	})
	t.Run("PWR-V0-036 an unresolved freshness is MUST_RUN, never reuse", func(t *testing.T) {
		s := &stubFresh{verdict: FreshVerdict{Verdict: "UNRESOLVED", Reason: "REVISION_UNRESOLVED"}}
		e := reusable(t, base(t), s.check, appMoved)
		wantRow(t, e, DecisionMustRun, ReasonFreshnessUnresolved)
		eq(t, "detail", e.Detail, "REVISION_UNRESOLVED")
	})
	t.Run("PWR-V0-036 freshness never overrides a non-revision change or a failure", func(t *testing.T) {
		s := &stubFresh{verdict: FreshVerdict{Verdict: "FRESH"}}
		blobMoved := cartRun(t, "1.63.0", reuseDecl("app-0", "tests-1", "cfg-1", "blob-0"), "expected", pass())
		wantRow(t, reusable(t, base(t), s.check, blobMoved), DecisionMustRun, ReasonFingerprintChanged)
		eq(t, "asked", len(s.asked), 0)
		failed := cartRun(t, "1.63.0", reuseDecl("app-0", "tests-1", "cfg-1", "blob-1"), "unexpected", fail("Error: boom", "", 0))
		wantRow(t, reusable(t, base(t), s.check, appMoved, failed), DecisionMustRun, ReasonPriorNotPassed)
		// Without the freshness condition a revision change stays FINGERPRINT_CHANGED.
		wantRow(t, reusable(t, base(t), nil, appMoved), DecisionMustRun, ReasonFingerprintChanged)
	})
	t.Run("PWR-V0-036 both revision pairs must be FRESH", func(t *testing.T) {
		both := cartRun(t, "1.63.0", reuseDecl("app-0", "tests-0", "cfg-1", "blob-1"), "expected", pass())
		check := func(oldRev, _, _ string) FreshVerdict {
			if oldRev == "tests-0" {
				return FreshVerdict{Verdict: "STALE", Paths: []string{"e2e/support/login.ts"}}
			}
			return FreshVerdict{Verdict: "FRESH"}
		}
		e := reusable(t, base(t), check, both)
		wantRow(t, e, DecisionMustRun, ReasonStale)
		eq(t, "paths", e.Paths, []string{"e2e/support/login.ts"})
		eq(t, "fields", e.Fields, []string{FieldApplicationRevision, FieldTestRevision})
	})
}

func TestReusableSummaryForm(t *testing.T) {
	t.Run("PWR-V0-040 the summary form keeps counts and drops rows", func(t *testing.T) {
		selection := reportJSON(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"),
			fxTest{file: "cart.spec.ts", title: "adds item", status: "expected", results: []fxResult{pass()}},
			fxTest{file: "cart.spec.ts", title: "removes item", status: "expected", results: []fxResult{pass()}})
		other := cartRun(t, "1.63.0", reuseDecl("app-9", "tests-1", "cfg-1", "blob-1"), "expected", pass())
		shard := mutate(t, other, func(doc map[string]any) {
			doc["config"].(map[string]any)["shard"] = map[string]any{"current": 1, "total": 2}
		})
		r := Reusable(mustParse(t, selection), runsOf(t, base(t), shard), nil)
		if len(r.Priors) == 0 || len(r.ShardSets) == 0 {
			t.Fatalf("fixture lacks per-run rows: %+v", r)
		}
		eq(t, "summary", r.Summary, ReuseSummary{Selected: 2, Reusable: 1, MustRun: 1, ByReason: map[string]int{ReuseExactPass: 1, ReasonNoPriorResult: 1}, Freshness: "DISABLED"})
		c := r.Compact()
		if c.Form != FormSummary || c.Tests != nil || c.Priors != nil || c.ShardSets != nil || !reflect.DeepEqual(c.Summary, r.Summary) || r.Form != FormFull {
			t.Fatalf("compact %+v", c)
		}
		raw, err := json.Marshal(c)
		if err != nil || strings.Contains(string(raw), `"priors"`) || strings.Contains(string(raw), `"shardSets"`) || strings.Contains(string(raw), `"tests"`) {
			t.Fatalf("summary keeps rows: %s %v", raw, err)
		}
	})
}

// timed sets a report's stats.startTime and run category.
func timed(t *testing.T, raw []byte, start, category string) []byte {
	t.Helper()
	return mutate(t, raw, func(doc map[string]any) {
		if start != "" {
			doc["stats"] = map[string]any{"startTime": start, "duration": 100}
		}
		if category != "" {
			doc["config"].(map[string]any)["metadata"].(map[string]any)["corvintRunCategory"] = category
		}
	})
}

func TestRedundancy(t *testing.T) {
	appMoved := cartRun(t, "1.63.0", reuseDecl("app-2", "tests-1", "cfg-1", "blob-1"), "expected", pass())
	failed := cartRun(t, "1.63.0", reuseDecl("app-1", "tests-1", "cfg-1", "blob-1"), "unexpected", fail("Error: boom", "", 0))
	undeclared := cartRun(t, "1.63.0", nil, "expected", pass())
	t.Run("PWR-V0-037 exact and same-file-content re-runs by category", func(t *testing.T) {
		r := Redundancy(runsOf(t,
			timed(t, base(t), "2026-10-01T10:00:00Z", "ci"),
			timed(t, base(t), "2026-10-01T11:00:00Z", "ci"),
			timed(t, appMoved, "2026-10-01T12:00:00Z", "local"),
			timed(t, base(t), "2026-10-01T11:00:00Z", "local"), // ties with run 2: not earlier than it
			timed(t, undeclared, "2026-10-01T13:00:00Z", ""),
		), nil)
		cats := map[string]CategoryTally{}
		for _, c := range r.Categories {
			cats[c.Category] = c
		}
		ci, local, und := cats["ci"], cats["local"], cats[CategoryUndeclared]
		eq(t, "ci", []any{ci.Runs, ci.Tests, ci.TotalMS, ci.ExactRerunTests, ci.ExactRerunMS, ci.ExactRerunShare}, []any{2, 2, int64(24), 1, int64(12), 0.5})
		eq(t, "local", []any{local.Runs, local.ExactRerunTests, local.SameFileContentRerunTests, local.SameFileContentRerunMS, local.SameFileContentRerunShare}, []any{2, 1, 1, int64(12), 0.5})
		eq(t, "undeclared", []any{und.UnfingerprintedTests, und.UnfingerprintedMS, und.ExactRerunTests}, []any{1, int64(12), 0})
		eq(t, "total", []any{r.Total.Runs, r.Total.Tests, r.Total.ExactRerunTests, r.Total.SameFileContentRerunTests, r.Total.TotalMinutes}, []any{5, 5, 2, 1, 0.0})
	})
	t.Run("PWR-V0-037 only a strictly earlier qualified pass counts", func(t *testing.T) {
		r := Redundancy(runsOf(t,
			timed(t, base(t), "2026-10-01T10:00:00Z", ""),
			timed(t, base(t), "2026-10-01T10:00:00Z", ""),
		), nil)
		eq(t, "tie", r.Total.ExactRerunTests, 0)
		r = Redundancy(runsOf(t, timed(t, failed, "2026-10-01T09:00:00Z", ""), timed(t, base(t), "2026-10-01T10:00:00Z", "")), nil)
		eq(t, "after failure", r.Total.ExactRerunTests, 0)
	})
	t.Run("PWR-V0-037 since keeps earlier runs as history and start-less runs are excluded", func(t *testing.T) {
		since := time.Date(2026, 10, 1, 10, 30, 0, 0, time.UTC)
		r := Redundancy(runsOf(t,
			timed(t, base(t), "2026-10-01T10:00:00Z", ""),
			timed(t, base(t), "2026-10-01T11:00:00Z", ""),
			base(t),
		), &since)
		eq(t, "roles", []string{r.Runs[0].Role + "/" + r.Runs[0].Reason, r.Runs[1].Role, r.Runs[2].Role + "/" + r.Runs[2].Reason},
			[]string{RunHistory + "/" + ReasonBeforeSince, RunCounted, RunExcluded + "/" + ReasonStartMissing})
		eq(t, "total", []any{r.Total.Runs, r.Total.Tests, r.Total.ExactRerunTests, r.Total.ExactRerunShare}, []any{1, 1, 1, 1.0})
		eq(t, "since", r.Since, "2026-10-01T10:30:00Z")
		c := r.Compact()
		if c.Form != FormSummary || c.Runs != nil || c.Categories != nil || c.Total != r.Total {
			t.Fatalf("compact %+v", c)
		}
	})
	t.Run("PWR-V0-037 a refused shard set is excluded", func(t *testing.T) {
		shard := mutate(t, timed(t, base(t), "2026-10-01T10:00:00Z", ""), func(doc map[string]any) {
			doc["config"].(map[string]any)["shard"] = map[string]any{"current": 1, "total": 2}
		})
		r := Redundancy(runsOf(t, shard), nil)
		eq(t, "role", r.Runs[0].Role+"/"+r.Runs[0].Reason, RunExcluded+"/"+ReasonShardRefused)
		eq(t, "counted", r.Total.Runs, 0)
	})
}

func TestRunFactsDeclaration(t *testing.T) {
	t.Run("PWR-V0-033 test revision, root path, category and start time are read, never inferred", func(t *testing.T) {
		decl := reuseDecl("app-1", "tests-1", "cfg-1", "blob-1")
		decl["testRootPath"] = "e2e"
		run := mustParse(t, timed(t, cartRun(t, "1.63.0", decl, "expected", pass()), "2026-10-01T10:00:00.5+02:00", "nightly"))
		eq(t, "facts", []any{run.TestRevision, run.TestRootPath, run.Category, run.RootDir, run.HasStartTime, run.StartTime.Format(time.RFC3339Nano)},
			[]any{"tests-1", "e2e", "nightly", fixtureRoot, true, "2026-10-01T08:00:00.5Z"})
		bare := mustParse(t, timed(t, cartRun(t, "1.63.0", nil, "expected", pass()), "yesterday", ""))
		eq(t, "bare", []any{bare.TestRevision, bare.Category, bare.HasStartTime}, []any{"", "", false})
		odd := mustParse(t, mutate(t, base(t), func(doc map[string]any) {
			doc["config"].(map[string]any)["metadata"].(map[string]any)["corvintRunCategory"] = 7
		}))
		eq(t, "malformed category keeps the revision", []any{odd.Category, odd.TestRevision}, []any{"", "tests-1"})
	})
}

func TestIllTypedStartTimeIsAbsent(t *testing.T) {
	t.Run("PWR-V0-033 PWR-V0-037 an ill-typed stats.startTime is absent, not a refused report", func(t *testing.T) {
		for _, stats := range []any{map[string]any{"startTime": 123}, map[string]any{"startTime": nil}, "not-an-object", 7} {
			raw := mutate(t, base(t), func(doc map[string]any) { doc["stats"] = stats })
			run := mustParse(t, raw)
			eq(t, "start time", run.HasStartTime, false)
			r := Redundancy([]*Run{run}, nil)
			eq(t, "excluded", r.Runs[0].Role+"/"+r.Runs[0].Reason, RunExcluded+"/"+ReasonStartMissing)
			wantRow(t, reusable(t, raw, nil, base(t)), DecisionReusable, ReuseExactPass)
			wantRow(t, reusable(t, base(t), nil, raw), DecisionReusable, ReuseExactPass)
			got, want := Stability(runsOf(t, raw, raw), 2), Stability(runsOf(t, base(t), base(t)), 2)
			if !reflect.DeepEqual(got.Tests, want.Tests) || !reflect.DeepEqual(got.Summary, want.Summary) {
				t.Fatalf("stability regressed for %v: %+v", stats, got)
			}
			loaded := func(r *Run) []Loaded { return []Loaded{{Run: r}} }
			cmp, ref := Compare(loaded(mustParse(t, base(t))), loaded(run)), Compare(loaded(mustParse(t, base(t))), loaded(mustParse(t, base(t))))
			if cmp.Verdict != ref.Verdict || !reflect.DeepEqual(cmp.Tests, ref.Tests) {
				t.Fatalf("compare regressed for %v: %+v", stats, cmp)
			}
		}
	})
}
