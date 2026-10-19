package testsource

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const globalTest = "test('works', () => {\n  expect(1).toBe(1);\n});\n"

// selectionFixture has a Jest and a Vitest runner at the root and a Playwright test with no
// Playwright runner.
func selectionFixture() map[string]string {
	return map[string]string{
		"package.json":   `{"devDependencies":{"jest":"29.7.0","vitest":"3.2.4"}}`,
		"jest.config.js": "module.exports = {\n  testPathIgnorePatterns: ['/node_modules/', '/legacy/'],\n};\n",
		"vitest.config.ts": "import { defineConfig } from 'vitest/config';\nexport default defineConfig({\n  test: {\n    include: ['src/**/*.test.ts'],\n" +
			"    coverage: { include: ['lib/**'] },\n  },\n});\n",
		"src/a.test.js":            globalTest,
		"src/b.tests.js":           globalTest,
		"src/w.test.ts":            "import { test, expect } from 'vitest';\n" + globalTest,
		"test/v.test.ts":           "import { test, expect } from 'vitest';\n" + globalTest,
		"legacy/old.test.js":       globalTest,
		"e2e/login.spec.ts":        "import { test, expect } from '@playwright/test';\ntest('login', async ({ page }) => {\n  await expect(page).toHaveURL('/');\n});\n",
		"test/helpers.js":          "export const value = 1;\n",
		"src/util.js":              globalTest,
		"node_modules/x/a.test.js": globalTest,
	}
}

func unselectedReport(t *testing.T, files map[string]string) UnselectedReport {
	t.Helper()
	root := t.TempDir()
	commitFiles(t, root, files)
	report, err := UnselectedFiles(OpenRepo(context.Background(), root), UnselectedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != UnselectedSchema || len(report.Revision) != 40 {
		t.Fatalf("report header = %q %q", report.Schema, report.Revision)
	}
	return report
}

func selectionOf(report UnselectedReport, name string) (SelectionFile, bool) {
	for _, f := range report.Files {
		if f.Path == name {
			return f, true
		}
	}
	return SelectionFile{}, false
}

func findingOf(report UnselectedReport, name string) (UnselectedFinding, bool) {
	for _, f := range report.Findings {
		if f.Path == name {
			return f, true
		}
	}
	return UnselectedFinding{}, false
}

// TestUnselectedReasonsTSAV0144 covers each reason, the excluding pattern, and that selected files
// are not reported (TSA-V0-140, TSA-V0-141, TSA-V0-143, TSA-V0-144, TSA-V0-145).
func TestUnselectedReasonsTSAV0144(t *testing.T) {
	report := unselectedReport(t, selectionFixture())
	want := map[string]UnselectedFinding{
		"e2e/login.spec.ts":  {Framework: RunnerPlaywright, Reason: ReasonNoRunnerForFramework},
		"legacy/old.test.js": {Framework: frameworkGlobals, Reason: ReasonExcluded, Runner: RunnerJest, Config: "jest.config.js", Pattern: "/legacy/"},
		"src/b.tests.js":     {Framework: frameworkGlobals, Reason: ReasonSuffixMismatch, Runner: RunnerJest, Config: "jest.config.js"},
		"test/v.test.ts":     {Framework: RunnerVitest, Reason: ReasonOutsideInclude, Runner: RunnerVitest, Config: "vitest.config.ts"},
	}
	if len(report.Findings) != len(want) {
		t.Fatalf("findings = %+v", report.Findings)
	}
	for name, w := range want {
		got, ok := findingOf(report, name)
		w.Code, w.Path = CodeTestFileUnselected, name
		if !ok || got != w {
			t.Errorf("%s finding = %+v, want %+v", name, got, w)
		}
	}
	for _, name := range []string{"src/a.test.js", "src/w.test.ts"} {
		file, ok := selectionOf(report, name)
		if !ok || file.Selection != SelectionSelected {
			t.Errorf("%s = %+v, want selected", name, file)
		}
		if _, reported := findingOf(report, name); reported {
			t.Errorf("selected %s was reported", name)
		}
	}
	for _, name := range []string{"test/helpers.js", "src/util.js", "node_modules/x/a.test.js"} {
		if file, ok := selectionOf(report, name); ok {
			t.Errorf("%s is not test-looking but was listed: %+v", name, file)
		}
	}
	if len(report.Runners) != 2 || report.Runners[0].Status != RunnerRead || report.Runners[1].Status != RunnerRead {
		t.Fatalf("runners = %+v", report.Runners)
	}
	if got := strings.Join(report.Runners[1].Include, ","); got != "src/**/*.test.ts" {
		t.Errorf("vitest include = %q; coverage.include must not count", got)
	}
}

// TestUnselectedPlaywrightAndMochaTSAV0141 reads Playwright's testDir/testIgnore and a Mocha
// .mocharc.json spec (TSA-V0-141, TSA-V0-142, TSA-V0-144).
func TestUnselectedPlaywrightAndMochaTSAV0141(t *testing.T) {
	pw := "import { test } from '@playwright/test';\ntest('x', async () => {});\n"
	report := unselectedReport(t, map[string]string{
		"package.json":         `{"devDependencies":{"@playwright/test":"1.55.0","mocha":"11.0.0"}}`,
		"playwright.config.ts": "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  testDir: './e2e',\n  testIgnore: ['**/wip/**'],\n});\n",
		".mocharc.json":        `{"spec": "test/**/*.spec.js"}`,
		"e2e/a.spec.ts":        pw,
		"e2e/wip/b.spec.ts":    pw,
		"tests/c.spec.ts":      pw,
		"test/unit/d.spec.js":  "describe('d', () => { it('works', () => {}); });\n",
		"test/unit/e.spec.ts":  "describe('e', () => { it('works', () => {}); });\n",
	})
	for name, want := range map[string]string{"e2e/a.spec.ts": SelectionSelected, "test/unit/d.spec.js": SelectionSelected,
		"e2e/wip/b.spec.ts": SelectionUnselected, "tests/c.spec.ts": SelectionUnselected, "test/unit/e.spec.ts": SelectionUnselected} {
		if file, _ := selectionOf(report, name); file.Selection != want {
			t.Errorf("%s = %+v, want %s", name, file, want)
		}
	}
	if f, _ := findingOf(report, "e2e/wip/b.spec.ts"); f.Reason != ReasonExcluded || f.Pattern != "**/wip/**" {
		t.Errorf("wip finding = %+v", f)
	}
	if f, _ := findingOf(report, "tests/c.spec.ts"); f.Reason != ReasonOutsideInclude {
		t.Errorf("outside testDir finding = %+v", f)
	}
	if f, _ := findingOf(report, "test/unit/e.spec.ts"); f.Reason != ReasonSuffixMismatch || f.Runner != RunnerMocha {
		t.Errorf("mocha .ts finding = %+v", f)
	}
}

// TestUnselectedAbstainsTSAV0142 shows a configuration the reader cannot read making the runner
// uncertain, so its files abstain rather than being reported (TSA-V0-142, TSA-V0-144).
func TestUnselectedAbstainsTSAV0142(t *testing.T) {
	cases := map[string]struct {
		files  map[string]string
		reason string
	}{
		"spread": {map[string]string{"package.json": `{"devDependencies":{"jest":"29.7.0"}}`,
			"jest.config.js": "const base = require('./base');\nmodule.exports = { ...base, testMatch: ['**/*.check.js'] };\n"}, "runner-config-spread"},
		"unparsable json": {map[string]string{"package.json": `{"devDependencies":{"jest":"29.7.0"}}`,
			"jest.config.json": `{"testMatch": [`}, "runner-config-unparsed"},
		"dynamic":         {map[string]string{"jest.config.js": "module.exports = { testMatch: patterns() };\n"}, "runner-config-value-dynamic"},
		"unsupported key": {map[string]string{"jest.config.js": "module.exports = { testRegex: 'x' };\n"}, "runner-config-key-unsupported"},
		"projects":        {map[string]string{"playwright.config.ts": "export default { projects: [{ name: 'a', testMatch: '**/*.x.ts' }] };\n"}, "runner-config-project-selection"},
		"negated glob":    {map[string]string{"jest.config.js": "module.exports = { testMatch: ['**/!(skip).js'] };\n"}, "runner-pattern-unsupported"},
		"two configs":     {map[string]string{"jest.config.js": "module.exports = {};\n", "jest.config.ts": "export default {};\n"}, "runner-config-multiple"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c.files["src/a.test.js"] = "import { test } from '@playwright/test';\nimport { expect } from '@jest/globals';\n" + globalTest
			c.files["src/b.test.js"] = globalTest
			if strings.Contains(name, "projects") {
				c.files["src/b.test.js"] = "import { test, expect } from '@playwright/test';\n" + globalTest
			}
			report := unselectedReport(t, c.files)
			file, _ := selectionOf(report, "src/b.test.js")
			if file.Selection != SelectionUncertain || file.Reason != c.reason {
				t.Fatalf("b = %+v, want uncertain %s; runners %+v", file, c.reason, report.Runners)
			}
			if a, _ := selectionOf(report, "src/a.test.js"); a.Selection != SelectionUncertain || a.Reason != "framework-ambiguous" {
				t.Errorf("two-framework file = %+v", a)
			}
			if len(report.Findings) != 0 {
				t.Errorf("an abstaining reading reported %+v", report.Findings)
			}
		})
	}
}

// TestUnselectedNoEvidenceTSAV0144 abstains with no runner evidence at all, with an unmodelled
// runner, and for a node:test file (TSA-V0-141, TSA-V0-143, TSA-V0-144).
func TestUnselectedNoEvidenceTSAV0144(t *testing.T) {
	report := unselectedReport(t, map[string]string{"src/a.test.js": globalTest})
	if file, _ := selectionOf(report, "src/a.test.js"); file.Selection != SelectionUncertain || file.Reason != "no-runner-configuration" {
		t.Errorf("no runner evidence = %+v", file)
	}
	report = unselectedReport(t, map[string]string{
		"package.json":        `{"devDependencies":{"cypress":"14.0.0","jest":"29.7.0"}}`,
		"cypress/e2e/a.cy.js": "describe('a', () => { it('works', () => {}); });\n",
		"src/n.test.mjs":      "import { test } from 'node:test';\ntest('n', () => {});\n",
	})
	if file, _ := selectionOf(report, "cypress/e2e/a.cy.js"); file.Selection != SelectionUncertain || file.Reason != "runner-unmodelled" {
		t.Errorf("cypress file = %+v", file)
	}
	if file, _ := selectionOf(report, "src/n.test.mjs"); file.Selection != SelectionUncertain || file.Reason != "framework-unmodelled" {
		t.Errorf("node:test file = %+v", file)
	}
	if len(report.Findings) != 0 {
		t.Errorf("findings = %+v", report.Findings)
	}
}

// TestUnselectedIndirectFrameworkTSAV0143 attributes a fixture-imported test to any runner.
func TestUnselectedIndirectFrameworkTSAV0143(t *testing.T) {
	s := parseSource("a.spec.ts", "import { test, expect } from './fixtures';\ntest('a', async () => {});\n")
	if got := s.framework(); got != frameworkIndirect {
		t.Errorf("fixture import = %q", got)
	}
	s = parseSource("a.spec.js", "const { test } = require('./fixtures');\ntest('a', () => {});\n")
	if got := s.framework(); got != frameworkIndirect {
		t.Errorf("fixture require = %q", got)
	}
	s = parseSource("a.spec.js", "import { render } from './render';\ntest('a', () => {});\n")
	if got := s.framework(); got != frameworkGlobals {
		t.Errorf("unrelated import = %q", got)
	}
}

// TestUnselectedGlobDialectTSAV0142 pins the glob dialect.
func TestUnselectedGlobDialectTSAV0142(t *testing.T) {
	cases := []struct {
		glob, path string
		match      bool
	}{
		{"**/?(*.)+(spec|test).?([mc])[jt]s?(x)", "/r/src/a.test.ts", true},
		{"**/?(*.)+(spec|test).?([mc])[jt]s?(x)", "/r/src/a.spec.mjs", true},
		{"**/?(*.)+(spec|test).?([mc])[jt]s?(x)", "/r/src/a.tests.js", false},
		{"**/__tests__/**/*.?([mc])[jt]s?(x)", "/r/src/__tests__/deep/a.jsx", true},
		{"**/*.{test,spec}.?(c|m)[jt]s?(x)", "a.spec.cts", true},
		{"**/*.{test,spec}.?(c|m)[jt]s?(x)", "dir/a.check.ts", false},
		{"src/*.test.js", "src/deep/a.test.js", false},
		{"test/[!x]*.js", "test/a.js", true},
		{"test/[!x]*.js", "test/x.js", false},
	}
	for _, c := range cases {
		re, ok := globRegexp(c.glob)
		if !ok || re.MatchString(c.path) != c.match {
			t.Errorf("%q vs %q: ok=%v match=%v, want %v", c.glob, c.path, ok, ok && re.MatchString(c.path), c.match)
		}
	}
	for _, unsupported := range []string{"", "!a.js", "**/!(x).js", "a\\b.js", "{a,b", "a)b"} {
		if _, ok := globRegexp(unsupported); ok {
			t.Errorf("%q compiled, want unsupported", unsupported)
		}
	}
}

// TestSelectedSourceValidityRefusesUnselectedTSAV0146 refuses witness credit to the tests of an
// unselected file and leaves a selected file's tests alone (TSA-V0-146).
func TestSelectedSourceValidityRefusesUnselectedTSAV0146(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, selectionFixture())
	repo := OpenRepo(context.Background(), root)
	report, err := SelectedSourceValidity(repo, SourceValidityOptions{Files: []string{"legacy/old.test.js", "src/a.test.js", "src/util.js"}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Refused() {
		t.Fatal("an unselected file's test was not refused")
	}
	for _, test := range report.Tests {
		refused := test.Hygiene.State == testvalidity.HygieneIneligible && test.Hygiene.Reason == CodeTestFileUnselected
		if strings.HasPrefix(test.Test, "legacy/old.test.js:") != refused {
			t.Errorf("%s hygiene = %+v", test.Test, test.Hygiene)
		}
	}
	found := false
	for _, file := range report.Files {
		for _, f := range file.Findings {
			if f.Rule == CodeTestFileUnselected {
				found = true
				if file.Path != "legacy/old.test.js" || f.Severity != testvalidity.SeverityInvalid || f.Cause != "excluded: /legacy/" {
					t.Errorf("finding %+v on %s", f, file.Path)
				}
			}
		}
	}
	if !found {
		t.Fatal("no TEST-FILE-UNSELECTED finding")
	}
	plain, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"src/a.test.js"}})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := SelectedSourceValidity(repo, SourceValidityOptions{Files: []string{"src/a.test.js"}})
	if err != nil || len(plain.Tests) != len(selected.Tests) || plain.Refused() != selected.Refused() {
		t.Errorf("selected file changed: %+v vs %+v (%v)", plain, selected, err)
	}
}

// TestUnselectedRevisionRefusalTSAV0145 refuses an unresolvable revision instead of reporting.
func TestUnselectedRevisionRefusalTSAV0145(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"src/a.test.js": globalTest})
	_, err := UnselectedFiles(OpenRepo(context.Background(), root), UnselectedOptions{Revision: "no-such-rev"})
	var refusal *Error
	if !errors.As(err, &refusal) {
		t.Fatalf("unknown revision err = %v", err)
	}
}
