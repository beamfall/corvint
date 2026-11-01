package typescript

import (
	"slices"
	"strings"
	"testing"
)

// globalIgnoreFixture is a config with a literal global testIgnore and two projects, one of
// which declares its own testIgnore.
func globalIgnoreFixture(t *testing.T, ignore string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  testIgnore: `+ignore+`,
  projects: [
    { name: "chromium" },
    { name: "firefox", testIgnore: "**/slow/**" },
  ],
})
`)
	write(t, root, "tests/support/fixture.ts", `export const user = "u"`)
	write(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"; import { user } from "./support/fixture"; test("login", () => user)`)
	write(t, root, "tests/legacy/old.spec.ts", `import { test } from "@playwright/test"; import { user } from "../support/fixture"; test("old", () => user)`)
	write(t, root, "tests/slow/big.spec.ts", `import { test } from "@playwright/test"; import { user } from "../support/fixture"; test("big", () => user)`)
	return root
}

// TestPlaywrightGlobalTestIgnoreIsAnExclusion_TJAA_V0_770 (V1-3524): a literal global
// testIgnore no longer refuses the static profile; every project without its own testIgnore
// inherits it as an exclusion, a project's own testIgnore replaces it, and a non-literal global
// testIgnore still refuses.
func TestPlaywrightGlobalTestIgnoreIsAnExclusion_TJAA_V0_770(t *testing.T) {
	root := globalIgnoreFixture(t, `"**/legacy/**"`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, unknown := range plan.Unknown {
		if strings.Contains(unknown.Detail, "testIgnore") {
			t.Fatalf("a literal global testIgnore still refuses: %+v", unknown)
		}
	}
	ignores := map[string][]string{}
	for _, project := range plan.Projects {
		ignores[project.Name] = project.TestIgnore
	}
	if !slices.Equal(ignores["chromium"], []string{`"**/legacy/**"`}) || !slices.Equal(ignores["firefox"], []string{`"**/slow/**"`}) {
		t.Fatalf("project testIgnore = %v", ignores)
	}
	if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
		t.Fatalf("narrowing = %+v, want NARROWED", plan.Narrowed)
	}
	candidates := narrowingCandidates(plan.Narrowed)
	for _, want := range []string{"chromium tests/login.spec.ts", "chromium tests/slow/big.spec.ts", "firefox tests/legacy/old.spec.ts", "firefox tests/login.spec.ts"} {
		if !slices.Contains(candidates, want) {
			t.Errorf("candidates %v lack %s", candidates, want)
		}
	}
	for _, unwanted := range []string{"chromium tests/legacy/old.spec.ts", "firefox tests/slow/big.spec.ts"} {
		if slices.Contains(candidates, unwanted) {
			t.Errorf("candidates %v hold the ignored %s", candidates, unwanted)
		}
	}

	dynamic, err := SelectPlaywright(globalIgnoreFixture(t, `process.env.IGNORE`), "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	refused := false
	for _, unknown := range dynamic.Unknown {
		refused = refused || strings.Contains(unknown.Detail, "global testIgnore")
	}
	if !refused {
		t.Fatalf("a dynamic global testIgnore no longer refuses: %+v", dynamic.Unknown)
	}
}
