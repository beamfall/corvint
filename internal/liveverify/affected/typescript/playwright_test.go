package typescript

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestPlaywrightProjectSelectionBindsVariantsAndSetupRelations(t *testing.T) {
	root := playwrightFixture(t)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone {
		t.Fatalf("scope=%s fallback=%s unknown=%v", plan.Scope, plan.Fallback, plan.Unknown)
	}
	want := []string{
		"typescript:playwright:angular:tests/login.spec.ts",
		"typescript:playwright:chromium:tests/login.spec.ts",
		"typescript:playwright:cleanup:tests/global.teardown.ts",
		"typescript:playwright:react:tests/login.spec.ts",
		"typescript:playwright:setup:tests/global.setup.ts",
	}
	if got := playwrightSelectionIDs(plan); !slices.Equal(got, want) {
		t.Fatalf("selected=%v want=%v", got, want)
	}
	if containsPlaywrightTest(plan, "tests/other.spec.ts") {
		t.Fatalf("unrelated test selected: %+v", plan.Selected)
	}
	angular := playwrightProject(plan, "angular")
	if angular.Browser != "chromium" || angular.Device != "" || angular.Grep != "/@angular/" || angular.Metadata == "" {
		t.Fatalf("angular identity=%+v", angular)
	}
	chromium := playwrightProject(plan, "chromium")
	if chromium.Browser != "chromium" || chromium.Device != "Desktop Chrome" {
		t.Fatalf("chromium identity=%+v", chromium)
	}
	if !hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownExternalApplication) {
		t.Fatalf("execution unknown missing: %v", plan.Unknown)
	}
}

func TestPlaywrightSetupAndConfigChangesWidenThroughDeclaredRelations(t *testing.T) {
	root := playwrightFixture(t)
	setup, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/global.setup.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.Scope != affected.ScopeBounded || !containsPlaywrightTest(setup, "tests/other.spec.ts") {
		t.Fatalf("setup change did not select dependent project suites: scope=%s selected=%v", setup.Scope, playwrightSelectionIDs(setup))
	}
	config, err := SelectPlaywright(root, "playwright.config.ts", []string{"playwright.config.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Excluded) != 0 || len(config.Selected) != 8 {
		t.Fatalf("config change must select all eight fixture variants: selected=%d excluded=%v", len(config.Selected), config.Excluded)
	}
}

func TestPlaywrightDynamicImportsFailClosedToFullRelevantSuite(t *testing.T) {
	root := playwrightFixture(t)
	write(t, root, "tests/login.spec.ts", `
import { test } from "@playwright/test"
const moduleName = "./pages/login"
test("@angular @react login", async () => { await import(moduleName) })
`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 || len(plan.Selected) < 20 {
		t.Fatalf("scope=%s fallback=%s selected=%d excluded=%d unknown=%v", plan.Scope, plan.Fallback, len(plan.Selected), len(plan.Excluded), plan.Unknown)
	}
	if !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, FrontierDynamicImport) {
		t.Fatalf("dynamic import unknown missing: %v", plan.Unknown)
	}
}

func TestPlaywrightDynamicProjectSetAbstainsFromRunnableUnits(t *testing.T) {
	root := playwrightFixture(t)
	write(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({ projects: makeProjects(process.env.TARGET) })
`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Projects) != 0 || len(plan.Selected) != 0 {
		t.Fatalf("dynamic project set was approximated: %+v", plan)
	}
	if !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectSet) {
		t.Fatalf("project-set unknown missing: %v", plan.Unknown)
	}
}

func TestPlaywrightPartialProjectSetAbstainsFromRunnableUnits(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [{ name: "known" }, ...otherProjects] }`)
	write(t, root, "tests/example.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/example.spec.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || len(plan.Selected) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectSet) {
		t.Fatalf("partial project set emitted runnable units: %+v", plan)
	}
}

func TestPlaywrightConfigParserRejectsUnconsumedAndComputedIdentity(t *testing.T) {
	for _, source := range []string{
		`export default defineConfig({ projects: [{ name: "p" }] }, dynamicConfig)`,
		`export default { projects: [{ name: "p", use: { browserName: chosenBrowser } }] }`,
		`export default { projects: [{ name: "p", use: { ["browserName"]: chosenBrowser } }] }`,
		`export default { projects: [{ name: "p", use: { ...devices["Desktop Chrome Canary"] } }] }`,
	} {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
		if len(unknown) == 0 {
			t.Fatalf("dynamic identity was accepted: %s", source)
		}
	}
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", `
const decoy = defineConfig({ projects: [{ name: "wrong" }] })
export default defineConfig({ projects: [{ name: "right" }] })
`)
	if len(unknown) != 0 || len(projects) != 1 || projects[0].Name != "right" {
		t.Fatalf("exported config not selected exactly: projects=%+v unknown=%+v", projects, unknown)
	}
}

func TestPlaywrightMatchersUseAbsolutePathsAndZeroSegmentGlobstar(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default {
  testDir: "tests",
  projects: [
    { name: "glob", testMatch: "**/tests/**/*.spec.ts" },
    { name: "regex", testMatch: /\/tests\/example\.spec\.ts$/ },
  ],
}`)
	write(t, root, "tests/example.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/example.spec.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if got := playwrightSelectionIDs(plan); !slices.Equal(got, []string{
		"typescript:playwright:glob:tests/example.spec.ts",
		"typescript:playwright:regex:tests/example.spec.ts",
	}) {
		t.Fatalf("absolute matcher selection=%v unknown=%v", got, plan.Unknown)
	}
}

func TestPlaywrightStaticMatcherAdmitsCustomFixtureTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [{ name: "custom", testMatch: /custom\.ts$/ }] }`)
	write(t, root, "tests/fixtures.ts", `export { test } from "@playwright/test"`)
	write(t, root, "tests/custom.ts", `import { test } from "./fixtures"; test("x", () => {})`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/custom.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPlaywrightTest(plan, "tests/custom.ts") {
		t.Fatalf("custom fixture test omitted: scope=%s selected=%v unknown=%v", plan.Scope, playwrightSelectionIDs(plan), plan.Unknown)
	}
}

func TestPlaywrightComputedStringsAndUnsupportedGlobsWiden(t *testing.T) {
	for _, matcher := range []string{`'**/' + 'example.spec.ts'`, `"**/[ab].spec.ts"`, `"**/test[ab].spec.ts"`, `"**/{*.spec,*.test}.ts"`} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		write(t, root, "playwright.config.ts", `export default { projects: [{ name: "p", testMatch: `+matcher+` }] }`)
		write(t, root, "tests/a.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/a.spec.ts"})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != affected.ScopeUnknown || !containsPlaywrightTest(plan, "tests/a.spec.ts") || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectMembership) {
			t.Fatalf("matcher %s narrowed unsafely: %+v", matcher, plan)
		}
	}
}

func TestPlaywrightUnknownMembershipRetainsCustomFixtureTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [{ name: "p", testMatch: [/other\.ts/, computedMatcher] }] }`)
	write(t, root, "tests/fixtures.ts", `export { test } from "@playwright/test"`)
	write(t, root, "tests/custom.ts", `import { test } from "./fixtures"; test("x", () => {})`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/custom.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || !containsPlaywrightTest(plan, "tests/custom.ts") {
		t.Fatalf("unknown membership discarded custom fixture test: %+v", plan)
	}
}

func TestPlaywrightSetupDependentsExpandTransitively(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [
  { name: "setup", testMatch: /setup\.ts/ },
  { name: "middle", testMatch: /middle\.ts/, dependencies: ["setup"] },
  { name: "app", testMatch: /app\.ts/, dependencies: ["middle"] },
] }`)
	for _, relative := range []string{"tests/setup.ts", "tests/middle.ts", "tests/app.ts"} {
		write(t, root, relative, `import { test } from "@playwright/test"; test("x", () => {})`)
	}
	plan, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/setup.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPlaywrightTest(plan, "tests/app.ts") {
		t.Fatalf("transitive dependent missing: %v", playwrightSelectionIDs(plan))
	}
}

func TestPlaywrightSourceDigestTracksSamePathContentChanges(t *testing.T) {
	root := playwrightFixture(t)
	before, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "tests/pages/login.ts", `export const login = () => "changed"`)
	after, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("source digest ignored a same-path content change")
	}
}

func TestPlaywrightSelectionBytesAreDeterministic(t *testing.T) {
	root := playwrightFixture(t)
	first, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectPlaywright(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := first.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := second.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("non-deterministic plan:\n%s\n%s", firstBytes, secondBytes)
	}
}

func playwrightFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `
import { defineConfig, devices } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  projects: [
    {
      name: "setup",
      testMatch: /.*\.setup\.ts/,
      teardown: "cleanup",
      use: { browserName: "chromium" },
    },
    {
      name: "cleanup",
      testMatch: /.*\.teardown\.ts/,
      use: { browserName: "chromium" },
    },
    {
      name: "chromium",
      testIgnore: /.*\.(setup|teardown)\.ts/,
      grep: /@chromium/,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      name: "angular",
      testIgnore: /.*\.(setup|teardown)\.ts/,
      grep: /@angular/,
      metadata: { framework: "angular" },
      use: { browserName: "chromium" },
      dependencies: ["setup"],
    },
    {
      name: "react",
      testIgnore: /.*\.(setup|teardown)\.ts/,
      grep: /@react/,
      metadata: { framework: "react" },
      use: { browserName: "chromium" },
      dependencies: ["setup"],
    },
  ],
})
`)
	write(t, root, "tests/pages/login.ts", `export const login = () => "ok"`)
	write(t, root, "tests/fixtures/session.ts", `export const session = "ready"`)
	write(t, root, "tests/scenarios/login.ts", `export { login } from "../pages/login"`)
	write(t, root, "tests/login.spec.ts", `
import { test } from "@playwright/test"
import { login } from "./scenarios/login"
test("@chromium @angular @react login", () => login())
`)
	write(t, root, "tests/other.spec.ts", `import { test } from "@playwright/test"; test("other", () => {})`)
	write(t, root, "tests/global.setup.ts", `import { test } from "@playwright/test"; test("setup", () => {})`)
	write(t, root, "tests/global.teardown.ts", `import { test } from "@playwright/test"; test("cleanup", () => {})`)
	return root
}

func playwrightSelectionIDs(plan PlaywrightPlan) []string {
	ids := make([]string, 0, len(plan.Selected))
	for _, selected := range plan.Selected {
		ids = append(ids, selected.ID)
	}
	return ids
}

func containsPlaywrightTest(plan PlaywrightPlan, want string) bool {
	for _, selected := range plan.Selected {
		if selected.Test == want {
			return true
		}
	}
	return false
}

func playwrightProject(plan PlaywrightPlan, name string) PlaywrightProject {
	for _, project := range plan.Projects {
		if project.Name == name {
			return project
		}
	}
	return PlaywrightProject{}
}

func hasPlaywrightUnknown(plan PlaywrightPlan, axis, reason string) bool {
	for _, unknown := range plan.Unknown {
		if unknown.Axis == axis && unknown.Reason == reason {
			return true
		}
	}
	return false
}

func TestPlaywrightGlobAndRegexMatchers(t *testing.T) {
	for _, testCase := range []struct {
		raw   string
		match string
	}{
		{raw: `"**/*.spec.ts"`, match: "nested/login.spec.ts"},
		{raw: `"**/*.{spec,test}.ts"`, match: "nested/login.test.ts"},
		{raw: `"**/tests/**/*.spec.ts"`, match: "tests/login.spec.ts"},
		{raw: `/.*\.setup\.ts/`, match: "nested/global.setup.ts"},
	} {
		matcher, ok := compilePlaywrightMatcher(testCase.raw)
		if !ok || !matcher.re.MatchString(testCase.match) {
			t.Errorf("matcher %s did not match %s", testCase.raw, testCase.match)
		}
	}
	if matcher, ok := compilePlaywrightMatcher(`makePattern()`); ok || matcher.re != nil {
		t.Fatal("dynamic matcher was accepted")
	}
}

func TestPlaywrightProjectIdentityEscapesNames(t *testing.T) {
	if got := playwrightIDEscape("Angular / React"); got != "Angular%20%2F%20React" || strings.Contains(got, " ") {
		t.Fatalf("escaped=%q", got)
	}
}
