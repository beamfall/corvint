package doccorpus

import (
	"context"
	"strings"
	"testing"
)

// flowGaps builds an adoption fixture with edit applied, optionally rewrites
// the built artifact in memory, and returns the flow's gaps by kind.
func flowGaps(t *testing.T, edit func(*ProviderRecord), mutate func(*Artifact)) map[string]Gap {
	t.Helper()
	root, manifest := adoptionFixture(t, edit)
	a, err := Build(context.Background(), root, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(a)
	}
	r, err := Query(a, Request{Operation: "gaps"}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Gap{}
	for _, v := range r.Results {
		if g, ok := v.(Gap); ok && g.Subject == "adapter:flow" {
			out[g.Kind] = g
		}
	}
	return out
}

func reviewedJoin(p *ProviderRecord) {
	for i := range p.Relations {
		if p.Relations[i].ID == "adapter:test-join" {
			p.Relations[i].Evidence.Trust = "reviewed"
		}
	}
}

func joinRelation(t *testing.T, a *Artifact) *Relation {
	t.Helper()
	for i := range a.Relations {
		if a.Relations[i].ID == "adapter:test-join" {
			return &a.Relations[i]
		}
	}
	t.Fatal("fixture lost adapter:test-join")
	return nil
}

func TestDCPV1150ProviderReportedReviewYieldsUnauthenticatedGap(t *testing.T) {
	t.Parallel()
	var rel Relation
	gaps := flowGaps(t, reviewedJoin, func(a *Artifact) { rel = *joinRelation(t, a) })
	if rel.Evidence.Trust != "generated" || rel.Evidence.ReportedTrust != "reviewed" {
		t.Fatalf("import must keep provider review attributed: %+v", rel.Evidence)
	}
	if _, ok := gaps["missing-asserting-e2e-tests"]; ok {
		t.Fatal("provider-reported review still yields missing-asserting-e2e-tests")
	}
	g, ok := gaps["asserting-e2e-tests-unauthenticated-review"]
	if !ok || !strings.Contains(g.Reason, "attributed, not authenticated") {
		t.Fatalf("weaker distinct gap missing or unexplained: %+v", gaps)
	}
}

func TestDCPV1151AuthenticatedReviewClearsAssertingGap(t *testing.T) {
	t.Parallel()
	gaps := flowGaps(t, nil, func(a *Artifact) {
		rel := joinRelation(t, a)
		rel.Evidence.Trust, rel.Evidence.ReportedTrust = "reviewed", ""
	})
	for _, kind := range []string{"missing-asserting-e2e-tests", "asserting-e2e-tests-unauthenticated-review"} {
		if _, ok := gaps[kind]; ok {
			t.Fatalf("satisfying asserting link still reports %s", kind)
		}
	}
	if _, ok := gaps["missing-manual-cases"]; !ok {
		t.Fatal("unrelated flow gaps must still be reported")
	}
}

func TestDCPV1152AssertingGapNamesTheFailedCondition(t *testing.T) {
	t.Parallel()
	setLink := func(role, typ, confidence string) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			reviewedJoin(p)
			p.Details["adapter:test-join"].TestLink.Role = role
			p.Details["adapter:test-join"].TestLink.JoinConfidence = confidence
			for i := range p.Relations {
				if p.Relations[i].ID == "adapter:test-join" {
					p.Relations[i].Type = typ
				}
			}
		}
	}
	authenticate := func(a *Artifact) {
		rel := joinRelation(t, a)
		rel.Evidence.Trust, rel.Evidence.ReportedTrust = "reviewed", ""
	}
	cases := []struct {
		name   string
		edit   func(*ProviderRecord)
		mutate func(*Artifact)
		reason string
	}{
		{"declared confidence", setLink("asserting", "asserts", "declared"), nil, "not exact, review not authenticated"},
		{"heuristic confidence", setLink("asserting", "asserts", "heuristic"), authenticate, "not exact;"},
		{"navigating role", setLink("navigating", "navigates", "exact"), authenticate, "not asserting;"},
		{"inferred confidence", nil, func(a *Artifact) {
			authenticate(a)
			a.Details["adapter:test-join"].TestLink.JoinConfidence = "inferred"
		}, "not exact;"},
		{"related role", nil, func(a *Artifact) {
			authenticate(a)
			a.Details["adapter:test-join"].TestLink.Role = "related"
		}, "not asserting;"},
		{"unsupported", nil, func(a *Artifact) {
			authenticate(a)
			joinRelation(t, a).Evidence.State = "unknown"
		}, "not supported;"},
		{"unreviewed", nil, nil, "not reviewed;"},
		{"no link", func(p *ProviderRecord) {
			delete(p.Details, "adapter:test-join")
			p.Relations = p.Relations[:0]
		}, nil, "no test link;"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gaps := flowGaps(t, c.edit, c.mutate)
			g, ok := gaps["missing-asserting-e2e-tests"]
			if !ok || !strings.Contains(g.Reason, c.reason) {
				t.Fatalf("want missing-asserting-e2e-tests naming %q, got %+v", c.reason, gaps)
			}
			if _, ok := gaps["asserting-e2e-tests-unauthenticated-review"]; ok {
				t.Fatal("weaker gap must only replace a link whose sole failure is unauthenticated review")
			}
		})
	}
}
