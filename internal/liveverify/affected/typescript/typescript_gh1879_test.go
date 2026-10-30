package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TJAA-V0-111 (GitHub #1879, V1-2779): a package-manager lockfile is never a test asset, and a
// path declared in .corvint/non-test-asset-paths.json raises no cross-language test-asset
// frontier; every other ambiguous asset still does.
func TestLockfilesAndDeclaredPathsAreNotTestAssets_GH1879(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"test":"node --test"}}`)
	write(t, root, "tests/a.test.js", `import "node:test"`)
	write(t, root, "e2e/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	write(t, root, "maestro/shrinkwrap.yaml", "lockfileVersion: 5\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if frontierHas(result, FrontierCrossLanguageTestAsset) {
		t.Fatalf("lockfile raised a test-asset frontier: %v", result.Frontier)
	}
	write(t, root, "e2e/flows/login.yaml", "appId: example\n")
	result, err = New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	assertFrontier(t, result, FrontierCrossLanguageTestAsset)
	write(t, root, NonTestAssetPathsPath, `{"profile":"`+NonTestAssetPathsProfile+`","paths":["e2e/flows/"]}`)
	result, err = New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if frontierHas(result, FrontierCrossLanguageTestAsset) {
		t.Fatalf("declared non-test asset raised a frontier: %v", result.Frontier)
	}
	write(t, root, NonTestAssetPathsPath, `{"profile":"`+NonTestAssetPathsProfile+`","paths":["e2e/flows/","e2e/"]}`)
	if _, err := New().Units(context.Background(), root); err == nil {
		t.Fatal("unordered non-test-asset declaration accepted")
	}
}

// TJAA-V0-112, TJAA-V0-111: in a Playwright plan a cross-language asset frontier widens only when
// a closure file or a unit a dirty path reaches may name the asset, and the plan records the
// non-test-asset declaration it was built with.
func TestPlaywrightTestAssetFrontierScoped_GH1879(t *testing.T) {
	dirty := []string{"tests/support/fixture.ts"}
	root := outOfSuiteFixture(t)
	write(t, root, "e2e/flows/login.yaml", "appId: example\n")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "unnamed asset", plan)
	if got := informationalReasons(plan); !slices.Equal(got, []string{FrontierCrossLanguageTestAsset}) || !strings.Contains(plan.Informational[0].Detail, "e2e/flows/login.yaml") {
		t.Fatalf("informational %+v", plan.Informational)
	}
	if plan.NonTestAssets != nil {
		t.Fatalf("record without a declaration: %+v", plan.NonTestAssets)
	}

	named := outOfSuiteFixture(t)
	write(t, named, "e2e/flows/login.yaml", "appId: example\n")
	write(t, named, "tests/support/fixture.ts", `import { readFileSync } from "node:fs"
export const user = readFileSync("e2e/flows/login.yaml", "utf8")`)
	plan, err = selectPlaywrightStatic(named, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "asset named by a closure file", plan)

	write(t, named, NonTestAssetPathsPath, `{"profile":"`+NonTestAssetPathsProfile+`","paths":["e2e/flows/login.yaml"]}`)
	plan, err = selectPlaywrightStatic(named, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "declared non-test asset", plan)
	if record := plan.NonTestAssets; record == nil || record.Path != NonTestAssetPathsPath || len(record.SHA256) != 64 || !slices.Equal(record.Paths, []string{"e2e/flows/login.yaml"}) {
		t.Fatalf("record %+v", plan.NonTestAssets)
	}
}
