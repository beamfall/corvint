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
// uncertain, so its files abstain rather than being reported (TSA-V0-142, TSA-V0-144); an export
// that is not a literal object abstains as runner-config-export-dynamic (TSA-V0-645).
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
		"projects":        {map[string]string{"playwright.config.ts": "const base = {};\nexport default { projects: [base] };\n"}, "runner-config-project-selection"},
		"negated glob":    {map[string]string{"jest.config.js": "module.exports = { testMatch: ['**/!(skip).js'] };\n"}, "runner-pattern-unsupported"},
		"two configs":     {map[string]string{"jest.config.js": "module.exports = {};\n", "jest.config.ts": "export default {};\n"}, "runner-config-multiple"},
		"shorthand key": {map[string]string{"jest.config.js": "const testMatch = ['**/*.tests.js'];\nmodule.exports = { testMatch };\n"},
			"runner-config-key-dynamic"},
		"computed key": {map[string]string{"jest.config.js": "const k = 'testMatch';\nmodule.exports = { [k]: ['**/*.tests.js'] };\n"},
			"runner-config-key-dynamic"},
		"member assignment": {map[string]string{"jest.config.js": "module.exports = {};\nmodule.exports.testMatch = ['**/*.tests.js'];\n"},
			"runner-config-key-dynamic"},
		"required export": {map[string]string{"jest.config.js": "module.exports = require('./base');\n"},
			"runner-config-export-dynamic"},
		"function export": {map[string]string{"jest.config.js": "module.exports = makeConfig({ testMatch: ['**/*.tests.js'] });\n"},
			"runner-config-export-dynamic"},
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

// TestUnselectedRunnerSemanticsTSAV0141 pins runner-specific matching: Playwright matches the
// absolute path case-insensitively, Mocha skips hidden files and selects an explicitly named file
// whatever its extension, brace ranges expand, and a Vitest workspace makes explicit Vitest files
// abstain (TSA-V0-141, TSA-V0-142, TSA-V0-144); a named const export is read and a Mocha dot
// segment matches only a pattern segment that names its dot (TSA-V0-645).
func TestUnselectedRunnerSemanticsTSAV0141(t *testing.T) {
	pw := "import { test } from '@playwright/test';\ntest('x', async () => {});\n"
	mocha := "describe('d', () => { it('works', () => {}); });\n"
	cases := map[string]struct {
		files map[string]string
		want  map[string]string
	}{
		"playwright testMatch keeps testDir in the path": {map[string]string{
			"package.json":         `{"devDependencies":{"@playwright/test":"1.55.0"}}`,
			"playwright.config.ts": "export default { testDir: './e2e', testMatch: 'e2e/**/*.spec.ts' };\n",
			"e2e/a.spec.ts":        pw,
		}, map[string]string{"e2e/a.spec.ts": SelectionSelected}},
		"playwright testIgnore sees testDir": {map[string]string{
			"package.json":         `{"devDependencies":{"@playwright/test":"1.55.0"}}`,
			"playwright.config.ts": "export default { testDir: './e2e', testIgnore: '**/e2e/**' };\n",
			"e2e/a.spec.ts":        pw,
		}, map[string]string{"e2e/a.spec.ts": SelectionUnselected}},
		"playwright is case-insensitive": {map[string]string{
			"package.json":         `{"devDependencies":{"@playwright/test":"1.55.0"}}`,
			"playwright.config.ts": "export default { testMatch: '**/A.spec.ts' };\n",
			"e2e/a.spec.ts":        pw,
		}, map[string]string{"e2e/a.spec.ts": SelectionSelected}},
		"brace range expands": {map[string]string{
			"package.json":         `{"devDependencies":{"@playwright/test":"1.55.0"}}`,
			"playwright.config.ts": "export default { testMatch: '**/test{1..3}.spec.ts' };\n",
			"e2e/test1.spec.ts":    pw,
			"e2e/test4.spec.ts":    pw,
		}, map[string]string{"e2e/test1.spec.ts": SelectionSelected, "e2e/test4.spec.ts": SelectionUnselected}},
		"mocha skips hidden files": {map[string]string{
			"package.json":         `{"devDependencies":{"mocha":"11.0.0"}}`,
			"test/.hidden.spec.js": mocha,
			"test/shown.spec.js":   mocha,
		}, map[string]string{"test/.hidden.spec.js": SelectionUnselected, "test/shown.spec.js": SelectionSelected}},
		"named const export is read": {map[string]string{
			"package.json":   `{"devDependencies":{"jest":"29.7.0"}}`,
			"jest.config.ts": "import type { Config } from 'jest';\nconst config: Config = { testMatch: ['**/a.test.ts'] };\nexport default config;\n",
			"src/a.test.ts":  globalTest,
			"src/b.test.ts":  globalTest,
		}, map[string]string{"src/a.test.ts": SelectionSelected, "src/b.test.ts": SelectionUnselected}},
		"mocha hidden segment needs its own dot pattern": {map[string]string{
			"package.json":                   `{"devDependencies":{"mocha":"11.0.0"}}`,
			".mocharc.json":                  `{"spec":"test/.fixtures/**/*.spec.js"}`,
			"test/.fixtures/x.spec.js":       mocha,
			"test/.fixtures/.deep/y.spec.js": mocha,
		}, map[string]string{"test/.fixtures/x.spec.js": SelectionSelected, "test/.fixtures/.deep/y.spec.js": SelectionUnselected}},
		"mocha explicit file outside extension": {map[string]string{
			"package.json":    `{"devDependencies":{"mocha":"11.0.0"}}`,
			".mocharc.json":   `{"spec":"test/a.test.cjs","extension":["js"]}`,
			"test/a.test.cjs": mocha,
			"test/b.test.cjs": mocha,
		}, map[string]string{"test/a.test.cjs": SelectionSelected, "test/b.test.cjs": SelectionUnselected}},
		"inline globstar stays in one segment": {map[string]string{
			"package.json":        `{"devDependencies":{"mocha":"11.0.0"}}`,
			".mocharc.json":       `{"spec":"test/**.spec.js"}`,
			"test/a.spec.js":      mocha,
			"test/deep/b.spec.js": mocha,
		}, map[string]string{"test/a.spec.js": SelectionSelected, "test/deep/b.spec.js": SelectionUnselected}},
		"vitest workspace abstains for explicit vitest": {map[string]string{
			"package.json":        `{"devDependencies":{"vitest":"3.2.4"}}`,
			"vitest.workspace.ts": "export default [{ test: { name: 'unit', include: ['**/*.tests.ts'] } }];\n",
			"src/a.tests.ts":      "import { test } from 'vitest';\n" + globalTest,
		}, map[string]string{"src/a.tests.ts": SelectionUncertain}},
		"vitest workspace without a vitest dependency": {map[string]string{
			"vitest.workspace.ts": "export default ['packages/*'];\n",
			"src/a.tests.ts":      "import { test } from 'vitest';\n" + globalTest,
		}, map[string]string{"src/a.tests.ts": SelectionUncertain}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			report := unselectedReport(t, c.files)
			for file, want := range c.want {
				if got, _ := selectionOf(report, file); got.Selection != want {
					t.Errorf("%s = %+v, want %s; runners %+v", file, got, want, report.Runners)
				}
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
	if got := s.framework(nil); got != frameworkIndirect {
		t.Errorf("fixture import = %q", got)
	}
	s = parseSource("a.spec.js", "const { test } = require('./fixtures');\ntest('a', () => {});\n")
	if got := s.framework(nil); got != frameworkIndirect {
		t.Errorf("fixture require = %q", got)
	}
	s = parseSource("a.spec.js", "import { render } from './render';\ntest('a', () => {});\n")
	if got := s.framework(nil); got != frameworkGlobals {
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
		{"test/**.spec.js", "test/a.spec.js", true},
		{"test/**.spec.js", "test/deep/a.spec.js", false},
		{"a/**", "a/b/c.js", true},
		{"**/test{1..3}.spec.ts", "e2e/test2.spec.ts", true},
		{"**/test{1..3}.spec.ts", "e2e/test4.spec.ts", false},
		{"{src,lib}/**/*.{js,ts}", "lib/x/y.ts", true},
		{"{src,lib}/**/*.{js,ts}", "app/y.ts", false},
	}
	for _, c := range cases {
		re, ok := globRegexp(c.glob)
		if !ok || re.MatchString(c.path) != c.match {
			t.Errorf("%q vs %q: ok=%v match=%v, want %v", c.glob, c.path, ok, ok && re.MatchString(c.path), c.match)
		}
	}
	for _, unsupported := range []string{"", "!a.js", "**/!(x).js", "a\\b.js", "{a,b", "a)b", "{a..c}.js", "{01..03}.js", "{a}.js", "@(**/a).js"} {
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
