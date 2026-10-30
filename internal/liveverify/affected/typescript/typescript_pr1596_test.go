package typescript

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// PR #1596 review, finding 1 (TJAA-V0-038, TJAA-V0-045): after a reserved word that ends a
// statement, ASI lets a line-starting `/` open a regular expression. Lexical classification there
// is unsupported, so the slash is ambiguous and the module keeps a frontier instead of reading the
// regex's quote as a string that swallows a real require.
func TestUnsupportedSlashPredecessorPreservesUncertainty_PR1596(t *testing.T) {
	for _, body := range []string{
		"debugger\n/'+/.test(\"\"); const u = '//'; const b = require('./b');\n",
		"debugger\n/'a/.test(\"\"); const u = '//'; const b = require('./b');\n",
		"for (;;) { break\n/'+/.test(\"\"); const u = '//'; const b = require('./b') }\n",
		"x = 1\u00a0/'/.test(s); const u = '//'; const b = require('./b');\n",
		"debugger\n/a\\/\\//.test(s); const b = require('./b');\n",
	} {
		scan := scanImportSites("src/helper.ts", body)
		if scan.err == nil && scan.dynamic < 0 && scan.unresolved < 0 && !slices.Contains(scan.refs, "./b") {
			t.Fatalf("%q: a real require was dropped without a frontier: %+v", body, scan)
		}
	}
	root := playwrightFixture(t)
	write(t, root, "tests/pages/helper.ts", "debugger\n/'+/.test(\"\"); const u = '//'; const b = require('./b');\n")
	write(t, root, "tests/pages/b.ts", "export const b = 1\n")
	write(t, root, "tests/helper.spec.ts", "import { test } from '@playwright/test'\nimport './pages/helper'\ntest('@angular h', () => {})\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !frontierHas(result, FrontierScanUnresolved) {
		edge := false
		for _, unit := range result.Units {
			if slices.Contains(unit.Sources, "tests/pages/helper.ts") && slices.Contains(unit.Imports, "typescript:tests/pages/b.ts") {
				edge = true
			}
		}
		if !edge {
			t.Fatalf("helper's require of b.ts is neither an edge nor a frontier: %v", result.Frontier)
		}
	}
	// Words that are operands keep division, so ordinary arithmetic is not widened.
	for _, body := range []string{
		"const a = this\n/ 2; const b = require('./b')",
		"const a = x\n/ 2 / 'y'.length; const b = require('./b')",
	} {
		if scan := scanImportSites("src/a.ts", body); scan.unresolved >= 0 || !slices.Contains(scan.refs, "./b") {
			t.Fatalf("%q: %+v", body, scan)
		}
	}
}

// PR #1596 review, finding 3 (TJAA-V0-040): naming sites costs one pass per file, not one
// whole-body search and line count per unresolved import.
func TestFrontierSitesAreLinear_PR1596(t *testing.T) {
	var body strings.Builder
	for index := range 100000 {
		fmt.Fprintf(&body, "import './missing%06d';\n", index)
	}
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/many.ts", body.String())
	done := make(chan struct{})
	var sites frontierSet
	go func() {
		defer close(done)
		_, sites, _ = New().units(root, false)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("frontier site naming did not finish within 60s for 100,000 unresolved imports")
	}
	if len(sites[FrontierUnresolvedImport]) != 100000 || !sites.has(FrontierUnresolvedImport, "src/many.ts:100000") {
		t.Fatalf("sites=%d", len(sites[FrontierUnresolvedImport]))
	}
}

// PR #1596 review, finding 4 (TJAA-V0-040): the site is the parsed specifier's line, never an
// earlier comment or string that happens to quote the same text.
func TestFrontierSiteIsTheParsedSpecifier_PR1596(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/a.ts", "// Example: './missing'\nimport value from './missing';\nconst s = \"x\" // 'undeclared-pkg'\nimport y from 'undeclared-pkg'\n")
	_, sites, err := New().units(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if !sites.has(FrontierUnresolvedImport, "src/a.ts:2") || len(sites[FrontierUnresolvedImport]) != 1 {
		t.Fatalf("relative import site: %v", sites[FrontierUnresolvedImport])
	}
	if !sites.has(FrontierPathAlias, "src/a.ts:4") || len(sites[FrontierPathAlias]) != 1 {
		t.Fatalf("bare import site: %v", sites[FrontierPathAlias])
	}
}
