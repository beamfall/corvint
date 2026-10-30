package doccorpus

import (
	"context"
	"slices"
	"testing"
)

var mixedQuerySources = map[string]string{
	"src/request.ts": "export function parseRequest() { return 1; }\nexport const readerLimit = 4;\n",
	"src/guide.md":   "# Guide\n\nThe corpus serves records.\n\nA request reader reads each corpus request and parses it.\n\nThe parse step checks the request.\n",
	"src/other.md":   "# Other\n\nUnrelated text.\n",
}

// mixedQueryCorpus builds the native mixed-query corpus, with and without a
// runtime query index, so a test checks index/scan parity.
func mixedQueryCorpus(t *testing.T) []*Artifact {
	t.Helper()
	root, m := ownedSourceCorpus(t, mixedQuerySources, []string{"src/other.md"}, false)
	a := buildOwned(t, root, m)
	index, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	indexed := *a
	indexed.RuntimeIndex = index
	return []*Artifact{a, &indexed}
}

// DCS-V1-580 (V1-3068, GitHub #1970): a mixed query that holds a symbol's
// exact name as one identifier token ranks that symbol first, in tier
// identifier, above a document whose paragraphs hold more of the query's
// words.
func TestDCSV1580MixedQueryRanksExactSymbolToken(t *testing.T) {
	t.Parallel()
	const symbol = "native:symbol:src/request.ts:1:parseRequest"
	for _, a := range mixedQueryCorpus(t) {
		for _, query := range []string{"parseRequest corpus request reader", "corpus request reader `ParseRequest`"} {
			r := pageOf(t, a, Request{Operation: "search", Query: query, Limit: 10})
			if len(r.Results) == 0 || recordID(r.Results[0]) != symbol {
				t.Fatalf("%q: first result = %v, want %s", query, r.Results, symbol)
			}
			if rank := r.ResultRanks[symbol]; rank.Tier != "identifier" || rank.Rank != 1 {
				t.Fatalf("%q: symbol rank = %+v", query, rank)
			}
		}
		// A one-word query keeps its DCS-V1-541 identifier tier.
		if r := pageOf(t, a, Request{Operation: "search", Query: "parseRequest", Limit: 10}); recordID(r.Results[0]) != symbol {
			t.Fatalf("single identifier query: %v", r.Results)
		}
		// A plain word is not an identifier token.
		if tokens := identifierTokens("request reader"); tokens != nil {
			t.Fatalf("plain words became identifier tokens: %v", tokens)
		}
	}
}

// DCS-V1-581 (V1-3068, GitHub #1970): a document result's rank names the
// paragraph claim holding the most query words and that paragraph's anchor,
// not the document's first line. A symbol, whose declaration claim shares its
// own anchor, carries no match.
func TestDCSV1581DocumentResultCarriesMatchingParagraphAnchor(t *testing.T) {
	t.Parallel()
	const document = "native:file:src/guide.md"
	for _, a := range mixedQueryCorpus(t) {
		r := pageOf(t, a, Request{Operation: "search", Query: "parseRequest corpus request reader", Limit: 10})
		rank, ok := r.ResultRanks[document]
		if !ok {
			t.Fatalf("document not ranked: %v", r.Results)
		}
		if rank.MatchClaim != document+":paragraph:5" || rank.MatchAnchor == nil || rank.MatchAnchor.Start != 5 || rank.MatchAnchor.Path != "src/guide.md" {
			t.Fatalf("document match = %q %+v, want paragraph at line 5", rank.MatchClaim, rank.MatchAnchor)
		}
		if s := r.ResultRanks["native:symbol:src/request.ts:1:parseRequest"]; s.MatchClaim != "" || s.MatchAnchor != nil {
			t.Fatalf("symbol carries a match: %+v", s)
		}
		if _, err := Encode(r); err != nil {
			t.Fatal(err)
		}
	}
}

// DCS-V1-582 (V1-3069, GitHub #1971): a search reports, in query order, the
// query words no record holds, while miss stays empty because other words
// matched. An identifier word whose parts occur but whose compound does not
// is unmatched; a stop word is never reported; a fully matched query carries
// no field.
func TestDCSV1582SearchReportsUnmatchedTerms(t *testing.T) {
	t.Parallel()
	for _, a := range mixedQueryCorpus(t) {
		for _, include := range [][]string{nil, {"claims"}} {
			r := pageOf(t, a, Request{Operation: "search", Query: "the corpus zxqvw request ReaderParse zxqvw", Include: include, Limit: 10})
			if r.Miss != "" || len(r.Results) == 0 {
				t.Fatalf("partial match should not miss: miss=%q results=%d", r.Miss, len(r.Results))
			}
			if want := []string{"zxqvw", "ReaderParse"}; !slices.Equal(r.UnmatchedTerms, want) {
				t.Fatalf("include %v: unmatched_terms = %v, want %v", include, r.UnmatchedTerms, want)
			}
			if r := pageOf(t, a, Request{Operation: "search", Query: "corpus parseRequest", Include: include, Limit: 10}); r.UnmatchedTerms != nil {
				t.Fatalf("matched query reports unmatched terms: %v", r.UnmatchedTerms)
			}
		}
		if r := pageOf(t, a, Request{Operation: "search", Query: "zxqvw", Limit: 10}); r.Miss != "no-match" || !slices.Equal(r.UnmatchedTerms, []string{"zxqvw"}) {
			t.Fatalf("all-absent query: miss=%q unmatched=%v", r.Miss, r.UnmatchedTerms)
		}
	}
}
