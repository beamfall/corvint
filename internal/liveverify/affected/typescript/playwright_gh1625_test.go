package typescript

import (
	"strings"
	"testing"
)

// gh1625Use holds the GitHub #1625 `use` lines verbatim. The config adds a later `workers` value
// that uses Number.isInteger: before TJAA-V0-055 it refused the Number root and the earliest
// refused occurrence was the `parseInt` member name, which reproduced the issue's message
// "navigationTimeout is not a side-effect-free value at Number (playwright.config.ts:10); Number is
// not admitted: a numeric conversion name is used other than as a direct call at parseInt".
const gh1625Use = "    navigationTimeout: Number.parseInt(process.env.NAVIGATION_TIMEOUT || '60000'),\n    actionTimeout: Number.parseInt(process.env.ACTION_TIMEOUT || '30000'),\n"

func gh1625Config(extraUse string) string {
	return "import { defineConfig, devices } from '@playwright/test';\n\n" + gh1605Write + "\n\nexport default defineConfig({\n  testDir: './tests',\n  use: {\n    baseURL: process.env['HOSTNAME'] || 'http://127.0.0.1:3000',\n" + gh1625Use + extraUse + "  },\n  workers: Number.isInteger(Number.parseInt(process.env.WORKERS || '')) ? Number.parseInt(process.env.WORKERS || '') : undefined,\n  projects: [\n    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },\n    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },\n  ],\n});\n"
}

func gh1625Unknowns(projects []PlaywrightProject, unknown []PlaywrightUnknown, reason string) []PlaywrightUnknown {
	found := []PlaywrightUnknown{}
	for _, entry := range append(append([]PlaywrightUnknown{}, unknown...), playwrightExecution(projects)...) {
		if entry.Reason == reason {
			found = append(found, entry)
		}
	}
	return found
}

// TJAA-V0-055 (GitHub #1625 part 2): Number.parseInt(...), Number.parseFloat(...) and
// Number.isInteger(...) are admitted like parseInt(...); the `parseInt` member name of a
// `Number.parseInt` call does not refuse the parseInt root, so the issue's config keeps every
// project's identity with no unknown at all.
func TestPlaywrightNumberMemberCallsAreAdmitted_GH1625(t *testing.T) {
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1625Config(""))
	if len(unknown) != 0 || len(projects) != 2 || projects[0].Browser != "chromium" || projects[1].Browser != "firefox" {
		t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
	}
	if execution := playwrightExecution(projects); len(execution) != 0 {
		t.Fatalf("pure values were reported: %+v", execution)
	}
	scope := newPlaywrightPureScope(gh1625Config(""))
	for _, name := range []string{"Number", "parseInt", "parseFloat"} {
		if !scope.numeric[name] {
			t.Fatalf("%s refused: %+v", name, scope.numericBlock)
		}
	}
	for _, value := range []string{"Number.parseInt('1')", "Number.parseFloat('1.5')", "Number.isInteger(1)", "parseInt('1') + Number.parseInt('2')", "Number.isInteger(Number.parseInt(process.env.T || ''))"} {
		source := "import { defineConfig, devices } from '@playwright/test';\nconst n = parseInt('3');\nexport default defineConfig({ use: { navigationTimeout: " + value + " }, projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }] });\n"
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); len(unknown) != 0 || len(playwrightExecution(projects)) != 0 {
			t.Fatalf("%s: unknown=%+v execution=%+v", value, unknown, playwrightExecution(projects))
		}
	}
	// A conversion that is shadowed, written or read other than as a call is still refused.
	for _, setup := range []string{"Number.parseInt = (v) => v;", "Number.isInteger = (v) => true;", "const f = Number.isInteger;", "function parseInt(v) { return v; }", "const p = Number?.parseInt('1');"} {
		source := "import { defineConfig, devices } from '@playwright/test';\n" + setup + "\nexport default defineConfig({ use: { navigationTimeout: Number.parseInt('1') + parseInt('2') }, projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }] });\n"
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
		if len(gh1625Unknowns(projects, unknown, PlaywrightUnknownUseValueUnverified)) == 0 {
			t.Fatalf("%q: refused conversion was not reported: unknown=%+v execution=%+v", setup, unknown, playwrightExecution(projects))
		}
	}
	// An operand that is not proven primitive (a member read of a declared literal is admitted,
	// the literal itself is not) is not side-effect free under the conversion rule either.
	source := "import { defineConfig, devices } from '@playwright/test';\nconst options = { a: 1 };\nexport default defineConfig({ use: { navigationTimeout: Number.isInteger(options) }, projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }] });\n"
	if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); len(gh1625Unknowns(projects, unknown, PlaywrightUnknownUseValueUnverified)) == 0 {
		t.Fatalf("non-primitive operand was not reported: %+v", unknown)
	}
}

// TJAA-V0-054 (GitHub #1625 part 1): browser identity is derived only from the identity fields and
// the env names they read. A non-identity field (navigationTimeout, actionTimeout, baseURL, trace,
// ...) never leaves it unresolved, whatever its value form; one that is not side-effect free is
// named by a selection-axis playwright:use-value-unverified unknown at its key and line.
func TestPlaywrightNonIdentityFieldsNeverBlockIdentity_GH1625(t *testing.T) {
	// The issue's refusal before TJAA-V0-055 came from Number; any other value form is now
	// ignored for identity too.
	for _, extra := range []string{
		"    trace: compute(),\n",
		"    baseURL: getBaseUrl(process.env.ENV),\n",
		"    navigationTimeout: timeouts.navigation * 2,\n",
		"    storageState: path.join(__dirname, 'auth.json'),\n",
		"    extraHTTPHeaders: { Authorization: `Bearer ${token()}` },\n",
		"    video: process.env.CI ? (await mode()) : 'off',\n",
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1625Config(extra))
		if hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) || len(projects) != 2 || projects[0].Browser != "chromium" || projects[0].Device != "Desktop Chrome" || projects[1].Browser != "firefox" {
			t.Fatalf("%q blocked identity: projects=%+v unknown=%+v", extra, projects, unknown)
		}
		key := strings.TrimSpace(extra[:strings.Index(extra, ":")])
		notes := gh1625Unknowns(nil, unknown, PlaywrightUnknownUseValueUnverified)
		if len(notes) != 2 || len(unknown) != 2 || notes[0].Axis != PlaywrightAxisSelection || !strings.Contains(notes[0].Detail, key+" is not a side-effect-free value at `") || !strings.Contains(notes[0].Detail, "(playwright.config.ts:11)") {
			t.Fatalf("%q: notes=%+v", extra, notes)
		}
		if execution := playwrightExecution(projects); len(execution) != 0 {
			t.Fatalf("%q: unverified value is only an execution note: %+v", extra, execution)
		}
	}
	// Identity fields still decide: a computed one leaves the identity unresolved.
	for _, extra := range []string{
		"    browserName: process.env.BROWSER,\n",
		"    channel: process.env.CHANNEL,\n",
		"    defaultBrowserType: 'firefox',\n",
		"    launchOptions: { executablePath: process.env.E2E_RUN_ID },\n",
		"    launchOptions: { executablePath: process.env.CHROME, args: flags() },\n",
		"    viewport: size(),\n",
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1625Config(extra))
		if !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("%q: computed identity field was admitted: projects=%+v", extra, projects)
		}
	}
	// A non-identity value that rewrites a descriptor in the config refuses the devices root,
	// so the spread still blocks.
	source := gh1625Config("    trace: (devices['Desktop Chrome'].defaultBrowserType = 'firefox', 'on'),\n")
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("descriptor rewrite was admitted: %+v", unknown)
	}
}
