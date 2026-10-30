package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// GitHub #1593 repro rows: a regular-expression literal after `(` followed later on the line by a
// string holding `/`. Line 2 of each module must raise neither typescript:dynamic-import nor
// typescript:scan-unresolved (TJAA-V0-038).
var gh1593Lines = []string{
	`const a = s.replace(/[^/]+/, '').replace('/x', '');`,
	`const a = s.replace(/[^x]+/, '').replace('/x', '');`,
	`const a = s.replace(/[^\/]+/, '').replace('/x', '');`,
	`const a = s.replace(/[^/]+/, '').replace('x', '');`,
	`const path = url.replace(/^https?:\/\/[^/]+/, '');`,
	`const path = url.replace(/^https?:\/\/[^/]+/, '').replace('/api', '');`,
}

// TJAA-V0-038: the ambiguous-slash check reads tokens, so the closing slash of a regular
// expression after `(` is never an ambiguous division; every listed opener starts a literal.
func TestRegexLiteralAfterOpenerIsNotDivision_GH1593(t *testing.T) {
	for _, line := range gh1593Lines {
		body := "import { test } from '@playwright/test';\n" + line + "\n"
		if scan := scanImportSites("tests/a.spec.ts", body); scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 {
			t.Fatalf("%s: %+v", line, scan)
		}
	}
	for opener, line := range map[string]string{
		"(":          `f(/'/, '/x')`,
		",":          `f(a, /'/, '/x')`,
		"=":          `x = /'/; y = '/x'`,
		":":          `o = { k: /'/, j: '/x' }`,
		"[":          `a = [/'/, '/x']`,
		"!":          `b = !/'/.test(s) && '/x'`,
		"! twice":    `b = !!/'/.test(s) && '/x'`,
		"&":          `b = c && /'/.test(s) || '/x'`,
		"|":          `b = c || /'/.test(s) || '/x'`,
		"?":          `b = c ? /'/ : '/x'`,
		"{":          `{ /'/.test(s); f('/x') }`,
		";":          `;/'/.test(s); f('/x')`,
		"return":     `function f() { return /'/.test(s) || '/x' }`,
		"line start": `/'/.test(s); f('/x')`,
	} {
		for _, body := range []string{
			"import a from './a';\n" + line + "; const b = require('./b')\n",
			line + "\nconst b = require('./b')\n",
		} {
			scan := scanImportSites("src/a.ts", body)
			if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || !slices.Contains(scan.refs, "./b") {
				t.Fatalf("%s: %q: %+v", opener, body, scan)
			}
		}
	}
	// A `!` that starts a line is the prefix negation: no line terminator may precede a non-null
	// assertion. A comment before a slash is whitespace to the slash's verdict.
	for _, body := range []string{
		"const a = foo\n!/'/.test(s)\nconst b = require('./b') // '",
		"const a = foo\n  !/`/.test(s)\nconst b = require('./b')\n// `",
		"const a = /* c */ /'/.test(s); const b = require('./b')",
	} {
		for _, relative := range []string{"src/a.ts", "src/a.js"} {
			if scan := scanImportSites(relative, body); scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || !slices.Contains(scan.refs, "./b") {
				t.Fatalf("%s %q: %+v", relative, body, scan)
			}
		}
	}
	commented := "const a = /* c */ /'/; if (x) /'/.test(y); const u = '//'; const m = require('./lazy')"
	if scan := scanImportSites("src/a.ts", commented); scan.err != nil || scan.unresolved != strings.Index(commented, "/'/.test") {
		t.Fatalf("an ambiguity after a commented regular expression was missed: %+v", scan)
	}
	// A postfix non-null assertion ends an operand, so the slash after it is division.
	if scan := scanImportSites("src/a.ts", "const q = x! / 2; const y = '/x'; const b = require('./b')"); scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 || !slices.Contains(scan.refs, "./b") {
		t.Fatalf("non-null assertion: %+v", scan)
	}
}

// TJAA-V0-039: a slash whose predecessor needs grammar state and whose two readings move a
// literal boundary is typescript:scan-unresolved at its line, never typescript:dynamic-import.
func TestAmbiguousSlashIsScanUnresolved_GH1593(t *testing.T) {
	body := "import a from './a';\nif (c) {}\n/'/.test(s); const u = '//'; const b = require('./b')\n"
	scan := scanImportSites("src/a.ts", body)
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved != strings.Index(body, "/'") || lineAt(body, scan.unresolved) != 3 {
		t.Fatalf("closing brace ambiguity: %+v", scan)
	}
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "vitest.config.ts", `export default {}`)
	write(t, root, "src/a.ts", body)
	result, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	assertFrontier(t, result, FrontierScanUnresolved)
	if slices.Contains(result.Frontier, FrontierDynamicImport) || !sites.has(FrontierScanUnresolved, "src/a.ts:3") {
		t.Fatalf("frontier=%v sites=%v", result.Frontier, sites)
	}
}

// TJAA-V0-038, TJAA-V0-040: the issue's Playwright module raises no frontier, and a real dynamic
// import names its path and line in the plan's unknown detail.
func TestPlaywrightRegexReplaceKeepsSelectionAndNamesFrontierSites_GH1593(t *testing.T) {
	for _, line := range gh1593Lines {
		root := playwrightFixture(t)
		write(t, root, "tests/pages/url.ts", "import { test } from '@playwright/test';\nexport "+line+"\n")
		result, err := New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if frontierHas(result, FrontierDynamicImport) || frontierHas(result, FrontierScanUnresolved) {
			t.Fatalf("%s: frontier=%v", line, result.Frontier)
		}
	}
	root := playwrightFixture(t)
	write(t, root, "tests/login.spec.ts", "import { test } from \"@playwright/test\"\nconst moduleName = \"./pages/login\"\ntest(\"@angular @react login\", async () => { await import(moduleName) })\n")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/pages/login.ts"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, unknown := range plan.Unknown {
		if unknown.Reason == FrontierDynamicImport {
			found = unknown.Detail == "shared TypeScript graph frontier at tests/login.spec.ts:3 `import`"
		}
	}
	if !found {
		t.Fatalf("dynamic-import detail does not name its site: %v", plan.Unknown)
	}
}

// TJAA-V0-040: every frontier reason carries its sites, `path:line` where one line holds the
// cause, and the detail bounds them.
func TestFrontierSitesNameEveryReason_GH1593(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "vitest.config.ts", "export default {\n  test: { include: ['src/**'] },\n}\n")
	write(t, root, "src/a.test.ts", "import { test } from 'vitest'\nimport x from './missing'\nimport y from 'undeclared-package'\nconst z = require(name)\n")
	write(t, root, "src/b.ts", "const ok = 1\nconst t = `unterminated\n")
	write(t, root, "src/c.mts", "export const c = 1\n")
	result, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		FrontierConfig:           "vitest.config.ts:2",
		FrontierUnresolvedImport: "src/a.test.ts:2",
		FrontierPathAlias:        "src/a.test.ts:3",
		FrontierDynamicImport:    "src/a.test.ts:4",
		FrontierUnparsedSource:   "src/b.ts:2",
	}
	for reason, site := range want {
		if !slices.Contains(result.Frontier, reason) || !sites.has(reason, site) {
			t.Fatalf("%s: want site %s, frontier=%v sites=%v", reason, site, result.Frontier, sites)
		}
	}
	if !sites.has(FrontierUnparsedSource, "src/c.mts") {
		t.Fatalf("an unread module is named by path: %v", sites[FrontierUnparsedSource])
	}
	for _, reason := range result.Frontier {
		if len(sites[reason]) == 0 {
			t.Fatalf("%s has no site: %v", reason, sites)
		}
	}
	many := map[string]string{}
	for index := range 10 {
		many["src/f"+string(rune('a'+index))+".ts:1"] = ""
	}
	if detail := frontierDetail(many); !strings.HasSuffix(detail, " and 2 more") || strings.Count(detail, ".ts:1") != playwrightFrontierSiteBound {
		t.Fatalf("unbounded detail: %s", detail)
	}
	if frontierDetail(nil) != "shared TypeScript graph frontier" {
		t.Fatal("a reason without sites keeps the bare detail")
	}
}
