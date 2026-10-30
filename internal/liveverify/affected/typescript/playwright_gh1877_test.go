package typescript

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// outOfSuiteFixture is the GitHub #1877 shape: one Playwright spec that imports one support
// module, and application source that no spec imports.
func outOfSuiteFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `import { defineConfig } from "@playwright/test"
export default defineConfig({ testDir: "./tests", projects: [{ name: "chromium", use: { browserName: "chromium" } }] })
`)
	write(t, root, "tests/support/fixture.ts", `export const user = "u"`)
	write(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"
import { user } from "./support/fixture"
test("login", () => user)
`)
	write(t, root, "src/app.ts", `export const app = 1`)
	write(t, root, "src/routes/home.ts", `import { app } from "../app"; export const home = app`)
	return root
}

func writeUnreached(t *testing.T, root string, paths ...string) {
	t.Helper()
	write(t, root, PlaywrightUnreachedPathsPath, `{"profile":"`+PlaywrightUnreachedPathsProfile+`","paths":["`+strings.Join(paths, `","`)+`"]}`)
}

func outOfSuiteUnknown(plan PlaywrightPlan) (PlaywrightUnknown, bool) {
	for _, unknown := range plan.Unknown {
		if unknown.Reason == PlaywrightUnknownOutOfSuiteSource {
			return unknown, unknown.Axis == PlaywrightAxisSelection
		}
	}
	return PlaywrightUnknown{}, false
}

// TJAA-V0-106 (GitHub #1877, V1-2777): a dirty TypeScript/JavaScript path outside the Playwright
// closure is a SELECTION unknown that runs the full suite, never BOUNDED with zero selected.
func TestPlaywrightOutOfSuiteSourceWidens_GH1877(t *testing.T) {
	root := outOfSuiteFixture(t)
	reached, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"tests/support/fixture.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if reached.Scope != affected.ScopeBounded || !slices.Equal(playwrightSelectionIDs(reached), []string{"typescript:playwright:chromium:tests/login.spec.ts"}) {
		t.Fatalf("baseline selection: scope=%s selected=%v unknown=%+v", reached.Scope, playwrightSelectionIDs(reached), reached.Unknown)
	}
	for _, dirty := range [][]string{{"src/app.ts"}, {"src/routes/home.ts"}, {"src/app.ts", "tests/support/fixture.ts"}} {
		plan, err := selectPlaywrightStatic(root, "playwright.config.ts", dirty)
		if err != nil {
			t.Fatal(err)
		}
		unknown, selection := outOfSuiteUnknown(plan)
		if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || !selection || len(plan.Selected) != 1 || len(plan.Excluded) != 0 {
			t.Fatalf("%v: scope=%s fallback=%s selected=%d unknown=%+v", dirty, plan.Scope, plan.Fallback, len(plan.Selected), plan.Unknown)
		}
		if !strings.Contains(unknown.Detail, dirty[0]) || !strings.Contains(unknown.Detail, PlaywrightUnreachedPathsPath) {
			t.Fatalf("%v: detail %q", dirty, unknown.Detail)
		}
		if plan.Unreached != nil {
			t.Fatalf("%v: unreached record without a declaration: %+v", dirty, plan.Unreached)
		}
	}
}

// TJAA-V0-106, TJAA-V0-107: a dirty path the repository declares unreached, which the closure
// provably leaves out, is a bounded exclusion whose record and exclusions cite the declaration.
func TestPlaywrightUnreachedDeclarationExcludesAndCites_GH1877(t *testing.T) {
	root := outOfSuiteFixture(t)
	writeUnreached(t, root, "src/")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"src/app.ts", "src/routes/home.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone || len(plan.Selected) != 0 {
		t.Fatalf("declared unreached did not bound: scope=%s selected=%d unknown=%+v", plan.Scope, len(plan.Selected), plan.Unknown)
	}
	if _, found := outOfSuiteUnknown(plan); found {
		t.Fatalf("declared path still unknown: %+v", plan.Unknown)
	}
	record := plan.Unreached
	if record == nil || record.State != PlaywrightDeclarationApplied || record.Path != PlaywrightUnreachedPathsPath || len(record.SHA256) != 64 || !slices.Equal(record.Matched, []string{"src/app.ts", "src/routes/home.ts"}) {
		t.Fatalf("record %+v", record)
	}
	if len(plan.Excluded) != 1 || plan.Excluded[0].Unreached != PlaywrightUnreachedPathsPath {
		t.Fatalf("exclusions %+v", plan.Excluded)
	}
	// A declaration covering only one of two out-of-suite paths still widens for the other.
	writeUnreached(t, root, "src/routes/")
	partial, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"src/app.ts", "src/routes/home.ts"})
	if err != nil {
		t.Fatal(err)
	}
	unknown, selection := outOfSuiteUnknown(partial)
	if partial.Scope != affected.ScopeUnknown || !selection || !strings.HasPrefix(unknown.Detail, "src/app.ts ") || partial.Unreached == nil || !slices.Equal(partial.Unreached.Matched, []string{"src/routes/home.ts"}) {
		t.Fatalf("partial: scope=%s unknown=%+v record=%+v", partial.Scope, partial.Unknown, partial.Unreached)
	}
}

// TJAA-V0-106: a declared path the Playwright closure may load refuses the declaration, and a
// malformed declaration is an error rather than silently ignored.
func TestPlaywrightUnreachedDeclarationRefusals_GH1877(t *testing.T) {
	// A declared path the config imports reaches every spec, so it is selected, never excluded.
	root := outOfSuiteFixture(t)
	write(t, root, "playwright.config.ts", `import { defineConfig } from "@playwright/test"
import { port } from "./src/env"
export default defineConfig({ testDir: "./tests", projects: [{ name: "chromium", use: { browserName: "chromium" } }] })
`)
	write(t, root, "src/env.ts", `export const port = 4200`)
	writeUnreached(t, root, "src/")
	imported, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"src/env.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := outOfSuiteUnknown(imported); found || imported.Unreached != nil || len(imported.Selected) != 1 || len(imported.Excluded) != 0 {
		t.Fatalf("config import: scope=%s selected=%d unknown=%+v record=%+v", imported.Scope, len(imported.Selected), imported.Unknown, imported.Unreached)
	}
	// When closure membership is incomplete, a declared path may still be loaded: refuse.
	root = outOfSuiteFixture(t)
	write(t, root, "tests/login.spec.ts", `import { test } from "@playwright/test"
import { user } from "./support/fixture"
test("login", async () => { await import(process.env.EXTRA_MODULE); return user })
`)
	writeUnreached(t, root, "src/")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"src/app.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 {
		t.Fatalf("incomplete closure did not widen: scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
	unknown, selection := outOfSuiteUnknown(plan)
	if plan.Unreached == nil || plan.Unreached.State != PlaywrightDeclarationRefused || !selection || !strings.Contains(unknown.Detail, "Playwright closure may load it") {
		t.Fatalf("record %+v unknown %+v", plan.Unreached, plan.Unknown)
	}
	for _, body := range []string{`{"profile":"other/0","paths":["src/"]}`, `{"profile":"` + PlaywrightUnreachedPathsProfile + `","paths":["src/b.ts","src/a.ts"]}`, `{"profile":"` + PlaywrightUnreachedPathsProfile + `","paths":[".corvint/x.json"]}`, `{"profile":"` + PlaywrightUnreachedPathsProfile + `","paths":["src/"],"extra":1}`} {
		malformed := outOfSuiteFixture(t)
		write(t, malformed, PlaywrightUnreachedPathsPath, body)
		if _, err := selectPlaywrightStatic(malformed, "playwright.config.ts", []string{"src/app.ts"}); err == nil {
			t.Fatalf("malformed declaration accepted: %s", body)
		}
	}
}
