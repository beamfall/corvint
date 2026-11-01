package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestPathAliasResolvesUnderALiteralGlobalTestIgnore_TJAA_V0_770 (V1-3524, V1-0970): in a
// Playwright-only repository whose config carries a literal global testIgnore, the general
// adapter keeps the membership static (no executable-config site, which the base raised at the
// testIgnore line), resolves a tsconfig paths alias reached through an extends chain, and selects
// the spec that imports the changed module through it; an alias no tsconfig names keeps the
// fail-closed undeclared-bare-import frontier (TJAA-V0-771).
func TestPathAliasResolvesUnderALiteralGlobalTestIgnore_TJAA_V0_770(t *testing.T) {
	root := globalIgnoreFixture(t, `"**/legacy/*.spec.ts"`)
	write(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  testIgnore: "**/legacy/*.spec.ts",
  projects: [{ name: "chromium" }],
})
`)
	write(t, root, "tsconfig.base.json", `{"compilerOptions":{"baseUrl":".","paths":{"@app/*":["src/*"]}}}`)
	write(t, root, "tsconfig.json", `{"extends":"./tsconfig.base.json"}`)
	write(t, root, "src/app.ts", `export const app = 1`)
	write(t, root, "tests/app.spec.ts", `import { test } from "@playwright/test"; import { app } from "@app/app"; test("app", () => app)`)
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/app.ts"})
	if !slices.Contains(plan.SelectedTests(), "tests/app.spec.ts") {
		t.Fatalf("selected %v, want tests/app.spec.ts through @app/app", plan.SelectedTests())
	}
	for _, unknown := range plan.Unknown {
		if strings.HasPrefix(unknown.Detail, FrontierPathAlias) || strings.HasPrefix(unknown.Detail, FrontierConfig) {
			t.Fatalf("unknown %+v, want the alias resolved and the config static", unknown)
		}
	}
	write(t, root, "tests/missing.spec.ts", `import { test } from "@playwright/test"; import { x } from "@nowhere/x"; test("m", () => x)`)
	graph, err = affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	unresolved := false
	for _, unknown := range affected.Select(graph, []string{"src/app.ts"}).Unknown {
		unresolved = unresolved || strings.HasPrefix(unknown.Detail, FrontierBareImportUndeclared)
	}
	if !unresolved {
		t.Fatal("a bare specifier no tsconfig claims and no manifest declares lost its fail-closed frontier")
	}
}
