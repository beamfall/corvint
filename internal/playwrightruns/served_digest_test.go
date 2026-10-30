package playwrightruns

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// served is a declaration whose run served the named build (GitHub #1873).
func served(app, digest string) map[string]any {
	d := identity(app, "cfg-1", blobsA)
	if digest != "" {
		d["servedArtifactDigest"] = digest
	}
	return d
}

func servedRun(t *testing.T, app, digest string, results ...fxResult) []byte {
	t.Helper()
	return reportJSON(t, "1.63.0", served(app, digest), cartTest(false, results...))
}

func TestServedDigestStability(t *testing.T) {
	t.Run("PWR-V0-055 runs pool only at one served digest and a disagreeing build is named", func(t *testing.T) {
		deployed := servedRun(t, "app-2", "sha256:new", pass())
		stale := servedRun(t, "app-2", "sha256:old", pass())
		r := Stability(runsOf(t, deployed, deployed, stale), 2)
		if len(r.Tests) != 2 {
			t.Fatalf("want two pools, got %+v", r.Tests)
		}
		for _, e := range r.Tests {
			if e.Inputs == nil || e.Inputs.ServedArtifactDigest == "" {
				t.Fatalf("pool without served digest: %+v", e)
			}
			if e.Inputs.ServedArtifactDigest == "sha256:old" && (e.Verdict != Insufficient || !slices.Equal(e.Runs, []int{3})) {
				t.Fatalf("stale pool = %+v", e)
			}
		}
		for i, want := range []string{ServedDigestDisagrees, ServedDigestDisagrees, ServedDigestDisagrees} {
			if r.Runs[i].ServedDigest != want {
				t.Fatalf("run %d servedDigest = %q", i+1, r.Runs[i].ServedDigest)
			}
		}
		if r.Summary.ServedDigestDisagreeingRuns != 3 || r.Summary.ServedDigestUnknownRuns != 0 {
			t.Fatalf("summary = %+v", r.Summary)
		}
	})
	t.Run("PWR-V0-055 one build per revision is DECLARED and pools", func(t *testing.T) {
		r := Stability(runsOf(t, servedRun(t, "app-1", "sha256:a", pass()), servedRun(t, "app-1", "sha256:a", pass()),
			servedRun(t, "app-2", "sha256:b", pass())), 2)
		eq(t, "states", []string{r.Runs[0].ServedDigest, r.Runs[1].ServedDigest, r.Runs[2].ServedDigest}, []string{ServedDigestDeclared, ServedDigestDeclared, ServedDigestDeclared})
		if r.Summary.Deterministic != 1 || r.Summary.ServedDigestDisagreeingRuns != 0 {
			t.Fatalf("summary = %+v", r.Summary)
		}
	})
	t.Run("PWR-V0-055 a run without a served digest states it unknown and never pools with a declared one", func(t *testing.T) {
		r := Stability(runsOf(t, servedRun(t, "app-1", "", pass()), servedRun(t, "app-1", "sha256:a", pass()), reportJSON(t, "1.63.0", nil, cartTest(true, pass()))), 2)
		eq(t, "states", []string{r.Runs[0].ServedDigest, r.Runs[1].ServedDigest, r.Runs[2].ServedDigest}, []string{ServedDigestUnknown, ServedDigestDeclared, ServedDigestUnknown})
		if r.Summary.ServedDigestUnknownRuns != 2 || r.Summary.Deterministic != 0 || len(r.Tests) != 2 {
			t.Fatalf("report = %+v", r)
		}
	})
}

func TestServedDigestCompare(t *testing.T) {
	assertion := "Error: expect(locator).toHaveText(expected) failed"
	t.Run("PWR-V0-056 a candidate that served the baseline build is UNDEPLOYED, not PARITY", func(t *testing.T) {
		base := servedRun(t, "app-1", "sha256:a", pass())
		cand := servedRun(t, "app-2", "sha256:a", pass())
		r := Compare(loaded(t, base, base), loaded(t, cand, cand))
		if r.Verdict != Undeployed || !slices.Equal(r.Reasons, []string{ReasonServedDigestUnchanged}) || len(r.Unknowns) != 0 || r.Summary.Same != 1 {
			t.Fatalf("report = %+v", r)
		}
		if r.Reports[SideCandidate][0].ServedDigest != ServedDigestDeclared {
			t.Fatalf("facts = %+v", r.Reports)
		}
	})
	t.Run("PWR-V0-056 UNDEPLOYED outranks a test difference and keeps its reason", func(t *testing.T) {
		base := servedRun(t, "app-1", "sha256:a", pass())
		cand := servedRun(t, "app-2", "sha256:a", fail(assertion, fixtureRoot+"/cart.spec.ts", 9))
		r := Compare(loaded(t, base), loaded(t, cand))
		if r.Verdict != Undeployed || !slices.Equal(r.Reasons, []string{ReasonServedDigestUnchanged, ReasonTestsChanged}) {
			t.Fatalf("report = %+v", r)
		}
	})
	t.Run("PWR-V0-056 a different build at the same revision is UNDEPLOYED", func(t *testing.T) {
		r := Compare(loaded(t, servedRun(t, "app-1", "sha256:a", pass())), loaded(t, servedRun(t, "app-1", "sha256:b", pass())))
		if r.Verdict != Undeployed || !slices.Equal(r.Reasons, []string{ReasonServedDigestDiffersAtRev}) {
			t.Fatalf("report = %+v", r)
		}
	})
	t.Run("PWR-V0-056 a deployed candidate compares as before", func(t *testing.T) {
		r := Compare(loaded(t, servedRun(t, "app-1", "sha256:a", pass())), loaded(t, servedRun(t, "app-2", "sha256:b", pass())))
		if r.Verdict != Parity || len(r.Reasons) != 0 || len(r.Unknowns) != 0 {
			t.Fatalf("report = %+v", r)
		}
	})
	t.Run("PWR-V0-056 differing builds within a side are INCOMPARABLE", func(t *testing.T) {
		r := Compare(loaded(t, servedRun(t, "app-1", "sha256:a", pass()), servedRun(t, "app-1", "sha256:b", pass())),
			loaded(t, servedRun(t, "app-2", "sha256:c", pass())))
		if r.Verdict != Incomparable || !slices.Equal(r.Reasons, []string{ReasonInputsDifferInSide}) {
			t.Fatalf("report = %+v", r)
		}
	})
	t.Run("PWR-V0-056 a report without a served digest adds the unknown and never UNDEPLOYED", func(t *testing.T) {
		r := Compare(loaded(t, servedRun(t, "app-1", "sha256:a", pass())), loaded(t, servedRun(t, "app-2", "", pass())))
		if r.Verdict != Parity || !slices.Equal(r.Unknowns, []string{UnknownServedDigest}) || r.Reports[SideCandidate][0].ServedDigest != ServedDigestUnknown {
			t.Fatalf("report = %+v", r)
		}
	})
}

func TestServedDigestReuseAndShards(t *testing.T) {
	t.Run("PWR-V0-057 a prior pass at another served build is FINGERPRINT_CHANGED", func(t *testing.T) {
		decl := func(digest string) map[string]any {
			d := reuseDecl("app-1", "tests-1", "cfg-1", "blob-1")
			if digest != "" {
				d["servedArtifactDigest"] = digest
			}
			return d
		}
		selection := cartRun(t, "1.63.0", decl("sha256:new"), "expected", pass())
		for _, prior := range []string{"sha256:old", ""} {
			e := reusable(t, selection, nil, cartRun(t, "1.63.0", decl(prior), "expected", pass()))
			wantRow(t, e, DecisionMustRun, ReasonFingerprintChanged)
			eq(t, "fields", e.Fields, []string{FieldServedArtifactDigest})
		}
		wantRow(t, reusable(t, selection, nil, cartRun(t, "1.63.0", decl("sha256:new"), "expected", pass())), DecisionReusable, ReuseExactPass)
	})
	t.Run("PWR-V0-057 reusable run summaries state the served digest, never empty", func(t *testing.T) {
		decl := reuseDecl("app-1", "tests-1", "cfg-1", "blob-1")
		withDigest := reuseDecl("app-1", "tests-1", "cfg-1", "blob-1")
		withDigest["servedArtifactDigest"] = "sha256:new"
		runs := runsOf(t, cartRun(t, "1.63.0", withDigest, "expected", pass()), cartRun(t, "1.63.0", decl, "expected", pass()),
			reportJSON(t, "1.63.0", nil, cartTest(false, pass())))
		r := Reusable(runs[0], runs[1:], nil)
		eq(t, "states", []string{r.Selection.ServedDigest, r.Priors[0].ServedDigest, r.Priors[1].ServedDigest},
			[]string{ServedDigestDeclared, ServedDigestUnknown, ServedDigestUnknown})
		out, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), `"servedDigest":""`) {
			t.Fatalf("empty servedDigest in %s", out)
		}
	})
	t.Run("PWR-V0-057 shards that served different builds do not form one run", func(t *testing.T) {
		a, b := served("app-1", "sha256:a"), served("app-1", "sha256:b")
		if shardGroupKey(&Run{InputsDeclared: true, Inputs: Inputs{ApplicationRevision: "app-1", ServedArtifactDigest: a["servedArtifactDigest"].(string)}, Shard: &Shard{Total: 2}}) ==
			shardGroupKey(&Run{InputsDeclared: true, Inputs: Inputs{ApplicationRevision: "app-1", ServedArtifactDigest: b["servedArtifactDigest"].(string)}, Shard: &Shard{Total: 2}}) {
			t.Fatal("shard group key ignores the served digest")
		}
	})
}
