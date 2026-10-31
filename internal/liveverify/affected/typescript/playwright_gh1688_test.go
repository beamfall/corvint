package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// gh1688Repository is the GitHub #1688 shape: a literal testDir inside defineConfig, project
// dependencies computed from the environment, literal grep/grepInvert projects, and specs that
// reach a shared fixture through a tsconfig alias or a relative import.
func gh1688Repository(t *testing.T, projects string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0","dotenv":"16"}}`)
	write(t, root, "tsconfig.json", `{ "compilerOptions": { "baseUrl": ".", "paths": { "lib/*": ["./lib/*"] } } }`)
	write(t, root, "playwright.config.ts", "import { defineConfig, devices } from '@playwright/test';\nconst deps = process.env.X === '1' ? ['setup'] : ['setup'];\nexport default defineConfig({\n  testDir: './tests',\n  projects: [\n"+projects+"  ],\n});\n")
	write(t, root, "lib/fixtures.ts", "import { test as base } from '@playwright/test';\nexport const test = base.extend({});\n")
	write(t, root, "tests/a.spec.ts", "import { test } from 'lib/fixtures';\ntest('a @view-only', async () => {});\n")
	write(t, root, "tests/b.spec.ts", "import { test } from '@playwright/test';\ntest('b', { annotation: { type: 'x', description: 'y' } }, async () => {});\n")
	write(t, root, "tests/c.spec.ts", "import { test } from '../lib/fixtures';\ntest('c', async () => {});\n")
	return root
}

const gh1688Projects = "    { name: 'view', grep: /@view-only/, use: { ...devices['Desktop Chrome'] }, dependencies: deps },\n    { name: 'rest', grepInvert: /@view-only/, use: { ...devices['Desktop Chrome'] }, dependencies: deps },\n    { name: 'setup', use: { ...devices['Desktop Chrome'] } },\n"

// AFP-V0-423 (GitHub #1688 item 1): a literal testDir is never the executable-config site. A config
// whose only computed value is a project's dependencies raises no executable-config frontier, and
// one that the static profile refuses for a setup project's testMatch names testMatch, not the
// earlier testDir.
func TestPlaywrightConfigFrontierNamesBlocker_GH1688(t *testing.T) {
	for _, row := range []struct {
		label, projects, want string
	}{
		{"computed dependencies only", gh1688Projects, ""},
		{"setup project testMatch", "    { name: 'setup', testMatch: /^.*\\.setup\\.ts/ },\n" + gh1688Projects, FrontierConfig + ":playwright.config.ts:6 `testMatch`"},
	} {
		t.Run(row.label, func(t *testing.T) {
			result, err := New().Units(context.Background(), gh1688Repository(t, row.projects))
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

// AFP-V0-424 (GitHub #1688 item 2): the dynamic-source-reachability selection unknown names what
// left the shared graph unresolved: an unowned dirty path by its path, and a language frontier
// reason by the sites that raised it.
func TestPlaywrightDynamicSourceNamesSites_GH1688(t *testing.T) {
	root := gh1688Repository(t, gh1688Projects)
	write(t, root, "lib/fixtures.ts", "import { test as base } from '@playwright/test';\nconst extra = await import(process.env.EXTRA ?? './none');\nexport const test = base.extend({});\n")
	write(t, root, "data/seed.json", "{}\n")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"lib/fixtures.ts", "data/seed.json"})
	if err != nil {
		t.Fatal(err)
	}
	detail := ""
	for _, unknown := range plan.Unknown {
		if unknown.Reason == PlaywrightUnknownDynamicSource {
			detail = unknown.Detail
		}
	}
	for _, want := range []string{"the shared TypeScript graph has an unresolved selection frontier: ", "UNOWNED_DIRTY_PATH data/seed.json", FrontierDynamicImport + " at lib/fixtures.ts:2 `import`"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("dynamic-source detail %q lacks %q (unknowns %+v)", detail, want, plan.Unknown)
		}
	}
}

// AFP-V0-425 (GitHub #1688 item 3): a details object with no tag key, such as one holding only an
// annotation from a helper, is tag-free, and so is a spec that fills a field or pushes onto its own
// array; when a file stays inexact, the stop names the construct and its line.
func TestPlaywrightTagClosureDetailsWithoutTag_GH1688(t *testing.T) {
	smoke := func() []grepListCase {
		cases := grepReproCases()
		cases[0].tags = []string{"smoke"}
		return cases
	}
	helper := map[string]string{"e2e/annotation.ts": "export const annotation = (id: string) => ({ type: 'issue', description: id });\n"}
	for _, row := range []struct {
		label, plain string
		extra        map[string]string
		cases        func() []grepListCase
		stop         string
	}{
		{"annotation-only details object", "import { annotation } from './annotation';\ntest('plain', { annotation: annotation('some-id') }, async ({ request }) => {\n  const res = await request.get('/x');\n});\n", helper, grepReproCases, ""},
		{"page fill and own array push", "test.describe('d', { tag: '@smoke' }, () => {\n  test('plain', { annotation: { type: 'a', description: 'b' } }, async ({ page }) => {\n    const ids: string[] = [];\n    ids.push('x');\n    await page.fill('#q', ids[0]);\n  });\n});\n", nil, smoke, ""},
		{"reflective name stops at its line", "test('plain', { annotation: { type: 'a', description: 'b' } }, async () => {\n  keep(JSON.parse('{}'));\n});\n", nil, grepReproCases, "e2e/plain.spec.ts holds a tag or construct that is not one literal @\\S+ string (line 2: `JSON` can build or enumerate a property at run time)"},
		{"computed member access stops at its line", "test('plain', async () => {\n  const ids = [1];\n  for (let i = 0; i < 1; i++) ids[i];\n});\n", nil, grepReproCases, "(line 3: a computed member access can reach a tag array without naming it)"},
	} {
		t.Run(row.label, func(t *testing.T) {
			root := grepFixture(t, grepRepro)
			write(t, root, "e2e/plain.spec.ts", row.plain)
			for file, content := range row.extra {
				write(t, root, file, content)
			}
			unverified, omissions := gh1647Omission(t, root, row.cases())
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

// AFP-V0-426 (GitHub #1688 item 4): in Playwright mode the e2e-runtime-dependency execution unknown
// counts the test files that depend on the Playwright runner in one summary instead of naming
// @playwright/test at each, and still names any other runtime dependency.
func TestPlaywrightRuntimeDependencySummary_GH1688(t *testing.T) {
	runtimeDetail := func(t *testing.T, root string) string {
		t.Helper()
		plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"lib/fixtures.ts"})
		if err != nil {
			t.Fatal(err)
		}
		for _, unknown := range plan.Unknown {
			if unknown.Reason == FrontierE2ERuntimeDependency && unknown.Axis == PlaywrightAxisExecution {
				return unknown.Detail
			}
		}
		t.Fatalf("no execution e2e-runtime-dependency unknown: %+v", plan.Unknown)
		return ""
	}
	root := gh1688Repository(t, gh1688Projects)
	if detail := runtimeDetail(t, root); detail != "shared TypeScript graph frontier: 3 test file(s) depend on the Playwright runner @playwright/test" {
		t.Fatalf("detail %q", detail)
	}
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0","cypress":"13","dotenv":"16"}}`)
	write(t, root, "cypress/e2e/d.cy.ts", "import 'cypress';\ndescribe('d', () => { it('d', () => {}); });\n")
	if detail := runtimeDetail(t, root); !strings.HasPrefix(detail, "shared TypeScript graph frontier at cypress/e2e/d.cy.ts") || !strings.HasSuffix(detail, "; 3 test file(s) depend on the Playwright runner @playwright/test") {
		t.Fatalf("detail %q", detail)
	}
}
