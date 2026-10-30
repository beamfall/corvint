package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// DCP-V1-760: GH1996-config-module-keys is the labelled retrieval case.
// Keys and leading comments belong to the pinned module, not just key symbols.
func TestDCPV1760ConfigModuleKeysAndComments(t *testing.T) {
	t.Parallel()
	const config = "import { defineConfig } from 'playwright';\nexport default defineConfig({\n  // Chronometer deadline\n  timeout: 30000,\n  /* Rerun allowance */\n  retries: 2,\n  /**\n   * Assertion horizon\n   */\n  expect: { timeout: 5000 },\n});\n"
	files := map[string]string{
		"src/playwright.config.ts": config,
		"src/noise.md":             "# Config\n\nThe config runs tests and reads files.\n",
		"src/local.ts":             "const local = { hiddenLocalKey: 1 };\nexport function unrelated() {}\n",
	}
	root, m := ownedSourceCorpus(t, files, []string{"src/noise.md"}, false)
	a := buildOwned(t, root, m)
	const module = "native:file:src/playwright.config.ts"
	for _, query := range []string{"playwright timeout retries expect", "timeout retries expect", "chronometer deadline", "rerun allowance", "assertion horizon"} {
		order := searchOrders(t, a, Request{Operation: "search", Query: query, Limit: 100})
		if len(order) == 0 || order[0] != module {
			t.Fatalf("GH1996-config-module-keys %q: %v, want module first", query, order)
		}
	}
	for _, tc := range []struct {
		name       string
		start, end int
	}{{"timeout", 3, 4}, {"retries", 5, 6}, {"expect", 7, 10}} {
		id := fmt.Sprintf("%s:object-key:%d:%s", module, tc.end, tc.name)
		i := slices.IndexFunc(a.Claims, func(c Claim) bool { return c.ID == id })
		if i < 0 {
			t.Fatalf("missing module claim %s", id)
		}
		c := a.Claims[i]
		anchor := c.Evidence.Anchors[0]
		pinned, err := span([]byte(config), tc.start, tc.end)
		if err != nil || c.Subject != module || anchor.Start != tc.start || anchor.End != tc.end || anchor.Revision != m.Repository.Revision || anchor.Blob != git(t, root, "rev-parse", m.Repository.Revision+":src/playwright.config.ts") || anchor.SHA256 != Digest([]byte(config)) || anchor.SpanSHA256 != Digest(pinned) {
			t.Fatalf("module key evidence is not pinned: %+v err=%v", c, err)
		}
		if c.Text+"\n" != string(pinned) {
			t.Fatalf("claim is not an exact source excerpt: %q vs %q", c.Text, pinned)
		}
	}
	for _, c := range a.Claims {
		if strings.Contains(c.ID, ":object-key:") && strings.Contains(c.Text, "hiddenLocalKey") {
			t.Fatal("unexported object leaked into module claims")
		}
	}
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, idx := range []*QueryIndex{nil, index} {
		a.RuntimeIndex = idx
		r := pageOf(t, a, Request{Operation: "search", Query: "chronometer deadline", Limit: 1})
		if rank := r.ResultRanks[module]; rank.MatchAnchor == nil || rank.MatchAnchor.Start != 3 || rank.MatchAnchor.End != 4 {
			t.Fatalf("search does not carry the matching comment evidence: %+v", rank)
		}
	}
}

// DCP-V1-760: bounded comments never consume code, detached or partial blocks.
func TestDCPV1760ObjectCommentBounds(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"// attached\nkey: 1", 1},
		{"// detached\n\nkey: 1", 3},
		{"/* complete */\nkey: 1", 1},
		{"/* closed */ code /* tail */\nkey: 1", 2},
		{"code /* inline */\nkey: 1", 2},
		{" * partial */\nkey: 1", 2},
		{strings.Repeat("// bounded\n", 33) + "key: 1", 2},
		{"/*\n" + strings.Repeat(" * bounded\n", 32) + " */\nkey: 1", 35},
		{"// " + strings.Repeat("x", maxSpanBytes) + "\nkey: 1", 2},
	} {
		lines := strings.Split(tc.text, "\n")
		if got := objectCommentStart(lines, len(lines)); got != tc.want {
			t.Errorf("%d lines: got start %d, want %d", len(lines), got, tc.want)
		}
	}
}

// DCP-V1-760: default/named literals and duplicate keys retain valid module
// claims; nested keys and expressions are governed by the existing lexer.
func TestDCPV1760ConfigModuleExportForms(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		"export default { quartzBudget: 1, quartzBudget: 2, rerunCount: 3 };\n",
		"export const settings = { quartzBudget: 1, rerunCount: 3 };\n",
		"const settings = { quartzBudget: 1, rerunCount: 3 };\nexport default settings;\n",
	} {
		root, m := ownedSourceCorpus(t, map[string]string{"src/config.ts": source, "src/noise.md": "# Noise\n"}, []string{"src/noise.md"}, false)
		a := buildOwned(t, root, m)
		order := searchOrders(t, a, Request{Operation: "search", Query: "quartz budget rerun count", Limit: 20})
		if !slices.Contains(order, "native:file:src/config.ts") {
			t.Fatalf("module did not match keys: %q: %v", source, order)
		}
		claims := 0
		for _, c := range a.Claims {
			if strings.HasPrefix(c.ID, "native:file:src/config.ts:object-key:") {
				claims++
			}
		}
		if claims != 2 {
			t.Fatalf("expected two unique module key claims, got %d: %q", claims, source)
		}
	}
}

// DCP-V1-761: GH1996-common-only is the labelled weak-match case. The paired
// rare-hit query retains a correct, low-scoring hit even with absent words.
func TestDCPV1761SearchQualityUsesCorpusIDF(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i := range 24 {
			id := fmt.Sprintf("adapter:common-%d", i)
			p.Subjects = append(p.Subjects, Subject{ID: id, Name: "common configuration", Kind: "module", Provider: p.ID, Evidence: ev})
			if i == 0 {
				p.Claims = append(p.Claims, Claim{ID: id + ":needle", Subject: id, Text: "quartzneedle", Provider: p.ID, Evidence: ev})
			}
		}
	})
	x, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []*QueryIndex{nil, x} {
		a.RuntimeIndex = index
		for _, include := range [][]string{nil, {"claims"}} {
			r := pageOf(t, a, Request{Operation: "search", Query: "common configuration zxqmissing", Include: include, Limit: 100})
			q := r.SearchQuality
			if q == nil || q.Schema != "corvint-corpus-search-quality/1" || q.Outcome != "weak" || q.Reason != "all-rare-query-terms-unmatched" || q.CorpusSubjects != len(a.Subjects) || q.RareIDFThreshold != math.Ln2 || !slices.Equal(q.RareUnmatchedTerms, []string{"zxqmissing"}) {
				t.Fatalf("GH1996-common-only: %+v", q)
			}
			if r.Miss != "" || len(r.Results) < 24 || !slices.Equal(r.UnmatchedTerms, []string{"zxqmissing"}) {
				t.Fatalf("weak signal altered legacy hits/miss/unmatched fields: %+v", r)
			}
			data, err := Encode(r)
			var back Receipt
			if err != nil || json.Unmarshal(data, &back) != nil || back.SearchQuality == nil || back.SearchQuality.Outcome != "weak" {
				t.Fatalf("quality wire roundtrip: %v", err)
			}
			// DCI-V1-020: one rare match no longer hides five absent rare words, which carry
			// most of the query's IDF mass; the low-scoring hit is still retained.
			r = pageOf(t, a, Request{Operation: "search", Query: "common configuration quartzneedle zxqmissing zzzabsent zzzunknown zzzunseen zzzlost", Include: include, Limit: 100})
			if q := r.SearchQuality; q == nil || q.Reason != "unmatched-terms-carry-most-idf-mass" || q.UnmatchedIDFShare <= 0.5 || !slices.Equal(q.RareUnmatchedTerms, []string{"zxqmissing", "zzzabsent", "zzzunknown", "zzzunseen", "zzzlost"}) {
				t.Fatalf("absent words carrying most IDF mass were not weak: %+v", q)
			}
			if rank, ok := r.ResultRanks["adapter:common-0"]; !ok || rank.Score >= 0.25 {
				t.Fatalf("rare-hit control must retain a score below 0.25: %+v", rank)
			}
			for _, query := range []string{"common configuration", "quartzneedle"} {
				r = pageOf(t, a, Request{Operation: "search", Query: query, Include: include, Limit: 100})
				if len(r.Results) == 0 || r.SearchQuality != nil {
					t.Fatalf("rare match or no missing term was weakened: %q %+v", query, r.SearchQuality)
				}
				data, err = Encode(r)
				if err != nil || strings.Contains(string(data), "search_quality") {
					t.Fatalf("unaffected receipt changed: %v", err)
				}
			}
		}
		if r := pageOf(t, a, Request{Operation: "search", Query: "zxqmissing", Limit: 10}); r.Miss != "no-match" || r.SearchQuality != nil {
			t.Fatalf("true miss changed: %+v", r)
		}
		if r := pageOf(t, a, Request{Operation: "claims", Limit: 10}); r.SearchQuality != nil {
			t.Fatal("non-search receipt changed")
		}
		if r := pageOf(t, a, Request{Operation: "locate", Query: "common configuration zxqmissing", Limit: 10}); r.SearchQuality != nil {
			t.Fatal("locate's shared ranker acquired the search-only wire member")
		}
	}
}

// DCP-V1-761: rarity follows the corpus denominator and existing compound /
// inflection matching, independent of any result's normalized score. The ln(2)
// boundary still separates the two reasons: a matched rare word no longer
// suppresses the signal when the absent words carry most of the query's IDF
// mass (DCI-V1-020), but it changes the reason.
func TestDCPV1761SearchQualityRarityBoundary(t *testing.T) {
	const allRare, mostMass = "all-rare-query-terms-unmatched", "unmatched-terms-carry-most-idf-mass"
	for _, tc := range []struct {
		name, query string
		n           int
		postings    map[string][]int
		reason      string
	}{
		{"common-majority", "shared absentneedle", 10, map[string][]int{"shared": {0, 1, 2, 3, 4, 5}}, allRare},
		{"half-is-common", "shared absentneedle", 10, map[string][]int{"shared": {0, 1, 2, 3, 4}}, allRare},
		{"matched-minority", "shared absentneedle", 10, map[string][]int{"shared": {0, 1, 2, 3}}, mostMass},
		{"larger-corpus-makes-match-rare", "shared absentneedle", 20, map[string][]int{"shared": {0, 1, 2, 3, 4, 5}}, mostMass},
		{"inflection-is-common", "conditions absentneedle", 10, map[string][]int{"condition": {0, 1, 2, 3, 4, 5}}, allRare},
		{"compound-is-absent", "ReaderParse absentneedle", 10, map[string][]int{"reader": {0}, "parse": {1}}, allRare},
		{"compound-is-present", "ReaderParse absentneedle", 10, map[string][]int{"reader-parse": {0}}, mostMass},
		{"stopwords", "the shared", 10, map[string][]int{"shared": {0, 1, 2, 3, 4, 5}}, ""},
		// DCI-V1-020: two rare matches outweigh one absent word; a repeated word counts once.
		{"rare-matches-outweigh-absent", "alpha beta absentneedle", 10, map[string][]int{"alpha": {0}, "beta": {1}}, ""},
		{"repeated-word-counts-once", "alpha alpha Alpha absentneedle", 10, map[string][]int{"alpha": {0}}, mostMass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := weakSearchQuality(tc.query, tc.n, tc.postings)
			if got := ""; q != nil {
				got = q.Reason
				if got != tc.reason || (got == mostMass) != (q.UnmatchedIDFShare > 0.5) {
					t.Fatalf("quality=%+v, want reason %q", q, tc.reason)
				}
			} else if tc.reason != "" {
				t.Fatalf("quality=nil, want reason %q", tc.reason)
			}
		})
	}
}
