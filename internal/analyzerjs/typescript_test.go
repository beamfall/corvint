package analyzerjs

import "testing"

// TypeScript is not a separate candidate: `docs/specs/analyzer-candidate-profiles.md`
// binds one family, `javascript-typescript`, to this package, and its `js.source`
// row admits `.ts`, `.tsx`, `.mts`, and `.cts` alongside the JavaScript suffixes.
// These tests pin what that already-shipped family actually extracts from
// TypeScript source, so the coverage claim rests on discriminating assertions
// rather than on the pinned parity corpus, which exercises almost none of it.

// TestTypeScriptDeclarationSyntaxStaysInert pins the TypeScript-only grammar the
// bounded tokenizer must traverse without losing the static `require` that
// follows it. Type syntax never binds a module, so every case yields exactly the
// one specifier and nothing the annotation happened to mention.
func TestTypeScriptDeclarationSyntaxStaysInert(t *testing.T) {
	for name, source := range map[string]string{
		"interface-declaration":          `interface I { a: string; } require("fs");`,
		"generic-interface-heritage":     `interface I<T> extends B<T> {} require("fs");`,
		"type-alias":                     `type T = { a: string }; require("fs");`,
		"union-type":                     `let u: A | B | C; require("fs");`,
		"intersection-type":              `let u: A & B; require("fs");`,
		"tuple-labelled-type":            `let t: [a: string, b: number]; require("fs");`,
		"readonly-array-type":            `let a: readonly string[]; require("fs");`,
		"keyof-typeof-query":             `type K = keyof typeof obj; require("fs");`,
		"conditional-infer-type":         `type E<T> = T extends Array<infer U> ? U : never; require("fs");`,
		"mapped-type":                    `type M = { [K in keyof T]: T[K] }; require("fs");`,
		"template-literal-type":          "type T = `a${B}c`; require(\"fs\");",
		"generic-class-heritage":         `class C<T> extends B<T> {} require("fs");`,
		"generic-function-decl":          `function f<T>(x: T): T { return x; } require("fs");`,
		"generic-method":                 `class C { m<T>(x: T) {} } require("fs");`,
		"call-site-type-argument":        `const v = fn<number>(1); require("fs");`,
		"optional-parameter":             `function f(a?: string) {} require("fs");`,
		"definite-assignment":            `let x!: number; require("fs");`,
		"non-null-assertion":             `const y = x!.foo; require("fs");`,
		"const-assertion":                `const x = [1] as const; require("fs");`,
		"satisfies-operator":             `const c = {} satisfies R; require("fs");`,
		"accessor-modifiers":             `class C { private readonly x = 1; } require("fs");`,
		"constructor-parameter-property": `class C { constructor(private x: number) {} } require("fs");`,
		"abstract-member":                `abstract class C { abstract m(): void; } require("fs");`,
		"overload-signatures":            `function f(a: string): void; function f(a: any): void {} require("fs");`,
		"const-enum":                     `const enum E { A } require("fs");`,
		"nested-namespace":               `namespace A.B { export const x = 1; } require("fs");`,
		"declare-global":                 `declare global { interface W {} } require("fs");`,
		"module-augmentation":            `declare module "m" { interface I {} } require("fs");`,
		"parameter-decorator":            `class C { m(@Inject() x: T) {} } require("fs");`,
		"export-assignment":              `export = Foo; require("fs");`,
		"triple-slash-directive":         "/// <reference types=\"node\" />\nrequire(\"fs\");",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 1 || imports[0] != "fs" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
}

// TestTypeScriptTypeOnlyImportsRemainStaticSpecifiers pins that `import type`
// and its inline spelling still yield the specifier. Erasing them would silently
// drop real module edges: the form is a compile-time-only import of a real file,
// and `js.import.static` records the specifier, not the binding's runtime fate.
func TestTypeScriptTypeOnlyImportsRemainStaticSpecifiers(t *testing.T) {
	for name, source := range map[string]string{
		"import-type-named":     `import type { Foo } from "m";`,
		"import-type-default":   `import type Foo from "m";`,
		"import-type-namespace": `import type * as ns from "m";`,
		"inline-type-specifier": `import { type Foo, Bar } from "m";`,
		"export-type-from":      `export type { Foo } from "m";`,
		"import-equals-require": `import foo = require("m");`,
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 1 || imports[0] != "m" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
}

// TestTypeScriptExpressionLeadingAngleFailsClosed pins the one TypeScript
// production this tokenizer does not lex. At expression start `<` opens either a
// JSX element or a type-parameter list, and the two are separable only by
// unbounded lookahead past the closing `>` for a parenthesized parameter list and
// an arrow. Until that lookahead exists the whole request fails closed, which
// ACP-004 requires of ambiguous data; it never yields a partial or wrong fact.
//
// These assertions are a boundary, not a desired behaviour. A future change that
// adds the disambiguation must update this test deliberately rather than
// discover the ambiguity by moving receipt bytes.
func TestTypeScriptExpressionLeadingAngleFailsClosed(t *testing.T) {
	for name, source := range map[string]string{
		"generic-arrow-trailing-comma": `const f = <T,>(x: T) => x; require("fs");`,
		"generic-arrow-extends-bound":  `const f = <T extends object>(x: T) => x; require("fs");`,
		"legacy-angle-type-assertion":  `const y = <string>x; require("fs");`,
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != errLexicalInput {
				t.Fatalf("expected fail-closed errLexicalInput, imports=%v err=%v", imports, err)
			}
		})
	}
}

// TestTypeScriptSourceClassification pins the `js.source` classify value for every
// suffix the family admits. The spec's fact matrix closes this value to exactly
// `javascript`, `typescript`, or `test`, so a new suffix cannot invent a fourth.
func TestTypeScriptSourceClassification(t *testing.T) {
	for path, want := range map[string]string{
		"src/a.js":       "javascript",
		"src/a.jsx":      "javascript",
		"src/a.mjs":      "javascript",
		"src/a.cjs":      "javascript",
		"src/a.ts":       "typescript",
		"src/a.tsx":      "typescript",
		"src/a.mts":      "typescript",
		"src/a.cts":      "typescript",
		"src/a.test.ts":  "test",
		"src/a.spec.tsx": "test",
		"src/a.test.js":  "test",
	} {
		t.Run(path, func(t *testing.T) {
			candidate, err := Analyze(candidateRequest(input("input-1", "js.source", path, `import "m";`)))
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			var classify *Fact
			for index := range candidate.Facts {
				if candidate.Facts[index].Kind == "js.source" {
					classify = &candidate.Facts[index]
				}
			}
			if classify == nil {
				t.Fatalf("no js.source fact in %#v", candidate.Facts)
			}
			if classify.Predicate != "classifies" || classify.Value != want || classify.Subject != path || classify.InstanceID != path {
				t.Fatalf("fact=%#v want value %q", *classify, want)
			}
		})
	}
}

// TestNextAppRouterSegmentsRejectAtPathGrammar pins that Next.js app-router
// syntax is rejected by the closed logical-path grammar, which admits only
// `[A-Za-z0-9._@+~-]{1,128}` per segment. Route groups `(marketing)`,
// interception `(.)photo`, and dynamic segments `[id]` all carry bytes outside
// that set. A parallel route `@modal` uses only permitted bytes and is admitted.
//
// This is a deliberate boundary, not an oversight. Admitting these paths would
// require widening a path grammar shared by every family, and the facts they
// would enable — route identity — have no tuple in the closed `js.*` fact matrix.
func TestNextAppRouterSegmentsRejectAtPathGrammar(t *testing.T) {
	for name, logical := range map[string]string{
		"interception-and-dynamic": "app/@modal/(.)photo/[id]/default.tsx",
		"dynamic-segment":          "app/[id]/page.tsx",
		"route-group":              "app/(marketing)/page.tsx",
	} {
		t.Run(name, func(t *testing.T) {
			if logicalPath(logical) {
				t.Fatalf("logical path %q unexpectedly admitted", logical)
			}
			_, err := Analyze(candidateRequest(input("input-1", "js.source", logical, `import "m";`)))
			if FailureReason(err) != "INVALID_PATH" {
				t.Fatalf("reason=%v err=%v", FailureReason(err), err)
			}
		})
	}
	for _, logical := range []string{"app/page.tsx", "src/app/layout.tsx", "app/photo/default.mts", "app/@modal/default.tsx"} {
		if !logicalPath(logical) {
			t.Fatalf("ordinary app path %q unexpectedly rejected", logical)
		}
	}
}

// TestPathSegmentsMayStartWithSegmentPunctuation proves a segment may start
// with any byte of `[A-Za-z0-9._@+~-]`, unlike an identifier, while `.`, `..`,
// and bytes outside that set still reject.
func TestPathSegmentsMayStartWithSegmentPunctuation(t *testing.T) {
	for _, logical := range []string{"pages/_app.tsx", ".storybook/main.ts", "src/__tests__/a.test.ts", "a/-b/@c/+d/~e.ts"} {
		if _, err := Analyze(candidateRequest(input("input-1", "js.source", logical, `import "m";`))); err != nil {
			t.Fatalf("path %q: err=%v", logical, err)
		}
	}
	for _, logical := range []string{"./a.ts", "a/../b.ts", "a/$b.ts", "a/!b.ts", "a/:b.ts"} {
		if logicalPath(logical) {
			t.Fatalf("path %q unexpectedly admitted", logical)
		}
	}
}

// TestTypeScriptCandidateFactsEndToEnd proves the family emits both permitted
// `js.source` tuples for a TypeScript unit through the public entry point, with
// imports sorted and bound to the input handle that produced them.
func TestTypeScriptCandidateFactsEndToEnd(t *testing.T) {
	source := "import type { A } from \"zeta\";\nimport { b } from \"alpha\";\nexport type { C } from \"middle\";\n"
	candidate, err := Analyze(candidateRequest(input("input-1", "js.source", "src/app.ts", source)))
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(candidate.Facts) != 4 {
		t.Fatalf("facts=%#v", candidate.Facts)
	}
	want := []struct{ kind, value string }{
		{"js.import.static", "alpha"},
		{"js.import.static", "middle"},
		{"js.import.static", "zeta"},
		{"js.source", "typescript"},
	}
	for index, expected := range want {
		got := candidate.Facts[index]
		if got.Kind != expected.kind || got.Value != expected.value {
			t.Fatalf("fact %d = %#v want %v", index, got, expected)
		}
		if got.InputHandle != "input-1" || got.RelatedHandle != "-" {
			t.Fatalf("fact %d witness = %#v", index, got)
		}
	}
}
