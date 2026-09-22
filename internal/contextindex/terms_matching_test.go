package contextindex

import (
	"reflect"
	"strings"
	"testing"
)

// termsMatchingCorpus holds the shapes terms() treats specially, so a divergence
// in any of them fails here rather than as a receipt diff three layers up.
var termsMatchingCorpus = []string{
	"",
	"a",
	"ab",
	"abc",
	"func EnforceSessionRevocation() bool { return true }",
	"session_revocation_enforced",
	"HTTPServerHandler parseHTTPRequest",
	"v2 v10 API2Client",
	"queries parses busses aliases status radius analysis",
	"stringify classify verify identify",
	"argument arguments parameter parameters maximum minimum generate generator sync synchronous",
	"the and with from where which feature find change code",
	"trailing- -leading --double--  a--b",
	"kebab-case-identifier and snake_case_identifier",
	"9Digit lowerUPPER UPPERlower ABCDef",
	"// comment: retrieval budget, packet(size)=2000; path/to/file.go:14",
	"café naïve Ω identifier",
	// The two runes ToLower maps into ASCII, which is why the streaming path
	// keeps a fallback at all: each joins tokens a byte scan reads as separated.
	"\u212Aelvin sessionRevocation",
	"session\u212Aey and \u0130dentifier",
	"ba\u0130s ba\u212Aes",
	"KELVIN K sign",
	"İstanbul dotted capital",
	"emoji \U0001f600 between words",
	"mixed café_camelCase-hyphen",
	strings.Repeat("longIdentifierSegment", 40),
	strings.Repeat("x", 300) + "Boundary",
}

var termsMatchingKeepSets = []string{
	"session revocation enforce enforced device expiry",
	"http server handler parse request",
	"query queries parse parses bus busses alias aliases",
	"stringify string classify class verify identify",
	"argument parameter max maximum min minimum gen generate sync synchronous",
	"the and with feature",
	"trailing leading double kebab case identifier snake",
	"digit lower upper abcdef abc def v2 v10 api2 client api",
	"comment retrieval budget packet size path file go",
	"caf cafe naive identifier kelvin kelvi sign stanbul dotted capital emoji between words mixed hyphen camel",
	"session sessionkey key identifier idisentifier bais bakes ba session-revocation revocation",
	"longidentifiersegment long segment boundary x",
}

// referenceTermsMatching is the definition termsMatching must meet: the
// unfiltered term set of the text, intersected with the keep-set.
func referenceTermsMatching(text string, keep map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for term := range terms(text) {
		if _, ok := keep[term]; ok {
			result[term] = struct{}{}
		}
	}
	return result
}

func TestTermsMatchingEqualsFilteredTerms(t *testing.T) {
	keepSets := make([]map[string]struct{}, 0, len(termsMatchingKeepSets)+1)
	for _, phrase := range termsMatchingKeepSets {
		keepSets = append(keepSets, stringSet(phrase))
	}
	union := make(map[string]struct{})
	for _, text := range termsMatchingCorpus {
		for term := range terms(text) {
			union[term] = struct{}{}
		}
	}
	keepSets = append(keepSets, union)
	for _, text := range termsMatchingCorpus {
		for _, keep := range keepSets {
			want := referenceTermsMatching(text, keep)
			got := termsMatching(text, keep)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("termsMatching(%q, %v):\n got = %v\nwant = %v", text, sortedKeys(keep), sortedKeys(got), sortedKeys(want))
			}
		}
	}
}

// TestTermsMatchingEmptyKeepSetIsEmpty pins the early-out: nothing can survive an
// intersection with an empty set, so no text may produce a term.
func TestTermsMatchingEmptyKeepSetIsEmpty(t *testing.T) {
	for _, text := range termsMatchingCorpus {
		if got := termsMatching(text, map[string]struct{}{}); len(got) != 0 {
			t.Fatalf("termsMatching(%q, {}) = %v, want empty", text, sortedKeys(got))
		}
	}
}

// FuzzTermsMatchingEqualsFilteredTerms drives the same property over arbitrary
// text and an arbitrary keep-set, which is the only way to reach the byte
// sequences the corpus above did not think of.
func FuzzTermsMatchingEqualsFilteredTerms(f *testing.F) {
	for index, text := range termsMatchingCorpus {
		f.Add(text, termsMatchingKeepSets[index%len(termsMatchingKeepSets)])
	}
	f.Fuzz(func(t *testing.T, text, keepPhrase string) {
		keep := stringSet(keepPhrase)
		// A keep-set drawn only from the text's own terms exercises the admitted
		// branch of every guard, which an unrelated phrase rarely reaches.
		for term := range terms(text) {
			if len(term)%2 == 0 {
				keep[term] = struct{}{}
			}
		}
		want := referenceTermsMatching(text, keep)
		got := termsMatching(text, keep)
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("termsMatching(%q, %q):\n got = %v\nwant = %v", text, keepPhrase, sortedKeys(got), sortedKeys(want))
		}
	})
}

func sortedKeys(values map[string]struct{}) []string {
	return intersection(values, values)
}
