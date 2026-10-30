package doccorpus

import (
	"slices"
	"testing"
)

var tsConfigSources = map[string]string{
	"src/vite.config.ts": "import { defineConfig } from 'vite'\nexport default defineConfig({\n  root: 'src',\n  optimizeDeps: { include: [] },\n})\n",
	"src/limits.ts":      "export const optimizeLimit = 3;\nexport const named = {\n  retryBudget: 2,\n}\n",
	"src/readme.md":      "# Config\n\nThe optimize step reads deps.\n",
}

// DCS-V1-583 (V1-3073, GitHub #1975): the top-level keys of an exported
// object literal, default or named, are native declarations with their line,
// whether the native provider owns the source or only a records provider
// claims it, and docs_search for a key ranks it above unrelated consts.
func TestDCSV1583ExportedObjectKeysAreDeclarations(t *testing.T) {
	t.Parallel()
	for name, owned := range map[string][]string{"native-owned": {"src/readme.md"}, "records-owned": {"src/vite.config.ts", "src/limits.ts"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, m := ownedSourceCorpus(t, tsConfigSources, owned, false)
			a := buildOwned(t, root, m)
			var ids []string
			for _, s := range subjectsWithPrefix(a, "native:symbol:src/") {
				ids = append(ids, s.ID)
			}
			for _, want := range []string{"native:symbol:src/vite.config.ts:3:root", "native:symbol:src/vite.config.ts:4:optimizeDeps", "native:symbol:src/limits.ts:3:retryBudget"} {
				if !slices.Contains(ids, want) {
					t.Fatalf("declarations %v lack %s", ids, want)
				}
			}
			if slices.Contains(ids, "native:symbol:src/vite.config.ts:4:include") {
				t.Fatalf("a nested key became a declaration: %v", ids)
			}
			for query, want := range map[string]string{"optimizeDeps": "native:symbol:src/vite.config.ts:4:optimizeDeps", "optimize deps": "native:symbol:src/vite.config.ts:4:optimizeDeps", "retryBudget": "native:symbol:src/limits.ts:3:retryBudget"} {
				order := searchOrders(t, a, Request{Operation: "search", Query: query})
				if len(order) == 0 || order[0] != want {
					t.Fatalf("%q: order %v, want %s first", query, order, want)
				}
				if at, unrelated := slices.Index(order, want), slices.Index(order, "native:symbol:src/limits.ts:1:optimizeLimit"); unrelated >= 0 && unrelated < at {
					t.Fatalf("%q: unrelated const ranks above the key: %v", query, order)
				}
			}
		})
	}
}
