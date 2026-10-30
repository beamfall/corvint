package corpusbridge

import (
	"strings"
	"testing"
)

// DCP-V1-579: the docs_search tool description says that include ["claims"] adds claim hits to the
// subject hits, ranked together and labelled in result_kinds, and names docs_claims for claims
// only; no other tool carries that sentence.
func TestDCPV1579SearchDescriptionSaysClaimsAreAdded(t *testing.T) {
	want := `include ["claims"] adds claim hits to the subject hits, ranked together and labelled in ` +
		`result_kinds; for claims only, use the claims operation (corvint.docs_claims).`
	if SearchIncludeDescription != want {
		t.Fatalf("SearchIncludeDescription = %q, want %q", SearchIncludeDescription, want)
	}
	root, _ := lazyFixture(t)
	registry, err := New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tool := range registry.Tools() {
		has := strings.Contains(tool.Description, want)
		if tool.Name == "corvint.docs_search" {
			found = true
			if !has {
				t.Fatalf("docs_search description %q lacks %q", tool.Description, want)
			}
		} else if has {
			t.Fatalf("%s carries the search include text", tool.Name)
		}
	}
	if !found {
		t.Fatal("docs_search is not advertised")
	}
}
