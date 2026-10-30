package typescript

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// gh1884Fixture is the GitHub #1884 layout: server code in another language, browser code no spec
// imports, and a Playwright suite under pw/ whose runner import resolves, so no shared graph
// frontier is present to widen the plan by accident.
func gh1884Fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "app/models/home.rb", "class Home\n  TITLE = 'home'\nend\n")
	write(t, root, "web/home.ts", "export const title = 'home'\n")
	write(t, root, "pw/package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "pw/playwright.config.ts", `import { defineConfig } from "@playwright/test"
export default defineConfig({ testDir: "./tests", projects: [{ name: "chromium", use: { browserName: "chromium" } }] })
`)
	write(t, root, "pw/tests/home.spec.ts", `import { test, expect } from "@playwright/test"
test("home", async ({ page }) => { await page.goto("/"); await expect(page).toHaveTitle("home") })
`)
	return root
}

// TJAA-V0-121 (GitHub #1884, V1-2785): a changed path outside the suite's reach is a SELECTION
// unknown whatever its language, so fail closed never depends on an unrelated graph frontier.
func TestPlaywrightOutOfSuiteSourceAnyLanguage_GH1884(t *testing.T) {
	for _, dirty := range []string{"web/home.ts", "app/models/home.rb"} {
		root := gh1884Fixture(t)
		plan, err := selectPlaywrightStatic(root, "pw/playwright.config.ts", []string{dirty})
		if err != nil {
			t.Fatal(err)
		}
		unknown, selection := outOfSuiteUnknown(plan)
		if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || !selection || len(plan.Excluded) != 0 {
			t.Fatalf("%s: scope=%s fallback=%s unknown=%+v", dirty, plan.Scope, plan.Fallback, plan.Unknown)
		}
		if !strings.Contains(unknown.Detail, dirty) || !strings.Contains(unknown.Detail, PlaywrightUnreachedPathsPath) {
			t.Fatalf("%s: detail %q", dirty, unknown.Detail)
		}
	}
	// A declaration that covers the server path bounds the plan and cites the declaration.
	root := gh1884Fixture(t)
	writeUnreached(t, root, "app/")
	plan, err := selectPlaywrightStatic(root, "pw/playwright.config.ts", []string{"app/models/home.rb"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeBounded || len(plan.Selected) != 0 || plan.Unreached == nil || plan.Unreached.State != PlaywrightDeclarationApplied {
		t.Fatalf("declared: scope=%s selected=%d unknown=%+v record=%+v", plan.Scope, len(plan.Selected), plan.Unknown, plan.Unreached)
	}
	// A declared path outside the graph that may configure the run, sits in a test directory, or
	// that a closure source names is refused, and the plan still widens.
	for _, refused := range []struct{ dirty, declared, why string }{
		{"pw/package.json", "pw/", "may configure"},
		{"pw/tests/data/home.yml", "pw/tests/", "inside Playwright test directory pw/tests"},
		{"app/models/home.rb", "app/", "pw/tests/home.spec.ts:3"},
	} {
		root := gh1884Fixture(t)
		write(t, root, "pw/tests/data/home.yml", "title: home\n")
		write(t, root, "pw/tests/home.spec.ts", `import { test, expect } from "@playwright/test"
test("home", async ({ page }) => { await page.goto("/"); await expect(page).toHaveTitle("home") })
const server = "../app/models/home.rb"
`)
		if refused.dirty != "app/models/home.rb" {
			write(t, root, "pw/tests/home.spec.ts", `import { test } from "@playwright/test"
test("home", async () => {})
`)
		}
		writeUnreached(t, root, refused.declared)
		plan, err := selectPlaywrightStatic(root, "pw/playwright.config.ts", []string{refused.dirty})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != affected.ScopeUnknown || plan.Unreached == nil || plan.Unreached.State != PlaywrightDeclarationRefused {
			t.Fatalf("%s: scope=%s unknown=%+v record=%+v", refused.dirty, plan.Scope, plan.Unknown, plan.Unreached)
		}
		found := false
		for _, unknown := range plan.Unknown {
			found = found || unknown.Reason == PlaywrightUnknownOutOfSuiteSource && strings.Contains(unknown.Detail, refused.dirty) && strings.Contains(unknown.Detail, refused.why)
		}
		if !found {
			t.Fatalf("%s: no refusal naming %q: %+v", refused.dirty, refused.why, plan.Unknown)
		}
	}
}
