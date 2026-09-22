package contextindex

import (
	"sort"
	"strings"
	"testing"
)

// webEdges is the assertion shape every case below shares: the sorted specifier
// set for one JavaScript or TypeScript source, routed through sourceImports so
// the extension dispatch is exercised rather than bypassed.
func webEdges(t *testing.T, sourcePath, text string) []string {
	t.Helper()
	extraction := sourceImports(sourcePath, text)
	edges := make([]string, 0, len(extraction.imports))
	for specifier := range extraction.imports {
		edges = append(edges, specifier)
	}
	sort.Strings(edges)
	return edges
}

func assertEdges(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("edges = %q, want %q", got, want)
	}
}

// TestWebImportsRejectsCommentedSpecifiers pins the first fabricated-edge class
// the line scanner produced. Both fixtures are reductions of real corpus files:
// beamfall/internal/web/app/src/areas/enroll/index.ts documents how consumers
// import it, and beamfall-design-system/_from-app/components/Screens/_shared/
// image-slot.js contains the prose `tell "never set" from "just deleted"`, which
// the scanner recorded as a module named `just deleted`.
func TestWebImportsRejectsCommentedSpecifiers(t *testing.T) {
	text := `// consumers write import { Gate } from "@/areas/enroll" to reach it.
/*
 * export * from "./ghost";
 */
// the merge below can't tell "never set" from "just deleted" and would
export { Gate } from "./Gate";
`
	assertEdges(t, webEdges(t, "areas/enroll/index.ts", text), "./Gate")
}

// TestWebImportsRejectsQuotedSpecifiers pins the second class. The second
// fixture is the one that produced the most fabrications in the corpus: the
// scanner matched the word `import` INSIDE one string literal and closed on the
// opening quote of the NEXT one, recording the punctuation between them as a
// module name. beamfall-web/test/routes.test.ts:167 is the original.
func TestWebImportsRejectsQuotedSpecifiers(t *testing.T) {
	text := `import { spawnSync } from "node:child_process";
const cases = ['import * as Select from "@radix-ui/react-select";'];
spawnSync(process.execPath, ["--import", "tsx", "server.ts"]);
`
	assertEdges(t, webEdges(t, "test/routes.test.ts", text), "node:child_process")
}

// TestWebImportsRejectsTemplateLiteralSpecifiers pins the third and fourth
// classes together, because a template literal is the one construct whose body
// spans lines: the specifier the scanner recorded for
// beamfall-web/test/tizen-avplay-smoke.ts was the import line of a vite config
// this file GENERATES, and elsewhere in the corpus it was a whole paragraph of
// program text.
func TestWebImportsRejectsTemplateLiteralSpecifiers(t *testing.T) {
	inline := "const one = `import { defineConfig } from 'vite'`;\nimport { real } from \"./real\";\n"
	assertEdges(t, webEdges(t, "test/smoke.ts", inline), "./real")

	spanning := "function viteConfig() {\n  return `import { defineConfig } from 'vite'\nexport default defineConfig({ root: '${root}' })\n`;\n}\nimport { real } from \"./real\";\n"
	assertEdges(t, webEdges(t, "test/tizen-avplay-smoke.ts", spanning), "./real")
}

// TestWebImportsLexesSubstitutionBodiesAsCode pins that a `${...}` body is
// code: a `}` or backtick inside one of its string literals, or a `}` closing
// an object literal inside it, neither ends the substitution nor opens a
// template. Either mis-lex moves the template boundary, so a nested template's
// `import("./ghost")` became live code and a real import after the template was
// swallowed as an unterminated literal (GPK-V0-027(c)).
func TestWebImportsLexesSubstitutionBodiesAsCode(t *testing.T) {
	for _, item := range []struct{ name, text string }{
		{"brace in string", "const s = `${\"}\" + `${import(\"./ghost\")}`}`;\nimport { real } from \"./real\";\n"},
		{"object literal", "const s = `${ {a: 1}.a ?? `${import(\"./ghost\")}` }`;\nimport { real } from \"./real\";\n"},
		{"backtick in string", "const s = `${\"`\"}`;\nimport { real } from \"./real\";\n"},
	} {
		extraction := sourceImports("src/templated.ts", item.text)
		if extraction.refusal != "" {
			t.Fatalf("%s: complete file reported refusal %q", item.name, extraction.refusal)
		}
		assertEdges(t, webEdges(t, "src/templated.ts", item.text), "./real")
	}
}

// TestWebImportsEndsLineCommentAtEveryLineTerminator pins that a `//` comment
// ends at any ECMAScript LineTerminator, not only `\n`: an import after a lone
// `\r`, U+2028, or U+2029 on the same `\n`-line is code, and skipping to `\n`
// dropped its edge (GPK-V0-027(c)).
func TestWebImportsEndsLineCommentAtEveryLineTerminator(t *testing.T) {
	for _, terminator := range []string{"\r", "\r\n", "\u2028", "\u2029"} {
		text := "// note" + terminator + "import { real } from \"./real\";\n// import \"./ghost\";\n"
		assertEdges(t, webEdges(t, "src/terminated.js", text), "./real")
	}
}

// TestWebImportsKeepsLiteralDynamicImportAndDropsComputed records a deliberate
// split. `import("./beta-hooks.js")` names a module statically -- the file it
// names is in the repository, and every ESM loader resolves it at build time --
// so dropping it would be an under-approximation, the one direction the import
// table cannot survive (see goImports, parse.go). `import(expression)` names no
// module the index can know, and contributes nothing rather than guessing.
// integrations/opencode/src/index.js:272 and
// conformance/harness-event-v0/capture_opencode.mjs:4 are the two originals.
func TestWebImportsKeepsLiteralDynamicImportAndDropsComputed(t *testing.T) {
	literal := "const { hooks } = await import(\"./beta-hooks.js\")\n"
	assertEdges(t, webEdges(t, "src/index.js", literal), "./beta-hooks.js")

	computed := "const { Plugin } = await import(pathToFileURL(modulePath))\nconst templated = await import(`./${name}.js`)\n"
	assertEdges(t, webEdges(t, "capture.mjs", computed))
}

// TestWebImportsKeepsEveryStaticForm is the false-negative guard. Removing
// fabricated edges is only an improvement if the real ones survive, and a line
// scanner with no grammar could regress any of these without failing anything
// else in the suite.
func TestWebImportsKeepsEveryStaticForm(t *testing.T) {
	text := `import "./side-effect";
import def from "./default";
import { named } from "./named";
import * as ns from "./namespace";
import type { T } from "./type";
import mixed, { other } from "./mixed";
import {
  first,
  second,
} from "./multiline";
export { x } from "./reexport";
export * from "./star";
export * as grouped from "./starAs";
export type { U } from "./typeReexport";
`
	assertEdges(t, webEdges(t, "src/app.ts", text),
		"./default", "./mixed", "./multiline", "./named", "./namespace", "./reexport",
		"./side-effect", "./star", "./starAs", "./type", "./typeReexport")
}

// TestWebImportsIgnoresNonStatementKeywords pins the two places the words
// `import` and `export` appear without opening a module statement. The method
// call is real: extensions/vscode/src/extension.ts:546 calls
// `this.testing.import(observation)`.
func TestWebImportsIgnoresNonStatementKeywords(t *testing.T) {
	text := `const base = import.meta.url;
this.testing.import(observation);
const table = { import: "./not-a-module", export: "./neither" };
export const value = 1;
`
	assertEdges(t, webEdges(t, "src/extension.ts", text))
}

// TestWebImportsReportsUnterminatedConstructs is the countability guard. A file
// that ends inside a block comment or a template literal had every line after
// the opener consumed as literal text, so its edge set is short. Returning the
// short set silently is indistinguishable from a file that imports nothing,
// which is exactly the loss Index.Unparsed exists to make countable.
func TestWebImportsReportsUnterminatedConstructs(t *testing.T) {
	for _, item := range []struct{ name, text, reason string }{
		{"block comment", "import { a } from \"./a\";\n/* import { b } from \"./b\";\n", noteUnterminatedComment},
		{"template literal", "import { a } from \"./a\";\nconst t = `unclosed\nimport { b } from \"./b\";\n", noteUnterminatedString},
	} {
		extraction := sourceImports("src/broken.ts", item.text)
		if extraction.refusal != item.reason {
			t.Fatalf("%s: refusal = %q, want %q", item.name, extraction.refusal, item.reason)
		}
		if extraction.reason != UnparsedWebLexical {
			t.Fatalf("%s: reason = %q, want %q", item.name, extraction.reason, UnparsedWebLexical)
		}
		if _, kept := extraction.imports["./a"]; !kept {
			t.Fatalf("%s: edges before the opener were dropped: %v", item.name, extraction.imports)
		}
	}
	clean := sourceImports("src/whole.ts", "import { a } from \"./a\";\n")
	if clean.refusal != "" {
		t.Fatalf("a complete file reported refusal %q", clean.refusal)
	}
}

// TestWebImportsReadsRegexLiteralsAfterExpressionOpeners: a `/` after one of
// the punctuators that must start an expression opens a regular expression
// closing on its line, so a backtick or quote inside it opens nothing and the
// import between two such literals keeps its edge without a refusal. After a
// word the `/` stays division.
func TestWebImportsReadsRegexLiteralsAfterExpressionOpeners(t *testing.T) {
	text := "const r = /`[/]\\/'/g;\nimport x from \"./a\";\nconst q = [/`/];\nconst n = total / 2; import y from \"./b\";\n"
	extraction := sourceImports("src/regex.ts", text)
	if extraction.refusal != "" {
		t.Fatalf("refusal = %q, want none", extraction.refusal)
	}
	assertEdges(t, webEdges(t, "src/regex.ts", text), "./a", "./b")
}

// TestSourceImportsKeepsTheScannerOffTheWebPath pins the blast radius. The
// unstructured scanner mirrors the frozen oracle's fallback
// (src/context_corvint_index.py:1213-1217) for every language with no extractor
// of its own, and this change must not have moved any of them.
func TestSourceImportsKeepsTheScannerOffTheWebPath(t *testing.T) {
	text := "// from \"commented/out\"\n"
	scanned := sourceImports("notes/reference.md", text)
	if _, invented := scanned.imports["commented/out"]; !invented {
		t.Fatalf("non-web scanner stopped over-approximating: %v", scanned.imports)
	}
	if !scanned.approximate {
		t.Fatal("scanner-derived edges are not marked approximate")
	}
	if lexed := sourceImports("notes/reference.ts", text); len(lexed.imports) != 0 {
		t.Fatalf("web lexer kept a commented specifier: %v", lexed.imports)
	}
}

// TestWebImportsKeepsStatementAfterDotSpecifier pins that the property-access
// guard (`x.import`) keys on a `.` punctuation token, not on any token spelled
// `.`: a semicolon-free `import a from "."` must not hide the next statement.
func TestWebImportsKeepsStatementAfterDotSpecifier(t *testing.T) {
	text := "import a from \".\"\nimport b from \"./b\"\n"
	assertEdges(t, webEdges(t, "src/index.ts", text), ".", "./b")
}

// TestWebImportsReadsPastBindingNamedFrom pins that a binding spelled `from`
// is a clause word, not the keyword: only a `from` followed by a string
// literal closes the clause, as the oracle's `\bfrom\s+['"]` requires.
func TestWebImportsReadsPastBindingNamedFrom(t *testing.T) {
	text := "import { from } from \"./x\";\nexport { from as f } from \"./y\";\n"
	assertEdges(t, webEdges(t, "src/index.ts", text), "./x", "./y")
}

// TestWebImportsEndsUnclosedQuoteAtCarriageReturn pins that an unclosed `'`/`"`
// literal ends at `\r` as well as `\n`, both forbidden raw inside one, so an
// import after a lone `\r` keeps its edge; U+2028 and U+2029 are legal inside
// a literal and do not end it (GPK-V0-027(c)).
func TestWebImportsEndsUnclosedQuoteAtCarriageReturn(t *testing.T) {
	assertEdges(t, webEdges(t, "src/carriage.js", "const s = 'open\rimport { real } from \"./real\";\n"), "./real")
	for _, separator := range []string{" ", " "} {
		text := "const s = 'a" + separator + "import \"./ghost\";'; import { real } from \"./real\";\n"
		assertEdges(t, webEdges(t, "src/separated.js", text), "./real")
	}
}

// TestWebImportsKeepsQuotedCRLFContinuationInsideTheLiteral pins that a `\`
// before `\r\n` continues a `'`/`"` literal through both bytes, so an import
// spelled in the literal's remainder contributes no edge (GPK-V0-027(c)).
func TestWebImportsKeepsQuotedCRLFContinuationInsideTheLiteral(t *testing.T) {
	text := "const a = 'x\\\r\ny; import z from \"./ghost\"'\r\nimport w from \"./real\"\r\n"
	assertEdges(t, webEdges(t, "src/continued.js", text), "./real")
}
