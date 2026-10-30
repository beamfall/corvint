package typescript

import (
	"slices"
	"strings"
	"testing"
)

// gh1605Division is the GitHub #1605 row 1 line: division after a call, then a line comment.
const gh1605Division = "const now = Date.now() / 1000; // Convert to seconds for comparison"

// gh1605Write is the GitHub #1605 row 3 top-level write: `??=` whose right side reads process.pid.
const gh1605Write = "process.env.E2E_RUN_ID ??= `${new Date().toISOString().replace(/[:.]/g, '-')}-${process.pid}`;"

// gh1605Config is the issue's config shape: the top-level write and the bracketed baseURL read.
func gh1605Config(baseURL, extraUse string) string {
	return "import { defineConfig, devices } from '@playwright/test';\n\n" + gh1605Write + "\n\nexport default defineConfig({\n  testDir: './tests',\n  use: {\n    " + baseURL + "\n" + extraUse + "  },\n  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],\n});\n"
}

// TJAA-V0-046 (GitHub #1605 row 1): a `/` after the `)` of a call is division, and the `//` after
// it is a comment, so the module raises neither scan-unresolved nor dynamic-import and keeps its
// imports. A `)` that closes a statement head stays ambiguous.
func TestDivisionAfterCallBeforeLineComment_GH1605(t *testing.T) {
	for _, line := range []string{
		gh1605Division,
		"const half = total(a, b) / 2; // halve it, see '/docs'",
		"const x = rows[0]() / 2 // '",
		"const y = f()(1) / 2 // `",
		"const z = obj.size(\n  a,\n) / 2 // '",
	} {
		body := "import { helper } from './helper';\n" + line + "\nconst b = require('./b');\n"
		for _, relative := range []string{"tests/a.spec.ts", "src/a.js"} {
			scan := scanImportSites(relative, body)
			if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || !slices.Contains(scan.refs, "./helper") || !slices.Contains(scan.refs, "./b") {
				t.Fatalf("%s %q: %+v", relative, line, scan)
			}
		}
	}
	for _, line := range []string{
		"if (c) /'/.test(s); const u = '//'; const b = require('./b')",
		"while (c) /'/.test(s); const u = '//'; const b = require('./b')",
		"for (;;) /'/.test(s); const u = '//'; const b = require('./b')",
		"x = f('a') /'/.test(s); const u = '//'; const b = require('./b')",
		"x = typeof (a) /'/.test(s); const u = '//'; const b = require('./b')",
		"with (o) /'/.test(s); const u = '//'; const b = require('./b')",
		"declare function f(a)\n/'/.test(s); const u = '//'; const b = require('./b')",
		"function f(a): void\n/'/.test(s); const u = '//'; const b = require('./b')",
		"x = f(/re/) /'/.test(s); const u = '//'; const b = require('./b')",
		"x = f(\u00e9) /'/.test(s); const u = '//'; const b = require('./b')",
		"x = f(" + strings.Repeat("a,", 130) + "a) /'/.test(s); const u = '//'; const b = require('./b')",
	} {
		if scan := scanImportSites("src/a.ts", line); scan.err == nil && scan.unresolved < 0 && scan.dynamic < 0 && !slices.Contains(scan.refs, "./b") {
			t.Fatalf("%q: a statement-head slash was read as division without a frontier: %+v", line, scan)
		}
	}
	if scan := scanImportSites("src/a.ts", "if (c) /'/.test(s); const u = '//'; const b = require('./b')"); scan.unresolved < 0 {
		t.Fatalf("an if-head slash is no longer ambiguous: %+v", scan)
	}

	root := playwrightFixture(t)
	write(t, root, "tests/pages/clock.ts", "import { test } from '@playwright/test';\nexport "+gh1605Division+"\n")
	write(t, root, "tests/clock.spec.ts", "import { test } from '@playwright/test'\nimport { now } from './pages/clock'\ntest('@chromium clock', async () => { void now })\n")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/clock.ts"})
	if err != nil {
		t.Fatal(err)
	}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == FrontierScanUnresolved || unknown.Reason == FrontierDynamicImport {
			t.Fatalf("division line raised %+v", unknown)
		}
	}
	if !slices.ContainsFunc(plan.Selected, func(unit PlaywrightSelection) bool { return unit.Test == "tests/clock.spec.ts" }) {
		t.Fatalf("clock spec not selected: %+v", plan.Selected)
	}
}

// TJAA-V0-047, TJAA-V0-048, TJAA-V0-049 (GitHub #1605 rows 2 and 3): the issue's config resolves
// the browser identity. `process.env['HOSTNAME']` and `process.env["HOSTNAME"]` are the same read
// as `process.env.HOSTNAME`, and the top-level `??=` write whose right side reads process.pid does
// not refuse the process root, because no identity field reads E2E_RUN_ID.
func TestPlaywrightIdentityIgnoresUnrelatedFieldsAndWrites_GH1605(t *testing.T) {
	for _, baseURL := range []string{
		"baseURL: process.env['HOSTNAME'] || 'http://127.0.0.1:3000',",
		`baseURL: process.env["HOSTNAME"] || 'http://127.0.0.1:3000',`,
		"baseURL: process.env.HOSTNAME || 'http://127.0.0.1:3000',",
	} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1605Config(baseURL, "    trace: process.env.CI ? 'on' : 'off',\n"))
		if len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "chromium" || projects[0].Device != "Desktop Chrome" {
			t.Fatalf("%s: projects=%+v unknown=%+v", baseURL, projects, unknown)
		}
	}
	// A process scalar read is admitted as a value too.
	if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1605Config("baseURL: `http://host-${process.pid}`,", "")); len(unknown) != 0 {
		t.Fatalf("process.pid value: %+v", unknown)
	}
	// Grouped reads that are only read stay admitted.
	for _, setup := range []string{"const n = (process.pid) + 1;", "const h = (process.env.HOSTNAME || '').length;", "const s = (process.env.HOSTNAME || '').split(',');", "const w = Number(process.env.WORKERS);"} {
		source := strings.Replace(gh1605Config("baseURL: process.env['HOSTNAME'],", ""), gh1605Write, gh1605Write+"\n"+setup, 1)
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source); len(unknown) != 0 || len(projects) != 1 || projects[0].Browser != "chromium" {
			t.Fatalf("%q: projects=%+v unknown=%+v", setup, projects, unknown)
		}
	}
	// An identity field that reads the written name, a call or write on process, and a computed
	// identity value still leave the identity unresolved.
	for _, row := range []struct{ baseURL, extra string }{
		{"baseURL: process.env['HOSTNAME'],", "    launchOptions: { executablePath: process.env.E2E_RUN_ID },\n"},
		{"baseURL: process.env['HOSTNAME'],", "    channel: process.env['CHANNEL'],\n"},
		{"baseURL: process.env['HOSTNAME'],", "    browserName: process.env.BROWSER,\n"},
		{"baseURL: process.env['HOSTNAME'],", "    defaultBrowserType: 'firefox',\n"},
		{"baseURL: process.env['HOSTNAME'],", "    browserName: process.platform === 'darwin' ? 'webkit' : 'chromium',\n"},
	} {
		if projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", gh1605Config(row.baseURL, row.extra)); !playwrightIdentityUnverified(projects, unknown) {
			t.Fatalf("%s was admitted: %+v", row.extra, unknown)
		}
	}
	for _, setup := range []string{
		"process.pid = 1;",
		"process.pid++;",
		"delete process.pid;",
		"const cwd = process.cwd();",
		"process.exit(process.pid);",
		"const p = process;",
		"const env = process.env;",
		// Calls, tags, constructions and writes through grouping parentheses.
		"const n = (process.pid)();",
		"const n = ((process.pid))();",
		"const n = (process.pid)?.();",
		"const n = (process.pid)`x`;",
		"const n = new (process.pid)();",
		"const n = new (process.pid);",
		"(process).x = 1;",
		"(process.pid)++;",
		"(process.pid)--;",
		"++(process.pid);",
		"(process.pid).x = 1;",
		"(process.pid)['x'] = 1;",
		"(process.pid).x++;",
		"delete (process.pid).x;",
		"const n = (process.env.HOSTNAME)();",
	} {
		source := strings.Replace(gh1605Config("baseURL: process.env['HOSTNAME'],", ""), gh1605Write, gh1605Write+"\n"+setup, 1)
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", source)
		found := false
		for _, entry := range append(unknown, playwrightExecution(projects)...) {
			// A delete or update token refuses every root, so the devices spread blocks first;
			// otherwise the refused process root marks baseURL unverified (TJAA-V0-054).
			if strings.Contains(setup, "delete") || strings.Contains(setup, "++") || strings.Contains(setup, "--") {
				found = found || entry.Reason == PlaywrightUnknownBrowserIdentity && strings.Contains(entry.Detail, "devices is not admitted: the config has a delete, ++, -- or escape token")
			} else {
				found = found || entry.Reason == PlaywrightUnknownUseValueUnverified && strings.Contains(entry.Detail, "process is not admitted")
			}
		}
		if !found {
			t.Fatalf("%q was admitted: %+v", setup, unknown)
		}
	}
}
