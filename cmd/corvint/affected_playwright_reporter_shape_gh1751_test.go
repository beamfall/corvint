package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// GitHub #1751, reporter shape (every scenario in the thread, on one committed range): the base
// holds a Playwright project whose testDir is tests/, JSON metadata under docs/migrations/, and a
// standalone Node CLI (scripts/assemble-test-behavior-contracts.mjs) that names one of those JSON
// files at line 11. Only package.json and the CLI's own `node --test` file run it; nothing in the
// Playwright closure imports or names it. The range, planned with --base and a two-step discovery
// receipt, adds the documentation declaration, changes one spec and three JSON files.
//
//   - S1 (original report): without a declaration the dirty JSON paths are UNOWNED_DIRTY_PATH and
//     the plan is the full suite.
//   - S2 (18:37 retest, ask 1): with the declaration the out-of-closure CLI is recorded as an
//     out-of-closure reader, not a contradiction, and the plan narrows to the changed spec.
//   - S3 (ask 3): the declaration added in the range is not UNOWNED_DIRTY_PATH.
//   - S4 (original asks): the receipt records the declaration's digest and matched paths, and each
//     exclusion names the declaration.
func TestAffectedPlaywrightDeclarationReporterShape_GH1751(t *testing.T) {
	const declarationPath = ".corvint/documentation-paths.json"
	const cli = "scripts/assemble-test-behavior-contracts.mjs"
	declaration := `{"profile":"corvint-documentation-paths/0","paths":["docs/migrations/"]}`
	declarationSum := sha256.Sum256([]byte(declaration))
	cliBody := strings.Join([]string{
		"#!/usr/bin/env node",
		`import { readFileSync, writeFileSync, readdirSync } from "node:fs"`,
		`import { join } from "node:path"`,
		"",
		"export function assemble(dir) {",
		"  return readdirSync(dir).filter((name) => name.endsWith(\".json\")).map((name) => JSON.parse(readFileSync(join(dir, name), \"utf8\")))",
		"}",
		"",
		"if (import.meta.url === `file://${process.argv[1]}`) {",
		`  const contracts = assemble(process.argv[2])`,
		`  writeFileSync("docs/migrations/test-behavior-contracts.json", JSON.stringify(contracts))`,
		"}",
		"",
	}, "\n")
	files := map[string]string{
		"package.json": `{"scripts":{"contracts":"node scripts/assemble-test-behavior-contracts.mjs","test:node":"node --test scripts/"},"devDependencies":{"@playwright/test":"1.61.0"}}`,
		"playwright.config.ts": `import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  reporter: [["list"]],
  projects: [{ name: "chromium", use: { browserName: "chromium" } }],
})
`,
		"tests/page.spec.ts":  `import { test } from "@playwright/test"; test("page", () => {})`,
		"tests/other.spec.ts": `import { test } from "@playwright/test"; test("other", () => {})`,
		cli:                   cliBody,
		"scripts/assemble-test-behavior-contracts.test.mjs": "import { test } from \"node:test\"\nimport { assemble } from \"./assemble-test-behavior-contracts.mjs\"\ntest(\"assemble\", () => assemble(\"fixtures\"))\n",
		"docs/migrations/contracts/_setup.json":             `{"v":1}`,
		"docs/migrations/contracts/checkout.json":           `{"v":1}`,
		"docs/migrations/test-behavior-contracts.json":      `[]`,
	}
	dirty := []string{"docs/migrations/contracts/_setup.json", "docs/migrations/contracts/checkout.json", "docs/migrations/test-behavior-contracts.json"}
	fixture := func(t *testing.T, withDeclaration bool) (string, string) {
		t.Helper()
		root := t.TempDir()
		for name, body := range files {
			writePlaywrightCLIFile(t, root, name, body)
		}
		affectedGit(t, root, "init", "-q")
		affectedGit(t, root, "config", "user.email", "corvint@example.test")
		affectedGit(t, root, "config", "user.name", "Corvint Test")
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "base")
		base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page changed", () => {})`)
		for _, path := range dirty {
			writePlaywrightCLIFile(t, root, path, `{"v":2}`)
		}
		if withDeclaration {
			writePlaywrightCLIFile(t, root, declarationPath, declaration)
		}
		affectedGit(t, root, "add", ".")
		affectedGit(t, root, "commit", "-qm", "range")
		return root, base
	}
	plan := func(t *testing.T, root, base string) typescript.PlaywrightPlan {
		t.Helper()
		config, err := os.ReadFile(filepath.Join(root, "playwright.config.ts"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(config)
		digest, err := typescript.ObservePlaywrightSources(root, "playwright.config.ts")
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(typescript.PlaywrightDiscovery{Profile: "playwright-discovery/0", Revision: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")), Config: typescript.PlaywrightConfigIdentity{Path: "playwright.config.ts", SHA256: hex.EncodeToString(sum[:])}, SourceDigest: digest, Units: []typescript.PlaywrightDiscoveryUnit{{Project: "chromium", Test: "tests/other.spec.ts"}, {Project: "chromium", Test: "tests/page.spec.ts"}}}, json.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		discovery := filepath.Join(t.TempDir(), "discovery.json")
		if err := os.WriteFile(discovery, body, 0o600); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		if code := runContext(t.Context(), []string{"--root", root, "affected", "--base", base, "--playwright-config", "playwright.config.ts", "--playwright-discovery", discovery}, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr.String())
		}
		var receipt playwrightAffectedReceipt
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
		return receipt.Plan
	}

	t.Run("S1 no declaration keeps the full suite", func(t *testing.T) {
		root, base := fixture(t, false)
		got := plan(t, root, base)
		if got.Fallback != typescript.PlaywrightFallbackFullSuite || got.Declaration != nil {
			t.Fatalf("fallback=%s declaration=%+v", got.Fallback, got.Declaration)
		}
		for _, path := range dirty {
			if !gh1751HasUnknown(got, typescript.PlaywrightUnknownDynamicSource, "UNOWNED_DIRTY_PATH "+path) {
				t.Fatalf("%s: unknown=%+v", path, got.Unknown)
			}
		}
	})
	t.Run("S2-S4 declaration narrows to the changed spec", func(t *testing.T) {
		root, base := fixture(t, true)
		got := plan(t, root, base)
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone || got.Declaration == nil {
			t.Fatalf("scope=%s fallback=%s declaration=%+v unknown=%+v", got.Scope, got.Fallback, got.Declaration, got.Unknown)
		}
		d := *got.Declaration
		if d.State != typescript.PlaywrightDeclarationApplied || d.Path != declarationPath || d.SHA256 != hex.EncodeToString(declarationSum[:]) || strings.Join(d.Matched, ",") != strings.Join(dirty, ",") {
			t.Fatalf("declaration=%+v", d)
		}
		if len(d.OutOfClosure) != 1 || d.OutOfClosure[0].Path != "docs/migrations/test-behavior-contracts.json" || !strings.HasPrefix(d.OutOfClosure[0].Site, cli+":11 ") {
			t.Fatalf("out of closure=%+v", d.OutOfClosure)
		}
		for _, unknown := range got.Unknown {
			if strings.Contains(unknown.Detail, "UNOWNED_DIRTY_PATH") {
				t.Fatalf("unknown=%+v", got.Unknown)
			}
		}
		if len(got.Selected) != 1 || got.Selected[0].Test != "tests/page.spec.ts" || len(got.Excluded) != 1 || got.Excluded[0].Test != "tests/other.spec.ts" || got.Excluded[0].Declaration != declarationPath {
			t.Fatalf("selected=%+v excluded=%+v", got.Selected, got.Excluded)
		}
	})
}
