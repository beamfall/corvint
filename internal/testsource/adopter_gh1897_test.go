package testsource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestLintNodeAssertTSAV0649 proves a node:test file asserting through node:assert (a namespace,
// default or named binding, strict or not) or the test context's t.assert is not no-assertion, and
// a node:test file with no assertion still is (TSA-V0-649).
func TestLintNodeAssertTSAV0649(t *testing.T) {
	asserting := map[string]string{
		"default":   "import test from 'node:test';\nimport assert from 'node:assert/strict';\ntest('a', () => {\n  assert.equal(1, 1);\n});\n",
		"namespace": "import { test } from 'node:test';\nimport * as assert from 'node:assert';\ntest('a', () => {\n  assert.deepStrictEqual([1], [1]);\n});\n",
		"named":     "import { test } from 'node:test';\nimport { strictEqual as same } from 'assert';\ntest('a', () => {\n  same(1, 1);\n});\n",
		"required":  "const test = require('node:test');\nconst assert = require('node:assert');\ntest('a', () => {\n  assert(true);\n});\n",
		"context":   "import { test } from 'node:test';\ntest('a', (t) => {\n  t.assert.ok(true);\n});\n",
	}
	for name, text := range asserting {
		if got := rulesOf(LintText("test/a.test.js", text).Findings)[RuleNoAssertion]; got != 0 {
			t.Errorf("%s: no-assertion fired on a node:assert test", name)
		}
	}
	bare := "import { test } from 'node:test';\nimport assert from 'node:assert';\ntest('a', () => {\n  const x = assert;\n});\n"
	if got := rulesOf(LintText("test/a.test.js", bare).Findings)[RuleNoAssertion]; got != 1 {
		t.Errorf("a test that only names assert: no-assertion = %d, want 1", got)
	}
}

// TestLintAllLinesWithoutBaseTSAV0650 proves --all-lines needs no base: every finding at the
// revision is reported with an empty base, and a changed-lines run without a base still refuses
// (TSA-V0-650).
func TestLintAllLinesWithoutBaseTSAV0650(t *testing.T) {
	root := t.TempDir()
	head := commitFiles(t, root, map[string]string{"e2e/legacy.spec.ts": oldSpec})
	repo := OpenRepo(context.Background(), root)
	all, err := Lint(repo, LintOptions{Revision: "HEAD", AllLines: true})
	if err != nil {
		t.Fatal(err)
	}
	if all.Base != "" || all.Revision != head || all.Scope != "all-lines" || len(all.Findings) == 0 {
		t.Fatalf("all-lines without base = %+v", all)
	}
	if _, err := Lint(repo, LintOptions{Revision: "HEAD"}); err == nil {
		t.Fatal("a changed-lines lint without a base did not refuse")
	}
}

// TestTemplateTitlesTSAV0651 proves a template-literal title with a substitution is shown as its
// source text, never with the internal NUL substitution mark (TSA-V0-651).
func TestTemplateTitlesTSAV0651(t *testing.T) {
	text := "const label = 'x';\ntest.describe(`Suite (${label})`, () => {\n  test(`one ${label} two`, async () => {});\n});\n"
	got := []string{}
	for _, d := range parseSource("e2e/a.spec.ts", text).decls {
		got = append(got, d.title)
	}
	if strings.Join(got, "|") != "Suite (${label})|one ${label} two" {
		t.Fatalf("titles = %q", got)
	}
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": strings.ReplaceAll(cartA, "'cart'", "`cart ${label}`"), "e2e/b.spec.ts": cartB})
	options := duplicateOptions()
	options.MaxLineTests = 3
	report, err := Duplicates(OpenRepo(context.Background(), root), options)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(report)
	if len(report.Unresolved) == 0 || !strings.Contains(string(encoded), "cart ${label} \\u003e adds an item") || strings.Contains(string(encoded), `\u0000`) {
		t.Fatalf("duplicates = %s", encoded)
	}
}

// TestUnselectedConfigLineTSAV0648 proves an uncertain runner configuration names the line that
// triggered its reason, on the runner and on each file that abstains on it (TSA-V0-648).
func TestUnselectedConfigLineTSAV0648(t *testing.T) {
	spec := "import { test, expect } from '@playwright/test';\n" + globalTest
	cases := map[string]struct {
		config string
		reason string
		line   int
	}{
		"unbalanced": {"import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  testDir: './e2e',\n" +
			"  webServer: { command: 'npm start' }],\n});\n", "runner-config-unparsed", 4},
		"projects": {"const base = {};\nexport default {\n  projects: [\n    base,\n  ],\n};\n", "runner-config-project-selection", 4},
		"spread":   {"const base = {};\nexport default {\n  ...base,\n};\n", "runner-config-spread", 3},
	}
	for name, c := range cases {
		report := unselectedReport(t, map[string]string{"playwright.config.ts": c.config, "e2e/a.spec.ts": spec})
		if len(report.Runners) != 1 || report.Runners[0].Reason != c.reason || report.Runners[0].Line != c.line {
			t.Errorf("%s: runners = %+v, want %s at line %d", name, report.Runners, c.reason, c.line)
		}
		if file, _ := selectionOf(report, "e2e/a.spec.ts"); file.Selection != SelectionUncertain || file.ConfigLine != c.line {
			t.Errorf("%s: file = %+v", name, file)
		}
	}
}

// TestLintTryFinallyTSAV0652 proves an assertion in a finally-only try, or in a try whose catch
// always rethrows, is not conditional, while a catch that swallows or rethrows only on a branch
// still makes it conditional (TSA-V0-652).
func TestLintTryFinallyTSAV0652(t *testing.T) {
	cases := map[string]struct {
		handler string
		want    int
	}{
		"finally only":       {"finally {\n    await ctx.close();\n  }", 0},
		"rethrow":            {"catch (e) {\n    console.log(e);\n    throw e;\n  } finally {\n    await ctx.close();\n  }", 0},
		"swallow":            {"catch (e) {\n    console.log(e);\n  }", 1},
		"swallow finally":    {"catch {\n  } finally {\n    await ctx.close();\n  }", 1},
		"branch rethrow":     {"catch (e) {\n    if (flaky) throw e;\n  }", 1},
		"early return":       {"catch (e) {\n    if (flaky) return;\n    throw e;\n  }", 1},
		"nested block throw": {"catch (e) {\n    { throw e; }\n  }", 1},
	}
	for name, c := range cases {
		text := "test('t', async ({ browser }) => {\n  const ctx = await browser.newContext();\n  try {\n" +
			"    const page = await ctx.newPage();\n    await expect(page.locator('h1')).toBeVisible();\n  } " + c.handler + "\n});\n"
		report := LintText("e2e/a.spec.ts", text)
		if got := rulesOf(report.Findings)[RuleConditional]; got != c.want {
			t.Errorf("%s: conditional-assertion = %d, want %d: %+v", name, got, c.want, report.Findings)
		}
	}
}

// TestUnselectedRegexInSubstitutionTSAV0838 proves a regex literal inside a template-literal
// substitution leaves the runner config readable (TSA-V0-838).
func TestUnselectedRegexInSubstitutionTSAV0838(t *testing.T) {
	spec := "import { test, expect } from '@playwright/test';\n" + globalTest
	config := "import { defineConfig } from '@playwright/test';\nconst a = `${x.replace(/[:.]/g, \"-\")}`;\nexport default defineConfig({ testDir: './e2e' });\n"
	report := unselectedReport(t, map[string]string{"playwright.config.ts": config, "e2e/a.spec.ts": spec})
	for _, r := range report.Runners {
		if r.Reason == "runner-config-unparsed" {
			t.Errorf("runner config unparsed: %+v", r)
		}
	}
}

// TestUnselectedRegexInSubstitutionShapesTSAV0847 proves a regex literal leaves the runner config
// readable wherever only an expression may start inside a template substitution: right after
// `${`, after an arrow, and after a keyword such as `return` or `typeof`; a division after an
// operand reads too, while a comment in a substitution still leaves it unparsed (TSA-V0-847).
func TestUnselectedRegexInSubstitutionShapesTSAV0847(t *testing.T) {
	spec := "import { test, expect } from '@playwright/test';\n" + globalTest
	for _, line := range []string{
		"const a = `${/[:.]/.test(x) ? 'a' : 'b'}`;",
		"const a = `${xs.map((v) => /[:.]/.test(v))}`;",
		"const a = `${(() => { return /[:.]/g.source; })()}`;",
		"const a = `${typeof /x/}`;",
		"const a = `${total / 2}-${xs[0] / 3}`;",
	} {
		config := "import { defineConfig } from '@playwright/test';\n" + line + "\nexport default defineConfig({ testDir: './e2e' });\n"
		report := unselectedReport(t, map[string]string{"playwright.config.ts": config, "e2e/a.spec.ts": spec})
		for _, r := range report.Runners {
			if r.Reason == "runner-config-unparsed" {
				t.Errorf("%s: runner config unparsed: %+v", line, r)
			}
		}
	}
	config := "import { defineConfig } from '@playwright/test';\nconst a = `${a /* } */}`;\nexport default defineConfig({ testDir: './e2e' });\n"
	report := unselectedReport(t, map[string]string{"playwright.config.ts": config, "e2e/a.spec.ts": spec})
	if len(report.Runners) == 0 || report.Runners[0].Reason != "runner-config-unparsed" {
		t.Errorf("a comment in a substitution read as code: %+v", report.Runners)
	}
}
