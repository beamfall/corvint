package contextindex

import (
	"context"
	"strings"
	"testing"
)

// TestGPKV0095StripWebNonCodeBlanksNestedTemplates pins the nested-aware web
// mask (GPK-V0-095, V1-3140): a template literal is blanked whole, its `${}`
// substitutions and every literal nested in them included, so a backtick
// inside a substitution never ends the outer template and string data never
// becomes code. Each case names the text that must stay masked and the code
// that must survive it.
func TestGPKV0095StripWebNonCodeBlanksNestedTemplates(t *testing.T) {
	cases := []struct {
		name, text string
		masked     []string
		kept       []string
	}{
		{"nested template in a substitution", "const s = `${`leak()`}`; after();", []string{"leak"}, []string{"const s =", "after()"}},
		{"doubly nested template", "x = `a${`b${`leak()`}c`}d`; after();", []string{"leak"}, []string{"after()"}},
		{"decorator text inside a nested template", "const text = `${`@Component({templateUrl: './view.html'})`}`; after();", []string{"Component", "templateUrl"}, []string{"after()"}},
		{"quoted backtick inside a substitution", "x = `${ '`' } leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"double-quoted backtick inside a substitution", "x = `${ \"`\" } leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"object braces inside a substitution", "x = `${ {a: `y`}.a } leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"block comment holding a backtick inside a substitution", "x = `${ /* ` */ 1 } leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"line comment holding a backtick inside a substitution", "x = `${ 1 // `\n} leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"line comment in a substitution ends at CR", "x = `${ 1 // c\r} leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"line comment in a substitution ends at U+2028", "x = `${ 1 // c\u2028} leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"line comment in a substitution ends at U+2029", "x = `${ 1 // c\u2029} leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"escaped backtick in the outer template", "x = `a\\` leak()`; after();", []string{"leak"}, []string{"after()"}},
		{"escaped dollar is not a substitution", "x = `\\${`; after();", nil, []string{"after()"}},
		{"unterminated outer template masks to the end", "x = `${`a`} leak()\nmore()", []string{"leak", "more"}, []string{"x ="}},
		{"unterminated substitution masks to the end", "x = `${ leak(\nmore()", []string{"leak", "more"}, []string{"x ="}},
		{"string and comment masks unchanged", "a('`'); /* `x` */ b(\"`\"); // `\nafter();", []string{"x"}, []string{"a(", "b(", "after()"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripWebNonCode(c.text)
			if len(got) != len(c.text) || strings.Count(got, "\n") != strings.Count(c.text, "\n") {
				t.Fatalf("mask moved offsets or newlines: %q -> %q", c.text, got)
			}
			for i := range c.text {
				if c.text[i] == '\n' && got[i] != '\n' {
					t.Fatalf("newline at %d not kept: %q", i, got)
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

// TestGPKV0095DependencyLinksIgnoreCallsInsideNestedTemplates is the
// reproduction through the GPK-V0-047 dependency-link stage that reads the
// mask: a call spelled inside a template nested in a substitution is string
// data and links nothing, while a real call in the same body still links.
func TestGPKV0095DependencyLinksIgnoreCallsInsideNestedTemplates(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"lib/root.js": "export function rootValue(input) {\n  const label = `${`decoyHelper(input)`}`;\n  return realHelper(label);\n}\n" +
			"export function decoyHelper(input) { return input; }\n" +
			"export function realHelper(input) { return input; }\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var caller Symbol
	for _, symbol := range index.Symbols {
		if symbol.Path == "lib/root.js" && symbol.Name == "rootValue" {
			caller = symbol
		}
	}
	if caller.Name == "" {
		t.Fatal("rootValue not indexed")
	}
	names := map[string]bool{}
	for _, call := range webDirectCalls(index, caller) {
		names[call.callee.Name] = true
	}
	if names["decoyHelper"] {
		t.Fatalf("a call inside a nested template linked: %v", names)
	}
	if !names["realHelper"] {
		t.Fatalf("the real call no longer links: %v", names)
	}
}

// TestGPKV0095ImpactReadsNoImportInsideNestedTemplates is the impact
// reproduction V1-3140 asked for. `corvint impact` reads web imports with the
// nested-aware web lexer (webimports.go), not stripWebNonCode, so an import
// spelled inside a nested template binds nothing there; a real import of the
// same module still reaches impact.
func TestGPKV0095ImpactReadsNoImportInsideNestedTemplates(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"src/target.ts": "export function target() { return 1; }\n",
		"src/decoy.ts":  "export const text = `${`import { target } from './target'`}`;\n",
		"src/real.ts":   "import { target } from './target';\nexport const value = target();\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ImpactSyntax(index, []string{"src/target.ts"}, 20)
	if err != nil {
		t.Fatal(err)
	}
	keys := strings.Join(resultKeys(receipt), " ")
	if strings.Contains(keys, "src/decoy.ts") {
		t.Fatalf("impact read an import inside a nested template: %s", keys)
	}
	if !strings.Contains(keys, "src/real.ts") {
		t.Fatalf("impact lost the real importer: %s", keys)
	}
}
