package contextindex

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// countFilter is the lineTokens/roleTerms reading before CRK-V0-002: every
// countTerms term of text that wanted holds, sorted.
func countFilter(text string, wanted map[string]struct{}) []string {
	matched := make([]string, 0)
	for term := range countTerms(text) {
		if _, ok := wanted[term]; ok {
			matched = append(matched, term)
		}
	}
	sort.Strings(matched)
	return matched
}

func TestMatchTermsEqualsCountTermsFiltered(t *testing.T) {
	texts := []string{
		"", "a", "ab", "fooBarBaz FOOBar fooBAR 9abc abc9 x9Y foo foo",
		"café naïve Ünïcode straße 日本語 mixedÜpper",
		"a1B2c3 __init__ HTTPServer parseJSONResponse v2Beta",
		strings.Repeat("x", 200) + " " + strings.Repeat("Ab", 70),
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, strings.Split(string(data), "\n")...)
	}
	wanted := map[string]struct{}{"foo": {}, "bar": {}, "http": {}, "server": {}, "json": {}, "compiler": {}, "lexical": {},
		"terms": {}, "abc9": {}, "9abc": {}, "x9": {}, "ab": {}, "caf": {}, strings.Repeat("x", 200): {}, "row": {}, "path": {}}
	for _, text := range texts {
		if got, want := matchTerms(text, wanted), countFilter(text, wanted); !reflect.DeepEqual(got, want) {
			t.Fatalf("matchTerms(%q) = %v, want %v", text, got, want)
		}
	}
	if got := matchTerms("nothing here", wanted); got == nil || len(got) != 0 {
		t.Fatalf("an unmatched line returns %#v, want an empty non-nil slice", got)
	}
}
