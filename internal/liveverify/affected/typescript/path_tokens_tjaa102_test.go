package typescript

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1751GenericRoot is a Vitest repository that declares docs/ as documentation
// (AFP-V0-160), with one test whose body is reader and one test that names no docs path.
func gh1751GenericRoot(t *testing.T, reader string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, ".corvint/documentation-paths.json", `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`)
	write(t, root, "docs/contracts/_setup.json", `{"v":1}`)
	write(t, root, "docs/guide.md", "# guide\n")
	write(t, root, "tests/contract.test.ts", reader)
	write(t, root, "tests/other.test.ts", "import { test } from 'vitest'\ntest('other', () => {})\n")
	return root
}

// GitHub #1751 (V1-2661, TJAA-V0-102): the generic affected plan selects a TypeScript test that
// reads a declared documentation path through a string literal, as it selects a Go or shell
// reader, instead of classifying the path as documentation that no unit reads.
func TestGenericPlanSelectsTypeScriptDocumentationReader_GH1751(t *testing.T) {
	const head = "import { readFileSync } from 'node:fs'\nimport { test } from 'vitest'\n"
	for _, tc := range []struct {
		name, reader string
		selected     bool
	}{
		{"single-quoted literal", head + "test('c', () => readFileSync('docs/contracts/_setup.json'))\n", true},
		{"double-quoted literal", head + `test('c', () => readFileSync("./docs/contracts/_setup.json"))` + "\n", true},
		{"template text before a substitution", head + "const name = '_setup'\ntest('c', () => readFileSync(`docs/contracts/${name}.json`))\n", true},
		{"escaped separators", head + `test('c', () => readFileSync('docs\/contracts\/_setup.json'))` + "\n", true},
		{"regular-expression contents", head + "test('c', () => expect(/contracts\\/_setup\\.json/.test(p)).toBe(true))\n", true},
		{"file name alone", head + "test('c', () => readFileSync(join(dir, '_setup.json')))\n", true},
		{"line comment only", head + "// docs/contracts/_setup.json\ntest('c', () => {})\n", false},
		{"block comment only", head + "/* docs/contracts/_setup.json */\ntest('c', () => {})\n", false},
		{"other docs path", head + "test('c', () => readFileSync('docs/guide.md'))\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := gh1751GenericRoot(t, tc.reader)
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"docs/contracts/_setup.json"})
			var got []string
			for _, selection := range plan.Selected {
				got = append(got, strings.Join(selection.Tests, ","))
				if selection.Witness.Kind != affected.WitnessPathLiteralReader || selection.Witness.DirtyPath != "docs/contracts/_setup.json" {
					t.Fatalf("witness %+v", selection.Witness)
				}
			}
			var want []string
			if tc.selected {
				want = []string{"tests/contract.test.ts"}
			}
			if !reflect.DeepEqual(got, want) || len(plan.Unknown) != 0 || plan.Scope != affected.ScopeBounded {
				t.Fatalf("selected=%v want %v; plan %+v", got, want, plan)
			}
			if len(plan.NonSource) != 1 || plan.NonSource[0].Class != affected.ClassDeclaredDocumentation {
				t.Fatalf("nonSource %+v", plan.NonSource)
			}
		})
	}
}

// A TypeScript unit whose distinct path tokens exceed the bound keeps none, and a dirty path the
// plan reaches by no other rule names it as a path-token-bound frontier (TJAA-V0-102, AFP-V0-021).
func TestTypeScriptPathTokensBoundRefusesWithNamedUnknown_GH1751(t *testing.T) {
	previous := maxPathTokens
	maxPathTokens = 2
	t.Cleanup(func() { maxPathTokens = previous })
	root := gh1751GenericRoot(t, "import { test } from 'vitest'\nconst a = ['a/b.json', 'c/d.json', 'e/f.json']\ntest('c', () => {})\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/contracts/_setup.json"})
	want := "typescript:" + affected.PathTokenBound + ":" + unitID(runnerVitest, "tests/contract.test.ts")
	found := false
	for _, unknown := range plan.Unknown {
		found = found || unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == want
	}
	if !found || plan.Scope != affected.ScopeUnknown {
		t.Fatalf("want unknown %s; plan %+v", want, plan)
	}
}

// pathTokens reads string, template-text and escape-decoded contents only, never comments or
// code, and keeps path-shaped tokens (TJAA-V0-102).
func TestTypeScriptPathTokensReadLiteralContentsOnly_TJAA102(t *testing.T) {
	body := "import x from './x.js' // see docs/a.md\n/* b/c.md */\nconst p = obj.dir/2\nconst s = 'docs\\/e.md', t = `f/${g}/h.json`, n = '1.5', w = 'word', r = /docs\\/r\\.md/g.test(p)\n"
	got, bounded := pathTokens(body)
	want := []string{"./x.js", "/h.json", "docs/e.md", "docs/r.md", "f/"}
	if bounded || !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens %v bounded=%v, want %v", got, bounded, want)
	}
}
