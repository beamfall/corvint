// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// blankedTokens is the lexicon as it stood before AZU-V0-8920: every
// expansion blanked, a braced one at its first `}`.
func blankedTokens(text string) []string {
	var tokens []string
	for _, token := range pathToken.FindAllString(expansion.ReplaceAllString(text, " "), -1) {
		if pathShaped(token) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// firstValueExpansion is the first version of the rule, which the round 2
// review refuted: it rewrote the text at the first `}` and blanked the
// result, so a nested default lost the token the blanked lexicon kept.
var firstValueExpansion = regexp.MustCompile(`\$\{(?:[A-Za-z_][A-Za-z0-9_]*|[0-9]+|[@*])(?::?[-=+])([^}]*)\}`)

// AZU-V0-8920: the value-word rule only adds tokens. Every text built from
// the pieces below, balanced or not, keeps each token the blanked lexicon
// gave it before the rule, so no change can lose a selection to the rule.
func TestValueWordsOnlyAddTokens_AZU_V0_8920(t *testing.T) {
	pieces := []string{
		"${1:-", "${ROOT:-.}", "${A:+", "${B=", "${C:?", "${D#", "${", "}", "}}",
		"$(dirname $0)", "$(", ")", "$tmp", "\\", "\\}", "\"", "'", " ", "\n",
		"/conformance/register.md", "docs/a.md", "x/y", ".", "-",
	}
	texts, refuted := 0, ""
	var build func(prefix string, left int)
	build = func(prefix string, left int) {
		texts++
		before := blankedTokens(prefix)
		if first := blankedTokens(firstValueExpansion.ReplaceAllString(prefix, " $1 ")); refuted == "" && slices.ContainsFunc(before, func(token string) bool { return !slices.Contains(first, token) }) {
			refuted = prefix
		}
		after := map[string]bool{}
		lexValueWords(prefix, maxValueWordDepth, func(token string) { after[token] = true })
		for _, token := range before {
			if !after[token] {
				t.Fatalf("%q: the rule lost token %q (kept %v)", prefix, token, after)
			}
		}
		if left == 0 {
			return
		}
		for _, piece := range pieces {
			build(prefix+piece, left-1)
		}
	}
	build("", 4)
	if texts < 300000 {
		t.Fatalf("only %d texts", texts)
	}
	// The property has teeth: the first version of the rule fails it.
	if refuted == "" {
		t.Fatal("no text refutes the first version of the rule: the pieces exercise too little")
	}
	t.Logf("%d texts; the first version of the rule loses a token on %q", texts, refuted)
}

// AZU-V0-8920: valueWords matches braces, joins closed defaults in place and
// gives an unclosed word the rest of the text.
func TestValueWordsMatchBraces_AZU_V0_8920(t *testing.T) {
	for _, tc := range []struct {
		text, joined string
		words        []string
	}{
		{"${1:-${ROOT:-.}/conformance/register.md}", "./conformance/register.md", []string{"${ROOT:-.}/conformance/register.md"}},
		{"a/${X:-b}/c ${Y:+d/e}", "a/b/c d/e", []string{"b", "d/e"}},
		{"${A:-${B:-${C:-x/y.md}}}", "x/y.md", []string{"${B:-${C:-x/y.md}}"}},
		{"${A:-${B:-x/y.md}", "${A:-${B:-x/y.md}", []string{"${B:-x/y.md}"}},
		{"${A:-x\\}y}z", "x\\}yz", []string{"x\\}y"}},
		{"${A:?x/y.md} ${B#x/y.md} ${#C} ${D}", "${A:?x/y.md} ${B#x/y.md} ${#C} ${D}", nil},
		{"${A%${B:-x/y.md}}", "${A%x/y.md}", []string{"x/y.md"}},
		{"${@:-a/b} ${*-c/d} ${12:=e/f}", "a/b c/d e/f", []string{"a/b", "c/d", "e/f"}},
		{"${", "${", nil},
		{"${A:-", "${A:-", []string{""}},
	} {
		words, joined := valueWords(tc.text, maxValueWordDepth)
		if !slices.Equal(words, tc.words) || joined != tc.joined {
			t.Errorf("valueWords(%q) = %q, %q; want %q, %q", tc.text, words, joined, tc.words, tc.joined)
		}
	}
	// The depth bound stops the descent, not the tokens of the blanked text.
	deep := strings.Repeat("${A:-", maxValueWordDepth+4) + "p/q.md" + strings.Repeat("}", maxValueWordDepth+4)
	found := map[string]bool{}
	lexValueWords(deep+" r/s.md", maxValueWordDepth, func(token string) { found[token] = true })
	if !found["r/s.md"] {
		t.Errorf("tokens beside a too-deep word are lost: %v", found)
	}
}
