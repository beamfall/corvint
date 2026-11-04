package gateselect

import (
	"slices"
	"testing"
)

// TestFastTierLiteralLexiconDiffers_AFPV0765 (V1-2206) pins the
// literal-lexicon difference from the affected graph's lexer: this index
// splits a literal with none of the AFP-V0-506 and V1-1818 rules, so a `..\x`
// path or a short sed component yields other tokens than the affected lexicon
// gives. A closing tag (`</script>`, `&lt;/script&gt;`, AFP-V0-505/840) yields
// the planner's unanchored token and the anchored one (AFP-V0-2242). The comment-mode difference is measured
// over the repository by TestTwoGoLexersAgreeBeyondComments_AFPV0765 in
// internal/liveverify/affected.
func TestFastTierLiteralLexiconDiffers_AFPV0765(t *testing.T) {
	for literal, want := range map[string][]string{
		"</script>":     {"/script", "script"},
		"&LT;/a/b&gt;":  {"LT", "/a/b", "a/b", "gt"},
		"</":            {"/"},
		`..\fixture`:    {"..", "fixture"},
		"sed 's/a b'":   {"sed", "s/a", "b"},
		"docs/%s/x.txt": {"docs/", "/x.txt"},
	} {
		if got := pathTokens(literal, "example.test/m"); !slices.Equal(got, want) {
			t.Errorf("fast-tier tokens of %q = %v, want %v", literal, got, want)
		}
	}
}
