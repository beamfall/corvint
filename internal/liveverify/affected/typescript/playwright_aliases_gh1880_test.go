package typescript

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-113 (GitHub #1880, V1-2780): a `../` baseUrl or paths target that stays inside the
// repository root is accepted; one that escapes it, or an absolute value, stays refused.
func TestTypeScriptAliasesAcceptInsideRootParents_GH1880(t *testing.T) {
	for _, tc := range []struct {
		name, body, base, refusal string
		valid                     bool
	}{
		{"inside baseUrl", `{"compilerOptions":{"baseUrl":"../shared","paths":{"@s/*":["src/*"]}}}`, "packages/shared", "", true},
		{"inside paths", `{"compilerOptions":{"paths":{"@s/*":["../shared/src/*"]}}}`, "packages/web", "", true},
		{"escaping baseUrl", `{"compilerOptions":{"baseUrl":"../../../out"}}`, "", "baseUrl", false},
		{"absolute baseUrl", `{"compilerOptions":{"baseUrl":"/abs"}}`, "", "baseUrl", false},
		{"escaping paths", `{"compilerOptions":{"paths":{"@s/*":["../../../x/*"]}}}`, "", "@s/*", false},
	} {
		got := parseTypeScriptAliases("packages/web/tsconfig.json", tc.body)
		if got.valid != tc.valid || got.refusal != tc.refusal || tc.valid && got.base != tc.base {
			t.Errorf("%s: valid=%v refusal=%q base=%q", tc.name, got.valid, got.refusal, got.base)
		}
	}
}

// TJAA-V0-114: a relative `extends` chain inside the root is followed with TypeScript's
// inheritance, a cycle or unresolved target is refused as "extends", and a chain longer than
// typeScriptExtendsBound is refused with the bound named at the extends key.
func TestTypeScriptAliasesFollowExtendsChain_GH1880(t *testing.T) {
	bodies := map[string]string{
		"tsconfig.base.json":          `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["src/*"]}}}`,
		"packages/web/tsconfig.json":  `{"extends":"../../tsconfig.base.json"}`,
		"packages/api/tsconfig.json":  `{"extends":"../../tsconfig.base","compilerOptions":{"paths":{"@api/*":["lib/*"]}}}`,
		"packages/cli/tsconfig.json":  `{"extends":"../../tsconfig.base.json","compilerOptions":{"baseUrl":"."}}`,
		"packages/pkg/tsconfig.json":  `{"extends":"@tsconfig/node20/tsconfig.json"}`,
		"packages/miss/tsconfig.json": `{"extends":"../../missing.json"}`,
		"packages/out/tsconfig.json":  `{"extends":"../../../outside.json"}`,
		"cycle/a.json":                `{"extends":"./b.json"}`,
		"cycle/b.json":                `{"extends":"./a.json"}`,
	}
	for _, tc := range []struct {
		name, base, refusal string
		paths               []string
	}{
		{"packages/web/tsconfig.json", ".", "", []string{"@app/*"}},
		// paths declared without a baseUrl in the child resolve against the inherited baseUrl.
		{"packages/api/tsconfig.json", ".", "", []string{"@api/*"}},
		// the child's own baseUrl wins; inherited paths resolve against it.
		{"packages/cli/tsconfig.json", "packages/cli", "", []string{"@app/*"}},
		{"packages/pkg/tsconfig.json", "", "extends", nil},
		{"packages/miss/tsconfig.json", "", "extends", nil},
		{"packages/out/tsconfig.json", "", "extends", nil},
		{"cycle/a.json", "", "extends", nil},
	} {
		got := parseTypeScriptAliasesIn(tc.name, bodies)
		if got.valid != (tc.refusal == "") || got.refusal != tc.refusal || got.valid && (got.base != tc.base || !slices.Equal(sortedKeys(got.paths), tc.paths)) {
			t.Errorf("%s: valid=%v refusal=%q base=%q paths=%v", tc.name, got.valid, got.refusal, got.base, sortedKeys(got.paths))
		}
	}
	// A chain of exactly the bound resolves; one hop more is refused with the bound named.
	chain := map[string]string{}
	for depth := range typeScriptExtendsBound + 2 {
		chain[fmt.Sprintf("c%d/tsconfig.json", depth)] = fmt.Sprintf(`{"extends":"../c%d/tsconfig.json"}`, depth+1)
	}
	chain[fmt.Sprintf("c%d/tsconfig.json", typeScriptExtendsBound+2)] = `{"compilerOptions":{"baseUrl":"."}}`
	if got := parseTypeScriptAliasesIn(fmt.Sprintf("c%d/tsconfig.json", 2), chain); !got.valid || got.base != "c10" {
		t.Errorf("chain at the bound: valid=%v refusal=%q base=%q", got.valid, got.refusal, got.base)
	}
	if got := parseTypeScriptAliasesIn("c0/tsconfig.json", chain); got.valid || got.refusal != "extends depth bound 8" || got.refusalKey != "extends" {
		t.Errorf("chain over the bound: valid=%v refusal=%q key=%q", got.valid, got.refusal, got.refusalKey)
	}
}

// TJAA-V0-113, TJAA-V0-115: a Playwright testDir with a `../` that stays inside the root is
// accepted; one that escapes it is a project-membership unknown.
func TestPlaywrightTestDirInsideRootParent_GH1880(t *testing.T) {
	for _, tc := range []struct {
		testDir string
		want    string
	}{{"../e2e", "e2e"}, {"../../x", ""}} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		write(t, root, "web/playwright.config.ts", `import { defineConfig } from "@playwright/test"
export default defineConfig({ testDir: "`+tc.testDir+`", projects: [{ name: "chromium", use: { browserName: "chromium" } }] })
`)
		write(t, root, "e2e/login.spec.ts", `import { test } from "@playwright/test"; test("login", () => {})`)
		plan, err := selectPlaywrightStatic(root, "web/playwright.config.ts", []string{"e2e/login.spec.ts"})
		if err != nil {
			t.Fatal(err)
		}
		membership := hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectMembership)
		if tc.want != "" && (membership || plan.Scope != affected.ScopeBounded || !slices.Equal(playwrightSelectionIDs(plan), []string{"typescript:playwright:chromium:e2e/login.spec.ts"})) {
			t.Fatalf("%s: scope=%s selected=%v unknown=%+v", tc.testDir, plan.Scope, playwrightSelectionIDs(plan), plan.Unknown)
		}
		if tc.want == "" && (!membership || plan.Scope != affected.ScopeUnknown) {
			t.Fatalf("%s: escape accepted: scope=%s unknown=%+v", tc.testDir, plan.Scope, plan.Unknown)
		}
	}
}

// TJAA-V0-114: end to end, a spec that imports a support module through a path alias inherited
// from the root tsconfig.json by `extends` is selected when that module changes.
func TestPlaywrightSelectsThroughInheritedAlias_GH1880(t *testing.T) {
	root := outOfSuiteFixture(t)
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@support/*":["tests/support/*"]}}}`)
	write(t, root, "tests/tsconfig.json", `{"extends":"../tsconfig.json","compilerOptions":{"strict":true}}`)
	write(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"
import { user } from "@support/fixture"
test("login", () => user)
`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/support/fixture.ts"})
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "inherited alias", plan)
}

// TJAA-V0-114: end to end through real source observation, a root tsconfig.json whose aliases
// live in `extends: "./tsconfig.base.json"` (or the extension-less spelling) resolves them: the
// Playwright selection, the tag closure and the general graph all follow the alias, and the base
// file is bound into the Playwright source digest.
func TestPlaywrightSelectsThroughObservedBaseConfig_GH1880(t *testing.T) {
	for _, extends := range []string{"./tsconfig.base.json", "./tsconfig.base"} {
		root := outOfSuiteFixture(t)
		write(t, root, "tsconfig.json", `{"extends":"`+extends+`","compilerOptions":{"strict":true}}`)
		write(t, root, "tsconfig.base.json", `{"compilerOptions":{"baseUrl":".","paths":{"@support/*":["tests/support/*"]}}}`)
		write(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"
import { user } from "@support/fixture"
test("login", () => user)
`)
		plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/support/fixture.ts"})
		if err != nil {
			t.Fatal(err)
		}
		assertBoundedLogin(t, extends, plan)
		if hasPlaywrightUnknownReason(plan.Unknown, FrontierPathAlias) || hasPlaywrightUnknownReason(plan.Informational, FrontierPathAlias) {
			t.Fatalf("%s: path-alias frontier remains: %+v %+v", extends, plan.Unknown, plan.Informational)
		}
		sources := newPlaywrightTagSources(root, "playwright.config.ts")
		if !sources.boundFiles()["tsconfig.base.json"] {
			t.Fatalf("%s: the tag closure does not bind tsconfig.base.json", extends)
		}
		if target, ok := sources.resolveAlias("tests/login.spec.ts", "@support/fixture"); !ok || target != "tests/support/fixture.ts" {
			t.Fatalf("%s: tag closure alias = %q %v", extends, target, ok)
		}
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		if general := affected.Select(graph, []string{"tests/support/fixture.ts"}); !slices.Contains(selectedTests(general), "tests/login.spec.ts") || general.FrontierSites[FrontierPathAlias] != 0 {
			t.Fatalf("%s: general selection %v (unknown %v)", extends, selectedTests(general), general.Unknown)
		}
		before, err := ObservePlaywrightSources(root, "playwright.config.ts")
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, "tsconfig.base.json", `{"compilerOptions":{"baseUrl":".","paths":{"@support/*":["src/*"]}}}`)
		if after, err := ObservePlaywrightSources(root, "playwright.config.ts"); err != nil || after == before {
			t.Fatalf("%s: tsconfig.base.json is not bound: before=%s after=%s err=%v", extends, before, after, err)
		}
	}
}

// TJAA-V0-114: under a second runner the general graph keeps aliases unresolved, and an alias a
// root tsconfig.json inherits from an observed base config still raises the path-alias frontier,
// pinned at the base config's paths key rather than silently treated as a package import.
func TestInheritedAliasUnderSecondRunnerStaysFrontier_GH1880(t *testing.T) {
	root := gh1604Repository(t, "", `{ "strict": true }`, gh1604Config, `{"devDependencies":{"@playwright/test":"1.61.0","vitest":"3"}}`)
	write(t, root, "tsconfig.json", "{\n  \"extends\": \"./tsconfig.base.json\"\n}\n")
	write(t, root, "tsconfig.base.json", "{\n  \"compilerOptions\": {\n    \"paths\": { \"lib/*\": [\"./lib/*\"] }\n  }\n}\n")
	write(t, root, "lib/unit/math.test.ts", "import { test } from 'vitest';\nimport { TestTimingLogger } from 'lib/helpers/TestTimingLogger';\ntest('u', () => { void TestTimingLogger; });\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		FrontierPathAlias + ":tsconfig.base.json:3 `\"paths\":`",
		FrontierPathAlias + ":lib/unit/math.test.ts `vitest`",
	} {
		if !slices.Contains(result.Frontier, want) {
			t.Fatalf("frontier %v lacks %q", result.Frontier, want)
		}
	}
}
