package doccorpus

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// moduleFlowCorpus builds the flow-relation corpus: flow adapter:flow anchors src/value.go, the
// joined fr:a-produces-j record's source anchors src/view.ts, and src/report.py is reached by no
// flow. A supplied claim about src/report.py cites it; own, when set, adds a declared relation
// from flow adapter:flow-b to it.
func moduleFlowCorpus(t *testing.T, scope *ProductionScope, own bool) *Artifact {
	t.Helper()
	root, m := flowRelationFixtureWith(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		p.Claims = append(p.Claims, Claim{ID: "adapter:claim:report", Subject: "native:file:src/report.py", Text: "Reports the count", Provider: p.ID, Evidence: ev})
		p.Details["adapter:claim:report"] = RecordDetails{ClaimKind: "behavior"}
		if own {
			p.Relations = append(p.Relations, Relation{ID: "adapter:flow-b-report", From: "adapter:flow-b", To: "native:file:src/report.py", Type: "implements", Provider: p.ID, Evidence: ev})
		}
	}, nil, nil)
	m.ProductionScope = scope
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func ownershipGaps(t *testing.T, a *Artifact, kinds []string) (Receipt, []OwnershipGap) {
	t.Helper()
	r, err := Query(a, Request{Operation: "gaps", Kinds: kinds, Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	out := []OwnershipGap{}
	for _, v := range r.Results {
		if g, ok := v.(OwnershipGap); ok {
			out = append(out, g)
		}
	}
	return r, out
}

func ownershipRow(t *testing.T, a *Artifact) map[string]any {
	t.Helper()
	r, err := Query(a, Request{Operation: "coverage", Limit: MaxResults}, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range r.Results {
		if row, ok := v.(map[string]any); ok && row["metric"] == "module_flow_ownership" {
			return row
		}
	}
	return nil
}

var testProductionScope = &ProductionScope{Include: []string{"src/**"}, Exclude: []string{"src/*_test.go"}}

// TestDCPV1720DCPV1721DCPV1722ModuleWithoutFlowGaps: under a declared production scope each
// in-scope module no flow or variation reaches through a declared relation or anchor is a
// module-without-flow gap, a claim-cited one also a module-claims-without-flow gap listing the
// citing claims; both are kind-filtered and totalled like every gap, and a declared relation
// makes the module owned (DCP-V1-720, DCP-V1-721, DCP-V1-722).
func TestDCPV1720DCPV1721DCPV1722ModuleWithoutFlowGaps(t *testing.T) {
	t.Parallel()
	a := moduleFlowCorpus(t, testProductionScope, false)
	r, gaps := ownershipGaps(t, a, []string{ModuleWithoutFlow, ModuleClaimsWithoutFlow})
	if len(gaps) != 2 || gaps[0].Subject != "native:file:src/report.py" || gaps[0].Kind != ModuleWithoutFlow || gaps[1].Kind != ModuleClaimsWithoutFlow || !slices.Equal(gaps[1].Claims, []string{"adapter:claim:report"}) {
		t.Fatalf("ownership gaps %+v", gaps)
	}
	if r.KindTotals[ModuleWithoutFlow] != 1 || r.KindTotals[ModuleClaimsWithoutFlow] != 1 || !strings.Contains(gaps[0].Reason, "production scope sha256:") {
		t.Fatalf("kind totals %v reason %q", r.KindTotals, gaps[0].Reason)
	}
	if _, only := ownershipGaps(t, a, []string{ModuleClaimsWithoutFlow}); len(only) != 1 || only[0].Kind != ModuleClaimsWithoutFlow {
		t.Fatalf("claims-only filter %+v", only)
	}
	if byID, err := Query(a, Request{Operation: "gaps", ID: "native:file:src/view.ts", Kinds: []string{ModuleWithoutFlow}}, "fresh", nil); err != nil || len(byID.Results) != 0 {
		t.Fatalf("owned module reported: %+v %v", byID.Results, err)
	}
	if _, owned := ownershipGaps(t, moduleFlowCorpus(t, testProductionScope, true), []string{ModuleWithoutFlow, ModuleClaimsWithoutFlow}); len(owned) != 0 {
		t.Fatalf("relation-owned module reported: %+v", owned)
	}
	paged, err := Query(a, Request{Operation: "gaps", Kinds: []string{ModuleWithoutFlow, ModuleClaimsWithoutFlow}, Limit: 1}, "fresh", nil)
	if err != nil || len(paged.Results) != 1 || paged.NextOffset == nil {
		t.Fatalf("paged gaps %+v %v", paged, err)
	}
	app, tests := workspaceRepos(t)
	m := workspaceManifest(t, app, tests)
	m.ProductionScope = &ProductionScope{Include: []string{"e2e:pages/**", "src/**"}}
	multi, err := Build(context.Background(), app, m)
	if err != nil {
		t.Fatal(err)
	}
	subjects := []string{}
	_, unowned := ownershipGaps(t, multi, []string{ModuleWithoutFlow})
	for _, g := range unowned {
		subjects = append(subjects, g.Subject)
	}
	if !slices.Equal(subjects, []string{"native:file:e2e:pages/login.page.ts", "native:file:src/app.go"}) || !strings.Contains(unowned[0].Reason, "e2e:pages/login.page.ts") {
		t.Fatalf("multi-repository scope %v %+v", subjects, unowned)
	}
	for _, bad := range []*ProductionScope{
		{Include: []string{}},
		{Include: []string{"src/b", "src/a"}},
		{Include: []string{"src/**x"}},
		{Include: []string{"nope:src/**"}},
		{Include: []string{"src/**"}, Exclude: []string{}},
	} {
		root, single := fixture(t)
		single.ProductionScope = bad
		if _, err := Build(context.Background(), root, single); err == nil || !strings.Contains(err.Error(), "production scope") {
			t.Fatalf("scope %+v admitted: %v", bad, err)
		}
	}
}

// TestDCPV1720DCPV1723UndeclaredScopeAndOwnershipCoverage: coverage counts owned, unowned and
// out-of-scope modules with the scope digest; an undeclared scope is reported as undeclared and
// counts nothing, and a corpus without flows or a declaration keeps its receipts (DCP-V1-720,
// DCP-V1-723).
func TestDCPV1720DCPV1723UndeclaredScopeAndOwnershipCoverage(t *testing.T) {
	t.Parallel()
	row := ownershipRow(t, moduleFlowCorpus(t, testProductionScope, false))
	if row == nil || row["scope"] != "declared" || row["owned"] != 2 || row["unowned"] != 1 || row["out_of_scope"] != 1 || row["denominator"] != 3 || row["defined"] != true || row["scope_digest"] != productionScopeDigest(testProductionScope) {
		t.Fatalf("ownership row %#v", row)
	}
	undeclared := moduleFlowCorpus(t, nil, false)
	if row := ownershipRow(t, undeclared); row == nil || row["scope"] != "undeclared" || row["defined"] != false || row["owned"] != nil {
		t.Fatalf("undeclared row %#v", row)
	}
	r, gaps := ownershipGaps(t, undeclared, nil)
	if len(gaps) != 0 || !slices.Contains(r.Limitations, productionScopeUndeclared) {
		t.Fatalf("undeclared gaps %+v limitations %v", gaps, r.Limitations)
	}
	root, m := fixture(t)
	plain, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	if row := ownershipRow(t, plain); row != nil {
		t.Fatalf("corpus without flows gained a row %#v", row)
	}
	if r, _ := ownershipGaps(t, plain, nil); slices.Contains(r.Limitations, productionScopeUndeclared) {
		t.Fatalf("corpus without flows gained a limitation %v", r.Limitations)
	}
	if r, _ := ownershipGaps(t, plain, []string{ModuleWithoutFlow}); !slices.Contains(r.Limitations, productionScopeUndeclared) {
		t.Fatalf("module kind filter on an undeclared scope %v", r.Limitations)
	}
}
