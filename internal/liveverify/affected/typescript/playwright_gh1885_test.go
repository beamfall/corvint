package typescript

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-122 (GitHub #1885, V1-2786): a bare specifier installed in a node_modules directory of
// the importing file or an ancestor inside the root resolves as an external dependency, also when
// node_modules is a symlink to a store outside the root, so the runner import is no
// path-alias-unresolved frontier. A specifier installed nowhere stays a frontier.
func TestPlaywrightSymlinkedNodeModules_GH1885(t *testing.T) {
	for _, installed := range []bool{true, false} {
		store := t.TempDir()
		write(t, store, "node_modules/@playwright/test/package.json", `{"name":"@playwright/test","version":"1.61.0"}`)
		if !installed {
			if err := os.RemoveAll(filepath.Join(store, "node_modules/@playwright")); err != nil {
				t.Fatal(err)
			}
		}
		root := t.TempDir()
		write(t, root, ".gitignore", "node_modules\n")
		write(t, root, "pw/playwright.config.ts", `import { defineConfig } from "@playwright/test"
export default defineConfig({ testDir: "./tests", projects: [{ name: "chromium", use: { browserName: "chromium" } }] })
`)
		write(t, root, "pw/tests/home.spec.ts", `import { test, expect } from "@playwright/test"
test("home", async ({ page }) => { await page.goto("/"); await expect(page).toHaveTitle("home") })
`)
		write(t, root, "pw/tests/other.spec.ts", `import { test } from "@playwright/test"
test("other", async () => {})
`)
		if err := os.Symlink(filepath.Join(store, "node_modules"), filepath.Join(root, "pw/node_modules")); err != nil {
			t.Fatal(err)
		}
		plan, err := selectPlaywrightStatic(root, "pw/playwright.config.ts", []string{"pw/tests/home.spec.ts"})
		if err != nil {
			t.Fatal(err)
		}
		alias := false
		for _, unknown := range append(append([]PlaywrightUnknown{}, plan.Unknown...), plan.Informational...) {
			if unknown.Reason == FrontierPathAlias || strings.Contains(unknown.Detail, FrontierPathAlias) {
				alias = true
			}
		}
		if installed && (alias || plan.Scope != affected.ScopeBounded || len(plan.Selected) != 1 || plan.Selected[0].Test != "pw/tests/home.spec.ts") {
			t.Fatalf("installed: scope=%s selected=%v unknown=%+v informational=%+v", plan.Scope, playwrightSelectionIDs(plan), plan.Unknown, plan.Informational)
		}
		if !installed && (!alias || plan.Scope != affected.ScopeUnknown) {
			t.Fatalf("not installed: scope=%s unknown=%+v informational=%+v", plan.Scope, plan.Unknown, plan.Informational)
		}
	}
}

// TJAA-V0-122 (GitHub #1885 review): a node_modules entry that links into repository source
// outside every node_modules directory, under a manifest name no workspace package declares, is
// no installed dependency: the import keeps the path-alias frontier, so a change to the linked
// source is never excluded. A link into an in-repository store under node_modules (the pnpm
// layout) stays an installed dependency.
func TestInstalledPackageLinkIntoRepositorySource_GH1885(t *testing.T) {
	for _, layout := range []string{"workspace-link", "pnpm-store"} {
		root := t.TempDir()
		write(t, root, "package.json", `{"name":"app","private":true,"devDependencies":{"vitest":"1.6.0"}}`)
		write(t, root, "vitest.config.ts", "export default {}\n")
		write(t, root, "src/lib/package.json", `{"name":"other-name","main":"logic.js"}`)
		write(t, root, "src/lib/logic.js", "export const value = 1\n")
		write(t, root, "test/alias.test.js", "import { test } from 'vitest'\nimport { value } from 'alias'\ntest('alias', () => { if (value !== 1) throw new Error('value') })\n")
		target := "../src/lib"
		if layout == "pnpm-store" {
			write(t, root, "node_modules/.pnpm/alias@1.0.0/node_modules/alias/package.json", `{"name":"alias","version":"1.0.0"}`)
			target = ".pnpm/alias@1.0.0/node_modules/alias"
		}
		if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "node_modules", "alias")); err != nil {
			t.Fatal(err)
		}
		source := affected.DiskSource(root)
		installed := installedPackage(source, "test/alias.test.js", "alias")
		if installed != (layout == "pnpm-store") {
			t.Fatalf("%s: installed = %v", layout, installed)
		}
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"src/lib/logic.js"})
		if layout == "workspace-link" && (plan.FrontierSites[FrontierPathAlias] == 0 || plan.Scope == affected.ScopeBounded && !slices.Contains(selectedTests(plan), "test/alias.test.js")) {
			t.Fatalf("%s: scope=%s selected=%v unknown=%+v frontier=%v", layout, plan.Scope, selectedTests(plan), plan.Unknown, plan.FrontierSites)
		}
		if layout == "pnpm-store" && plan.FrontierSites[FrontierPathAlias] != 0 {
			t.Fatalf("%s: installed store link raised the alias frontier: %v", layout, plan.FrontierSites)
		}
		if tree := affected.FSSource(os.DirFS(root)); installedPackage(tree, "test/alias.test.js", "alias") {
			t.Fatalf("%s: an immutable tree source answered installed", layout)
		}
	}
}
