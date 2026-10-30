package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TJAA-V0-090 (GitHub #1701 item 1): a plain JSON.stringify call, such as a module-level helper
// that encodes a request body, leaves the tag closure exact because its value is a string; a
// JSON use whose value or replacer can reach a tag still stops and names its line.
func TestPlaywrightTagClosurePlainStringify_GH1701(t *testing.T) {
	const stop = "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line "
	helper := "function body(payload: unknown) {\n  return { data: Buffer.from(JSON.stringify(payload)).toString('base64') };\n}\n"
	for _, row := range []struct {
		label, plain, stop string
	}{
		{"helper encodes a request body", helper + "test('plain', async ({ request }) => {\n  await request.post('/x', body({ a: 1 }));\n});\n", ""},
		{"null replacer and space", "const text = JSON.stringify({ a: 1 }, null, 2);\ntest('plain', async () => {});\n", ""},
		{"stringify in a test body", "test('plain', { annotation: { type: 'a', description: 'b' } }, async () => {\n  const body = JSON.stringify({ a: 1 });\n});\n", ""},
		{"replacer function", "const text = JSON.stringify({ a: 1 }, (key, value) => value);\ntest('plain', async () => {});\n", stop + "1: `JSON` can build or enumerate a property at run time)"},
		{"replacer name", "const keep = (key: string, value: unknown) => value;\nconst text = JSON.stringify({ a: 1 }, keep);\ntest('plain', async () => {});\n", stop + "2: `JSON` can build or enumerate a property at run time)"},
		{"spread arguments", "const args = [{ a: 1 }];\nconst text = JSON.stringify(...args);\ntest('plain', async () => {});\n", stop + "2: `JSON` can build or enumerate a property at run time)"},
		{"stringify read without a call", "const encode = JSON.stringify;\ntest('plain', async () => {});\n", stop + "1: `JSON` can build or enumerate a property at run time)"},
		{"parse value can reach details", helper + "const details = JSON.parse('{}');\ntest('plain', details, async () => {});\n", stop + "4: `JSON` can build or enumerate a property at run time)"},
		{"shadowed JSON", "const JSON = { stringify: (value: unknown) => '' };\nconst text = JSON.stringify(1);\ntest('plain', async () => {});\n", stop + "1: `JSON` can build or enumerate a property at run time)"},
		{"optional member", "const text = JSON?.stringify(1);\ntest('plain', async () => {});\n", stop + "1: `JSON` can build or enumerate a property at run time)"},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", row.plain)
			unverified, omissions := gh1647Omission(t, root, grepReproCases())
			if row.stop == "" {
				if len(unverified) != 0 || len(omissions) != 0 {
					t.Fatalf("not verified: unverified=%+v omissions=%+v", unverified, omissions)
				}
				return
			}
			if len(omissions) != 1 || !strings.Contains(omissions[0].Detail, row.stop) {
				t.Fatalf("omissions=%+v, want a stop naming %q", omissions, row.stop)
			}
		})
	}
}

// TJAA-V0-091 (GitHub #1701 item 2): in path mode a literal string testMatch in a project entry
// resolves, so the config raises no executable-config frontier when every file it assigns is a
// Playwright test of the config; a computed, regular-expression, template or global testMatch, or
// a glob a test path's own segments cannot decide, still names the testMatch line.
func TestPlaywrightPathModeLiteralTestMatch_GH1701(t *testing.T) {
	const setup = "    { name: 'auth', testMatch: 'setup/global-setup.ts' },\n"
	at := FrontierConfig + ":playwright.config.ts:6 `testMatch`"
	for _, row := range []struct {
		label, projects, want string
		config                string
	}{
		{"literal string", setup + gh1688Projects, "", ""},
		{"array of literal strings", "    { name: 'auth', testMatch: ['setup/global-setup.ts', 'setup/*.setup.ts'] },\n" + gh1688Projects, "", ""},
		{"two literal projects", setup + "    { name: 'teardown', testMatch: \"setup/teardown.ts\" },\n" + gh1688Projects, "", ""},
		{"computed", "    { name: 'auth', testMatch: process.env.SETUP_MATCH },\n" + gh1688Projects, at, ""},
		{"template", "    { name: 'auth', testMatch: `setup/${'global'}-setup.ts` },\n" + gh1688Projects, at, ""},
		{"regular expression", "    { name: 'auth', testMatch: /global-setup\\.ts/ },\n" + gh1688Projects, at, ""},
		// TJAA-V0-092 (GitHub #1717): a whole-segment globstar that no test path's tail reaches
		// resolves; one whose tail a path outside the setup directory matches keeps the site.
		{"globstar", "    { name: 'auth', testMatch: 'setup/**/global-setup.ts' },\n" + gh1688Projects, "", ""},
		{"globstar tail reaches a test path", "    { name: 'auth', testMatch: 'setup/**/*.spec.ts' },\n" + gh1688Projects, at, ""},
		{"longer glob no test path reaches", "    { name: 'auth', testMatch: 'x/y/setup/global-setup.ts' },\n" + gh1688Projects, "", ""},
		{"longer than a test path", "    { name: 'auth', testMatch: 'repo/tests/setup/global-setup.ts' },\n" + gh1688Projects, at, ""},
		{"global literal", gh1688Projects, FrontierConfig + ":playwright.config.ts:5 `testMatch`", "import { defineConfig, devices } from '@playwright/test';\nconst deps = process.env.X === '1' ? ['setup'] : ['setup'];\nexport default defineConfig({\n  testDir: './tests',\n  testMatch: 'setup/global-setup.ts',\n  projects: [\n" + gh1688Projects + "  ],\n});\n"},
		{"spread project", "    { ...{ testMatch: 'setup/global-setup.ts' }, name: 'auth' },\n" + gh1688Projects, at, ""},
		// Codex review: an escaped key parses to a testMatch the raw body never names; it stays
		// unresolved at the escaped key's own line (TJAA-V0-104) instead of slicing an absent site.
		{"escaped key regular expression", "    { name: 'auth', \"test\\u004datch\": /global-setup/ },\n" + gh1688Projects, at, ""},
		{"escaped key literal", "    { name: 'auth', \"test\\u004datch\": 'setup/global-setup.ts' },\n" + gh1688Projects, at, ""},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := gh1688Repository(t, row.projects)
			if row.config != "" {
				write(t, root, "playwright.config.ts", row.config)
			}
			write(t, root, "tests/setup/global-setup.ts", "import { test as setup } from '@playwright/test';\nsetup('authenticate', async () => {});\n")
			write(t, root, "tests/setup/teardown.ts", "import { test as teardown } from '@playwright/test';\nteardown('clean', async () => {});\n")
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			config := slices.DeleteFunc(slices.Clone(result.Frontier), func(entry string) bool { return !strings.HasPrefix(entry, FrontierConfig) })
			if row.want == "" && len(config) != 0 || row.want != "" && !slices.Equal(config, []string{row.want}) {
				t.Fatalf("executable-config sites %q, want %q", config, row.want)
			}
		})
	}
}

// TJAA-V0-091: a literal testMatch that assigns a file the general adapter does not observe as a
// Playwright test of the config, here a Vitest test, stays unresolved and names that file.
// TJAA-V0-091 (Codex review): minimatch reads `??` as two UTF-16 code units, which match one
// non-BMP character, while the Go matcher reads one rune, so a path outside the matcher model
// keeps the testMatch site instead of being classified as support code.
func TestPlaywrightPathModeLiteralTestMatchOutsideMatcherModel_GH1701(t *testing.T) {
	root := gh1688Repository(t, "    { name: 'auth', testMatch: 'cases/??.ts' },\n"+gh1688Projects)
	write(t, root, "tests/cases/\U0001F600.ts", "import { test } from '@playwright/test';\ntest('emoji', async () => {});\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := FrontierConfig + ":playwright.config.ts:6 `testMatch`"
	if !slices.Contains(result.Frontier, want) {
		t.Fatalf("frontier %q lacks %q", result.Frontier, want)
	}
}

func TestPlaywrightPathModeLiteralTestMatchOwnsSupport_GH1701(t *testing.T) {
	root := gh1688Repository(t, "    { name: 'auth', testMatch: 'setup/global-setup.ts' },\n"+gh1688Projects)
	write(t, root, "tests/setup/global-setup.ts", "import { it } from 'vitest';\nit('ready', () => {});\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := FrontierConfig + ":playwright.config.ts:6 `owns tests/setup/global-setup.ts`" // TJAA-V0-104
	if !slices.Contains(result.Frontier, want) {
		t.Fatalf("frontier %q lacks %q", result.Frontier, want)
	}
}
