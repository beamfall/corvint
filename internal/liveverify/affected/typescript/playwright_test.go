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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
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
	setup, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/global.setup.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if setup.Scope != affected.ScopeBounded || !containsPlaywrightTest(setup, "tests/other.spec.ts") {
		t.Fatalf("setup change did not select dependent project suites: scope=%s selected=%v", setup.Scope, playwrightSelectionIDs(setup))
	}
	config, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"playwright.config.ts"})
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 || len(plan.Selected) != 8 {
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/example.spec.ts"})
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/example.spec.ts"})
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/custom.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsPlaywrightTest(plan, "tests/custom.ts") {
		t.Fatalf("custom fixture test omitted: scope=%s selected=%v unknown=%v", plan.Scope, playwrightSelectionIDs(plan), plan.Unknown)
	}
}

func TestPlaywrightComputedStringsAndUnsupportedGlobsWiden(t *testing.T) {
	for _, matcher := range []string{`'**/' + 'example.spec.ts'`, `"**/[ab].spec.ts"`, `"**/test[ab].spec.ts"`, `"**/{*.spec,*.test}.ts"`,
		`"**/@(a|b).spec.ts"`, `"**/a.spec.+(ts|js)"`, `"**/{a}.spec.ts"`, `"**/a{1..3}.spec.ts"`, `"**/\\a.spec.ts"`,
		// GitHub #709 review round 5: `**` that is not a whole path component, and regex flags Go
		// does not map exactly (sticky, global, unicode, indices, unicode sets), are not modelled.
		`"**/tests/**.spec.ts"`, `"**/a**.spec.ts"`, `"**/***/a.spec.ts"`, `"tests/**a/*.spec.ts"`,
		`/a\.spec\.ts$/y`, `/a\.spec\.ts$/g`, `/a\.spec\.ts$/u`, `/a\.spec\.ts$/d`, `/a\.spec\.ts$/v`,
		// GitHub #709 review round 6: regex bodies Go and JavaScript read differently.
		`/\A.*a\.spec\.ts$/`, `/\sa\.spec\.ts$/`, `/(?i)a\.spec\.ts$/`, `/(a)\1\.spec\.ts$/`, `/[[:alpha:]]\.spec\.ts$/`, `/\x61\.spec\.ts$/`,
		// GitHub #709 review round 7: a leading-zero repeat bound is repetition only in JavaScript, and
		// a character outside the Basic Multilingual Plane is two UTF-16 code units in JavaScript.
		`/a{01}\.spec\.ts$/`, "/\U0001F600?a\\.spec\\.ts$/", "\"**/\U0001F600*.spec.ts\"",
		// GitHub #709 review round 8: minimatch collapses runs of `/`, and a leading `/` becomes
		// `**//` once Playwright prefixes `**/`.
		`"**/tests//*.spec.ts"`, `"tests//a.spec.ts"`, `"/tests/*.spec.ts"`,
		// GitHub #709 review round 9: a brace alternative holding `/` can expand to `//`.
		`"tests/{/,x}*.spec.ts"`, `"{tests/,x}a.spec.ts"`} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
		write(t, root, "playwright.config.ts", `export default { projects: [{ name: "p", testMatch: `+matcher+` }] }`)
		write(t, root, "tests/a.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
		plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/a.spec.ts"})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Scope != affected.ScopeUnknown || len(plan.Selected) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectMembership) {
			t.Fatalf("matcher %s narrowed unsafely: %+v", matcher, plan)
		}
	}
}

func TestPlaywrightUnknownMembershipAbstainsFromFileUnits(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [{ name: "p", testMatch: [/other\.ts/, computedMatcher] }] }`)
	write(t, root, "tests/fixtures.ts", `export { test } from "@playwright/test"`)
	write(t, root, "tests/custom.ts", `import { test } from "./fixtures"; test("x", () => {})`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/custom.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || len(plan.Selected) != 0 {
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
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/setup.ts"})
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
	first, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
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

func TestPlaywrightRequirementClaims(t *testing.T) {
	root := playwrightFixture(t)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("TJAA-V0-010 binds configuration and project identity", func(t *testing.T) {
		angular := playwrightProject(plan, "angular")
		chromium := playwrightProject(plan, "chromium")
		if plan.Config.SHA256 == "" || angular.Grep != "/@angular/" || angular.Metadata == "" || chromium.Device != "Desktop Chrome" {
			t.Fatalf("config=%+v angular=%+v chromium=%+v", plan.Config, angular, chromium)
		}
	})
	t.Run("TJAA-V0-011 emits project-distinct physical units", func(t *testing.T) {
		want := []string{
			"typescript:playwright:angular:tests/login.spec.ts",
			"typescript:playwright:chromium:tests/login.spec.ts",
			"typescript:playwright:react:tests/login.spec.ts",
		}
		for _, id := range want {
			if !slices.Contains(playwrightSelectionIDs(plan), id) {
				t.Fatalf("project unit %s missing from %v", id, playwrightSelectionIDs(plan))
			}
		}
	})
	t.Run("TJAA-V0-012 dynamic identity widens selection", func(t *testing.T) {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", `export default { projects: [{ name: "p", use: { browserName: chosenBrowser } }] }`)
		if len(projects) != 1 || !hasPlaywrightUnknown(PlaywrightPlan{Unknown: unknown}, PlaywrightAxisSelection, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
		}
	})
	t.Run("TJAA-V0-013 source closure expands setup relations", func(t *testing.T) {
		for _, test := range []string{"tests/login.spec.ts", "tests/global.setup.ts", "tests/global.teardown.ts"} {
			if !containsPlaywrightTest(plan, test) {
				t.Fatalf("related test %s missing from %v", test, playwrightSelectionIDs(plan))
			}
		}
	})
	t.Run("TJAA-V0-014 grep and metadata are identity not exclusions", func(t *testing.T) {
		angular := playwrightProject(plan, "angular")
		if angular.Grep != "/@angular/" || angular.Metadata == "" || !slices.Contains(playwrightSelectionIDs(plan), "typescript:playwright:angular:tests/login.spec.ts") {
			t.Fatalf("angular=%+v selected=%v", angular, playwrightSelectionIDs(plan))
		}
	})
	t.Run("TJAA-V0-015 selection unknown retains the full relevant suite", func(t *testing.T) {
		unknownRoot := playwrightFixture(t)
		write(t, unknownRoot, "tests/login.spec.ts", `import { test } from "@playwright/test"; const p = "./pages/login"; test("x", async () => import(p))`)
		unknownPlan, selectErr := selectPlaywrightStatic(unknownRoot, "playwright.config.ts", []string{"tests/pages/login.ts"})
		if selectErr != nil {
			t.Fatal(selectErr)
		}
		if unknownPlan.Scope != affected.ScopeUnknown || unknownPlan.Fallback != PlaywrightFallbackFullSuite || len(unknownPlan.Excluded) != 0 {
			t.Fatalf("scope=%s fallback=%s excluded=%v", unknownPlan.Scope, unknownPlan.Fallback, unknownPlan.Excluded)
		}
	})
	t.Run("TJAA-V0-016 fixed inputs produce identical bounded bytes", func(t *testing.T) {
		second, selectErr := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
		if selectErr != nil {
			t.Fatal(selectErr)
		}
		firstBytes, firstErr := plan.Canonical()
		secondBytes, secondErr := second.Canonical()
		if firstErr != nil || secondErr != nil || !bytes.Equal(firstBytes, secondBytes) || plan.SourceDigest == "" {
			t.Fatalf("firstErr=%v secondErr=%v sourceDigest=%q", firstErr, secondErr, plan.SourceDigest)
		}
	})
	t.Run("TJAA-V0-017 mixed qualification fixture covers required variants", func(t *testing.T) {
		for _, project := range []string{"angular", "chromium", "react", "setup", "cleanup"} {
			if playwrightProject(plan, project).Name == "" {
				t.Fatalf("project %s missing from %+v", project, plan.Projects)
			}
		}
	})
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

// GitHub #709 review round 4: Playwright's createFileMatcher prefixes `**/` to a string glob that
// lacks it and matches globs case-insensitively (minimatch nocase); its default testMatch is such
// a glob, so `B.SPEC.ts` is a test. A regular expression keeps its own flags.
func TestPlaywrightStringGlobsArePrefixedAndCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default {
  testDir: "tests",
  projects: [
    { name: "relative", testMatch: ["**/a.spec.ts", "b.spec.ts"] },
    { name: "nocase", testMatch: "**/*.E2E.ts" },
    { name: "default" },
    { name: "regex", testMatch: /b\.SPEC\.ts$/ },
    { name: "flags", testMatch: /b\.SPEC\.ts$/ims },
    { name: "globstar", testMatch: "tests/**" },
  ],
}`)
	write(t, root, "tests/a.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	write(t, root, "tests/deep/b.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	write(t, root, "tests/C.SPEC.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	write(t, root, "tests/d.e2e.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/deep/b.spec.ts", "tests/C.SPEC.ts", "tests/d.e2e.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if got := playwrightSelectionIDs(plan); !slices.Equal(got, []string{
		"typescript:playwright:default:tests/C.SPEC.ts",
		"typescript:playwright:default:tests/deep/b.spec.ts",
		"typescript:playwright:flags:tests/deep/b.spec.ts",
		"typescript:playwright:globstar:tests/C.SPEC.ts",
		"typescript:playwright:globstar:tests/d.e2e.ts",
		"typescript:playwright:globstar:tests/deep/b.spec.ts",
		"typescript:playwright:nocase:tests/d.e2e.ts",
		"typescript:playwright:relative:tests/deep/b.spec.ts",
	}) {
		t.Fatalf("selection=%v unknown=%v", got, plan.Unknown)
	}
}

// GitHub #709 review round 5: the config scanners skip a template literal with its nested
// substitutions and templates, and refuse one that is unterminated or nested past the bound.
func TestPlaywrightScannersSkipNestedTemplates(t *testing.T) {
	if items, ok := playwrightSplitTopLevel("a: `${`x,y`}`, b: 1"); !ok || !slices.Equal(items, []string{"a: `${`x,y`}`", "b: 1"}) {
		t.Fatalf("split=%q ok=%v", items, ok)
	}
	if colon := playwrightTopLevelColon("`${`:`}`"); colon != -1 {
		t.Fatalf("colon inside a nested template found at %d", colon)
	}
	if value, next, ok := playwrightBalancedValue("(`${`)`}`)", 1, ')'); !ok || value != "`${`)`}`" || next != 10 {
		t.Fatalf("balanced=%q next=%d ok=%v", value, next, ok)
	}
	deep := strings.Repeat("`${", jsTemplateMaxDepth+1) + strings.Repeat("}`", jsTemplateMaxDepth+1)
	for _, refused := range []string{"`${`", "`${ '}' `", deep} {
		if _, ok := playwrightSplitTopLevel(refused); ok {
			t.Fatalf("split accepted %q", refused)
		}
		if _, _, ok := playwrightBalancedValue(refused+")", 0, ')'); ok {
			t.Fatalf("balanced accepted %q", refused)
		}
		if colon := playwrightTopLevelColon(refused + ":"); colon != -1 {
			t.Fatalf("colon after %q found at %d", refused, colon)
		}
	}
}

// GitHub #709 review round 5 follow-up: a test path holding a line terminator other than LF is one
// Go and JavaScript regular expressions can disagree on, so static membership widens.
func TestPlaywrightLineTerminatorPathWidensMembership(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `export default { projects: [{ name: "p", testIgnore: /a.b/ }] }`)
	write(t, root, "tests/a\u2028b.spec.ts", `import { test } from "@playwright/test"; test("x", () => {})`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/a\u2028b.spec.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || len(plan.Selected) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownProjectMembership) {
		t.Fatalf("line-terminator path narrowed: %+v", plan)
	}
}

// GitHub #709 review round 6: only regex constructs JavaScript (without u) and Go RE2 read the
// same way are static; `\A` is a literal A in JavaScript and a begin-text anchor in Go.
func TestPlaywrightRegexBodyAllowlist(t *testing.T) {
	for _, body := range []string{`\d+\.spec\.ts$`, `\.`, `[a-z]{2,3}`, `(?:a|b)`, `^\/x\/(a)*?b{2}c{1,}d{0,1000}?$`, `a{0}b{0,0}c{10,100}`, "\u00e9\uffff", `[^/\]\-]+\/\w\W\D\b\B\t\n\r\f\v.`, `[-a-c_]|x+?`} {
		if !playwrightRegexBodyStatic(body) {
			t.Errorf("refused %q", body)
		}
		if _, ok := compilePlaywrightMatcher("/" + body + "/i"); !ok {
			t.Errorf("allowlisted %q does not compile", body)
		}
	}
	for _, body := range []string{`\A.*b`, `a\z`, `\s`, `\S`, `(?i)a`, `(?<n>a)`, `(?P<n>a)`, `(?=a)`, `(?!a)`, `(?<=a)`, `(a)\1`, `\k<n>`,
		`[[:alpha:]]`, `[a[b]`, `[]`, `[^]`, `[\b]`, `\x41`, `\u0041`, `\0`, `\p{L}`, `\Q.\E`, `\cA`, `\e`, `a{`, `a{1001}`, `a{3,2}`, `a{,2}`,
		`a{1}{2}`, `{1}`, `*a`, `a**`, `a???`, `^*`, `\b+`, `a}`, `a]`, `[a`, `a\`, `(?`,
		// GitHub #709 review round 7: JavaScript reads a leading-zero bound as a repeat count and Go
		// as literal text; a non-BMP character is a surrogate pair in JavaScript and one rune in Go.
		`a{01}`, `a{00}`, `a{1,02}`, `a{1, 2}`, `a{ 1}`, `a{,2}`, "\U0001F600", "a\U0001F600?", "[\U0001F600]", "[a-\U0001F600]", "\\\U0001F600"} {
		if playwrightRegexBodyStatic(body) {
			t.Errorf("admitted %q", body)
		}
	}
}

// GitHub #709 review round 8: verdicts of the bundled minimatch 3.1.5 (Playwright 1.61.1
// createFileMatcher: `**/` prefix, nocase, dot) on /repo/e2e/b.spec.ts. A glob the static model
// would read differently is refused; `.` and `..` components and a trailing `/` match nothing in
// either, so they stay static and agree.
func TestPlaywrightGlobAgreesWithBundledMinimatch(t *testing.T) {
	const file = "/repo/e2e/b.spec.ts"
	for _, row := range []struct {
		glob    string
		matches bool
	}{
		{"**/e2e/*.spec.ts", true}, {"e2e/*.spec.ts", true}, {"**/E2E/B.SPEC.TS", true}, {"**/e2e/**", true},
		{"**/./e2e/*.spec.ts", false}, {"./e2e/*.spec.ts", false}, {"**/e2e/./*.spec.ts", false},
		{"**/x/../e2e/*.spec.ts", false}, {"**/*/../e2e/b.spec.ts", false}, {"**/e2e/*.spec.ts/", false},
		{"**/e2e/", false}, {"**/e2e/**/", false}, {"**/e2e/b.spec.ts/**", false},
	} {
		matcher, ok := compilePlaywrightMatcher(`"` + row.glob + `"`)
		if !ok {
			t.Errorf("%s: refused a glob the static model reads like minimatch", row.glob)
			continue
		}
		if got := playwrightAnyMatcher([]playwrightMatcher{matcher}, file); got != row.matches {
			t.Errorf("%s: matched=%v, minimatch %v", row.glob, got, row.matches)
		}
	}
	// minimatch collapses runs of `/` in the glob, so these match in Playwright.
	// GitHub #709 review round 9: brace expansion runs first, so an alternative holding `/` can
	// introduce `//` (`**/e2e/{/,x}*.spec.ts` matches); any such alternative is refused.
	for _, glob := range []string{"**/e2e//*.spec.ts", "/repo/e2e/*.spec.ts", "e2e//b.spec.ts", "**/e2e/**//b.spec.ts",
		"e2e/{/,x}*.spec.ts", "**/e2e/{x,/}*.spec.ts", "**/{e2e/,x}b.spec.ts", "**/{a/,x}e2e/b.spec.ts"} {
		if _, ok := compilePlaywrightMatcher(`"` + glob + `"`); ok {
			t.Errorf("%s: a repeated-slash glob was static", glob)
		}
	}
}

// GitHub #709 review round 10: a `/` after `=>` or a reserved word starts a regular expression;
// after an operand it is division; after a contextual keyword it is ambiguous and refused.
func TestPlaywrightSlashStartsRegex(t *testing.T) {
	for _, row := range []struct {
		source       string
		regex, known bool
	}{
		{"/a/", true, true}, {"x = /a/", true, true}, {"f(/a/", true, true}, {"() => /a/", true, true},
		{"return /a/", true, true}, {"typeof /a/", true, true}, {"x instanceof /a/", true, true},
		{"k in /a/", true, true}, {"new /a/", true, true}, {"delete /a/", true, true}, {"void /a/", true, true},
		{"throw /a/", true, true}, {"case /a/", true, true}, {"do /a/", true, true}, {"else /a/", true, true},
		{"x / 2", false, true}, {"(x) / 2", false, true}, {"a[0] / 2", false, true}, {"2 / 2", false, true},
		{"x.return / 2", false, true}, {"x?.in / 2", false, true}, {"returns / 2", false, true}, {"x >= /a/", true, true},
		{"of / 2", false, false}, {"yield /a/", false, false}, {"await /a/", false, false},
		{"éreturn / 2", false, false}, {"\\u0061return / 2", false, false},
		// GitHub #709 review round 11: a non-ASCII byte or an identifier escape directly before the
		// slash cannot be classified (a non-ASCII space would make it a regex).
		{"caf\u00e9 / 2", false, false}, {"\u03c0 / 2", false, false}, {"x =\u00a0/a/", false, false},
		{"a\\u0061 / 2", false, false}, {"a\\u{61} / 2", false, false}, {"x = {} / 2", false, true},
	} {
		// The slash under test is the first one in the source.
		if regex, known := playwrightSlashStartsRegex(row.source, strings.IndexByte(row.source, '/')); regex != row.regex || known != row.known {
			t.Errorf("%q: regex=%v known=%v, want %v %v", row.source, regex, known, row.regex, row.known)
		}
	}
	// The array splitter shares the rule: a regex after `=>` keeps its comma inside one item.
	items, ok := playwrightSplitTopLevel("() => /a,b/, 'c'")
	if !ok || len(items) != 2 {
		t.Errorf("split over an arrow regex = %q, %v", items, ok)
	}
	for _, source := range []string{"of / 2, 'c'", "caf\u00e9 / 2, 'c'", "\u03c0 / 2, 'c'", "a\\u0061 / 2, 'c'", "a\\u{61} / 2, 'c'"} {
		if _, ok := playwrightSplitTopLevel(source); ok {
			t.Errorf("split over an ambiguous slash in %q was accepted", source)
		}
		if _, ok := playwrightLex("const x = [" + source + "];\n"); ok {
			t.Errorf("lexed an ambiguous slash in %q", source)
		}
	}
}
