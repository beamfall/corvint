package typescript

import (
	"slices"
	"strings"
	"testing"
)

// TestVanillaTwistedFrontierSitesClassify_TJAA_V0_771 (V1-0970) pins the four classes of the
// VanillaTwisted Playwright run's typescript:path-alias-unresolved sites, none of them a tsconfig
// alias:
//   - an undeclared npm package (`express`, `pg`) is the undeclared-bare-import frontier at its
//     own site;
//   - the string "from" in a one-line exported word list is no import clause, so `, ` is no
//     specifier and raises nothing;
//   - a Vite query import resolves the path before its `?url`/`?raw` query, so a relative one is
//     an edge and a bare one is an undeclared bare import named without the query;
//   - a specifier a tsconfig paths pattern claims but cannot map keeps the alias frontier.
func TestVanillaTwistedFrontierSitesClassify_TJAA_V0_771(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["src/*"]}}}`)
	write(t, root, "e2e/server.test.ts", "import express from \"express\";\nimport { Client } from \"pg\";\nexport const s = [express, Client];\n")
	write(t, root, "src/wordlist.ts", "export const words = [\"frog\", \"from\", \"front\"];\n")
	write(t, root, "src/wasm.test.ts", "import wasmUrl from \"corecrypto-wasm-url?url\";\nimport raw from \"./data.txt?raw\";\nimport { words } from \"./wordlist?inline\";\nexport const u = [wasmUrl, raw, words];\n")
	write(t, root, "src/data.txt", "data\n")
	write(t, root, "src/alias.test.ts", "import { gone } from \"@app/gone\";\nexport const g = gone;\n")
	result, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	for site, reason := range map[string]string{
		"e2e/server.test.ts:1": FrontierBareImportUndeclared,
		"src/wasm.test.ts:1":   FrontierBareImportUndeclared,
		"src/alias.test.ts:1":  FrontierPathAlias,
	} {
		if !sites.has(reason, site) {
			t.Errorf("%s: want %s, sites %v", site, reason, sites)
		}
	}
	for reason, entries := range sites {
		for site, token := range entries {
			if siteFile(site) == "src/wordlist.ts" || token == ", " || token == "corecrypto-wasm-url?url" {
				t.Errorf("%s site %s token %q is a scanner artefact", reason, site, token)
			}
			// The blocked alias gate pins the tsconfig and its blocker as alias sites (TJAA-V0-058).
			if reason == FrontierPathAlias && siteFile(site) != "src/alias.test.ts" && !strings.HasPrefix(token, pinnedSite) {
				t.Errorf("a non-alias site is reported as a path alias: %s %q", site, token)
			}
		}
	}
	for _, unit := range result.Units {
		if slices.Contains(unit.Tests, "src/wasm.test.ts") && !slices.Contains(unit.Imports, "typescript:src/wordlist.ts") {
			t.Errorf("the ?inline import is no edge to src/wordlist.ts: %+v", unit)
		}
	}
}
