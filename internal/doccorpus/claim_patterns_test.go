package doccorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// distinctText returns claim text whose template and shape differ for every i < 676.
func distinctText(i int) string {
	return fmt.Sprintf("Op%c%c reads its input", 'a'+i%26, 'a'+i/26)
}

func patternAnchor(path string, start, end int, span, file string) Anchor {
	return Anchor{Repository: "repo", Revision: "rev", Path: path, Start: start, End: end, SpanSHA256: span, SHA256: file}
}

func patternClaim(id, subject, text string, anchor Anchor) Claim {
	return Claim{ID: id, Subject: subject, Text: text, Provider: "gen", Evidence: Evidence{Trust: "generated", Anchors: []Anchor{anchor}}}
}

// patternArtifact flags every pattern once: c01 token-disjoint, c02..c04 templated text,
// c05..c09 one anchor cluster, c10 and c11 broad anchors and c12..c21 one uniform endpoint
// family. A native claim and a supplied test-run claim share the cluster anchor but are not
// examined.
func patternArtifact() *Artifact {
	cluster := patternAnchor("src/a.go", 1, 2, "span-a", "file-a")
	a := &Artifact{Manifest: Manifest{Providers: []Provider{{ID: "native", Kind: "native"}, {ID: "gen", Kind: "records"}}}, Details: map[string]RecordDetails{"t1": {TestRun: &SuppliedTestRunDetails{}}}}
	for i, name := range []string{"Alpha", "Beta", "Gamma", "Delta"} {
		a.Subjects = append(a.Subjects, Subject{ID: fmt.Sprintf("s%d", i+1), Kind: "capability", Name: name, Provider: "gen"})
	}
	a.Subjects = append(a.Subjects, Subject{ID: "s5", Kind: "endpoint", Name: "Items", Provider: "gen"})
	other := func(i int) Anchor { return patternAnchor("src/b.go", 10+i, 10+i, fmt.Sprintf("span-%d", i), "file-b") }
	a.Claims = append(a.Claims, patternClaim("c01", "s1", "Validates the fooBar input", other(1)))
	for i, s := range []string{"s2", "s3", "s4"} {
		name := []string{"Beta", "Gamma", "Delta"}[i]
		a.Claims = append(a.Claims, patternClaim(fmt.Sprintf("c%02d", i+2), s, name+" returns the configured value", other(i+2)))
	}
	for i := 5; i <= 9; i++ {
		a.Claims = append(a.Claims, patternClaim(fmt.Sprintf("c%02d", i), "s1", distinctText(i), cluster))
	}
	a.Claims = append(a.Claims, patternClaim("c10", "s1", distinctText(10), patternAnchor("src/c.go", 1, 200, "span-c", "file-c")))
	a.Claims = append(a.Claims, patternClaim("c11", "s1", distinctText(11), patternAnchor("src/d.go", 1, 30, "whole-d", "whole-d")))
	for i := 12; i <= 21; i++ {
		a.Claims = append(a.Claims, patternClaim(fmt.Sprintf("c%02d", i), "s5", fmt.Sprintf("Handles the request for item %d quickly", i), other(i)))
	}
	small := patternClaim("c22", "s1", distinctText(22), patternAnchor("src/e.go", 1, 5, "whole-e", "whole-e"))
	native := patternClaim("n1", "s1", distinctText(23), cluster)
	native.Provider = "native"
	a.Claims = append(a.Claims, small, native, patternClaim("t1", "s1", distinctText(24), cluster))
	a.ClaimMentions = []ClaimMentionCheck{
		{ClaimID: "c01", Subject: "s1", State: MentionsUnresolved, Unresolved: 1, Mentions: []ClaimMention{{Token: "fooBar", Kind: "identifier", Resolution: MentionUnresolved}}},
		{ClaimID: "c02", Subject: "s2", State: MentionsConsistent, InSpan: 1, Mentions: []ClaimMention{{Token: "Beta", Kind: "code", Resolution: MentionResolvedInSpan}}},
	}
	return a
}

// DCX-V1-348: the pattern report gives each pattern's rule, thresholds, count, rate, groups and
// sorted bounded claim IDs over the examined claims; it is deterministic and changes nothing.
func TestDCXV1348CorpusPatternReport(t *testing.T) {
	t.Parallel()
	a := patternArtifact()
	before, _ := json.Marshal(a)
	r := PatternReport(a)
	want := map[string]string{
		"token-disjoint": "1 1 c01",
		"templated-text": "1 3 c02,c03,c04",
		"anchor-cluster": "1 5 c05,c06,c07,c08,c09",
		"broad-anchor":   "2 2 c10,c11",
		"uniform-shape":  "1 10 c12,c13,c14,c15,c16,c17,c18,c19,c20,c21",
	}
	if r.Schema != CorpusPatternsSchema || r.Population != 22 || r.Flagged != 21 || len(r.Patterns) != len(CorpusPatternNames) {
		t.Fatalf("report = %+v", r)
	}
	for i, p := range r.Patterns {
		got := fmt.Sprintf("%d %d %s", p.Groups, p.Count, strings.Join(p.Claims, ","))
		if p.Name != CorpusPatternNames[i] || got != want[p.Name] || p.Denominator != 22 || p.Rate == nil || *p.Rate != float64(int(float64(p.Count)/22*1e6+0.5))/1e6 || p.Rule == "" || len(p.Thresholds) == 0 || p.Omitted != 0 {
			t.Fatalf("%s = %s %+v", p.Name, got, p)
		}
	}
	first, _ := json.Marshal(r)
	again, _ := json.Marshal(PatternReport(a))
	if !bytes.Equal(first, again) {
		t.Fatal("pattern report is nondeterministic")
	}
	if after, _ := json.Marshal(a); !bytes.Equal(before, after) {
		t.Fatal("pattern report changed the artifact")
	}
	table := string(RenderPatternTable(r))
	if !strings.Contains(table, "examined 22  flagged 21") || !strings.Contains(table, "anchor-cluster") || !strings.Contains(table, "0.227273") {
		t.Fatalf("table:\n%s", table)
	}
	big := &Artifact{Manifest: Manifest{Providers: []Provider{{ID: "gen", Kind: "records"}}}, Subjects: []Subject{{ID: "s", Kind: "capability", Name: "S"}}}
	for i := 0; i < MaxPatternClaims+44; i++ {
		big.Claims = append(big.Claims, patternClaim(fmt.Sprintf("b%03d", i), "s", distinctText(i), patternAnchor("src/a.go", 1, 2, "span", "file")))
	}
	cluster := PatternReport(big).Patterns[2]
	if cluster.Name != "anchor-cluster" || cluster.Count != MaxPatternClaims+44 || len(cluster.Claims) != MaxPatternClaims || cluster.Omitted != 44 || cluster.Claims[0] != "b000" {
		t.Fatalf("bound: %s count %d listed %d omitted %d", cluster.Name, cluster.Count, len(cluster.Claims), cluster.Omitted)
	}
	empty := PatternReport(&Artifact{})
	if empty.Population != 0 || empty.Patterns[0].Rate != nil || !strings.Contains(strings.Join(empty.Limitations, ";"), "every rate is undefined") {
		t.Fatalf("empty report = %+v", empty)
	}
	if !strings.Contains(string(RenderPatternTable(empty)), "undefined") {
		t.Fatal("undefined rate rendered as a number")
	}
	root, m := adoptionFixture(t, mentionEdit)
	built, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	gaps := func() string {
		r, err := Query(built, Request{Operation: "gaps", Limit: MaxResults}, "fresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(r)
		return string(raw)
	}
	beforeGaps := gaps()
	if PatternReport(built).Patterns[0].Count != 0 {
		t.Fatal("a claim with an in-span mention was token-disjoint")
	}
	if gaps() != beforeGaps {
		t.Fatal("pattern report changed the gaps")
	}
	for _, name := range CorpusPatternNames {
		if strings.Contains(beforeGaps, `"kind":"`+name) {
			t.Fatalf("pattern %s became a gap", name)
		}
	}
}

// DCX-V1-350: --stratify patterns over-samples flagged claims reproducibly, capping a stratum at
// its population.
func TestDCXV1350StratifyPatternsOverSamples(t *testing.T) {
	t.Parallel()
	a := &Artifact{Manifest: Manifest{Providers: []Provider{{ID: "gen", Kind: "records"}}}, Subjects: []Subject{{ID: "s", Kind: "capability", Name: "S"}}}
	for i := 0; i < 30; i++ {
		anchor := patternAnchor("src/b.go", 10+i, 10+i, fmt.Sprintf("span-%d", i), "file-b")
		if i < 5 {
			anchor = patternAnchor("src/a.go", 1, 2, "span-a", "file-a")
		}
		a.Claims = append(a.Claims, patternClaim(fmt.Sprintf("c%02d", i), "s", distinctText(i), anchor))
	}
	draw := func(o AuditSampleOptions) (*AuditSample, []byte) {
		t.Helper()
		s, err := SampleClaims(a, o)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return s, raw
	}
	allocation := func(s *AuditSample) string {
		parts := []string{}
		for _, st := range s.Strata {
			parts = append(parts, fmt.Sprintf("%s=%d/%d", st.Name, st.Sampled, st.Population))
		}
		return strings.Join(parts, " ")
	}
	sample, raw := draw(AuditSampleOptions{Seed: 7, Size: 8, Stratify: "patterns"})
	if got := allocation(sample); got != "anchor-cluster=4/5 unflagged=4/25" {
		t.Fatalf("allocation = %s", got)
	}
	if sample.Design.Method != auditPatternSampleMethod || !strings.Contains(sample.Limitations[0], "over-sampled") {
		t.Fatalf("design = %+v %v", sample.Design, sample.Limitations)
	}
	if _, again := draw(AuditSampleOptions{Seed: 7, Size: 8, Stratify: "patterns"}); !bytes.Equal(raw, again) {
		t.Fatal("patterns sample is not reproducible")
	}
	for _, c := range sample.Claims {
		if c.Stratum == "anchor-cluster" && c.ClaimID >= "c05" || c.Stratum == "unflagged" && c.ClaimID < "c05" {
			t.Fatalf("claim %s in stratum %s", c.ClaimID, c.Stratum)
		}
	}
	proportional, _ := draw(AuditSampleOptions{Seed: 7, Size: 8, Stratify: "kind"})
	if proportional.Strata[0].Sampled != 8 {
		t.Fatalf("baseline = %s", allocation(proportional))
	}
	capped, _ := draw(AuditSampleOptions{Seed: 7, Size: 25, Stratify: "patterns"})
	if got := allocation(capped); got != "anchor-cluster=5/5 unflagged=20/25" {
		t.Fatalf("capped allocation = %s", got)
	}
	tie := weightedAllocation([]string{"a", "b"}, map[string]int{"a": 3, "b": 3}, func(string) int { return 1 }, 3)
	if tie["a"] != 2 || tie["b"] != 1 {
		t.Fatalf("tie = %v", tie)
	}
	if !ValidAuditStratify("patterns") {
		t.Fatal("patterns is not an admitted stratification")
	}
}

// DCX-V1-351: a stratified audit report gives a population-weighted overall figure that abstains
// when a populated stratum has no defined accuracy.
func TestDCXV1351WeightedAuditFigure(t *testing.T) {
	t.Parallel()
	groups := map[string]*AuditGroup{"a": {Accuracy: AuditRatio{Numerator: 1, Denominator: 2, Defined: true}}, "b": {Accuracy: AuditRatio{Numerator: 1, Denominator: 1, Defined: true}}}
	w := auditWeighted([]AuditStratum{{"a", 30, 2}, {"b", 10, 1}, {"c", 0, 0}}, groups)
	if !w.Defined || w.Value == nil || *w.Value != 0.625 || w.Population != 40 || len(w.UndefinedStrata) != 0 {
		t.Fatalf("weighted = %+v", w)
	}
	if w := auditWeighted([]AuditStratum{{"a", 30, 2}, {"d", 5, 1}}, groups); w.Defined || w.Value != nil || strings.Join(w.UndefinedStrata, ",") != "d" {
		t.Fatalf("abstention = %+v", w)
	}
	_, a, err := auditBuild(t, nil, one(t, nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	report := func(stratify string) *AuditReport {
		t.Helper()
		s, err := SampleClaims(a, AuditSampleOptions{Seed: 1, Size: MaxAuditSample, Stratify: stratify})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := Encode(s)
		r, err := ReportAudit(a, s, "sample.json", raw)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	patterns := report("patterns")
	if patterns.Weighted == nil || !patterns.Weighted.Defined || *patterns.Weighted.Value != 1 || patterns.Weighted.Population != len(a.Claims) {
		t.Fatalf("patterns weighted = %+v", patterns.Weighted)
	}
	if len(patterns.ByStratum) == 0 || !strings.Contains(string(RenderAuditTable(patterns)), "population-weighted") {
		t.Fatalf("per-stratum report = %+v", patterns.ByStratum)
	}
	kind := report("kind")
	if kind.Weighted == nil || kind.Weighted.Defined || len(kind.Weighted.UndefinedStrata) == 0 || !strings.Contains(strings.Join(kind.Limitations, ";"), "abstained") || !strings.Contains(string(RenderAuditTable(kind)), "abstain (undefined strata: ") {
		t.Fatalf("kind weighted = %+v", kind.Weighted)
	}
	if plain := report(""); plain.Weighted != nil {
		t.Fatalf("unstratified report has a weighted figure: %+v", plain.Weighted)
	}
	// Forged strata are refused: sampled above population, and strata that do not reconcile with
	// the sampled claims or the design totals. The last case is a sample whose second stratum is
	// dropped, which would otherwise yield a defined weighted figure from one stratum.
	for _, c := range []struct {
		name string
		edit func(*AuditSample)
		want string
	}{
		{"sampled above population", func(s *AuditSample) { s.Strata[0].Sampled = s.Strata[0].Population + 1 }, "sampled <= population"},
		{"duplicate stratum", func(s *AuditSample) { s.Strata = append(s.Strata, s.Strata[0]) }, "names must be unique"},
		{"claim in unlisted stratum", func(s *AuditSample) { s.Claims[0].Stratum = "forged" }, "unlisted stratum"},
		{"sampled count mismatch", func(s *AuditSample) { s.Strata[0].Sampled--; s.Strata[1].Sampled++ }, "sampled claims but the sample has"},
		{"population total mismatch", func(s *AuditSample) { s.Strata[0].Population++ }, "differ from the design"},
		{"dropped stratum", func(s *AuditSample) {
			drop := s.Strata[1].Name
			s.Strata = s.Strata[:1]
			claims := []AuditSampleClaim{}
			for _, cl := range s.Claims {
				if cl.Stratum != drop {
					claims = append(claims, cl)
				}
			}
			s.Claims, s.Design.Sampled = claims, len(claims)
		}, "differ from the design"},
	} {
		s, err := SampleClaims(a, AuditSampleOptions{Seed: 1, Size: 6, Stratify: "kind"})
		if err != nil || len(s.Strata) < 2 || s.Strata[0].Sampled == 0 || s.Strata[1].Sampled == 0 {
			t.Fatalf("fixture sample = %+v %v", s, err)
		}
		if raw, _ := Encode(s); func() error { _, err := ParseAuditSample(raw); return err }() != nil {
			t.Fatal("unforged sample refused")
		}
		c.edit(s)
		forged, _ := Encode(s)
		if _, err := ParseAuditSample(forged); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v, want %q", c.name, err, c.want)
		}
	}
}

// DCX-V1-352: optional pattern-rate thresholds refuse a corpus above them, naming each pattern.
func TestDCXV1352PatternRateThresholds(t *testing.T) {
	t.Parallel()
	a := patternArtifact()
	if err := CheckBuildThresholds(a, BuildThresholds{MaxPatternRates: map[string]float64{"uniform-shape": 0.5, "broad-anchor": 0.1}}); err != nil {
		t.Fatalf("rates within thresholds refused: %v", err)
	}
	err := CheckBuildThresholds(a, BuildThresholds{MaxPatternRates: map[string]float64{"uniform-shape": 0.3, "anchor-cluster": 0.2, "templated-text": 0.5}})
	var refused *Error
	if !errors.As(err, &refused) || refused.Code != "corpus-refused" || !strings.Contains(refused.Message, "anchor-cluster rate 5/22 = 0.227273 exceeds 0.2; uniform-shape rate 10/22 = 0.454545 exceeds 0.3") || strings.Contains(refused.Message, "templated-text") {
		t.Fatalf("thresholds = %v", err)
	}
	all, err := ParsePatternRates("0.5")
	if err != nil || len(all) != len(CorpusPatternNames) || all["broad-anchor"] != 0.5 {
		t.Fatalf("bare rate = %v %v", all, err)
	}
	named, err := ParsePatternRates("anchor-cluster=0.1,broad-anchor=0")
	if err != nil || len(named) != 2 || named["anchor-cluster"] != 0.1 || named["broad-anchor"] != 0 {
		t.Fatalf("named rates = %v %v", named, err)
	}
	for _, bad := range []string{"", "nope=0.1", "anchor-cluster=2", "anchor-cluster", "anchor-cluster=0.1,anchor-cluster=0.2", "anchor-cluster=0.1,"} {
		if _, err := ParsePatternRates(bad); err == nil {
			t.Fatalf("%q admitted", bad)
		}
	}
}
