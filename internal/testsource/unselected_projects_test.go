package testsource

import "testing"

const pwSpec = "import { test, expect } from '@playwright/test';\n" + globalTest

// pwSelections reads a Playwright configuration over the given test paths and returns each
// path's selection, reason and configuration line.
func pwSelections(t *testing.T, config string, paths ...string) (UnselectedReport, map[string]SelectionFile) {
	t.Helper()
	files := map[string]string{"playwright.config.ts": config}
	for _, name := range paths {
		files[name] = pwSpec
	}
	report := unselectedReport(t, files)
	got := map[string]SelectionFile{}
	for _, name := range paths {
		file, ok := selectionOf(report, name)
		if !ok {
			t.Fatalf("%s not classified; runners %+v", name, report.Runners)
		}
		got[name] = file
	}
	return report, got
}

// TestUnselectedPlaywrightProjectsTSAV0653 proves each Playwright project is read as its own
// selection over the top level it inherits: a path is selected when any project selects it, a
// project testDir and testIgnore apply to that project only, and a dynamic project entry, an
// empty projects array or a selection key nested below an entry still abstains with its line.
func TestUnselectedPlaywrightProjectsTSAV0653(t *testing.T) {
	config := "import { defineConfig, devices } from '@playwright/test';\nexport default defineConfig({\n" +
		"  testDir: './e2e',\n  testIgnore: '**/legacy/**',\n  projects: [\n" +
		"    { name: 'setup', testMatch: '**/*.setup.ts' },\n" +
		"    { name: 'chromium', testMatch: '**/*.pw.ts', use: { ...devices['Desktop Chrome'] } },\n" +
		"    { name: 'smoke', testDir: './smoke', testIgnore: '**/slow/**' },\n  ],\n});\n"
	report, got := pwSelections(t, config,
		"e2e/auth.setup.ts", "e2e/cart.pw.ts", "e2e/cart.spec.ts", "e2e/legacy/old.pw.ts",
		"smoke/a.spec.ts", "smoke/slow/b.spec.ts", "other/c.spec.ts")
	want := map[string]string{
		"e2e/auth.setup.ts":    SelectionSelected,
		"e2e/cart.pw.ts":       SelectionSelected,
		"e2e/cart.spec.ts":     SelectionUnselected,
		"e2e/legacy/old.pw.ts": SelectionUnselected,
		"smoke/a.spec.ts":      SelectionSelected,
		"smoke/slow/b.spec.ts": SelectionUnselected,
		"other/c.spec.ts":      SelectionUnselected,
	}
	for name, selection := range want {
		if got[name].Selection != selection {
			t.Errorf("%s = %+v, want %s; runners %+v", name, got[name], selection, report.Runners)
		}
	}
	if file := got["smoke/slow/b.spec.ts"]; file.Reason != ReasonExcluded || file.Pattern != "**/slow/**" {
		t.Errorf("project testIgnore = %+v", file)
	}
	if len(report.Runners) != 1 || len(report.Runners[0].Projects) != 3 || report.Runners[0].Projects[2].Directory != "smoke" {
		t.Errorf("runner projects = %+v", report.Runners)
	}

	abstain := map[string]struct {
		config, reason string
		line           int
	}{
		"variable entry": {"const base = {};\nexport default {\n  projects: [\n    { name: 'a' },\n    base,\n  ],\n};\n",
			"runner-config-project-selection", 5},
		"conditional entry": {"export default {\n  projects: [\n    process.env.CI ? { name: 'a' } : { name: 'b' },\n  ],\n};\n",
			"runner-config-project-selection", 3},
		"empty projects": {"export default {\n  projects: [],\n};\n", "runner-config-project-selection", 2},
		"nested key": {"export default {\n  projects: [\n    { name: 'a', use: {\n      testMatch: '**/*.x.ts' } },\n  ],\n};\n",
			"runner-config-project-selection", 4},
		"entry spread": {"const base = {};\nexport default {\n  projects: [\n    { ...base, name: 'a' },\n  ],\n};\n",
			"runner-config-spread", 4},
		"dynamic project value": {"export default {\n  projects: [\n    { name: 'a',\n      testMatch: pick() },\n  ],\n};\n",
			"runner-config-value-dynamic", 4},
	}
	for name, c := range abstain {
		report, got := pwSelections(t, c.config, "e2e/a.spec.ts")
		if len(report.Runners) != 1 || report.Runners[0].Reason != c.reason || report.Runners[0].Line != c.line {
			t.Errorf("%s: runners = %+v, want %s at line %d", name, report.Runners, c.reason, c.line)
		}
		if file := got["e2e/a.spec.ts"]; file.Selection != SelectionUncertain || file.ConfigLine != c.line {
			t.Errorf("%s: file = %+v", name, file)
		}
	}
}

// TestUnselectedPlaywrightRegexTSAV0654 proves a static Playwright regex testMatch or testIgnore
// is decided on the repository path only when no checkout directory can change its verdict
// (TJAA-V0-127); otherwise the path abstains as runner-pattern-root-dependent naming the pattern
// and its line, and a regex in another runner's configuration stays value-dynamic.
func TestUnselectedPlaywrightRegexTSAV0654(t *testing.T) {
	config := "export default {\n  testDir: 'e2e',\n  testMatch: /\\.e2e\\.ts$/,\n  testIgnore: [/slow\\.e2e\\.ts$/],\n};\n"
	_, got := pwSelections(t, config, "e2e/a.e2e.ts", "e2e/b.spec.ts", "e2e/c.slow.e2e.ts")
	if got["e2e/a.e2e.ts"].Selection != SelectionSelected {
		t.Errorf("end-anchored regex = %+v", got["e2e/a.e2e.ts"])
	}
	if file := got["e2e/b.spec.ts"]; file.Selection != SelectionUnselected {
		t.Errorf("unmatched end-anchored regex = %+v", file)
	}
	if file := got["e2e/c.slow.e2e.ts"]; file.Selection != SelectionUnselected || file.Reason != ReasonExcluded || file.Pattern != `/slow\.e2e\.ts$/` {
		t.Errorf("end-anchored testIgnore regex = %+v", file)
	}

	// An unanchored testIgnore could match the checkout directory and drop every test.
	_, got = pwSelections(t, "export default {\n  testDir: 'e2e',\n  testIgnore: /skip/,\n};\n", "e2e/a.spec.ts")
	if file := got["e2e/a.spec.ts"]; file.Selection != SelectionUncertain || file.Reason != ReasonPatternRootDependent || file.ConfigLine != 3 {
		t.Errorf("unanchored testIgnore = %+v", file)
	}

	// An unanchored regex that misses `/<path>` could still match the checkout directory.
	config = "export default {\n  testDir: 'e2e',\n  testMatch:\n    /e2e\\/.*\\.check\\.ts/,\n};\n"
	_, got = pwSelections(t, config, "e2e/a.check.ts", "e2e/b.spec.ts")
	if got["e2e/a.check.ts"].Selection != SelectionSelected {
		t.Errorf("matching unanchored regex = %+v", got["e2e/a.check.ts"])
	}
	if file := got["e2e/b.spec.ts"]; file.Selection != SelectionUncertain || file.Reason != ReasonPatternRootDependent ||
		file.Pattern != `/e2e\/.*\.check\.ts/` || file.ConfigLine != 3 {
		t.Errorf("root-dependent regex = %+v", file)
	}

	// A start-anchored testIgnore abstains only for a path an include selects.
	config = "export default {\n  testMatch: '**/*.spec.ts',\n  testIgnore: /^.*legacy/,\n};\n"
	_, got = pwSelections(t, config, "e2e/a.spec.ts", "e2e/b.pw.ts")
	if file := got["e2e/a.spec.ts"]; file.Selection != SelectionUncertain || file.Reason != ReasonPatternRootDependent || file.ConfigLine != 3 {
		t.Errorf("contextual testIgnore = %+v", file)
	}
	if file := got["e2e/b.pw.ts"]; file.Selection != SelectionUnselected {
		t.Errorf("unselected beside a contextual testIgnore = %+v", file)
	}

	// A flag outside i, m and s is not a static pattern.
	_, got = pwSelections(t, "export default {\n  testMatch: /\\.spec\\.ts$/y,\n};\n", "e2e/a.spec.ts")
	if file := got["e2e/a.spec.ts"]; file.Selection != SelectionUncertain || file.Reason != "runner-pattern-unsupported" {
		t.Errorf("sticky regex = %+v", file)
	}

	report := unselectedReport(t, map[string]string{
		"package.json":   `{"devDependencies":{"jest":"29.7.0"}}`,
		"jest.config.js": "module.exports = {\n  testMatch: [/\\.check\\.js$/],\n};\n",
		"src/a.test.js":  globalTest,
	})
	if file, _ := selectionOf(report, "src/a.test.js"); file.Selection != SelectionUncertain || file.Reason != "runner-config-value-dynamic" || file.ConfigLine != 2 {
		t.Errorf("jest regex = %+v", file)
	}
}

// TestUnselectedConfigLinesTSAV0655 proves runner-config-export-dynamic names the first export
// that is not literal and runner-config-key-dynamic names the first token it cannot place: a
// variable or string spelling a selection key, or the bracket of a computed key.
func TestUnselectedConfigLinesTSAV0655(t *testing.T) {
	cases := map[string]struct {
		config, reason string
		line           int
	}{
		"required export":   {"const x = 1;\nmodule.exports = require('./base');\n", "runner-config-export-dynamic", 2},
		"factory export":    {"// config\n\nmodule.exports = makeConfig({\n  testMatch: ['**/*.tests.js'],\n});\n", "runner-config-export-dynamic", 3},
		"shorthand key":     {"const testMatch = ['**/*.tests.js'];\nmodule.exports = {\n  testMatch,\n};\n", "runner-config-key-dynamic", 1},
		"computed key":      {"const k = 'testMatch';\nmodule.exports = {\n  [k]: ['**/*.tests.js'],\n};\n", "runner-config-key-dynamic", 1},
		"computed only":     {"module.exports = {\n  verbose: true,\n  [key()]: ['**/*.tests.js'],\n};\n", "runner-config-key-dynamic", 3},
		"member assignment": {"module.exports = {};\n\nmodule.exports.testMatch = ['**/*.tests.js'];\n", "runner-config-key-dynamic", 3},
	}
	for name, c := range cases {
		report := unselectedReport(t, map[string]string{
			"package.json":   `{"devDependencies":{"jest":"29.7.0"}}`,
			"jest.config.js": c.config,
			"src/a.test.js":  globalTest,
		})
		if len(report.Runners) != 1 || report.Runners[0].Reason != c.reason || report.Runners[0].Line != c.line {
			t.Errorf("%s: runners = %+v, want %s at line %d", name, report.Runners, c.reason, c.line)
		}
		if file, _ := selectionOf(report, "src/a.test.js"); file.Selection != SelectionUncertain || file.ConfigLine != c.line {
			t.Errorf("%s: file = %+v", name, file)
		}
	}
}
