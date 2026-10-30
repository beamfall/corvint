package typescript

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1604RetestCompilerOptions is the re-test comment's compilerOptions, in its order, with
// `baseUrl` the last key after `paths`.
const gh1604RetestCompilerOptions = `"target": "es2016", "module": "commonjs",
    "paths": { "lib/*": ["./lib/*"] },
    "moduleResolution": "node", "esModuleInterop": true, "allowSyntheticDefaultImports": true,
    "forceConsistentCasingInFileNames": true, "strict": true, "skipLibCheck": true,
    "baseUrl": "."`

// gh1604RetestTSConfigs are the comment's tsconfig.json in two spellings: compact, and the
// `tsc --init` template (comments, commented-out keys, end-of-line block comments) with the
// same keys uncommented and baseUrl appended last.
var gh1604RetestTSConfigs = map[string]string{
	"compact": "{\n  \"compilerOptions\": {\n    " + gh1604RetestCompilerOptions + "\n  },\n  \"include\": [\"**/*.ts\", \"**/*.js\"]\n}\n",
	"tsc --init template": `{
  "compilerOptions": {
    /* Visit https://aka.ms/tsconfig to read more about this file */
    "target": "es2016",                                  /* Set the JavaScript language version for emitted JavaScript and include compatible library declarations. */
    "module": "commonjs",                                /* Specify what module code is generated. */
    // "rootDir": "./",                                  /* Specify the root folder within your source files. */
    "paths": { "lib/*": ["./lib/*"] },                   /* Specify a set of entries that re-map imports to additional lookup locations. */
    "moduleResolution": "node",                          /* Specify how TypeScript looks up a file from a given module specifier. */
    // "baseUrl": "./",                                  /* Specify the base directory to resolve non-relative module names. */
    // "rootDirs": [],                                   /* Allow multiple folders to be treated as one when resolving modules. */
    // "typeRoots": [],                                  /* Specify multiple folders that act like './node_modules/@types'. */
    // "moduleSuffixes": [],                             /* List of file name suffixes to search when resolving a module. */
    "esModuleInterop": true,                             /* Emit additional JavaScript to ease support for importing CommonJS modules. */
    "allowSyntheticDefaultImports": true,                /* Allow 'import x from y' when a module doesn't have a default export. */
    "forceConsistentCasingInFileNames": true,            /* Ensure that casing is correct in imports. */
    "strict": true,                                      /* Enable all strict type-checking options. */
    "skipLibCheck": true,                                /* Skip type checking all .d.ts files. */
    "baseUrl": "."
  },
  "include": ["**/*.ts", "**/*.js"]
}
`,
}

// gh1604Lines returns a module whose line `line` is importLine, after relative and package imports.
func gh1604Lines(line int, importLine, rest string) string {
	lines := []string{}
	for len(lines) < line-1 {
		lines = append(lines, fmt.Sprintf("import { dep%d } from './dep%d';", len(lines), len(lines)%2))
	}
	return strings.Join(append(lines, importLine), "\n") + "\n" + rest
}

// gh1604RetestRepository writes the re-test comment's tree: the tsconfig above, a sibling
// tsconfig.typecheck.json, and the four example imports at their reported lines, each resolving
// to an index file, a plain .ts file or a dotted-basename .ts file. Specs import the changed
// fixture through the alias.
func gh1604RetestRepository(t *testing.T, tsconfig, config string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0","dotenv":"16","tsconfig-paths":"4"}}`)
	write(t, root, "tsconfig.json", tsconfig)
	write(t, root, "tsconfig.typecheck.json", "{\n  \"extends\": \"./tsconfig.json\",\n  \"compilerOptions\": { \"noEmit\": true }\n}\n")
	write(t, root, "playwright.config.ts", config)
	for _, directory := range []string{"lib/api/client", "lib/api/mappers", "lib/data/actions/api", "lib/data/builders"} {
		write(t, root, directory+"/dep0.ts", "export const dep0 = 0;\n")
		write(t, root, directory+"/dep1.ts", "export const dep1 = 1;\n")
	}
	write(t, root, "lib/configs/index.ts", "export const config = {};\n")
	write(t, root, "lib/data/types/index.ts", "export type Cart = {};\n")
	write(t, root, "lib/api/endpoints/TeetimesApiClient.ts", "export class TeetimesApiClient {}\n")
	write(t, root, "lib/helpers/common.helper.ts", "export const h = 1;\n")
	write(t, root, "lib/api/client/BaseApiClient.ts", gh1604Lines(14, "import { config } from 'lib/configs';", "export class BaseApiClient {}\n"))
	write(t, root, "lib/api/mappers/cart.mappers.ts", gh1604Lines(6, "import type { Cart } from 'lib/data/types';", "export const m = 1;\n"))
	write(t, root, "lib/data/actions/api/TeetimeApiDataActions.ts", gh1604Lines(1, "import { TeetimesApiClient } from 'lib/api/endpoints/TeetimesApiClient';", "export const a = 1;\n"))
	write(t, root, "lib/data/builders/club.builder.ts", gh1604Lines(8, "import { h } from 'lib/helpers/common.helper';", "export const b = h;\n"))
	write(t, root, "lib/fixtures/control.ts", "import { test as base } from '@playwright/test';\nimport { BaseApiClient } from 'lib/api/client/BaseApiClient';\nexport const test = base;\n")
	write(t, root, "tests/control.spec.ts", "import { test } from 'lib/fixtures/control';\ntest('control', async () => {});\n")
	write(t, root, "tests/club.spec.ts", "import { test } from 'lib/fixtures/control';\nimport { b } from 'lib/data/builders/club.builder';\nimport { a } from 'lib/data/actions/api/TeetimeApiDataActions';\nimport { m } from 'lib/api/mappers/cart.mappers';\ntest('club', async () => {});\n")
	write(t, root, "tests/other.spec.ts", "import { test } from '@playwright/test';\ntest('other', async () => {});\n")
	return root
}

// TJAA-V0-056 (GitHub #1604 re-test): with the comment's exact tsconfig (baseUrl last, after
// paths; include globs; a sibling tsconfig.typecheck.json that extends it), the four example
// imports resolve as tsc resolves them (directory index, plain .ts, dotted basename .ts), and
// `affected --path lib/fixtures/control.ts` selects the specs importing it through the alias.
func TestTSConfigRetestTreeResolvesAsTsc_GH1604(t *testing.T) {
	for name, tsconfig := range gh1604RetestTSConfigs {
		t.Run(name, func(t *testing.T) {
			root := gh1604RetestRepository(t, tsconfig, gh1604Config)
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			for owner, target := range map[string]string{
				"lib/api/client/BaseApiClient.ts":               "lib/configs/index.ts",
				"lib/api/mappers/cart.mappers.ts":               "lib/data/types/index.ts",
				"lib/data/actions/api/TeetimeApiDataActions.ts": "lib/api/endpoints/TeetimesApiClient.ts",
				"lib/data/builders/club.builder.ts":             "lib/helpers/common.helper.ts",
			} {
				if unit := unitWithPath(result, owner); !slices.Contains(unit.Imports, "typescript:"+target) {
					t.Fatalf("%s does not import %s: %+v", owner, target, unit)
				}
			}
			if frontierHas(result, FrontierPathAlias) {
				t.Fatalf("alias frontier remains: %v", result.Frontier)
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			for dirty, want := range map[string][]string{
				"lib/fixtures/control.ts":                {"tests/club.spec.ts", "tests/control.spec.ts"},
				"lib/configs/index.ts":                   {"tests/club.spec.ts", "tests/control.spec.ts"},
				"lib/helpers/common.helper.ts":           {"tests/club.spec.ts"},
				"lib/data/types/index.ts":                {"tests/club.spec.ts"},
				"lib/api/endpoints/TeetimesApiClient.ts": {"tests/club.spec.ts"},
			} {
				plan := affected.Select(graph, []string{dirty})
				if got := selectedTests(plan); !slices.Equal(got, want) {
					t.Fatalf("%s: selected %v, want %v (unknown %v)", dirty, got, want, plan.Unknown)
				}
			}
			plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"lib/fixtures/control.ts"})
			if err != nil || len(plan.Selected) != 2 {
				t.Fatalf("config mode: err=%v selected=%+v", err, plan.Selected)
			}
		})
	}
}

// TJAA-V0-057, TJAA-V0-058 (GitHub #1604 re-test): 368 per-module sites with none resolved is the
// shape of a blocked alias gate, where each module reported its first alias import and the 16-site
// bound, sorting `lib/...` first, hid the blocking identity and the tsconfig site. Under such a
// gate the tsconfig edges are still added, so the specs are selected; the gate stays a frontier
// whose blocker and tsconfig sites are named ahead of the bound.
func TestBlockedAliasGateSelectsAndNamesBlocker_GH1604(t *testing.T) {
	tsconfig := gh1604RetestTSConfigs["compact"]
	for _, row := range []struct{ name, config, extraPath, extra, blocker string }{
		{"playwright config loads tsconfig-paths", "import 'tsconfig-paths/register';\n" + gh1604Config, "", "", FrontierPathAlias + ":playwright.config.ts:1 `tsconfig`"},
		{"second runner config", gh1604Config, "tools/jest.config.js", "module.exports = { testMatch: ['**/*.unit.js'] };\n", FrontierPathAlias + ":tools/jest.config.js `jest`"},
		{"second runner test", gh1604Config, "scripts/check.test.mjs", "import test from 'node:test';\nimport { h } from 'lib/helpers/common.helper';\ntest('c', () => { void h; });\n", FrontierPathAlias + ":scripts/check.test.mjs `node-test`"},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := gh1604RetestRepository(t, tsconfig, row.config)
			if row.extraPath != "" {
				write(t, root, row.extraPath, row.extra)
			}
			// More missing alias targets than the bound, each sorting before the blocker.
			for index := range 20 {
				write(t, root, fmt.Sprintf("lib/aa/m%02d.ts", index), fmt.Sprintf("import { gone } from 'lib/gone/g%02d';\nexport const m = gone;\n", index))
			}
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{row.blocker, FrontierPathAlias + ":tsconfig.json:4 `\"paths\":`", FrontierPathAlias + ":+4"} {
				if !slices.Contains(result.Frontier, want) {
					t.Fatalf("frontier lacks %q:\n%s", want, strings.Join(result.Frontier, "\n"))
				}
			}
			// The blocker and tsconfig sites precede every ordinary `lib/...` site (TJAA-V0-058).
			firstOrdinary := slices.IndexFunc(result.Frontier, func(entry string) bool {
				return strings.HasPrefix(entry, FrontierPathAlias+":lib/")
			})
			if blocker, tsconfig := slices.Index(result.Frontier, row.blocker), slices.Index(result.Frontier, FrontierPathAlias+":tsconfig.json:4 `\"paths\":`"); firstOrdinary < 0 || blocker > firstOrdinary || tsconfig > firstOrdinary {
				t.Fatalf("pinned sites are not first:\n%s", strings.Join(result.Frontier, "\n"))
			}
			if slices.Contains(result.Frontier, FrontierPathAlias+":lib/api/client/BaseApiClient.ts:14 `lib/configs`") {
				t.Fatalf("resolvable alias import is a site: %v", result.Frontier)
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"lib/fixtures/control.ts"})
			if got := selectedTests(plan); !slices.Equal(got, []string{"tests/club.spec.ts", "tests/control.spec.ts"}) || plan.FrontierSites[FrontierPathAlias] == 0 {
				t.Fatalf("selected %v, alias sites %d (unknown %v)", got, plan.FrontierSites[FrontierPathAlias], plan.Unknown)
			}
			if plan.Scope != affected.ScopeUnknown || !slices.ContainsFunc(plan.Unknown, func(unknown affected.Unknown) bool {
				return unknown.Reason == affected.UnknownLanguageFrontier && unknown.Detail == row.blocker
			}) {
				t.Fatalf("blocked gate plan is not unknown at its blocker: scope %s unknown %v", plan.Scope, plan.Unknown)
			}
		})
	}
}

// TJAA-V0-058: a pinned site is always rendered, ahead of the bound, in the general frontier and in
// the playwright-affected/0 shared-graph detail, and keeps its smallest token.
func TestPinnedFrontierSitesPrecedeTheBound_GH1604(t *testing.T) {
	frontier := frontierSet{}
	for index := range 20 {
		frontier.raiseToken(FrontierPathAlias, fmt.Sprintf("a/m%02d.ts:1", index), "lib/x")
	}
	frontier.raiseToken(FrontierPathAlias, "z.json:2", "paths")
	frontier.pin(FrontierPathAlias, "z.json:2")
	frontier.raiseToken(FrontierPathAlias, "z.json:2", "baseUrl")
	frontier.pin(FrontierPathAlias, "missing")
	entries := locatedFrontier(frontier)
	// The pinned site is first, then the 16 sorted unpinned sites, then the count.
	if len(entries) != 18 || entries[0] != FrontierPathAlias+":z.json:2 `baseUrl`" || entries[1] != FrontierPathAlias+":a/m00.ts:1 `lib/x`" || entries[16] != FrontierPathAlias+":a/m15.ts:1 `lib/x`" || entries[17] != FrontierPathAlias+":+4" {
		t.Fatalf("entries %v", entries)
	}
	if detail := frontierDetail(frontier[FrontierPathAlias]); !strings.HasPrefix(detail, "shared TypeScript graph frontier at z.json:2 `baseUrl`, a/m00.ts:1") || !strings.HasSuffix(detail, " and 12 more") {
		t.Fatalf("detail %q", detail)
	}
	// More pinned sites than the bound (a monorepo of alias tsconfigs and the blocker after them)
	// are all named; only unpinned sites are counted.
	for index := range 17 {
		site := fmt.Sprintf("apps/a%02d/tsconfig.json:3", index)
		frontier.raiseToken(FrontierPathAlias, site, `"paths":`)
		frontier.pin(FrontierPathAlias, site)
	}
	entries = locatedFrontier(frontier)
	if len(entries) != 18+16+1 || entries[0] != FrontierPathAlias+":apps/a00/tsconfig.json:3 `\"paths\":`" || entries[16] != FrontierPathAlias+":apps/a16/tsconfig.json:3 `\"paths\":`" || entries[17] != FrontierPathAlias+":z.json:2 `baseUrl`" || entries[18] != FrontierPathAlias+":a/m00.ts:1 `lib/x`" || entries[34] != FrontierPathAlias+":+4" || slices.Contains(entries, FrontierPathAlias+":a/m16.ts:1 `lib/x`") {
		t.Fatalf("entries %v", entries)
	}
	// Another reason keeps its own group, in sorted reason order.
	frontier.raiseToken(FrontierDynamicImport, "b.ts:1", "import(")
	if entries = locatedFrontier(frontier); entries[0] != FrontierDynamicImport+":b.ts:1 `import(`" || entries[1] != FrontierPathAlias+":apps/a00/tsconfig.json:3 `\"paths\":`" {
		t.Fatalf("entries %v", entries)
	}
	if detail := frontierDetail(frontier[FrontierPathAlias]); !strings.Contains(detail, "z.json:2 `baseUrl`") || !strings.HasSuffix(detail, " and 12 more") {
		t.Fatalf("detail %q", detail)
	}
}
