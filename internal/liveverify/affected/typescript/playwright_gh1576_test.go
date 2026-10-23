package typescript

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-035 (GitHub #1576): environment reads and numeric conversions in non-identity use keys,
// and trailing commas in array literals, keep the browser identity resolved; a dynamic identity
// key still fails closed.
func TestPlaywrightEnvUseValuesKeepIdentity_GH1576(t *testing.T) {
	config := func(setup, global, local string) string {
		return "import { defineConfig, devices } from '@playwright/test';\n" + setup + "\nexport default defineConfig({\n  use: " + global + ",\n  projects: [{ name: 'p', use: " + local + " }],\n});\n"
	}
	global := "{ baseURL: process.env.BASE_URL ?? 'http://localhost:3000', trace: process.env.CI ? 'retain-on-failure' : 'off', navigationTimeout: Number.parseInt(process.env.T || '60000'), actionTimeout: parseFloat(process.env.A ?? '5'), launchOptions: { args: ['--a', '--b',], }, }"
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config("", global, "{ ...devices['Desktop Firefox'], }"))
	if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "firefox" || projects[0].Device != "Desktop Firefox" {
		t.Fatalf("environment-driven non-identity use widened: projects=%+v unknown=%+v", projects, unknown)
	}
	for _, row := range []struct{ name, setup, global string }{
		{"dynamic identity key", "", "{ channel: process.env.CHANNEL }"},
		{"dynamic browserName", "", "{ browserName: process.env.BROWSER }"},
		{"process shadowed", "const process = { get env() { return go(); } };", "{ baseURL: process.env.BASE_URL }"},
		{"process.env replaced", "process.env = { BASE_URL: 'x' };", "{ baseURL: process.env.BASE_URL }"},
		{"process.env written inside a function", "function f() { process.env.BASE_URL = 'x'; }", "{ baseURL: process.env.BASE_URL }"},
		{"process member other than env", "", "{ baseURL: process.argv[2] }"},
		{"process passed to a call", "inspect(process);", "{ baseURL: process.env.BASE_URL }"},
		{"global object written", "globalThis.extra = 1;", "{ baseURL: process.env.BASE_URL }"},
		{"Number.parseInt replaced", "Number.parseInt = (v) => v;", "{ navigationTimeout: Number.parseInt(process.env.T || '60000') }"},
		{"parseInt shadowed", "function parseInt(v) { return v; }", "{ navigationTimeout: parseInt(process.env.T || '60000') }"},
		{"conversion of a non-primitive", "", "{ navigationTimeout: Number.parseInt(options) }"},
		{"member of the conversion", "", "{ navigationTimeout: Number.parseInt('1').toString() }"},
		{"other Number member", "", "{ navigationTimeout: Number.prototype }"},
	} {
		t.Run(row.name, func(t *testing.T) {
			projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(row.setup, row.global, "{ ...devices['Desktop Firefox'] }"))
			if !playwrightIdentityUnverified(projects, unknown) {
				t.Fatalf("unproven value was admitted: %s / %s: %+v", row.setup, row.global, unknown)
			}
		})
	}
}

// TJAA-V0-035 (GitHub #1576): a computed launchOptions.executablePath is an execution-axis
// unknown; it neither fails the browser identity nor removes the bounded selection.
func TestPlaywrightComputedExecutablePathIsExecutionUnknown_GH1576(t *testing.T) {
	root := playwrightFixture(t)
	body := readFixture(t, root, "playwright.config.ts")
	patched := strings.Replace(body, `use: { ...devices["Desktop Chrome"] },`, `use: { ...devices["Desktop Chrome"], launchOptions: { executablePath: process.env.CHROME_PATH, args: ["--no-sandbox",] } },`, 1)
	if patched == body {
		t.Fatal("fixture shape changed")
	}
	write(t, root, "playwright.config.ts", patched)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone || len(plan.Selected) == 0 {
		t.Fatalf("scope=%s fallback=%s selected=%d unknown=%v", plan.Scope, plan.Fallback, len(plan.Selected), plan.Unknown)
	}
	if !hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownBrowserExecutable) || hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("executable unknown not on the execution axis: %v", plan.Unknown)
	}
	if chromium := playwrightProject(plan, "chromium"); chromium.Browser != "chromium" || chromium.Device != "Desktop Chrome" {
		t.Fatalf("chromium identity=%+v", chromium)
	}
	// A launchOptions object with any other computed property still fails the identity.
	for _, value := range []string{
		"{ executablePath: process.env.CHROME_PATH, args: [flag] }",
		"{ executablePath: resolve() }",
		"{ ...base, executablePath: process.env.CHROME_PATH }",
	} {
		source := "import { defineConfig, devices } from '@playwright/test';\nexport default defineConfig({ projects: [{ name: 'p', use: { ...devices['Desktop Chrome'], launchOptions: " + value + " } }] });\n"
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !playwrightIdentityUnverified(projects, unknown) {
			t.Fatalf("launchOptions %s was admitted: %+v", value, unknown)
		}
	}
}

// TJAA-V0-036 (GitHub #1577): project dependencies bound to a top-level const whose initializer
// is a literal array or a conditional of literal arrays resolve to the union of the branches,
// for selection and for discovery; any other binding still refuses.
func TestPlaywrightConstDependenciesResolveToBranchUnion_GH1577(t *testing.T) {
	config := func(setup string) string {
		return "import { defineConfig } from '@playwright/test';\n" + setup + "\nexport default defineConfig({\n  projects: [\n    { name: 'setup', use: { browserName: 'chromium' } },\n    { name: 'auth', use: { browserName: 'chromium' } },\n    { name: 'e2e', use: { browserName: 'chromium' }, dependencies: deps },\n  ],\n});\n"
	}
	for _, row := range []struct {
		setup string
		want  []string
	}{
		{"const deps = ['setup',];", []string{"setup"}},
		{"const deps: string[] = process.env.CI\n  ? ['setup', 'auth']\n  : ['setup'];", []string{"auth", "setup"}},
		{"const deps = (process.env.A || process.env.B) ? [] : process.env.C ? ['auth'] : (['setup']);", []string{"auth", "setup"}},
	} {
		source := config(row.setup)
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
		e2e := slices.IndexFunc(projects, func(project PlaywrightProject) bool { return project.Name == "e2e" })
		if len(unknown) != 0 || len(projects) != 3 || e2e < 0 || !slices.Equal(projects[e2e].Dependencies, row.want) {
			t.Fatalf("%s: projects=%+v unknown=%+v", row.setup, projects, unknown)
		}
		if _, _, err := staticPlaywrightProjects("playwright.config.ts", []byte(source), false); err != nil {
			t.Fatalf("%s: discovery refused: %v", row.setup, err)
		}
	}
	for _, setup := range []string{
		"let deps = ['setup'];",
		"const deps = ['setup'];\ndeps.push('auth');",
		"const deps = ['setup'];\nconst alias = deps;",
		"const deps = process.env.CI ? list : ['setup'];",
		"const deps = ['setup', name];",
		"const deps = compute();",
		"const deps = ['setup'];\nconst deps2 = ['auth'];\n{ const deps = ['auth']; }",
		"export const deps = ['setup'];\nmutate(deps);",
		"const deps = ['setup'];\nconst holder = { value: deps };\nholder.value.push('auth');",
		"const deps = ['setup'];\nconst holder = { dependencies: deps };\nholder.dependencies.push('auth');",
		"const deps = ['setup'];\nconst other = { projects: [{ dependencies: deps }] };\nother.projects[0].dependencies.push('auth');",
		"const deps = ['setup'];\nconst pick = flag ? dependencies : deps;",
		"",
	} {
		source := config(setup)
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownProjectDependency) {
			t.Fatalf("%q: dynamic dependencies were resolved: %+v", setup, unknown)
		}
		if _, _, err := staticPlaywrightProjects("playwright.config.ts", []byte(source), false); err == nil {
			t.Fatalf("%q: discovery accepted dynamic dependencies", setup)
		}
	}
}

func readFixture(t *testing.T, root, relative string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
