package gateselect

import (
	"slices"
	"testing"
)

// TestFastTierLiteralLexiconDiffers_AFPV0765 (V1-2206) pins the
// literal-lexicon difference from the affected graph's lexer: this index
// splits a literal without the V1-1818 rules, so a short sed component yields
// other tokens than the affected lexicon gives. Both lexicons share the rest
// (AFP-V0-2242), where a rewrite only adds a token beside the written ones:
// the planner's AFP-V0-506 `..\` rewrite adds the climbing token to the
// written `..` and name, a closing tag (`</script>`, `&lt;/script&gt;`,
// AFP-V0-505/840) adds the unanchored token to the anchored one, and an
// import path under the module adds its root-anchored path. The comment-mode
// difference is measured over the repository by
// TestTwoGoLexersAgreeBeyondComments_AFPV0765 in internal/liveverify/affected.
func TestFastTierLiteralLexiconDiffers_AFPV0765(t *testing.T) {
	for literal, want := range map[string][]string{
		"</script>":                 {"/script", "script"},
		"&LT;/a/b&gt;":              {"LT", "/a/b", "a/b", "gt"},
		"</":                        {"/"},
		`..\fixture`:                {"..", "fixture", "../fixture"},
		`foo..\notes`:               {"foo..", "notes", "foo../notes"},
		`a\..\b`:                    {"a", "..", "b", "../b"},
		"example.test/m/internal/x": {"example.test/m/internal/x", "/internal/x"},
		"sed 's/a b'":               {"sed", "s/a", "b"},
		"docs/%s/x.txt":             {"docs/", "/x.txt"},
	} {
		if got := pathTokens(literal, "example.test/m"); !slices.Equal(got, want) {
			t.Errorf("fast-tier tokens of %q = %v, want %v", literal, got, want)
		}
	}
}
