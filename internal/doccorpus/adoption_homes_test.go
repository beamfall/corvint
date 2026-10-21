package doccorpus

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// suppliedHomes adds one supplied gap, one supplied status and one manual case
// link for the adoption fixture's flow (GitHub #1244).
func suppliedHomes(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	p.Gaps = []SuppliedGap{{ID: "adapter:gap:flow", Subject: "adapter:flow", Kind: "missing-asserting-e2e-tests", Reason: "curated ledger: no asserting end-to-end test", Provider: p.ID, Evidence: ev}}
	p.Claims = append(p.Claims, Claim{ID: "adapter:status:flow", Subject: "adapter:flow", Text: "Flow readiness is blocked", Provider: p.ID, Evidence: ev})
	p.Details["adapter:status:flow"] = RecordDetails{Status: &SuppliedStatusDetails{State: "blocked", Reason: "waiting on the asserting test", Supporting: []string{"adapter:test-join"}}}
	p.Subjects = append(p.Subjects, Subject{ID: "adapter:case:C123", Kind: "manual_case", Name: "C123 count manually", Provider: p.ID, Evidence: ev})
	p.Relations = append(p.Relations, Relation{ID: "adapter:case-link:C123", From: "adapter:case:C123", To: "adapter:flow", Type: "manual_case_for", Provider: p.ID, Evidence: ev})
}

func buildAdoption(t *testing.T, edit func(*ProviderRecord)) *Artifact {
	t.Helper()
	root, m := adoptionFixture(t, edit)
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func gapsFor(t *testing.T, a *Artifact, id string) (computed []Gap, supplied []SuppliedGap) {
	t.Helper()
	r, err := Query(a, Request{Operation: "gaps", ID: id, Limit: 100}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range r.Results {
		switch g := x.(type) {
		case Gap:
			computed = append(computed, g)
		case SuppliedGap:
			supplied = append(supplied, g)
		default:
			t.Fatalf("unexpected gap result %T", x)
		}
	}
	return computed, supplied
}

func hasGapKind(gaps []Gap, kind string) bool {
	for _, g := range gaps {
		if g.Kind == kind {
			return true
		}
	}
	return false
}

// DCP-V1-185, DCP-V1-186, DCP-V1-190: one supplied gap is admitted, counted
// and returned by docs_gaps labelled supplied, after the computed gaps.
func TestDCPV1185SuppliedGapAdmittedAndLabelled(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, suppliedHomes)
	if len(a.SuppliedGaps) != 1 || a.SuppliedGaps[0].Origin != "supplied" {
		t.Fatalf("supplied gap not retained with its origin: %+v", a.SuppliedGaps)
	}
	parity := map[string]int{}
	for _, p := range a.ImportParity {
		parity[p.SourceKind] = p.Admitted
	}
	for kind, want := range map[string]int{"supplied_gap": 1, "claim/status": 1, "subject/manual_case": 1, "relation/manual_case_for": 1} {
		if parity[kind] != want {
			t.Fatalf("import_parity %s = %d, want %d (%v)", kind, parity[kind], want, parity)
		}
	}
	computed, supplied := gapsFor(t, a, "adapter:flow")
	if len(supplied) != 1 || supplied[0].ID != "adapter:gap:flow" || supplied[0].Origin != "supplied" || supplied[0].Kind != "missing-asserting-e2e-tests" {
		t.Fatalf("docs_gaps lost the supplied gap: %+v", supplied)
	}
	r, _ := Query(a, Request{Operation: "gaps", ID: "adapter:flow", Limit: 100}, "fresh", nil)
	if _, last := r.Results[len(r.Results)-1].(SuppliedGap); !last || len(r.Citations) == 0 {
		t.Fatal("supplied gap must follow computed gaps and cite its evidence")
	}
	plain := buildAdoption(t, nil)
	before, _ := gapsFor(t, plain, "adapter:flow")
	// Only the manual case link (DCP-V1-189) may change a computed gap.
	kept := []Gap{}
	for _, g := range before {
		if g.Kind != "missing-manual-cases" {
			kept = append(kept, g)
		}
	}
	if fmt.Sprint(kept) != fmt.Sprint(computed) {
		t.Fatalf("supplied records changed computed gaps:\nbefore %v\nafter  %v", before, computed)
	}
	raw, err := Encode(a)
	if err != nil || !strings.Contains(string(raw), `"supplied_gaps"`) {
		t.Fatal("supplied gaps absent from the artifact", err)
	}
}

// DCP-V1-185: supplied gaps and statuses never raise trust; a provider-reported
// review is kept only as attributed reported trust.
func TestDCPV1185SuppliedRecordsNeverRaiseTrust(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		suppliedHomes(p)
		p.Gaps[0].Evidence.Trust = "reviewed"
		p.Gaps[0].Evidence.Derivation = "imported"
	})
	e := a.SuppliedGaps[0].Evidence
	if e.Trust != "generated" || e.ReportedTrust != "reviewed" {
		t.Fatalf("supplied gap trust raised: %+v", e)
	}
	for _, c := range a.Claims {
		if c.ID == "adapter:status:flow" && c.Evidence.Trust != "generated" {
			t.Fatalf("supplied status trust raised: %+v", c.Evidence)
		}
	}
}

// DCP-V1-187, DCP-V1-188: the supplied status is reachable from its subject
// through docs_get and counted per state by docs_coverage.
func TestDCPV1187SuppliedStatusQueryable(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, suppliedHomes)
	r, err := Query(a, Request{Operation: "get", ID: "adapter:flow", Limit: 100}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Details["adapter:status:flow"].Status; d == nil || d.State != "blocked" || len(d.Supporting) != 1 {
		t.Fatalf("docs_get lost the supplied status: %+v", r.Details)
	}
	r, err = Query(a, Request{Operation: "coverage", Limit: 100}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range r.Results {
		if m, ok := row.(map[string]any); ok && m["metric"] == "supplied_status:blocked" {
			found = m["value"] == 1 && m["denominator"] == 1 && m["origin"] == "supplied"
		}
	}
	if !found {
		t.Fatal("docs_coverage has no supplied status row")
	}
	plain := buildAdoption(t, nil)
	r, _ = Query(plain, Request{Operation: "coverage", Limit: 100}, "fresh", nil)
	out, _ := Encode(r)
	if strings.Contains(string(out), "supplied_status") {
		t.Fatal("coverage without supplied statuses changed")
	}
}

// DCP-V1-189: a manual case link suppresses missing-manual-cases for its target only.
func TestDCPV1189ManualCaseLinkSuppressesMissingManualCases(t *testing.T) {
	t.Parallel()
	plain := buildAdoption(t, nil)
	if computed, _ := gapsFor(t, plain, "adapter:flow"); !hasGapKind(computed, "missing-manual-cases") {
		t.Fatal("fixture flow should lack a manual disposition before the link")
	}
	a := buildAdoption(t, suppliedHomes)
	if computed, _ := gapsFor(t, a, "adapter:flow"); hasGapKind(computed, "missing-manual-cases") {
		t.Fatal("manual case link did not suppress missing-manual-cases")
	}
}

// DCP-V1-185, DCP-V1-187, DCP-V1-189, DCP-V1-192: malformed supplied records are refused.
func TestDCPV1185SuppliedRecordRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*ProviderRecord){
		"unqualified gap id":    func(p *ProviderRecord) { p.Gaps[0].ID = "other:gap" },
		"foreign gap owner":     func(p *ProviderRecord) { p.Gaps[0].Provider = "other" },
		"missing gap subject":   func(p *ProviderRecord) { p.Gaps[0].Subject = "adapter:absent" },
		"computed origin claim": func(p *ProviderRecord) { p.Gaps[0].Origin = "computed" },
		"empty gap kind":        func(p *ProviderRecord) { p.Gaps[0].Kind = "" },
		"empty gap reason":      func(p *ProviderRecord) { p.Gaps[0].Reason = "" },
		"unanchored gap":        func(p *ProviderRecord) { p.Gaps[0].Evidence.Anchors = nil },
		"verified gap":          func(p *ProviderRecord) { p.Gaps[0].Evidence.Trust = "verified" },
		"duplicate gap id":      func(p *ProviderRecord) { p.Gaps[0].ID = "adapter:flow" },
		"unknown status state":  func(p *ProviderRecord) { p.Details["adapter:status:flow"].Status.State = "verified" },
		"status without reason": func(p *ProviderRecord) { p.Details["adapter:status:flow"].Status.Reason = "" },
		"status missing support": func(p *ProviderRecord) {
			p.Details["adapter:status:flow"].Status.Supporting = []string{"adapter:absent"}
		},
		"status with claim kind": func(p *ProviderRecord) {
			p.Details["adapter:status:flow"] = RecordDetails{ClaimKind: "behavior", Status: p.Details["adapter:status:flow"].Status}
		},
		"status on a subject":      func(p *ProviderRecord) { p.Details["adapter:case:C123"] = p.Details["adapter:status:flow"] },
		"case link from non-case":  func(p *ProviderRecord) { p.Relations[len(p.Relations)-1].From = "adapter:count" },
		"case link missing target": func(p *ProviderRecord) { p.Relations[len(p.Relations)-1].To = "adapter:absent" },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			root, m := adoptionFixture(t, func(p *ProviderRecord) {
				suppliedHomes(p)
				edit(p)
			})
			if _, err := Build(context.Background(), root, m); err == nil {
				t.Fatal("invalid supplied record accepted")
			}
		})
	}
	t.Run("gaps outside the adoption schema", func(t *testing.T) {
		root, m := providerFixture(t, func(p *ProviderRecord) {
			p.Gaps = []SuppliedGap{{ID: "adapter:gap", Subject: "adapter:count", Kind: "k", Reason: "r", Provider: p.ID, Evidence: p.Subjects[0].Evidence}}
		})
		if _, err := Build(context.Background(), root, m); err == nil {
			t.Fatal("non-adoption provider admitted supplied gaps")
		}
	})
}

// DCP-V1-191: a /2 adoption provider admits observations up to the record
// limit; a /1 provider keeps the 4,096 bound.
func TestDCPV1191SchemaTwoObservationsUseRecordLimit(t *testing.T) {
	t.Parallel()
	many := func(n int) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			base := p.Observations[0]
			p.Observations = p.Observations[:0]
			for i := 0; i < n; i++ {
				o := base
				o.ID = fmt.Sprintf("adapter:observation:%05d", i)
				p.Observations = append(p.Observations, o)
			}
		}
	}
	a := buildAdoption(t, many(5000))
	if len(a.Observations) != 5000 {
		t.Fatalf("observations = %d, want 5000", len(a.Observations))
	}
	if _, err := BuildQueryIndex(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	root, m := providerFixture(t, many(MaxRecords+1))
	if _, err := Build(context.Background(), root, m); err == nil {
		t.Fatal("a /1 provider exceeded the 4,096 observation bound")
	}
}
