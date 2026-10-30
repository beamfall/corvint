package doccorpus

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// TestDCIV1020WeakSignalAtRealisticSize: in a corpus of thousands of subjects, a query whose
// only matches are rare but generic words ("pay", "out") and whose specific words are absent
// reports search_quality weak, because the absent words carry most of the query's IDF mass
// (GitHub #2006). A query whose matches carry the mass stays unmarked.
func TestDCIV1020WeakSignalAtRealisticSize(t *testing.T) {
	t.Parallel()
	nouns := []string{"ledger", "invoice", "account", "session", "token", "report", "schedule", "profile", "upload", "channel", "metric", "cache", "policy", "template", "routing", "audit"}
	verbs := []string{"create", "update", "delete", "render", "parse", "validate", "export", "import", "notify", "resolve"}
	a := buildAdoption(t, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i := range 4000 {
			name := fmt.Sprintf("%s %s step%d", verbs[i%len(verbs)], nouns[(i/len(verbs))%len(nouns)], i)
			switch {
			case i%300 == 7:
				name = fmt.Sprintf("pay %s step%d", nouns[i%len(nouns)], i)
			case i%300 == 11:
				name = fmt.Sprintf("checkout %s out step%d", nouns[i%len(nouns)], i)
			}
			p.Subjects = append(p.Subjects, Subject{ID: fmt.Sprintf("adapter:generated-%d", i), Name: name, Kind: "module", Provider: p.ID, Evidence: ev})
		}
	})
	x, err := BuildQueryIndex(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []*QueryIndex{nil, x} {
		a.RuntimeIndex = index
		r := pageOf(t, a, Request{Operation: "search", Query: "pay out to a cryptocurrency wallet", Limit: 20})
		q := r.SearchQuality
		if len(r.Results) == 0 || q == nil || q.Outcome != "weak" || q.Reason != "unmatched-terms-carry-most-idf-mass" || q.UnmatchedIDFShare <= 0.5 || !slices.Equal(q.RareUnmatchedTerms, []string{"cryptocurrency", "wallet"}) {
			t.Fatalf("generic rare matches hid the absent specific words: results=%d quality=%+v", len(r.Results), q)
		}
		if r := pageOf(t, a, Request{Operation: "search", Query: "pay ledger checkout wallet", Limit: 20}); r.SearchQuality != nil {
			t.Fatalf("matches carrying most IDF mass were weakened: %+v", r.SearchQuality)
		}
	}
}
