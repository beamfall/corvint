package typescript

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1604Repository writes the GitHub #1604 shape under prefix: a tsconfig `paths` alias
// `lib/*` -> `./lib/*` (with the given compilerOptions), a Playwright config, the three changed
// helpers, and specs that import them through the alias directly or through a relative import
// from another module.
func gh1604Repository(t *testing.T, prefix, compilerOptions, config, manifest string) string {
	t.Helper()
	root := t.TempDir()
	at := func(relative string) string { return strings.TrimPrefix(prefix+"/"+relative, "/") }
	write(t, root, at("package.json"), manifest)
	write(t, root, at("tsconfig.json"), "{\n  \"compilerOptions\": "+compilerOptions+"\n}\n")
	write(t, root, at("playwright.config.ts"), config)
	write(t, root, at("lib/fixtures/control.ts"), "import { test as base } from '@playwright/test';\nexport const test = base;\n")
	write(t, root, at("lib/helpers/TestTimingLogger.ts"), "export class TestTimingLogger {}\n")
	write(t, root, at("lib/api/client/ApiLogger.ts"), "export class ApiLogger {}\n")
	write(t, root, at("lib/api/client/ApiClient.ts"), "import { ApiLogger } from './ApiLogger';\nexport class ApiClient { logger = new ApiLogger(); }\n")
	write(t, root, at("tests/control.spec.ts"), "import { test } from 'lib/fixtures/control';\nimport { TestTimingLogger } from 'lib/helpers/TestTimingLogger';\ntest('control', async () => { new TestTimingLogger(); });\n")
	write(t, root, at("tests/client.spec.ts"), "import { test } from '@playwright/test';\nimport { ApiClient } from 'lib/api/client/ApiClient';\ntest('client', async () => { new ApiClient(); });\n")
	write(t, root, at("tests/logger.spec.ts"), "import { test } from '@playwright/test';\nimport { ApiLogger } from 'lib/api/client/ApiLogger';\ntest('logger', async () => { new ApiLogger(); });\n")
	write(t, root, at("tests/other.spec.ts"), "import { test } from '@playwright/test';\ntest('other', async () => {});\n")
	return root
}

const gh1604Config = "import { defineConfig, devices } from '@playwright/test';\n// Aliases come from tsconfig.json paths.\nexport default defineConfig({\n  testDir: './tests',\n  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],\n});\n"

func selectedTests(plan affected.Plan) []string {
	tests := []string{}
	for _, selection := range plan.Selected {
		tests = append(tests, selection.Tests...)
	}
	slices.Sort(tests)
	return tests
}

// TJAA-V0-050, TJAA-V0-051 (GitHub #1604): specs that import a changed module through a tsconfig
// `paths` alias, with or without `baseUrl`, directly or through a relative import from another
// module, are selected in --path mode. Targets resolve relative to the tsconfig's directory, a
// comment that mentions tsconfig and a dependency-only second runner do not block resolution.
func TestTSConfigPathAliasesSelectSpecs_GH1604(t *testing.T) {
	playwrightOnly := `{"devDependencies":{"@playwright/test":"1.61.0","dotenv":"16"}}`
	withJest := `{"devDependencies":{"@playwright/test":"1.61.0","jest":"29"}}`
	for _, row := range []struct{ name, prefix, options, manifest string }{
		{"paths without baseUrl", "", `{ "paths": { "lib/*": ["./lib/*"] }, "moduleResolution": "node" }`, playwrightOnly},
		{"paths with baseUrl", "", `{ "baseUrl": ".", "paths": { "lib/*": ["./lib/*"] }, "moduleResolution": "node" }`, playwrightOnly},
		{"nested tsconfig directory", "e2e", `{ "paths": { "lib/*": ["./lib/*"] } }`, playwrightOnly},
		{"dependency-only second runner", "", `{ "paths": { "lib/*": ["./lib/*"] } }`, withJest},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := gh1604Repository(t, row.prefix, row.options, gh1604Config, row.manifest)
			at := func(relative string) string { return strings.TrimPrefix(row.prefix+"/"+relative, "/") }
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatal(err)
			}
			for dirty, want := range map[string][]string{
				"lib/fixtures/control.ts":         {at("tests/control.spec.ts")},
				"lib/helpers/TestTimingLogger.ts": {at("tests/control.spec.ts")},
				"lib/api/client/ApiLogger.ts":     {at("tests/client.spec.ts"), at("tests/logger.spec.ts")},
			} {
				plan := affected.Select(graph, []string{at(dirty)})
				if got := selectedTests(plan); !slices.Equal(got, want) {
					t.Fatalf("%s: selected %v, want %v (unknown %v)", dirty, got, want, plan.Unknown)
				}
				if plan.FrontierSites[FrontierPathAlias] != 0 {
					t.Fatalf("%s: alias frontier remains: %v", dirty, plan.Unknown)
				}
			}
		})
	}
}

// TJAA-V0-051: an alias whose target does not exist at the observed revision is never guessed:
// the spec keeps no edge and the frontier names the spec line and the specifier it spelled.
func TestTSConfigPathAliasMissingTargetStaysFrontier_GH1604(t *testing.T) {
	root := gh1604Repository(t, "", `{ "paths": { "lib/*": ["./lib/*"] } }`, gh1604Config, `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "tests/missing.spec.ts", "import { test } from '@playwright/test';\nimport { Gone } from 'lib/helpers/Gone';\ntest('m', async () => {});\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := FrontierPathAlias + ":tests/missing.spec.ts:2 `lib/helpers/Gone`"
	if !slices.Contains(result.Frontier, want) {
		t.Fatalf("frontier %v lacks %q", result.Frontier, want)
	}
	if spec := testUnit(result, "tests/missing.spec.ts"); spec.ID == "" || slices.ContainsFunc(spec.Imports, func(id string) bool { return strings.Contains(id, "lib/") }) {
		t.Fatalf("missing alias target was guessed: %+v", spec)
	}
}

// TJAA-V0-050, TJAA-V0-052, TJAA-V0-053 (GitHub #1604): every typescript:* frontier site names
// file:line (or the file) and the specifier or token, including path-alias-unresolved,
// e2e-runtime-dependency, executable-config-unresolved and scan-unresolved; the identity that kept
// aliases unresolved and an unsupported tsconfig key are sites too (TJAA-V0-057).
func TestFrontierSitesNameFileLineAndToken_GH1604(t *testing.T) {
	root := gh1604Repository(t, "", `{ "paths": { "lib/*": ["./lib/*"] } }`, "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  testDir: './tests', testMatch: '**/*.spec.ts',\n  projects: [{ name: 'chromium' }],\n});\n", `{"devDependencies":{"@playwright/test":"1.61.0","vitest":"3"}}`)
	write(t, root, "lib/unit/math.test.ts", "import { test } from 'vitest';\nimport { TestTimingLogger } from 'lib/helpers/TestTimingLogger';\ntest('u', () => { void TestTimingLogger; });\n")
	write(t, root, "lib/helpers/slash.ts", "import x from './x';\nif (c) {}\n/'/.test(s); const u = '//'; const b = require('./b');\n")
	// A package `extends` is refused; an inside-root relative one resolves (TJAA-V0-114).
	write(t, root, "nested/tsconfig.json", "{\n  \"extends\": \"@tsconfig/node20/tsconfig.json\"\n}\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		FrontierPathAlias + ":tsconfig.json:2 `\"paths\":`",
		FrontierPathAlias + ":lib/unit/math.test.ts `vitest`",
		FrontierPathAlias + ":nested/tsconfig.json:2 `extends`",
		FrontierE2ERuntimeDependency + ":tests/other.spec.ts:1 `@playwright/test`",
		FrontierConfig + ":playwright.config.ts:3 `testMatch`",
		FrontierScanUnresolved + ":lib/helpers/slash.ts:3 `/'/.test(s);`",
		FrontierUnresolvedImport + ":lib/helpers/slash.ts:1 `./x`",
	} {
		if !slices.Contains(result.Frontier, want) {
			t.Fatalf("frontier lacks %q:\n%s", want, strings.Join(result.Frontier, "\n"))
		}
	}
	// TJAA-V0-057: under the blocked gate the imports tsconfig resolves are edges, not sites.
	for _, resolved := range []string{FrontierPathAlias + ":tests/control.spec.ts:1 `lib/fixtures/control`", FrontierPathAlias + ":tests/client.spec.ts:2 `lib/api/client/ApiClient`"} {
		if slices.Contains(result.Frontier, resolved) {
			t.Fatalf("resolved alias import is still a site: %q", resolved)
		}
	}
	if spec := testUnit(result, "tests/control.spec.ts"); !slices.Contains(spec.Imports, "typescript:lib/fixtures/control.ts") {
		t.Fatalf("blocked gate dropped the tsconfig edge: %+v", spec)
	}
	for _, entry := range result.Frontier {
		if !strings.Contains(entry, ":") || strings.Count(entry, ":") < 2 {
			t.Fatalf("bare frontier entry %q", entry)
		}
	}

	// With aliases resolved, an unsupported nearer tsconfig names its refusing key.
	root = gh1604Repository(t, "", `{ "paths": { "lib/*": ["./lib/*"] } }`, gh1604Config, `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "tests/tsconfig.json", "{\n  \"extends\": \"@tsconfig/node20/tsconfig.json\"\n}\n")
	result, err = New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := FrontierPathAlias + ":tests/tsconfig.json:2 `extends`"; !slices.Contains(result.Frontier, want) {
		t.Fatalf("frontier lacks %q: %v", want, result.Frontier)
	}

	// A real Playwright tsconfig option still blocks, named at its line.
	root = gh1604Repository(t, "", `{ "paths": { "lib/*": ["./lib/*"] } }`, "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  tsconfig: './tsconfig.e2e.json',\n});\n", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	result, err = New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := FrontierPathAlias + ":playwright.config.ts:3 `tsconfig`"; !slices.Contains(result.Frontier, want) {
		t.Fatalf("frontier lacks %q: %v", want, result.Frontier)
	}
}

// TJAA-V0-050: only a comment that cannot hide code is ignored when looking for a tsconfig option.
func TestPlaywrightConfigTSConfigToken_GH1604(t *testing.T) {
	for body, want := range map[string]bool{
		"// paths come from tsconfig.json\nexport default {}\n":                                         false,
		"/* tsconfig */ export default {}\n":                                                            false,
		"export default { tsconfig: './t.json' }\n":                                                     true,
		"const f = 'tsconfig.json'\nexport default {}\n":                                                true,
		"if (process.env.CI) /\\/\\//.test(x); export default defineConfig({ tsconfig: './t.json' })\n": true,
		"const t = `unterminated\n// tsconfig\n":                                                        true,
	} {
		if got := tsconfigToken(body) >= 0; got != want {
			t.Fatalf("%q: tsconfig found=%v, want %v", body, got, want)
		}
	}
}

// TJAA-V0-052: the general adapter names at most 16 sites per reason and counts the rest in one
// `reason:+N` entry, which the plan's frontierSites counts in full.
func TestFrontierSitesAreBoundedPerReason_GH1604(t *testing.T) {
	root := playwrightFixture(t)
	for index := range 20 {
		write(t, root, fmt.Sprintf("tests/many%02d.spec.ts", index), "import { test } from '@playwright/test'\ntest('@chromium m', async () => {})\n")
	}
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	named, extra := 0, ""
	for _, entry := range result.Frontier {
		if rest, found := strings.CutPrefix(entry, FrontierE2ERuntimeDependency+":"); found {
			if strings.HasPrefix(rest, "+") {
				extra = rest
			} else {
				named++
			}
		}
	}
	if named != frontierSiteBound || extra == "" {
		t.Fatalf("named=%d extra=%q frontier=%v", named, extra, result.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	_, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	total := len(sites[FrontierE2ERuntimeDependency])
	if sites := affected.Select(graph, []string{"tests/pages/login.ts"}).FrontierSites[FrontierE2ERuntimeDependency]; sites != total || extra != "+"+fmt.Sprint(total-frontierSiteBound) {
		t.Fatalf("frontierSites=%d extra=%s tests=%d", sites, extra, total)
	}
}
