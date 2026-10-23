package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1604RemainderConfig is the reporter's config form: a literal testDir and projects whose
// membership the closed static profile resolves.
const gh1604RemainderConfig = "import { defineConfig, devices } from '@playwright/test';\nexport default defineConfig({\n  testDir: './tests',\n  fullyParallel: true,\n  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],\n});\n"

// gh1604RemainderRepository writes the GitHub #1604 retest shape: Playwright specs that reach a
// changed helper through a tsconfig paths alias, and a node:test script beside them.
func gh1604RemainderRepository(t *testing.T, config, script, scriptHelper string) affected.Result {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "tsconfig.json", "{\n  \"compilerOptions\": {\n    \"baseUrl\": \".\",\n    \"paths\": { \"lib/*\": [\"./lib/*\"] }\n  }\n}\n")
	write(t, root, "playwright.config.ts", config)
	write(t, root, "lib/helpers/TestTimingLogger.ts", "export class TestTimingLogger {}\n")
	write(t, root, "lib/fixtures/control.ts", "import { test as base } from '@playwright/test';\nimport { TestTimingLogger } from 'lib/helpers/TestTimingLogger';\nexport const test = base;\nexport const logger = TestTimingLogger;\n")
	write(t, root, "tests/a.spec.ts", "import { test } from 'lib/fixtures/control';\ntest('a', async () => {});\n")
	write(t, root, "tests/b.spec.ts", "import { test } from '@playwright/test';\ntest('b', async () => {});\n")
	write(t, root, "scripts/assemble.test.mjs", script)
	write(t, root, "scripts/assemble.mjs", scriptHelper)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

const gh1604NodeTest = "import test from 'node:test';\nimport { assemble } from './assemble.mjs';\ntest('assemble', () => { assemble(); });\n"

// AFP-V0-389, AFP-V0-404: a node:test script whose loader never meets an alias does not keep the
// aliases unresolved, and a literal testDir/projects config whose profile-owned files are all
// Playwright tests of that config raises no executable-config frontier. The aliased edge selects
// the spec and no tsconfig, blocker or config site remains.
func TestAliasFreeRunnerAndStaticPlaywrightMembership_GH1604(t *testing.T) {
	result := gh1604RemainderRepository(t, gh1604RemainderConfig, gh1604NodeTest, "import { join } from 'node:path';\nexport const assemble = () => join('a', 'b');\n")
	for _, reason := range []string{FrontierPathAlias, FrontierConfig} {
		if frontierHas(result, reason) {
			t.Fatalf("%s remains:\n%s", reason, strings.Join(result.Frontier, "\n"))
		}
	}
	helper := unitWithPath(result, "lib/helpers/TestTimingLogger.ts")
	fixture := unitWithPath(result, "lib/fixtures/control.ts")
	if helper.ID == "" || !slices.Contains(fixture.Imports, helper.ID) || !slices.Contains(testUnit(result, "tests/a.spec.ts").Imports, fixture.ID) {
		t.Fatalf("aliased edges missing: helper=%+v fixture=%+v spec=%+v", helper, fixture, testUnit(result, "tests/a.spec.ts"))
	}
	if script := testUnit(result, "scripts/assemble.test.mjs"); script.ID == "" {
		t.Fatalf("node test lost: %+v", result.Units)
	}
}

// AFP-V0-389: the exemption holds only for a closure that provably meets no alias. A helper that
// names an alias, a module whose loads are not all static, or a workspace package keeps the
// script as the named blocker beside the tsconfig site.
func TestAliasReachingRunnerStillBlocks_GH1604(t *testing.T) {
	for _, row := range []struct{ name, helper string }{
		{"helper names an alias", "import { TestTimingLogger } from 'lib/helpers/TestTimingLogger';\nexport const assemble = () => new TestTimingLogger();\n"},
		{"helper loads dynamically", "export const assemble = (name) => import(name);\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			result := gh1604RemainderRepository(t, gh1604RemainderConfig, gh1604NodeTest, row.helper)
			for _, want := range []string{FrontierPathAlias + ":scripts/assemble.test.mjs `node-test`", FrontierPathAlias + ":tsconfig.json:3 `\"baseUrl\":`"} {
				if !slices.Contains(result.Frontier, want) {
					t.Fatalf("frontier lacks %q:\n%s", want, strings.Join(result.Frontier, "\n"))
				}
			}
		})
	}
}

// AFP-V0-404: any marker the profile does not resolve statically, a globalSetup, or a
// profile-owned file the general adapter does not classify as that config's Playwright test keeps
// the executable-config frontier.
func TestPlaywrightConfigFrontierKeptWhenMembershipDiffers_GH1604(t *testing.T) {
	plainScript := "export const assemble = () => 1;\n"
	for _, row := range []struct{ name, config, extraPath, extra string }{
		{"testMatch", strings.Replace(gh1604RemainderConfig, "fullyParallel: true", "testMatch: '**/*.e2e.ts'", 1), "", ""},
		{"globalSetup", strings.Replace(gh1604RemainderConfig, "fullyParallel: true", "globalSetup: './setup.ts'", 1), "", ""},
		{"computed testDir", strings.Replace(gh1604RemainderConfig, "'./tests'", "process.env.DIR", 1), "", ""},
		{"profile-owned file of another runner", gh1604RemainderConfig, "tests/unit/math.test.ts", "import test from 'node:test';\ntest('u', () => {});\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			result := gh1604RemainderRepositoryWith(t, row.config, plainScript, row.extraPath, row.extra)
			assertFrontier(t, result, FrontierConfig)
		})
	}
}

func gh1604RemainderRepositoryWith(t *testing.T, config, scriptHelper, extraPath, extra string) affected.Result {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", config)
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\ntest('a', async () => {});\n")
	write(t, root, "scripts/assemble.mjs", scriptHelper)
	if extraPath != "" {
		write(t, root, extraPath, extra)
	}
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// AFP-V0-389: a relative import that resolves to no observed module, such as a generated helper
// that is not checked in, may reach an alias, so the script keeps the named blocker.
func TestUnresolvedRelativeImportStillBlocks_GH1604(t *testing.T) {
	script := "import test from 'node:test';\nimport { make } from './generated-helper.mjs';\ntest('make', () => { make(); });\n"
	result := gh1604RemainderRepository(t, gh1604RemainderConfig, script, "export const assemble = () => 1;\n")
	for _, want := range []string{FrontierPathAlias + ":scripts/assemble.test.mjs `node-test`", FrontierPathAlias + ":tsconfig.json:3 `\"baseUrl\":`"} {
		if !slices.Contains(result.Frontier, want) {
			t.Fatalf("frontier lacks %q:\n%s", want, strings.Join(result.Frontier, "\n"))
		}
	}
}
