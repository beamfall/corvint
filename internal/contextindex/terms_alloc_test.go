package contextindex

import (
	"maps"
	"strings"
	"testing"
)

// referenceTerms is the tokenizer as it stood before V1-2001 removed its
// per-call rune copy, word slice and split slices.
func referenceTerms(text string) map[string]struct{} {
	runes := []rune(text)
	var split strings.Builder
	for index, current := range runes {
		if index > 0 {
			previous := runes[index-1]
			nextLower := index+1 < len(runes) && asciiLower(runes[index+1])
			if (asciiLower(previous) || asciiDigit(previous)) && asciiUpper(current) || asciiUpper(previous) && asciiUpper(current) && nextLower {
				split.WriteByte('-')
			}
		}
		split.WriteRune(current)
	}
	text = strings.ReplaceAll(pythonLower(split.String()), "_", "-")
	result := map[string]struct{}{}
	for _, raw := range asciiWords(text) {
		for _, token := range strings.Split(raw, "-") {
			if len(token) <= 1 || contains(stopWords, token) {
				continue
			}
			result[token] = struct{}{}
			if len(token) > 4 && strings.HasSuffix(token, "s") &&
				!strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "as") &&
				!strings.HasSuffix(token, "is") && !strings.HasSuffix(token, "us") {
				result[token[:len(token)-1]] = struct{}{}
			}
			for _, alias := range termAliases[token] {
				result[alias] = struct{}{}
			}
			if len(token) > 5 && strings.HasSuffix(token, "ify") {
				result[token[:len(token)-3]] = struct{}{}
			}
		}
		if len(raw) > 2 && !contains(stopWords, raw) {
			result[raw] = struct{}{}
		}
	}
	return result
}

// The allocation-free tokenizer derives exactly the former term sets, for
// ASCII and non-ASCII text alike (V1-2001).
func TestEvidenceTermsMatchReferenceTokenizer(t *testing.T) {
	cases := []string{
		"", "a", "HTTPServer parseJSONBody2 XMLHttpRequest",
		"order_invoice-payment customers classes status analysis",
		"İstanbul Ünïcode çamelCase straße ÀBCdef 日本語Text",
		"validate_requests Simplify Classify modify verify notify the and of",
		"Item123Value__double--dash -leading trailing- under_score_s",
		strings.Repeat("request Response handlerWidget order_invoice ", 50) + "Item99999Value",
	}
	into := map[string]struct{}{}
	for _, text := range cases {
		want := referenceTerms(text)
		if got := EvidenceTerms(text); !maps.Equal(got, want) {
			t.Fatalf("EvidenceTerms(%q) = %v, want %v", text, got, want)
		}
		clear(into)
		CollectEvidenceTerms(text, into)
		if !maps.Equal(into, want) {
			t.Fatalf("CollectEvidenceTerms(%q) = %v, want %v", text, into, want)
		}
	}
}
