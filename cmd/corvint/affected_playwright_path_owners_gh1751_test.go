package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// gh1751Fixture commits a one-project Playwright repository with a JSON data file under docs/,
// then changes one spec and that data file.
func gh1751Fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writePlaywrightCLIFile(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	writePlaywrightCLIFile(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  projects: [
    { name: "chromium", use: { browserName: "chromium" } },
  ],
})
`)
	writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page", () => {})`)
	writePlaywrightCLIFile(t, root, "tests/other.spec.ts", `import { test } from "@playwright/test"; test("other", () => {})`)
	writePlaywrightCLIFile(t, root, "docs/contracts/_setup.json", `{"v":1}`)
	for name, body := range files {
		writePlaywrightCLIFile(t, root, name, body)
	}
	affectedGit(t, root, "init", "-q")
	affectedGit(t, root, "config", "user.email", "corvint@example.test")
	affectedGit(t, root, "config", "user.name", "Corvint Test")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "fixture")
	writePlaywrightCLIFile(t, root, "tests/page.spec.ts", `import { test } from "@playwright/test"; test("page changed", () => {})`)
	writePlaywrightCLIFile(t, root, "docs/contracts/_setup.json", `{"v":2}`)
	return root
}

// gh1751Plan runs affected with a discovery receipt bound to the fixture's current sources.
func gh1751Plan(t *testing.T, root string) (playwrightAffectedReceipt, []byte, int, string) {
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
	code := runContext(t.Context(), []string{"--root", root, "affected", "--playwright-config", "playwright.config.ts", "--playwright-discovery", discovery}, strings.NewReader(""), &stdout, &stderr)
	var receipt playwrightAffectedReceipt
	if code == 0 {
		if err := json.Unmarshal(stdout.Bytes(), &receipt); err != nil {
			t.Fatal(err)
		}
	}
	return receipt, stdout.Bytes(), code, stderr.String()
}

// GitHub #1751 (V1-2646, TJAA-V0-101): the project-owned .corvint/documentation-paths.json
// declaration (AFP-V0-160) narrows a Playwright plan whose dirty data path no observed source
// names; the receipt records the declaration's path, digest and matched paths, and each exclusion
// names it. A declaration that an observed import, config or path literal contradicts is refused:
// the plan falls back to the full suite with an unknown naming the declaration and the evidence.
func TestAffectedPlaywrightPathDeclaration_GH1751(t *testing.T) {
	const declared = "docs/contracts/_setup.json"
	declaration := `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`
	declarationSum := sha256.Sum256([]byte(declaration))
	digest := hex.EncodeToString(declarationSum[:])
	plan := func(t *testing.T, files map[string]string) typescript.PlaywrightPlan {
		t.Helper()
		receipt, raw, code, stderr := gh1751Plan(t, gh1751Fixture(t, files))
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		if receipt.Plan.Fallback == typescript.PlaywrightFallbackNone && receipt.Plan.Discovery.State != "MATCHED" {
			t.Fatalf("discovery=%+v receipt=%s", receipt.Plan.Discovery, raw)
		}
		return receipt.Plan
	}
	withDeclaration := func(files map[string]string) map[string]string {
		files[".corvint/documentation-paths.json"] = declaration
		return files
	}

	t.Run("no declaration keeps the full suite", func(t *testing.T) {
		got := plan(t, map[string]string{})
		if got.Fallback != typescript.PlaywrightFallbackFullSuite || got.Declaration != nil {
			t.Fatalf("fallback=%s declaration=%+v", got.Fallback, got.Declaration)
		}
		if !gh1751HasUnknown(got, typescript.PlaywrightUnknownDynamicSource, "UNOWNED_DIRTY_PATH "+declared) {
			t.Fatalf("unknown=%+v", got.Unknown)
		}
	})
	t.Run("declaration matches and is recorded", func(t *testing.T) {
		got := plan(t, withDeclaration(map[string]string{
			// A route and a different directory spell parts of the path but name no file.
			"tests/other.spec.ts": `import { test } from "@playwright/test"; test("other", async ({ page }) => { await page.goto("/api/docs"); void "src/docs" })`,
		}))
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone {
			t.Fatalf("scope=%s fallback=%s unknown=%+v", got.Scope, got.Fallback, got.Unknown)
		}
		want := typescript.PlaywrightPathDeclaration{Matched: []string{declared}, Path: ".corvint/documentation-paths.json", SHA256: digest, State: typescript.PlaywrightDeclarationApplied}
		if got.Declaration == nil || !reflect.DeepEqual(*got.Declaration, want) {
			t.Fatalf("declaration=%+v want %+v", got.Declaration, want)
		}
		if len(got.Selected) != 1 || got.Selected[0].Test != "tests/page.spec.ts" || len(got.Excluded) != 1 {
			t.Fatalf("selected=%+v excluded=%+v", got.Selected, got.Excluded)
		}
		if excluded := got.Excluded[0]; excluded.Test != "tests/other.spec.ts" || excluded.Declaration != ".corvint/documentation-paths.json" {
			t.Fatalf("excluded=%+v", excluded)
		}
	})
	// TJAA-V0-102: the shared graph now records TypeScript path literals, so a route that TJAA-V0-101
	// does not count as a name still names the path's components by the generic reader rule
	// (AFP-V0-021). The plan selects that spec as a PATH_LITERAL_READER instead of excluding it; the
	// declaration still applies and is recorded.
	t.Run("route spelling selects the reader", func(t *testing.T) {
		got := plan(t, withDeclaration(map[string]string{
			"tests/other.spec.ts": `import { test } from "@playwright/test"; test("other", async ({ page }) => { await page.goto("/docs/contracts") })`,
		}))
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone || got.Declaration == nil || got.Declaration.State != typescript.PlaywrightDeclarationApplied {
			t.Fatalf("scope=%s fallback=%s declaration=%+v unknown=%+v", got.Scope, got.Fallback, got.Declaration, got.Unknown)
		}
		if len(got.Selected) != 2 || len(got.Excluded) != 0 || got.Selected[0].Test != "tests/other.spec.ts" || got.Selected[0].Witness.Kind != affected.WitnessPathLiteralReader || got.Selected[0].Witness.DirtyPath != declared {
			t.Fatalf("selected=%+v excluded=%+v", got.Selected, got.Excluded)
		}
	})
	// AFP-V0-442: every reader below is in the Playwright closure: a spec, a helper a spec imports, or
	// the config.
	for name, test := range map[string]struct{ file, body, site string }{
		"static import": {"tests/other.spec.ts", `import { test } from "@playwright/test"; import data from "../docs/contracts/_setup.json"; test("other", () => data)`, "tests/other.spec.ts:1"},
		"path literal":  {"tests/other.spec.ts", `import { test } from "@playwright/test"; import { readFileSync } from "node:fs"; test("other", () => readFileSync("docs/contracts/_setup.json"))`, "tests/other.spec.ts:1"},
		"escaped":       {"tests/other.spec.ts", `import { test } from "@playwright/test"; import { readFileSync } from "node:fs"; test("other", () => readFileSync("docs\u002fcontracts\u002f_setup\u002ejson"))`, "tests/other.spec.ts:1"},
		"directory":     {"tests/other.spec.ts", `import { test } from "@playwright/test"; import { readdirSync } from "node:fs"; test("other", () => readdirSync("docs/contracts"))`, "tests/other.spec.ts:1"},
		"template":      {"tests/other.spec.ts", "import { test } from \"@playwright/test\"; import { readdirSync } from \"node:fs\"; test(\"other\", () => readdirSync(`${process.cwd()}/docs/`))", "tests/other.spec.ts:1"},
		"file name":     {"tests/fixtures.ts", "import { join } from \"node:path\"\nexport const setup = join(\"docs\", \"contracts\", \"_setup.json\")\n", "tests/fixtures.ts:2"},
		"config":        {"playwright.config.ts", "import { defineConfig } from \"@playwright/test\"\nimport meta from \"./docs/contracts/_setup.json\"\nexport default defineConfig({ metadata: meta, projects: [ { name: \"chromium\", use: { browserName: \"chromium\" } } ] })\n", "playwright.config.ts:2"},
	} {
		t.Run("contradiction refused: "+name, func(t *testing.T) {
			files := map[string]string{test.file: test.body}
			if test.file == "tests/fixtures.ts" {
				files["tests/other.spec.ts"] = `import { test } from "@playwright/test"; import { setup } from "./fixtures"; test("other", () => setup)`
			}
			got := plan(t, withDeclaration(files))
			if got.Fallback != typescript.PlaywrightFallbackFullSuite || got.Declaration == nil || got.Declaration.State != typescript.PlaywrightDeclarationRefused || got.Declaration.SHA256 != digest {
				t.Fatalf("fallback=%s declaration=%+v unknown=%+v", got.Fallback, got.Declaration, got.Unknown)
			}
			if !gh1751HasUnknown(got, typescript.PlaywrightUnknownDeclaredPathContradicted, ".corvint/documentation-paths.json (sha256:"+digest+") declares "+declared, "but "+test.site+" ") {
				t.Fatalf("unknown=%+v", got.Unknown)
			}
			for _, excluded := range got.Excluded {
				if excluded.Declaration != "" {
					t.Fatalf("refused declaration still names an exclusion: %+v", excluded)
				}
			}
		})
	}
}

func gh1751HasUnknown(plan typescript.PlaywrightPlan, reason string, parts ...string) bool {
	for _, unknown := range plan.Unknown {
		if unknown.Reason != reason {
			continue
		}
		found := true
		for _, part := range parts {
			found = found && strings.Contains(unknown.Detail, part)
		}
		if found {
			return true
		}
	}
	return false
}
