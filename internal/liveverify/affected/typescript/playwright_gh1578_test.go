package typescript

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-037 (GitHub #1578): without --playwright-config, a page object that imports
// @playwright/test is support code rather than a test unit, and tsconfig aliases resolve as the
// config mode resolves them when Playwright is the only runner identity.
func TestGeneralAdapterPlaywrightMembershipAndAliases_GH1578(t *testing.T) {
	fixture := func(t *testing.T, manifest, config string) string {
		t.Helper()
		root := t.TempDir()
		write(t, root, "package.json", manifest)
		write(t, root, "playwright.config.ts", config)
		write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["./lib/*"]}}}`)
		write(t, root, "lib/pages/area/SomePage.ts", "import type { Page } from '@playwright/test'\nexport class SomePage { constructor(readonly page: Page) {} }\n")
		write(t, root, "tests/area/some.spec.ts", "import { test } from '@playwright/test'\nimport { SomePage } from 'lib/pages/area/SomePage'\ntest('x', async ({ page }) => { new SomePage(page) })\n")
		write(t, root, "tests/area/other.spec.ts", "import { test } from '@playwright/test'\ntest('y', async () => {})\n")
		return root
	}
	manifest := `{"devDependencies":{"@playwright/test":"1.61.0"}}`
	plain := "import { defineConfig } from '@playwright/test'\nexport default defineConfig({ use: { baseURL: 'http://localhost:3000' } })\n"

	root := fixture(t, manifest, plain)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if frontierHas(result, FrontierPathAlias) {
		t.Fatalf("Playwright-only tsconfig aliases were not resolved: %v", result.Frontier)
	}
	page := unitWithPath(result, "lib/pages/area/SomePage.ts")
	if page.ID == "" || len(page.Tests) != 0 || !slices.Equal(page.Sources, []string{"lib/pages/area/SomePage.ts"}) {
		t.Fatalf("page object became a test unit: %+v", page)
	}
	if spec := testUnit(result, "tests/area/some.spec.ts"); !slices.Contains(spec.Imports, page.ID) {
		t.Fatalf("aliased import edge missing: %+v", spec)
	}
	if other := testUnit(result, "tests/area/other.spec.ts"); other.ID == "" || slices.Contains(other.Imports, page.ID) {
		t.Fatalf("unrelated spec: %+v", other)
	}

	// Another runner identity, or a Playwright config that can name another tsconfig, keeps the
	// alias frontier. A second runner counts once a test it owns is observed: a dependency alone
	// owns no test and loads no module (TJAA-V0-050), and that test's loader must meet an alias
	// (AFP-V0-389).
	for _, row := range []struct{ name, manifest, config, extra string }{
		{"second runner", `{"devDependencies":{"@playwright/test":"1.61.0","vitest":"3"}}`, plain, "import { test } from 'vitest'\nimport { SomePage } from 'lib/pages/area/SomePage'\ntest('u', () => { void SomePage })\n"},
		{"config names a tsconfig", manifest, "import { defineConfig } from '@playwright/test'\nexport default defineConfig({ tsconfig: './tsconfig.e2e.json' })\n", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := fixture(t, row.manifest, row.config)
			if row.extra != "" {
				write(t, root, "lib/unit/helper.test.ts", row.extra)
			}
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			assertFrontier(t, result, FrontierPathAlias)
		})
	}

	// Under a config whose membership markers the general adapter does not model, a conventional
	// test path stays a test unit (the config frontier already widens), while a page object
	// outside the test conventions is still support code.
	root = fixture(t, manifest, "import { defineConfig } from '@playwright/test'\nexport default defineConfig({ testDir: './tests', testMatch: '**/*.e2e.ts' })\n")
	write(t, root, "tests/area/login.e2e.ts", "import { test } from '@playwright/test'\ntest('z', async () => {})\n")
	result, err = New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	assertFrontier(t, result, FrontierConfig)
	if unit := testUnit(result, "tests/area/login.e2e.ts"); unit.ID != unitID(runnerPlaywright, "tests/area/login.e2e.ts") {
		t.Fatalf("custom-matched conventional test was demoted: %+v", unit)
	}
	if page := unitWithPath(result, "lib/pages/area/SomePage.ts"); len(page.Tests) != 0 {
		t.Fatalf("page object became a test unit under a custom config: %+v", page)
	}
}

func unitWithPath(result affected.Result, relative string) affected.Unit {
	for _, unit := range result.Units {
		if slices.Contains(unit.Sources, relative) || slices.Contains(unit.Tests, relative) {
			return unit
		}
	}
	return affected.Unit{}
}
