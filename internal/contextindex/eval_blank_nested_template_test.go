package contextindex

import (
	"strings"
	"testing"
)

// TestGPKV0295BlankCommentsAndStringsNestedTemplates pins the GPK-V0-093
// admission mask's template reading (GPK-V0-295, V1-3150): in a web file a
// template literal is blanked whole, so a backtick inside a `${}` substitution
// never ends the outer template and string data never becomes a reference
// edge. Go raw strings keep ending at the next backtick.
func TestGPKV0295BlankCommentsAndStringsNestedTemplates(t *testing.T) {
	cases := []struct {
		name, path, text string
		masked           []string
		kept             []string
	}{
		{"nested template in a substitution", "a.ts", "const s = `${`leak()`}`; after();", []string{"leak"}, []string{"const s =", "after()"}},
		{"doubly nested template", "a.js", "x = `a${`b${`leak()`}c`}d`; after();", []string{"leak"}, []string{"after()"}},
		{"decorator text inside a nested template", "a.tsx", "const text = `${`@Component({templateUrl: './view.html'})`}`; after();", []string{"Component", "templateUrl"}, []string{"after()"}},
		{"quoted backtick inside a substitution", "a.mjs", "x = `${ '`' } leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"escaped backtick in the outer template", "a.cjs", "x = `a\\` leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"multi-line nested template keeps newlines", "a.jsx", "x = `${`\nleak()\n`}`;\nafter();", []string{"leak"}, []string{"after()"}},
		{"Go raw string ends at the next backtick", "a.go", "x := `a\\` + after()", []string{"a\\"}, []string{"after()"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := evalBlankCommentsAndStrings(c.path, c.text)
			if len(got) != len(c.text) {
				t.Fatalf("mask moved offsets: %q -> %q", c.text, got)
			}
			for i := range c.text {
				if (c.text[i] == '\n') != (got[i] == '\n') {
					t.Fatalf("newline at %d not kept: %q -> %q", i, c.text, got)
				}
			}
			for _, word := range c.masked {
				if strings.Contains(got, word) {
					t.Fatalf("%q leaked as code from %q: %q", word, c.text, got)
				}
			}
			for _, word := range c.kept {
				if !strings.Contains(got, word) {
					t.Fatalf("code %q was masked in %q: %q", word, c.text, got)
				}
			}
		})
	}
}
