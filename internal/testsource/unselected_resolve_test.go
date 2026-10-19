package testsource

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const scenarioSpecBody = "const test = scenarioTest;\ntest.describe('thing', () => {\n  test('works', async ({ page }) => {\n" +
	"    await page.goto('/');\n  });\n});\n"

// extendFixture is a Playwright-only repository whose specs bind test functions derived with
// `base.extend(...)` and re-exported across modules (GitHub #1213).
func extendFixture() map[string]string {
	return map[string]string{
		"playwright.config.ts": "import { defineConfig } from '@playwright/test';\nexport default defineConfig({ testDir: './tests' });\n",
		"fixtures/index.ts": "import { test as base, expect } from '@playwright/test';\n" +
			"export const test = base.extend<{ user: string }>({ user: async ({}, use) => { await use('u'); } });\nexport { expect };\n",
		"fixtures/scenario.fixture.ts": "import { test as base } from './index';\n" +
			"export const scenarioTest = base.extend<{ data: string }>({ data: async ({}, use) => { await use('d'); } });\n",
		"fixtures/barrel.ts": "export * from './scenario.fixture';\nexport * from './index';\n",
		"fixtures/unit.ts":   "import { test as base } from 'vitest';\nexport const unitTest = base.extend({});\n",
		"fixtures/make.ts":   "export function makeTest() { return null; }\n",
		// The issue's example, through a relative path and through a path alias.
		"tests/area/thing.spec.ts": "import { scenarioTest } from '../../fixtures/scenario.fixture';\n" + scenarioSpecBody,
		"tests/area/alias.spec.ts": "import { scenarioTest } from 'fixtures/scenario.fixture';\n" + scenarioSpecBody,
		"tests/area/barrel.spec.ts": "import { scenarioTest as test, expect } from '../../fixtures/barrel';\n" +
			"test('b', async ({ page }) => { await expect(page).toHaveURL('/'); });\n",
		"tests/area/unit.spec.ts":    "import { unitTest as test } from '../../fixtures/unit';\ntest('u', () => {});\n",
		"tests/area/globals.spec.ts": globalTest,
		"tests/area/made.spec.ts":    "import { makeTest } from '../../fixtures/make';\nconst test = makeTest();\ntest('m', () => {});\n",
	}
}

// TestUnselectedExtendResolutionTSAV0147 resolves re-exported and extend-derived test bindings
// transitively, still reports a spec no configured runner can reach, and leaves an unresolvable
// local binding uncertain with a named reason (TSA-V0-147).
func TestUnselectedExtendResolutionTSAV0147(t *testing.T) {
	report := unselectedReport(t, extendFixture())
	cases := []struct {
		path, framework, selection, reason string
	}{
		{"tests/area/thing.spec.ts", RunnerPlaywright, SelectionSelected, ""},
		{"tests/area/alias.spec.ts", frameworkIndirect, SelectionSelected, ""},
		{"tests/area/barrel.spec.ts", RunnerPlaywright, SelectionSelected, ""},
		{"tests/area/unit.spec.ts", RunnerVitest, SelectionUnselected, ReasonNoRunnerForFramework},
		{"tests/area/globals.spec.ts", frameworkGlobals, SelectionUnselected, ReasonNoRunnerForFramework},
		{"tests/area/made.spec.ts", frameworkUnresolved, SelectionUncertain, ReasonFrameworkUnresolved},
	}
	for _, c := range cases {
		file, ok := selectionOf(report, c.path)
		if !ok || file.Framework != c.framework || file.Selection != c.selection || file.Reason != c.reason {
			t.Errorf("%s = %+v (listed %v), want %s %s %q", c.path, file, ok, c.framework, c.selection, c.reason)
		}
		if _, reported := findingOf(report, c.path); reported != (c.selection == SelectionUnselected) {
			t.Errorf("%s reported = %v", c.path, reported)
		}
	}
}

// TestFrameworkResolutionBoundsTSAV0147 pins alias forms, the depth bound and conflicting
// bindings without Git.
func TestFrameworkResolutionBoundsTSAV0147(t *testing.T) {
	modules := map[string]string{
		"f/pw.ts":     "import { test as base } from '@playwright/test';\nexport default base.extend({});\n",
		"f/vi.ts":     "export { test as viTest } from 'vitest';\n",
		"f/req.js":    "const { test: base } = require('@playwright/test');\nconst derived = base.extend({});\nexport { derived as reqTest };\n",
		"f/cycle.ts":  "export { loop } from './cycle';\n",
		"f/l0.ts":     "export { test } from '@playwright/test';\n",
		"f/ns.ts":     "import * as pw from '@playwright/test';\nexport const nsTest = pw.test;\n",
		"f/global.ts": "export const g = 1;\n",
	}
	for i := 1; i <= maxResolveDepth+1; i++ {
		modules[fmt.Sprintf("f/l%d.ts", i)] = fmt.Sprintf("export { test } from './l%d';\n", i-1)
	}
	resolver := &moduleResolver{sources: map[string]*source{}, tables: map[string]moduleTable{}, read: func(name string) (string, bool) {
		text, ok := modules[name]
		return text, ok
	}}
	cases := []struct{ text, want string }{
		{"import pwTest from './f/pw';\nconst test = pwTest;\ntest('a', () => {});\n", RunnerPlaywright},
		{"const { reqTest: test } = require('./f/req');\ntest('a', () => {});\n", RunnerPlaywright},
		{"import { viTest } from './f/vi';\nimport pwTest from './f/pw';\nconst test = viTest;\nconst it = pwTest;\ntest('a', () => {});\nit('b', () => {});\n", frameworkAmbiguous},
		{"import { expect } from '@playwright/test';\nimport { viTest as test } from './f/vi';\ntest('a', () => {});\n", frameworkAmbiguous},
		{"import { loop as test } from './f/cycle';\ntest('a', () => {});\n", frameworkIndirect},
		{"import { test } from './f/l1';\ntest('a', () => {});\n", RunnerPlaywright},
		{fmt.Sprintf("import { test } from './f/l%d';\ntest('a', () => {});\n", maxResolveDepth+1), frameworkIndirect},
		{"import { nsTest as test } from './f/ns';\ntest('a', () => {});\n", frameworkIndirect},
		{"import { missing as test } from './f/absent';\ntest('a', () => {});\n", frameworkIndirect},
		{"function helper() { const test = makeIt(); }\ntest('a', () => {});\n", frameworkGlobals},
		{"const test = build.extend;\ntest('a', () => {});\n", frameworkUnresolved},
	}
	for _, c := range cases {
		if got := parseSource("a.spec.ts", c.text).framework(resolver); got != c.want {
			t.Errorf("%q = %q, want %q", c.text, got, c.want)
		}
	}
	if resolver.reads > maxResolveReads {
		t.Errorf("reads = %d", resolver.reads)
	}
}

// TestSelectedSourceValidityExtendTSAV0147 gives the issue's example no TEST-FILE-UNSELECTED
// finding and still refuses a spec no configured runner can reach (TSA-V0-146, TSA-V0-147).
func TestSelectedSourceValidityExtendTSAV0147(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, extendFixture())
	repo := OpenRepo(context.Background(), root)
	for _, c := range []struct {
		path    string
		refused bool
	}{{"tests/area/thing.spec.ts", false}, {"tests/area/alias.spec.ts", false}, {"tests/area/made.spec.ts", false}, {"tests/area/unit.spec.ts", true}} {
		report, err := SelectedSourceValidity(repo, SourceValidityOptions{Files: []string{c.path}})
		if err != nil {
			t.Fatal(err)
		}
		refused := false
		for _, test := range report.Tests {
			if test.Hygiene.State == testvalidity.HygieneIneligible && test.Hygiene.Reason == CodeTestFileUnselected {
				refused = true
			}
		}
		if refused != c.refused || len(report.Tests) == 0 {
			t.Errorf("%s refused = %v (%d tests), want %v", c.path, refused, len(report.Tests), c.refused)
		}
	}
}
