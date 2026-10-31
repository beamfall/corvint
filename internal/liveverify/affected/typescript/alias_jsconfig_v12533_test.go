package typescript

import (
	"context"
	"slices"
	"testing"
)

// TJAA-V0-960 (V1-2533): the general adapter's alias path reads the config Playwright reads,
// the nearest folder's tsconfig.json or else its jsconfig.json, and maps a specifier of a module
// that is not .ts or .tsx only under allowJs (a jsconfig.json allows it by default).
func TestGeneralAdapterJSConfigAliasesAndAllowJs_V12533(t *testing.T) {
	manifest := `{"devDependencies":{"@playwright/test":"1.61.0"}}`
	config := "import { defineConfig } from '@playwright/test'\nexport default defineConfig({ use: { baseURL: 'http://localhost:3000' } })\n"
	fixture := func(t *testing.T, files map[string]string) string {
		t.Helper()
		root := t.TempDir()
		write(t, root, "package.json", manifest)
		write(t, root, "playwright.config.ts", config)
		write(t, root, "lib/page.ts", "export const page = 1\n")
		write(t, root, "e2e/other/page.ts", "export const page = 2\n")
		for name, body := range files {
			write(t, root, name, body)
		}
		return root
	}
	edges := func(t *testing.T, root, spec string) (root1, nearer bool, frontier bool) {
		t.Helper()
		result, err := New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		unit := testUnit(result, spec)
		if unit.ID == "" {
			t.Fatalf("no test unit %s: %+v", spec, result.Units)
		}
		return slices.Contains(unit.Imports, unitWithPath(result, "lib/page.ts").ID),
			slices.Contains(unit.Imports, unitWithPath(result, "e2e/other/page.ts").ID),
			frontierHas(result, FrontierPathAlias)
	}
	tsconfig := `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["./lib/*"]}}}`
	spec := "import { test } from '@playwright/test'\nimport { page } from 'lib/page'\ntest('x', async () => { page })\n"

	t.Run("nearer jsconfig.json wins", func(t *testing.T) {
		root := fixture(t, map[string]string{
			"tsconfig.json":     tsconfig,
			"e2e/jsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["./other/*"]}}}`,
			"e2e/a.spec.ts":     spec,
		})
		root1, nearer, _ := edges(t, root, "e2e/a.spec.ts")
		if root1 || !nearer {
			t.Fatalf("e2e/a.spec.ts: root edge %v, jsconfig edge %v; want only the jsconfig edge", root1, nearer)
		}
	})
	t.Run("tsconfig.json beside a jsconfig.json is read first", func(t *testing.T) {
		root := fixture(t, map[string]string{
			"e2e/tsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["../lib/*"]}}}`,
			"e2e/jsconfig.json": `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["./other/*"]}}}`,
			"e2e/a.spec.ts":     spec,
		})
		if root1, nearer, _ := edges(t, root, "e2e/a.spec.ts"); !root1 || nearer {
			t.Fatalf("root edge %v, jsconfig edge %v; want only the tsconfig edge", root1, nearer)
		}
	})
	t.Run("a .js importer needs allowJs", func(t *testing.T) {
		root := fixture(t, map[string]string{"tsconfig.json": tsconfig, "tests/b.spec.js": spec})
		if root1, _, frontier := edges(t, root, "tests/b.spec.js"); root1 || !frontier {
			t.Fatalf("without allowJs: edge %v frontier %v; want no edge and the alias frontier", root1, frontier)
		}
		allowed := `{"compilerOptions":{"allowJs":true,"baseUrl":".","paths":{"lib/*":["./lib/*"]}}}`
		root = fixture(t, map[string]string{"tsconfig.json": allowed, "tests/b.spec.js": spec})
		if root1, _, frontier := edges(t, root, "tests/b.spec.js"); !root1 || frontier {
			t.Fatalf("with allowJs: edge %v frontier %v", root1, frontier)
		}
	})
	t.Run("jsconfig.json is a resolution input", func(t *testing.T) {
		if !(Language{}).ResolutionInput("e2e/jsconfig.json") || !(Language{}).ResolutionInput("jsconfig.base.json") {
			t.Fatal("a jsconfig*.json edit must keep the plan's frontier")
		}
	})
}
