package doccorpus

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const residualTestSubject = "native:symbol:src/value_test.go:3:TestCount"

// suppliedTestRun adds one supplied test-run receipt for the fixture's native
// test (GitHub #1244 residual, DCP-V1-210).
func suppliedTestRun(p *ProviderRecord) {
	ev := p.Subjects[0].Evidence
	p.Claims = append(p.Claims, Claim{ID: "adapter:run:1", Subject: residualTestSubject, Text: "TestCount passed in run 41", Provider: p.ID, Evidence: ev})
	p.Details["adapter:run:1"] = RecordDetails{TestRun: &SuppliedTestRunDetails{Outcome: "passed", Run: "ci-run-41", Origin: "supplied", Revision: p.Source.Revision, At: "2026-10-19T00:00:00Z"}}
}

// legacyCaseLink declares that the fixture's manual case is legacy case C123
// (DCP-V1-212).
func legacyCaseLink(p *ProviderRecord) {
	suppliedHomes(p)
	p.Details["adapter:case:C123"] = RecordDetails{LegacyCase: &LegacyCaseLinkDetails{ID: "C123"}}
}

func queryResults(t *testing.T, a *Artifact, request Request) Receipt {
	t.Helper()
	request.Limit = 100
	r, err := Query(a, request, "fresh", nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// DCP-V1-210, DCP-V1-211: a supplied test-run receipt is admitted, counted by
// import_parity, returned by docs_get with its supplied label, counted per
// outcome by docs_coverage, and changes no computed result or trust.
func TestDCPV1210SuppliedTestRunReceipt(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		suppliedTestRun(p)
		p.Claims[len(p.Claims)-1].Evidence.Trust = "reviewed"
		p.Claims[len(p.Claims)-1].Evidence.Derivation = "imported"
	})
	parity := map[string]int{}
	for _, p := range a.ImportParity {
		parity[p.SourceKind] = p.Admitted
	}
	if parity["claim/test_run"] != 1 {
		t.Fatalf("import_parity lacks claim/test_run: %v", parity)
	}
	r := queryResults(t, a, Request{Operation: "get", ID: residualTestSubject})
	if d := r.Details["adapter:run:1"].TestRun; d == nil || d.Outcome != "passed" || d.Origin != "supplied" || d.Run != "ci-run-41" {
		t.Fatalf("docs_get lost the supplied test run: %+v", r.Details)
	}
	for _, c := range a.Claims {
		if c.ID == "adapter:run:1" && (c.Evidence.Trust != "generated" || c.Evidence.ReportedTrust != "reviewed") {
			t.Fatalf("supplied test run raised trust: %+v", c.Evidence)
		}
	}
	r = queryResults(t, a, Request{Operation: "coverage"})
	found := false
	for _, row := range r.Results {
		if m, ok := row.(map[string]any); ok && m["metric"] == "supplied_test_run:passed" {
			found = m["value"] == 1 && m["denominator"] == 1 && m["origin"] == "supplied"
		}
	}
	if !found {
		t.Fatal("docs_coverage has no supplied test-run row")
	}
	plain := buildAdoption(t, nil)
	before, _ := gapsFor(t, plain, "")
	after, _ := gapsFor(t, a, "")
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("a supplied test run changed computed gaps:\nbefore %v\nafter  %v", before, after)
	}
	metrics := func(a *Artifact) string {
		rows := []string{}
		for _, row := range queryResults(t, a, Request{Operation: "coverage"}).Results {
			if m, ok := row.(map[string]any); ok && m["metric"] != nil && !strings.HasPrefix(fmt.Sprint(m["metric"]), "supplied_test_run:") {
				rows = append(rows, fmt.Sprint(m["metric"], m["value"], m["denominator"]))
			}
		}
		return strings.Join(rows, "\n")
	}
	claimed := map[string]bool{}
	for _, c := range plain.Claims {
		claimed[c.Subject] = true
	}
	unclaimed := ""
	for _, s := range plain.Subjects {
		if !claimed[s.ID] {
			unclaimed = s.ID
			break
		}
	}
	if unclaimed == "" {
		t.Fatal("fixture has no unclaimed subject")
	}
	b := buildAdoption(t, func(p *ProviderRecord) {
		suppliedTestRun(p)
		p.Claims[len(p.Claims)-1].Subject = unclaimed
	})
	if before, after := metrics(plain), metrics(b); before != after {
		t.Fatalf("a supplied test run changed other coverage metrics:\nbefore %s\nafter  %s", before, after)
	}
	out, _ := Encode(queryResults(t, plain, Request{Operation: "coverage"}))
	if bytes.Contains(out, []byte("supplied_test_run")) {
		t.Fatal("coverage without supplied test runs changed")
	}
}

// DCP-V1-210, DCP-V1-212: malformed test-run receipts and legacy case links are refused.
func TestDCPV1210ResidualRecordRefusals(t *testing.T) {
	t.Parallel()
	run := func(edit func(*SuppliedTestRunDetails)) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			suppliedTestRun(p)
			edit(p.Details["adapter:run:1"].TestRun)
		}
	}
	link := func(id string) func(*ProviderRecord) {
		return func(p *ProviderRecord) {
			legacyCaseLink(p)
			p.Details["adapter:case:C123"].LegacyCase.ID = id
		}
	}
	cases := map[string]func(*ProviderRecord){
		"unknown outcome":   run(func(x *SuppliedTestRunDetails) { x.Outcome = "flaky" }),
		"empty run":         run(func(x *SuppliedTestRunDetails) { x.Run = "" }),
		"unlabelled origin": run(func(x *SuppliedTestRunDetails) { x.Origin = "" }),
		"computed origin":   run(func(x *SuppliedTestRunDetails) { x.Origin = "computed" }),
		"invalid revision":  run(func(x *SuppliedTestRunDetails) { x.Revision = "main" }),
		"invalid time":      run(func(x *SuppliedTestRunDetails) { x.At = "yesterday" }),
		"test run on a subject": func(p *ProviderRecord) {
			suppliedTestRun(p)
			suppliedHomes(p)
			p.Details["adapter:case:C123"] = p.Details["adapter:run:1"]
		},
		"bare number case id":  link("123"),
		"leading zero case id": link("C0123"),
		"non-numeric case id":  link("C12x"),
		"legacy case on a flow": func(p *ProviderRecord) {
			legacyCaseLink(p)
			p.Details["adapter:flow"] = p.Details["adapter:case:C123"]
		},
		"one case id on two subjects": func(p *ProviderRecord) {
			legacyCaseLink(p)
			p.Subjects = append(p.Subjects, Subject{ID: "adapter:case:copy", Kind: "manual_case", Name: "copy", Provider: p.ID, Evidence: p.Subjects[0].Evidence})
			p.Details["adapter:case:copy"] = RecordDetails{LegacyCase: &LegacyCaseLinkDetails{ID: "C123"}}
		},
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			want := "invalid supplied test run"
			switch {
			case strings.HasSuffix(name, "case id") || name == "legacy case on a flow":
				want = "invalid legacy case link"
			case name == "one case id on two subjects":
				want = "legacy case linked by more than one manual case"
			}
			root, m := adoptionFixture(t, edit)
			if _, err := Build(context.Background(), root, m); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("invalid supplied record: got %v, want %q", err, want)
			}
		})
	}
}

// DCP-V1-212, DCP-V1-213: docs_get resolves a legacy case ID to the manual
// case subject that declares it and its links; an undeclared case ID returns a
// gap naming why it links to no subject.
func TestDCPV1212LegacyCaseIDLinksToSubject(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, legacyCaseLink)
	for _, indexed := range []bool{false, true} {
		b := *a
		if indexed {
			x, err := BuildQueryIndex(context.Background(), a)
			if err != nil {
				t.Fatal(err)
			}
			b.RuntimeIndex = x
		}
		r := queryResults(t, &b, Request{Operation: "get", ID: "C123"})
		ids := []string{}
		for _, v := range r.Results {
			ids = append(ids, recordID(v))
		}
		if fmt.Sprint(ids) != "[adapter:case:C123 adapter:case-link:C123]" || r.Details["adapter:case:C123"].LegacyCase == nil {
			t.Fatalf("indexed=%v: legacy case not resolved: %v %+v", indexed, ids, r.Details)
		}
		if rel := r.Results[1].(Relation); rel.Type != "manual_case_for" || rel.To != "adapter:flow" {
			t.Fatalf("legacy case link target lost: %+v", rel)
		}
		r = queryResults(t, &b, Request{Operation: "get", ID: "C999"})
		if len(r.Results) != 1 {
			t.Fatalf("undeclared case: %+v", r.Results)
		}
		if g, ok := r.Results[0].(Gap); !ok || g.Subject != "C999" || g.Kind != "legacy-case-unlinked" || g.Reason == "" {
			t.Fatalf("undeclared case has no unlinked gap: %+v", r.Results[0])
		}
	}
	// A record ID keeps its ordinary answer; a non-case ID still misses.
	if r := queryResults(t, a, Request{Operation: "get", ID: "adapter:case:C123"}); len(r.Results) != 1 || recordID(r.Results[0]) != "adapter:case:C123" {
		t.Fatalf("record get changed: %+v", r.Results)
	}
	if r := queryResults(t, a, Request{Operation: "get", ID: "adapter:absent"}); len(r.Results) != 0 {
		t.Fatalf("non-case miss changed: %+v", r.Results)
	}
}

// DCP-V1-214: capabilities list supplied gap IDs and the query index admits
// them, so docs_get by a supplied gap ID returns it with or without the index.
func TestDCPV1214SuppliedGapsInCapabilitiesAndIndex(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, suppliedHomes)
	var gaps Capability
	for _, c := range a.Capabilities {
		if c.Name == "gaps" {
			gaps = c
		}
	}
	if !slices.Contains(gaps.Records, "adapter:gap:flow") || gaps.Count != len(gaps.Records) {
		t.Fatalf("gaps capability does not report the supplied gap: %+v", gaps)
	}
	x, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if ref := x.IDs["adapter:gap:flow"]; ref.Kind != "supplied_gap" {
		t.Fatalf("query index lacks the supplied gap: %+v", ref)
	}
	if !slices.Contains(x.Paths[a.SuppliedGaps[0].Evidence.Anchors[0].Path], RecordRef{"supplied_gap", 0}) {
		t.Fatal("query index lacks the supplied gap anchor path")
	}
	for _, index := range []*QueryIndex{nil, x} {
		b := *a
		b.RuntimeIndex = index
		r := queryResults(t, &b, Request{Operation: "get", ID: "adapter:gap:flow"})
		if len(r.Results) != 1 || len(r.Citations) == 0 {
			t.Fatalf("indexed=%v: docs_get by supplied gap ID: %+v", index != nil, r.Results)
		}
		if g, ok := r.Results[0].(SuppliedGap); !ok || g.Origin != "supplied" {
			t.Fatalf("docs_get returned %+v", r.Results[0])
		}
	}
	plain := buildAdoption(t, nil)
	px, err := BuildQueryIndex(context.Background(), plain)
	if err != nil {
		t.Fatal(err)
	}
	for id, ref := range px.IDs {
		if ref.Kind == "supplied_gap" {
			t.Fatalf("plain corpus index gained a supplied gap %s", id)
		}
	}
}

// DCP-V1-215: a shard carrying status and test-run claims compiles under the
// incremental path with the same import_parity kinds as a cold build.
func TestDCPV1215IncrementalCountsSuppliedClaimKinds(t *testing.T) {
	t.Parallel()
	root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) {
		first.Observations = nil
		suppliedTestRun(first)
		first.Claims = append(first.Claims, Claim{ID: "adapter:status:flow", Subject: "adapter:flow", Text: "Flow readiness is ready", Provider: first.ID, Evidence: first.Subjects[0].Evidence})
		first.Details["adapter:status:flow"] = RecordDetails{Status: &SuppliedStatusDetails{State: "ready", Reason: "asserting test present", Supporting: []string{"adapter:test-join"}}}
	})
	ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
	a, _, stats, err := BuildIncremental(context.Background(), root, m, ids, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Compiled) != 2 {
		t.Fatalf("shards not compiled incrementally: %+v", stats)
	}
	cold, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Encode(a)
	want, _ := Encode(cold)
	if !bytes.Equal(got, want) {
		t.Fatal("incremental artifact differs from the cold build")
	}
}
