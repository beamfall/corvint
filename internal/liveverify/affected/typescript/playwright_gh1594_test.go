package typescript

import (
	"strings"
	"testing"
)

// gh1594Config is the GitHub #1594 config shape: a top-level dotenv load, process.env writes to
// keys no identity key reads, a setup project named after the Playwright docs' global-setup
// convention, and a `use` block of the issue's default forms.
const gh1594Config = `import { defineConfig, devices } from '@playwright/test';
require('dotenv').config();

// The window and self of the browser are never touched here; global-setup runs first.
process.env.TZ ??= 'UTC';
process.env.RUN_ID ??= ` + "`run-${Date.now()}`" + `;

export default defineConfig({
  testDir: './tests',
  use: {
    baseURL: process.env['HOSTNAME'] || 'http://localhost:3000',
    storageState: process.env.STATE ?? 'state.json',
    trace: process.env.CI === 'true' ? 'on-first-retry' : 'off',
    navigationTimeout: Number.parseInt(process.env.NAV_TIMEOUT || '60000'),
    launchOptions: { executablePath: process.env.CHROME_PATH },
  },
  projects: [
    { name: 'global-setup', testMatch: 'setup/global-setup.ts' },
    { name: 'chromium', use: { ...devices['Desktop Chrome'] }, dependencies: ['global-setup'] },
  ],
});
`

// TJAA-V0-041..043: the issue's config resolves every project's identity and reports the computed
// executablePath on the execution axis.
func TestPlaywrightEnvIdentityIssueConfig_GH1594(t *testing.T) {
	projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1594Config)
	if len(unknown) != 0 || len(projects) != 2 {
		t.Fatalf("projects=%+v unknown=%+v", projects, unknown)
	}
	for _, project := range projects {
		if project.Browser != "chromium" || len(project.execution) != 1 || project.execution[0].Reason != PlaywrightUnknownBrowserExecutable {
			t.Fatalf("%s: browser=%s execution=%+v", project.Name, project.Browser, project.execution)
		}
	}
	if projects[0].Name != "chromium" || projects[0].Device != "Desktop Chrome" {
		t.Fatalf("device identity lost: %+v", projects[0])
	}
}

// TJAA-V0-041: a global object name blocks only in code (or as a computed string key), never as
// string, template or comment text.
func TestPlaywrightGlobalNamesInLiteralsKeepEnvReads_GH1594(t *testing.T) {
	config := func(setup string) string {
		return "import { defineConfig } from '@playwright/test';\n" + setup + "\nexport default defineConfig({ use: { baseURL: process.env.BASE_URL || 'http://x', navigationTimeout: Number(process.env.T || '1') }, projects: [{ name: 'p' }] });\n"
	}
	for _, setup := range []string{
		"const name = 'global-setup';",
		"const files = { a: \"window.ts\", b: `self-${1}`, c: 'globalThis' };",
		"// window, self, global, globalThis\n/* globalThis.process = {} */",
		"const re = /global/;",
	} {
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(setup)); len(unknown) != 0 {
			t.Fatalf("%q: %+v", setup, unknown)
		}
	}
	for _, setup := range []string{
		"globalThis.process = { env: {} };",
		"const g = global;",
		"const w = window;",
		"self.Number = String;",
		"const p = scope['globalThis'];",
		"const t = `${globalThis.process}`;",
		"const g = Function('return ' + 'globalThis')();",
		"const F = [].constructor;",
		"const k = 'constructor';",
	} {
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(setup)); !playwrightIdentityUnverified(projects, unknown) {
			t.Fatalf("%q was admitted: %+v", setup, unknown)
		}
	}
}

// TJAA-V0-042: a top-level write to a process.env key keeps env reads admitted; a write an
// identity key reads, a non-statement write and a replacement of process.env do not.
func TestPlaywrightEnvWritesOutsideIdentity_GH1594(t *testing.T) {
	config := func(setup, use string) string {
		return "import { defineConfig } from '@playwright/test';\n" + setup + "\nexport default defineConfig({ use: " + use + ", projects: [{ name: 'p' }] });\n"
	}
	for _, setup := range []string{
		"process.env.TZ ??= 'UTC';",
		"process.env.TZ = 'UTC'\nprocess.env['RUN'] ||= 'x'\nprocess.env.FLAG &&= '1';",
		"if (ci) {}\nprocess.env.TZ = 'UTC';",
		"process.env.BASE_URL ??= 'http://x';",
	} {
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(setup, "{ baseURL: process.env.BASE_URL }")); len(unknown) != 0 {
			t.Fatalf("%q: %+v", setup, unknown)
		}
	}
	for _, row := range []struct{ setup, use string }{
		{"process.env.CHROME = '/bin/chrome';", "{ launchOptions: { executablePath: process.env.CHROME } }"},
		{"const x = process.env.TZ = 'UTC';", "{ baseURL: process.env.BASE_URL }"},
		{"if (ci) process.env.TZ = 'UTC';", "{ baseURL: process.env.BASE_URL }"},
		{"process.env.TZ += 'x';", "{ baseURL: process.env.BASE_URL }"},
		{"Object.assign(process.env, { TZ: 'UTC' });", "{ baseURL: process.env.BASE_URL }"},
		{"process.env = {};", "{ baseURL: process.env.BASE_URL }"},
	} {
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(row.setup, row.use)); !playwrightIdentityUnverified(projects, unknown) {
			t.Fatalf("%q / %s was admitted: %+v", row.setup, row.use, unknown)
		}
	}
}

// TJAA-V0-043: each listed default form in a non-identity key keeps the identity resolved.
func TestPlaywrightEnvDefaultForms_GH1594(t *testing.T) {
	for _, value := range []string{
		"process.env.X || 'literal'",
		"process.env.X ?? 'literal'",
		"process.env['X']",
		"process.env[\"X\"] || 'literal'",
		"process.env.X === 'lit' ? 'a' : 'b'",
		"Number.parseInt(process.env.X || 'lit')",
		"Number.parseInt(process.env['X'] ?? '60000', 10)",
	} {
		source := "import { defineConfig, devices } from '@playwright/test';\nexport default defineConfig({ use: { baseURL: " + value + " }, projects: [{ name: 'p', use: { ...devices['Desktop Firefox'] } }] });\n"
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
		if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "firefox" {
			t.Fatalf("%s: projects=%+v unknown=%+v", value, projects, unknown)
		}
	}
}

// TJAA-V0-044: browser-identity-unresolved names the blocking token and its config line, and the
// scope token that refused a root the value needs. Under TJAA-V0-054 a non-identity value is named
// the same way by the selection-axis playwright:use-value-unverified unknown instead.
func TestPlaywrightBrowserIdentityNamesBlockingToken_GH1594(t *testing.T) {
	rows := []struct{ source, want string }{
		{
			"import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  use: {\n    trace: compute(),\n  },\n  projects: [{ name: 'p' }],\n});\n",
			"p use value can run code the static selection does not model: trace is not a side-effect-free value at `compute` (playwright.config.ts:4)",
		},
		{
			"import { defineConfig } from '@playwright/test';\nconst g = globalThis;\nexport default defineConfig({\n  projects: [{ name: 'p', use: { baseURL: process.env.URL } }],\n});\n",
			"p use value can run code the static selection does not model: baseURL is not a side-effect-free value at `process` (playwright.config.ts:4); process is not admitted: the config names the global object at `globalThis` (playwright.config.ts:2)",
		},
		{
			"import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  projects: [{ name: 'p', use: { channel: process.env.CHANNEL } }],\n});\n",
			"p use.browserName/device is dynamic or unsupported: identity key channel is not a static literal at `process` (playwright.config.ts:3)",
		},
		{
			"import { defineConfig } from '@playwright/test';\nfunction f() { process.env.X = '1'; }\nexport default defineConfig({\n  projects: [{ name: 'p', use: { baseURL: process.env.URL } }],\n});\n",
			"; process is not admitted: process is used other than as a process.env.NAME read or a top-level process.env.NAME write at `process` (playwright.config.ts:2)",
		},
	}
	for _, row := range rows {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", row.source)
		found := false
		for _, entry := range unknown {
			if (entry.Reason == PlaywrightUnknownBrowserIdentity || entry.Reason == PlaywrightUnknownUseValueUnverified) && entry.Axis == PlaywrightAxisSelection && strings.HasSuffix(entry.Detail, row.want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("want detail ending %q, got %+v", row.want, unknown)
		}
	}
}

// PR #1596 review, finding 2 (TJAA-V0-041): reflective access spelled without a raw token (an
// escaped or computed key, a non-literal require, a dynamic import, vm) could turn string text
// that names the global object into code, so string text no longer exempts the env and numeric
// roots in such a config.
func TestPlaywrightReflectiveAccessRefusesEnvAndNumericRoots_PR1596(t *testing.T) {
	escaped := `import { defineConfig, devices } from '@playwright/test';
import * as pw from '@playwright/test';

const g = (() => {})['construc\u0074or']('return globalThis')();
g['Num' + 'ber'] = () => {
  pw['devi' + 'ces']['Desktop Chrome'].defaultBrowserType = 'firefox';
  return 1;
};

export default defineConfig({
  use: { navigationTimeout: Number('1') },
  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }],
});
`
	if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", escaped); !playwrightIdentityUnverified(projects, unknown) {
		t.Fatalf("escaped constructor access was admitted: %+v", unknown)
	}
	config := func(setup string) string {
		return "import { defineConfig } from '@playwright/test';\n" + setup + "\nexport default defineConfig({ use: { baseURL: process.env.BASE_URL || 'http://x', navigationTimeout: Number(process.env.T || '1') }, projects: [{ name: 'p' }] });\n"
	}
	for _, setup := range []string{
		"const h = (() => {})['constr' + 'uctor']('return globalThis')();",
		"const h = o[key]('return globalThis');",
		"const h = o['a\\x62'];",
		"const k = `return global\\u0054his`;",
		"require('node:' + 'vm').runInThisContext('globalThis.Number = String');",
		"require('node:vm').runInThisContext('globalThis.Number = String');",
		"await import('data:text/javascript,globalThis.Number=String');",
	} {
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(setup)); !playwrightIdentityUnverified(projects, unknown) {
			t.Fatalf("%q was admitted: %+v", setup, unknown)
		}
	}
	// Literal keys, type annotations, array literals and a literal require stay admitted.
	for _, setup := range []string{
		"require('dotenv').config(); const name = 'global-setup';",
		"const list: string[] = ['window.ts', 'self']; const first = list[0]; const k = cfg['name'];",
	} {
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(setup)); len(unknown) != 0 {
			t.Fatalf("%q: %+v", setup, unknown)
		}
	}
}
