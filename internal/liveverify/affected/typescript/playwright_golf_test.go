package typescript

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Synthetic golf-e2e shape: this is not an observation of the consumer checkout.
func golfPlaywrightFixture(t *testing.T) string {
	t.Helper()
	root := qualificationCorpus(t)
	write(t, root, "playwright.config.ts", strings.Replace(qualificationConfig, "defineConfig({ projects:", `defineConfig({ use: { browserName: "chromium", baseURL: "http://localhost:4200", trace: "retain-on-failure" }, globalSetup: "./support/global-setup.ts", projects:`, 1))
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*"],"@scenarios/*":["support/scenario*"],"@fixtures":["support/shared.ts"],"@workflows/*":["support/workflow*"]}}}`)
	write(t, root, "support/global-setup.ts", `import { setup } from "./setup-helper"; export default setup`)
	write(t, root, "support/setup-helper.ts", `export const setup = () => {}`)
	write(t, root, "support/workflow0.ts", `export { value } from "@pages/0"`)
	write(t, root, "support/scenario0.ts", `export { value } from "@workflows/0"`)
	write(t, root, "support/page0.ts", `export { value } from "support/helper0"`)
	write(t, root, "tests/case000.spec.ts", `import { test } from "@fixtures"; import { value } from "@scenarios/0"; test("golf", () => value)`)
	return root
}

func TestPlaywrightGolfQualification(t *testing.T) {
	t.Run("TJAA-V0-017 global use aliases and narrow closure", func(t *testing.T) {
		root := golfPlaywrightFixture(t)
		for _, dirty := range []string{"support/helper0.ts", "support/page0.ts", "support/workflow0.ts", "support/scenario0.ts"} {
			plan := qualifySelection(t, root, dirty)
			if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone || !slices.Equal(playwrightSelectionIDs(plan), qualificationOracle(0, 13)) {
				t.Fatalf("%s: selected=%v unknown=%v", dirty, playwrightSelectionIDs(plan), plan.Unknown)
			}
			if !hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownExternalApplication) {
				t.Fatal("external application frontier lost")
			}
			first, _ := plan.Canonical()
			repeated := qualifySelection(t, root, dirty)
			second, _ := repeated.Canonical()
			if !bytes.Equal(first, second) {
				t.Fatal("receipt bytes differ")
			}
		}
		shared, err := New().Units(root)
		if err != nil || !slices.Contains(shared.Frontier, FrontierPathAlias) {
			t.Fatal("general adapter alias boundary changed", err)
		}
	})
	t.Run("TJAA-V0-013 setup helper and config closure", func(t *testing.T) {
		root := golfPlaywrightFixture(t)
		for _, dirty := range []string{"support/setup-helper.ts", "setup/global.setup.ts", "playwright.config.ts"} {
			plan := qualifySelection(t, root, dirty)
			if plan.Scope != affected.ScopeBounded || !slices.Equal(playwrightSelectionIDs(plan), qualificationOracle(0, 117)) {
				t.Fatalf("%s: selected=%d unknown=%v", dirty, len(plan.Selected), plan.Unknown)
			}
		}
	})
	t.Run("TJAA-V0-015 known widening frontiers", func(t *testing.T) {
		for _, row := range []struct{ name, file, body, reason string }{
			{"computed import", "support/page0.ts", `export const value = import(process.env.PAGE)`, FrontierDynamicImport},
			{"missing alias", "support/page0.ts", `export { value } from "@missing/page"`, FrontierPathAlias},
			{"missing target", "support/page0.ts", `export { value } from "@pages/missing"`, FrontierPathAlias},
			{"inherited tsconfig", "tsconfig.json", `{"extends":"./base.json","compilerOptions":{"baseUrl":"."}}`, FrontierPathAlias},
			{"equal prefix alias overlap", "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*"],"@pages/*0":["support/helper0"]}}}`, FrontierPathAlias},
			{"multiple existing targets", "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@pages/*":["support/page*","support/helper*"]}}}`, FrontierPathAlias},
			{"ambiguous target", "support/helper0.js", `export const value = 1`, FrontierPathAlias},
			{"dynamic global use", "playwright.config.ts", `export default { use: inheritedUse, projects: [{name:"chromium"},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownBrowserIdentity},
			{"dynamic setup", "playwright.config.ts", `export default { globalSetup: setupPath, projects: [{name:"chromium"},{name:"angular"},{name:"react"},{name:"setup"},{name:"cleanup"}] }`, PlaywrightUnknownConfigSyntax},
		} {
			t.Run(row.name, func(t *testing.T) {
				root := golfPlaywrightFixture(t)
				write(t, root, row.file, row.body)
				plan := qualifySelection(t, root, "support/helper0.ts")
				if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, row.reason) {
					t.Fatalf("frontier did not widen: %+v", plan.Unknown)
				}
				for _, required := range qualificationOracle(0, 117) {
					if !slices.Contains(playwrightSelectionIDs(plan), required) {
						t.Fatalf("unsafe exclusion %s", required)
					}
				}
			})
		}
	})
}

func TestPlaywrightGlobalUseInheritance(t *testing.T) {
	for _, row := range []struct{ global, local, browser, device string }{
		{`{ browserName: "firefox", trace: "on" }`, `{ locale: "en-US" }`, "firefox", ""},
		{`{ browserName: "firefox" }`, `{ browserName: "webkit" }`, "webkit", ""},
		{`{ browserName: "firefox" }`, `{ ...devices["Desktop Chrome"] }`, "firefox", "Desktop Chrome"},
		{`{}`, `{ browserName: "firefox", ...devices["Desktop Chrome"] }`, "firefox", "Desktop Chrome"},
		{`{ ...devices["Desktop Firefox"] }`, `{ ...devices["Desktop Chrome"] }`, "chromium", "Desktop Chrome"},
		{`{ ...devices["Desktop Firefox"] }`, `{ browserName: "webkit" }`, "webkit", "Desktop Firefox"},
	} {
		browser, device, ok := playwrightInheritedUseIdentity(row.global, row.local)
		if !ok || browser != row.browser || device != row.device {
			t.Fatalf("%+v: %s %s %v", row, browser, device, ok)
		}
	}
}

func TestPlaywrightAliasResolutionBoundaries(t *testing.T) {
	t.Run("TJAA-V0-012 declared aliases remain closed", func(t *testing.T) {
		bodies := map[string]string{"src/exact.ts": "", "src/wild/x.ts": "", "a/local.ts": "", "src/fallback.ts": ""}
		config := parseTypeScriptAliases("tsconfig.json", `{
  // The source observer admits JSONC without executing configuration.
  "compilerOptions": { "baseUrl": ".", "paths": {
    "@x": ["src/exact.ts"], "@*": ["src/wild/*"], "fallback": ["missing", "src/fallback"],
  }, },
}`)
		if !config.valid {
			t.Fatal("JSONC rejected")
		}
		for _, raw := range []string{
			`{"compilerOptions":{"paths":{"@*t":["src/exact.ts"],"@*act":["src/fallback.ts"]}}}`,
			`{"compilerOptions":{"paths":{"@*act":["src/fallback.ts"],"@*t":["src/exact.ts"]}}}`,
			`{"compilerOptions":{"paths":{"@exact":["src/exact.ts","src/fallback.ts"]}}}`,
		} {
			ambiguous := parseTypeScriptAliases("tsconfig.json", raw)
			if got, unknown := ambiguous.resolve("@exact", bodies); got != "" || !unknown {
				t.Fatalf("ambiguous alias narrowed: %s -> %s, unknown=%v", raw, got, unknown)
			}
		}
		for _, row := range []struct{ ref, want string }{{"@x", "src/exact.ts"}, {"@x/x", ""}, {"fallback", "src/fallback.ts"}, {"src/exact", "src/exact.ts"}} {
			got, _ := config.resolve(row.ref, bodies)
			if got != row.want {
				t.Fatalf("%s: got %s want %s", row.ref, got, row.want)
			}
		}
		child := parseTypeScriptAliases("a/tsconfig.json", `{"compilerOptions":{"paths":{"@x":["local.ts"]}}}`)
		frontier := map[string]bool{}
		got := resolveTypeScriptAliases("a/example.spec.ts", []string{"@x"}, []typeScriptAliases{config, child}, bodies, frontier)
		if !slices.Equal(got, []string{"./local.ts"}) || len(frontier) != 0 {
			t.Fatalf("nearest config: %v %v", got, frontier)
		}
		for _, raw := range []string{
			`{"compilerOptions":{"baseUrl":"../outside"}}`,
			`{"compilerOptions":{"paths":{"@x":["../../outside"]}}}`,
			`{"compilerOptions":{"paths":{"@x":["x"],"@x":["y"]}}}`,
			`{"extends":"./base.json"}`,
			`{"compilerOptions":{"rootDirs":["src","generated"]}}`,
			`{"compilerOptions":{"moduleSuffixes":[".native",""]}}`,
		} {
			if parseTypeScriptAliases("tsconfig.json", raw).valid {
				t.Fatalf("unsupported config accepted: %s", raw)
			}
		}
	})
}
