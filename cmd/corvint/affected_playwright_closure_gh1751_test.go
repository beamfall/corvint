package main

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// GitHub #1751 retest (V1-2693, AFP-V0-442): only a source the Playwright run loads contradicts the
// documentation declaration. A standalone Node script outside the closure of the config, its
// tests and their imports is recorded in the declaration as an out-of-closure reader; a script a
// spec names, a reader whose closure membership is unknown, and an imported helper still refuse.
func TestAffectedPlaywrightDeclarationClosure_GH1751(t *testing.T) {
	const declared = "docs/contracts/_setup.json"
	const script = "scripts/check.mjs"
	declaration := `{"profile":"corvint-documentation-paths/0","paths":["docs/"]}`
	declarationSum := sha256.Sum256([]byte(declaration))
	digest := hex.EncodeToString(declarationSum[:])
	scriptBody := "import { readFileSync } from \"node:fs\"\nconsole.log(readFileSync(\"docs/contracts/_setup.json\", \"utf8\"))\n"
	plan := func(t *testing.T, root string) typescript.PlaywrightPlan {
		t.Helper()
		receipt, raw, code, stderr := gh1751Plan(t, root)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		if receipt.Plan.Fallback == typescript.PlaywrightFallbackNone && receipt.Plan.Discovery.State != "MATCHED" {
			t.Fatalf("discovery=%+v receipt=%s", receipt.Plan.Discovery, raw)
		}
		return receipt.Plan
	}

	t.Run("out-of-closure script is recorded, not refused", func(t *testing.T) {
		got := plan(t, gh1751Fixture(t, map[string]string{".corvint/documentation-paths.json": declaration, script: scriptBody}))
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone {
			t.Fatalf("scope=%s fallback=%s unknown=%+v", got.Scope, got.Fallback, got.Unknown)
		}
		want := typescript.PlaywrightPathDeclaration{
			Matched:      []string{declared},
			OutOfClosure: []typescript.PlaywrightOutOfClosureReader{{Path: declared, Site: script + ":2 `docs/contracts/_setup.json`"}},
			Path:         ".corvint/documentation-paths.json", SHA256: digest, State: typescript.PlaywrightDeclarationApplied,
		}
		if got.Declaration == nil || !reflect.DeepEqual(*got.Declaration, want) {
			t.Fatalf("declaration=%+v want %+v", got.Declaration, want)
		}
		if len(got.Selected) != 1 || got.Selected[0].Test != "tests/page.spec.ts" || len(got.Excluded) != 1 || got.Excluded[0].Declaration != ".corvint/documentation-paths.json" {
			t.Fatalf("selected=%+v excluded=%+v", got.Selected, got.Excluded)
		}
	})
	for name, test := range map[string]struct {
		files map[string]string
		why   string
	}{
		"script a spec names": {map[string]string{
			"tests/other.spec.ts": `import { test } from "@playwright/test"; import { execFileSync } from "node:child_process"; test("other", () => execFileSync("node", ["scripts/check.mjs"]))`,
		}, "reaches Playwright closure unit"},
		// A script a spec names, and the modules it imports, enter the closure (Codex review).
		"module of a script a spec names": {map[string]string{
			"tests/other.spec.ts": `import { test } from "@playwright/test"; import { execFileSync } from "node:child_process"; test("other", () => execFileSync("node", ["scripts/run.mjs"]))`,
			"scripts/run.mjs":     "import \"./check.mjs\"\n",
		}, "is in the Playwright closure"},
		"closure with a dynamic import": {map[string]string{
			"tests/other.spec.ts": `import { test } from "@playwright/test"; test("other", async () => { await import(process.env.MODULE ?? "") })`,
		}, "Playwright closure membership is unknown"},
	} {
		t.Run("membership refused: "+name, func(t *testing.T) {
			test.files[".corvint/documentation-paths.json"] = declaration
			test.files[script] = scriptBody
			got := plan(t, gh1751Fixture(t, test.files))
			if got.Fallback != typescript.PlaywrightFallbackFullSuite || got.Declaration == nil || got.Declaration.State != typescript.PlaywrightDeclarationRefused || len(got.Declaration.OutOfClosure) != 0 {
				t.Fatalf("fallback=%s declaration=%+v unknown=%+v", got.Fallback, got.Declaration, got.Unknown)
			}
			if !gh1751HasUnknown(got, typescript.PlaywrightUnknownDeclaredPathContradicted, "declares "+declared, "but "+script+":2 ", test.why) {
				t.Fatalf("unknown=%+v", got.Unknown)
			}
		})
	}

	// AFP-V0-443: a change that adds the declaration classifies the declaration itself instead of
	// raising UNOWNED_DIRTY_PATH, so the plan it narrows stays bounded.
	t.Run("declaration added in the range", func(t *testing.T) {
		root := gh1751Fixture(t, map[string]string{})
		writePlaywrightCLIFile(t, root, ".corvint/documentation-paths.json", declaration)
		got := plan(t, root)
		if got.Scope != affected.ScopeBounded || got.Fallback != typescript.PlaywrightFallbackNone || got.Declaration == nil || got.Declaration.State != typescript.PlaywrightDeclarationApplied {
			t.Fatalf("scope=%s fallback=%s declaration=%+v unknown=%+v", got.Scope, got.Fallback, got.Declaration, got.Unknown)
		}
		for _, unknown := range got.Unknown {
			if strings.Contains(unknown.Detail, "UNOWNED_DIRTY_PATH") {
				t.Fatalf("unknown=%+v", got.Unknown)
			}
		}
	})
}
