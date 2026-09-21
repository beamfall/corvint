package doccorpus

import (
	"context"
	"testing"
)

func legacyFixture(r *BehaviorRegistry) {
	a := r.Tests[0].Assertions[0]
	r.Legacy = []BehaviorLegacyCase{{ID: "old-count", Suite: "legacy summary", Case: "count", Evidence: r.Tests[0].Evidence, State: "executable", Fixtures: []string{"one-record"}, Roles: []string{"reader"}, Criteria: []BehaviorLegacyCriterion{{ID: "old-visible", Evidence: r.Tests[0].Evidence, Matcher: a.Matcher, Locator: a.Locator, Value: a.Value}}, Runtime: &BehaviorRuntime{}}}
	r.Tests[0].Fixtures, r.Tests[0].Roles = []string{"one-record"}, []string{"reader"}
	r.Tests[0].Legacy = []BehaviorLegacyMapping{{Criterion: a.Criterion, LegacyCase: "old-count", LegacyCriterion: "old-visible", Relation: "same", Review: a.Annotation}}
}

func TestBehaviorLegacyParity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*BehaviorRegistry)
		parity bool
		gap    string
	}{
		{"same qualified baseline", func(*BehaviorRegistry) {}, true, ""},
		{"stronger retains observable", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Relation = "stronger" }, true, ""},
		{"consolidated target drops branch", func(r *BehaviorRegistry) {
			extra := r.Legacy[0].Criteria[0]
			extra.ID = "old-error-branch"
			extra.Value = "error"
			r.Legacy[0].Criteria = append(r.Legacy[0].Criteria, extra)
		}, false, "missing-legacy-criterion"},
		{"same name different success", func(r *BehaviorRegistry) { r.Legacy[0].Criteria[0].Value = "2" }, false, "legacy-criterion-contradiction"},
		{"stronger cannot replace success", func(r *BehaviorRegistry) {
			r.Tests[0].Legacy[0].Relation = "stronger"
			r.Legacy[0].Criteria[0].Locator = "#other"
		}, false, "legacy-criterion-contradiction"},
		{"disabled baseline", func(r *BehaviorRegistry) { r.Legacy[0].State = "disabled" }, false, "legacy-runtime-unknown"},
		{"source review not runtime", func(r *BehaviorRegistry) { r.Legacy[0].Runtime = nil }, false, "legacy-runtime-unknown"},
		{"target unmapped", func(r *BehaviorRegistry) { r.Tests[0].Legacy = nil }, false, "unreviewed-legacy-join"},
		{"unreviewed mapping", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Review.Kind = "declared" }, false, "legacy-criterion-contradiction"},
		{"wrong legacy identity", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].LegacyCase = "same-name-other-case" }, false, "legacy-criterion-contradiction"},
		{"different preconditions", func(r *BehaviorRegistry) { r.Legacy[0].Roles = []string{"admin"} }, false, "legacy-precondition-mismatch"},
		{"new", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Relation = "new" }, false, "legacy-nonparity"},
		{"obsolete", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Relation = "obsolete" }, false, "legacy-nonparity"},
		{"blocked", func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Relation = "blocked" }, false, "legacy-nonparity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m := behaviorFixtureWithRun(t, func(r *BehaviorRegistry) { legacyFixture(r); tc.edit(r) }, true)
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			report := a.BehaviorContracts[0]
			if len(report.VerifiedTests) != 1 {
				t.Fatal("legacy classification changed target journey verification")
			}
			if (len(report.LegacyRuntimeParity) == 1) != tc.parity {
				t.Fatalf("parity=%v gaps=%+v", report.LegacyRuntimeParity, a.Gaps)
			}
			if tc.gap != "" {
				found := false
				for _, gap := range a.Gaps {
					if gap.Kind == tc.gap {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.gap, a.Gaps)
				}
			}
			data, err := Encode(a)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Open(context.Background(), root, data); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBehaviorLegacyRuntimeIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*BehaviorLegacyRun)
	}{
		{"source digest", func(r *BehaviorLegacyRun) { r.SourceSHA256 = "wrong" }},
		{"receipt", func(r *BehaviorLegacyRun) { r.RunSHA256 = "wrong" }},
		{"case", func(r *BehaviorLegacyRun) { r.CaseID = "wrong" }},
		{"project", func(r *BehaviorLegacyRun) { r.Project = "webkit" }},
		{"test", func(r *BehaviorLegacyRun) { r.TestID = "wrong" }},
		{"criterion", func(r *BehaviorLegacyRun) { r.Criteria[0].Value = "wrong" }},
		{"retry", func(r *BehaviorLegacyRun) { r.Retry = 1 }},
		{"cleanup", func(r *BehaviorLegacyRun) { r.Cleanup = "unknown" }},
		{"role", func(r *BehaviorLegacyRun) { r.Roles = []string{"admin"} }},
		{"revision", func(r *BehaviorLegacyRun) { r.Revisions.Docs.Revision = "wrong" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m := behaviorFixtureWithAllRuns(t, legacyFixture, true, tc.edit)
			a, err := Build(context.Background(), root, m)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.BehaviorContracts[0].LegacyRuntimeParity) != 0 {
				t.Fatal("mismatched legacy witness became parity")
			}
			if len(a.BehaviorContracts[0].VerifiedTests) != 1 {
				t.Fatal("legacy mismatch changed target verification")
			}
		})
	}
}

func TestBehaviorLegacyReverseParityCoverage(t *testing.T) {
	root, m := behaviorFixtureWithRun(t, func(r *BehaviorRegistry) {
		legacyFixture(r)
		criterion := r.Legacy[0].Criteria[0]
		criterion.ID, criterion.Value = "old-error", "error"
		r.Legacy[0].Criteria = append(r.Legacy[0].Criteria, criterion)
		other := r.Tests[0]
		other.ID, other.Title = "other-target", "error branch"
		other.Criteria = []string{"error-visible"}
		assertion := other.Assertions[0]
		assertion.ID, assertion.Criterion, assertion.Value = "error-visible", "error-visible", "wrong-success"
		other.Assertions = []BehaviorAssertion{assertion}
		mapping := other.Legacy[0]
		mapping.Criterion, mapping.LegacyCriterion = "error-visible", "old-error"
		other.Legacy = []BehaviorLegacyMapping{mapping}
		r.Tests = append(r.Tests, other)
		r.Flows[0].Criteria = append(r.Flows[0].Criteria, "error-visible")
		r.Flows[0].Tests = append(r.Flows[0].Tests, other.ID)
	}, true)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.BehaviorContracts[0].VerifiedTests) != 1 {
		t.Fatal("first target must retain its verified journey")
	}
	if len(a.BehaviorContracts[0].LegacyRuntimeParity) != 0 {
		t.Fatal("invalid second mapping hid dropped legacy branch and promoted first target parity")
	}
	contradiction, missing := false, false
	for _, gap := range a.Gaps {
		if gap.Subject == "other-target" && gap.Kind == "legacy-criterion-contradiction" {
			contradiction = true
		}
		if gap.Subject == "old-count" && gap.Kind == "missing-legacy-criterion" {
			missing = true
		}
	}
	if !contradiction || !missing {
		t.Fatalf("missing explicit parity gaps: %+v", a.Gaps)
	}
}
