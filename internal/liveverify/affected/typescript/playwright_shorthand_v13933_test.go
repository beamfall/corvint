package typescript

import (
	"strings"
	"testing"
)

// v13933Config is the VanillaTwisted shape: a shared `use` block with a shorthand baseURL, and
// projects identified by a literal browserName or a device spread.
func v13933Config(sharedUse, projectUse string) string {
	return "import { defineConfig, devices } from '@playwright/test';\n\n" +
		"const baseURL = \"http://127.0.0.1:5173\";\n" +
		"const channel = \"chrome\";\n" +
		"const browserName = \"firefox\";\n\n" +
		"export default defineConfig({\n  testDir: './e2e',\n" +
		"  use: { " + sharedUse + " },\n" +
		"  projects: [\n" +
		"    { name: 'chromium', use: { browserName: 'chromium'" + projectUse + " } },\n" +
		"    { name: 'mobile-safari', use: { ...devices['iPhone 13'], browserName: 'webkit' } },\n" +
		"  ],\n});\n"
}

// TJAA-V0-600 (V1-3933): a shorthand property for a non-identity option never decides the
// identity; one for an identity option stays unresolved and names the key.
func TestPlaywrightUseShorthandProperty_V13933(t *testing.T) {
	for _, shared := range []string{
		"baseURL, trace: 'on-first-retry', screenshot: 'only-on-failure', reducedMotion: 'reduce'",
		"trace: 'on', baseURL",
		"baseURL,",
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", v13933Config(shared, ""))
		if len(unknown) != 0 || len(projects) != 2 || projects[0].Browser != "chromium" || projects[1].Browser != "webkit" || projects[1].Device != "iPhone 13" {
			t.Fatalf("%q: projects=%+v unknown=%+v", shared, projects, unknown)
		}
	}
	// A shorthand in a project's own use layer is read the same way.
	if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", v13933Config("trace: 'on'", ", baseURL")); len(unknown) != 0 || len(projects) != 2 || projects[0].Browser != "chromium" {
		t.Fatalf("project shorthand: projects=%+v unknown=%+v", projects, unknown)
	}
	// An identity option written as a shorthand still leaves the identity unresolved.
	for _, key := range []string{"browserName", "channel"} {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", v13933Config("baseURL, "+key, ""))
		notes := gh1625Unknowns(nil, unknown, PlaywrightUnknownBrowserIdentity)
		if len(notes) == 0 {
			t.Fatalf("%s shorthand admitted: %+v", key, unknown)
		}
		for _, note := range notes {
			if !strings.Contains(note.Detail, "identity key "+key+" is a shorthand property at `"+key+"`") {
				t.Fatalf("%s: detail does not name it: %+v", key, note)
			}
		}
	}
	// A non-identifier item without a colon keeps refusing.
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", v13933Config("baseURL, 'trace'", "")); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("non-identifier item admitted: %+v", unknown)
	}
	// An undeclared shorthand binding is still a value Playwright reads; it never blocks identity.
	if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", v13933Config("storageState", "")); hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) || len(projects) != 2 {
		t.Fatalf("undeclared shorthand blocked identity: %+v", unknown)
	}
}

// TJAA-V0-601 (V1-3933): a device spread whose browser is not modelled keeps its identity when a
// layer names the browser literally, and leaves it unresolved, naming the spread, when none does.
func TestPlaywrightUnmodelledDeviceWithBrowserName_V13933(t *testing.T) {
	config := func(sharedUse, projectUse string) string {
		return "import { defineConfig, devices } from '@playwright/test';\n" +
			"export default defineConfig({\n  use: { " + sharedUse + " },\n" +
			"  projects: [{ name: 'phone', use: { " + projectUse + " } }],\n});\n"
	}
	for _, layers := range [][2]string{
		{"trace: 'on'", "...devices['iPhone 13'], browserName: 'webkit'"},
		{"trace: 'on'", "browserName: 'webkit', ...devices['iPhone 13']"},
		{"browserName: 'webkit'", "...devices['Pixel 5']"},
		{"...devices['iPhone 13']", "browserName: 'webkit'"},
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(layers[0], layers[1]))
		if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "webkit" || projects[0].Device == "" {
			t.Fatalf("%q: projects=%+v unknown=%+v", layers, projects, unknown)
		}
	}
	for _, layers := range [][2]string{
		{"trace: 'on'", "...devices['iPhone 13']"},
		{"...devices['iPhone 13']", "trace: 'on'"},
	} {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(layers[0], layers[1]))
		notes := gh1625Unknowns(nil, unknown, PlaywrightUnknownBrowserIdentity)
		if len(notes) != 1 || !strings.Contains(notes[0].Detail, `device "iPhone 13" has no modelled browser at `+"`...devices['iPhone`") || !strings.Contains(notes[0].Detail, "(playwright.config.ts:") {
			t.Fatalf("%q: unmodelled device without a browser: %+v", layers, unknown)
		}
	}
}
