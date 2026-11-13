package contextindex

import (
	"strings"
	"testing"
)

// TestGPKV0395StripWebNonCodeBlanksRegexLiterals pins the regular-expression
// reading of the web mask (GPK-V0-395, V1-5501): a regex literal is blanked
// whole, so a quote in it never opens a string that masks the code after it,
// while a slash that divides and the slashes of JSX stay code.
func TestGPKV0395StripWebNonCodeBlanksRegexLiterals(t *testing.T) {
	cases := []struct {
		name, text string
		masked     []string
		kept       []string
	}{
		{"quote in a class after =", "const re = /[^&\\s\"']+/g;\nfunction after() {}", []string{"[^&", "+/g"}, []string{"const re =", "function after()"}},
		{"escaped quote set as a call argument", "s.replace(/[\"\\\\]/g, esc);\nexport function after() {}", []string{"[\""}, []string{"s.replace(", ", esc)", "export function after()"}},
		{"apostrophe after an arrow", "const ok = (x) => /a'b/.test(x);\nfunction after() {}", []string{"a'b"}, []string{"const ok = (x) =>", ".test(x)", "function after()"}},
		{"after return", "function f(s) { return /it's/.test(s); }\nfunction after() {}", []string{"it's"}, []string{"return", ".test(s)", "function after()"}},
		{"slash inside a class does not close", "x = /[/']/; after();", []string{"[/"}, []string{"x =", "after()"}},
		{"escaped slash does not close", "x = /a\\/'/; after();", []string{"a\\"}, []string{"x =", "after()"}},
		{"comment opener inside a regex", "x = /https?:\\/\\//; after();", []string{"https"}, []string{"x =", "after()"}},
		{"division keeps both operands", "const r = a / b / c;", nil, []string{"const r = a / b / c;"}},
		{"division after a call", "const r = f(x) / 2; after();", nil, []string{"f(x) / 2", "after()"}},
		{"division after a string", "const r = 'a' / 2; after();", nil, []string{"/ 2", "after()"}},
		{"JSX closing tag", "return <div>{a}/{b}</div>;\nfunction after() {}", nil, []string{"</div>", "{a}/{b}", "function after()"}},
		{"self-closing JSX", "return <br />;\nfunction after() {}", nil, []string{"<br />", "function after()"}},
		{"unterminated slash stays code", "x = a\n/ b; after();", nil, []string{"/ b", "after()"}},
		{"line comment unchanged", "x = 1; // c'\nafter();", []string{"c'"}, []string{"after()"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripWebNonCode(c.text)
			if len(got) != len(c.text) || strings.Count(got, "\n") != strings.Count(c.text, "\n") {
				t.Fatalf("mask moved offsets or newlines: %q -> %q", c.text, got)
			}
			for _, word := range c.masked {
				if strings.Contains(got, word) {
					t.Fatalf("%q stayed code in %q: %q", word, c.text, got)
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
