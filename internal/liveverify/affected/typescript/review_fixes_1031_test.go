package typescript

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-771 (Codex review): ECMAScript whitespace, not only Go's ASCII `\s`, may precede and
// follow the `from` keyword, so `import value<U+00A0>from "./dep"` stays an edge and a change to
// dep.ts still selects the importing test.
func TestECMAScriptWhitespaceAroundFrom_TJAA_V0_771(t *testing.T) {
	for _, space := range []string{"\u00a0", "\u000b", "\u000c", "\ufeff", "\u2003", "\u3000"} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
		write(t, root, "src/dep.ts", "export const value = 1;\n")
		write(t, root, "src/use.test.ts", "import value"+space+"from"+space+"\"./dep\";\nexport const v = value;\n")
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"src/dep.ts"})
		if !slices.Contains(selectedTests(plan), "src/use.test.ts") {
			t.Errorf("%U: selected %v, want src/use.test.ts through its import", []rune(space)[0], selectedTests(plan))
		}
	}
}

// TJAA-V0-771 (Codex review): with no alias config, an alias-shaped undeclared import (`@/x`,
// `~/x`, `#x`) is only the undeclared-bare-import frontier, never also a path alias.
func TestAliasShapedUndeclaredImportIsNoAlias_TJAA_V0_771(t *testing.T) {
	for _, specifier := range []string{"@/missing", "~/missing", "#missing"} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
		write(t, root, "src/use.test.ts", "import x from \""+specifier+"\";\nexport const v = x;\n")
		_, sites, err := New().units(root, false)
		if err != nil {
			t.Fatal(err)
		}
		if !sites.has(FrontierBareImportUndeclared, "src/use.test.ts:1") {
			t.Errorf("%s: want %s, sites %v", specifier, FrontierBareImportUndeclared, sites)
		}
		if len(sites[FrontierPathAlias]) != 0 {
			t.Errorf("%s: no alias config, yet path alias sites %v", specifier, sites[FrontierPathAlias])
		}
	}
	// A config that claims the specifier keeps the alias reason.
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@/*":["src/*"]}}}`)
	write(t, root, "src/use.test.ts", "import x from \"@/missing\";\nexport const v = x;\n")
	_, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !sites.has(FrontierPathAlias, "src/use.test.ts:1") || sites.has(FrontierBareImportUndeclared, "src/use.test.ts:1") {
		t.Errorf("a claimed alias: sites %v", sites)
	}
}

// TJAA-V0-761 (Codex review): a `<` in an initializer may be a comparison, so a statement whose
// declarator boundaries depend on it attributes its change to no guessed name: the change is
// UNRESOLVED. Annotation generics and a leading type-parameter list still split per name.
func TestComparisonInMultiBindingExportIsUnresolved_TJAA_V0_761(t *testing.T) {
	for _, row := range []struct{ before, after string }{
		{"export const a = 1 < 2, b = 3 > 4, c = 5\n", "export const a = 1 < 2, b = 3 > 5, c = 5\n"},
		{"export const a = x < y, b = 2\n", "export const a = x < y, b = 3\n"},
		{"export const m = new Map<string, number>(), n = 1\n", "export const m = new Map<string, number>(), n = 2\n"},
	} {
		change := ChangedSymbols("m.ts", []byte(row.before), true, []byte(row.after), true)
		if change.State != PlaywrightSymbolsUnresolved {
			t.Errorf("%q: %+v, want UNRESOLVED", row.before, change)
		}
	}
	// An ambiguous statement left unchanged attributes nothing and needs no boundary.
	unchanged := ChangedSymbols("m.ts", []byte("export const a = 1 < 2, b = 3 > 4\nexport const z = 1\n"), true, []byte("export const a = 1 < 2, b = 3 > 4\nexport const z = 2\n"), true)
	if unchanged.State != PlaywrightSymbolsResolved || !slices.Equal(unchanged.Modified, []string{"z"}) {
		t.Errorf("unchanged ambiguous statement: %+v", unchanged)
	}
	// A comparison in the last declarator splits nothing it could hide.
	last := ChangedSymbols("m.ts", []byte("export const a = 1, b = x < y\n"), true, []byte("export const a = 2, b = x < y\n"), true)
	if last.State != PlaywrightSymbolsResolved || !slices.Equal(last.Modified, []string{"a"}) {
		t.Errorf("trailing comparison: %+v", last)
	}
}
