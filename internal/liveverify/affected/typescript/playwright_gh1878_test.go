package typescript

import (
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func informationalReasons(plan PlaywrightPlan) []string {
	var reasons []string
	for _, entry := range plan.Informational {
		if entry.Axis != PlaywrightAxisInformational || !strings.Contains(entry.Detail, "cannot change this selection") {
			return []string{"malformed:" + entry.Reason}
		}
		reasons = append(reasons, entry.Reason)
	}
	return reasons
}

func assertBoundedLogin(t *testing.T, label string, plan PlaywrightPlan) {
	t.Helper()
	if plan.Scope != affected.ScopeBounded || plan.Fallback != PlaywrightFallbackNone || !slices.Equal(playwrightSelectionIDs(plan), []string{"typescript:playwright:chromium:tests/login.spec.ts"}) {
		t.Fatalf("%s: scope=%s fallback=%s selected=%v unknown=%+v informational=%+v", label, plan.Scope, plan.Fallback, playwrightSelectionIDs(plan), plan.Unknown, plan.Informational)
	}
}

func assertFullSuite(t *testing.T, label string, plan PlaywrightPlan) {
	t.Helper()
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Excluded) != 0 {
		t.Fatalf("%s: scope=%s fallback=%s unknown=%+v informational=%+v", label, plan.Scope, plan.Fallback, plan.Unknown, plan.Informational)
	}
}

// TJAA-V0-108, TJAA-V0-109 (GitHub #1878, V1-2778): a frontier site in a unit outside the
// Playwright closure that no dirty path reaches is informational and never forces
// FULL_RELEVANT_SUITE; the same site widens once it is dirty, in the closure, or in a unit on a
// reverse-reachability path from a dirty file.
func TestPlaywrightFrontierOutsideClosureIsInformational_GH1878(t *testing.T) {
	dirty := []string{"tests/support/fixture.ts"}
	root := outOfSuiteFixture(t)
	write(t, root, "src/plugins.ts", `export const load = (name: string) => import(name)`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "outside closure", plan)
	if got := informationalReasons(plan); !slices.Equal(got, []string{FrontierDynamicImport}) || !strings.Contains(plan.Informational[0].Detail, "src/plugins.ts:1") {
		t.Fatalf("informational %+v", plan.Informational)
	}

	// Dirty site file: relevant.
	plan, err = selectPlaywrightStatic(root, "playwright.config.ts", append([]string{"src/plugins.ts"}, dirty...))
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "dirty site", plan)
	if len(plan.Informational) != 0 {
		t.Fatalf("dirty site still informational: %+v", plan.Informational)
	}

	// Site in a unit that imports the dirty helper (reverse-reachable from a dirty file): relevant.
	reverse := outOfSuiteFixture(t)
	write(t, reverse, "src/plugins.ts", `import { user } from "../tests/support/fixture"
export const load = (name: string) => import(name + user)`)
	plan, err = selectPlaywrightStatic(reverse, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "reverse reachable", plan)
	if len(plan.Informational) != 0 {
		t.Fatalf("reverse-reachable site still informational: %+v", plan.Informational)
	}

	// Site in a module a spec loads (in the closure): relevant.
	inside := outOfSuiteFixture(t)
	write(t, inside, "tests/support/fixture.ts", `export const user = "u"
export const load = (name: string) => import(name)`)
	plan, err = selectPlaywrightStatic(inside, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "in closure", plan)
}

// TJAA-V0-108: the over-bound source frontier widens a Playwright plan only when its file can
// change the answer, and the AFP-V0-444 site stays reported as informational otherwise.
func TestPlaywrightOverBoundSourceScopedToClosure_GH1878(t *testing.T) {
	dirty := []string{"tests/support/fixture.ts"}
	root := outOfSuiteFixture(t)
	write(t, root, "src/generated/big.ts", "export const big = \""+strings.Repeat("x", affected.MaxSourceBytes+1)+"\"\n")
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "over-bound outside closure", plan)
	if got := informationalReasons(plan); !slices.Equal(got, []string{FrontierOverBoundSource}) || !strings.Contains(plan.Informational[0].Detail, "src/generated/big.ts") || !strings.Contains(plan.Informational[0].Detail, affected.GeneratedPathsPath) {
		t.Fatalf("informational %+v", plan.Informational)
	}
	write(t, root, "tests/support/fixture.ts", `import { big } from "../../src/generated/big"
export const user = "u" + big`)
	plan, err = selectPlaywrightStatic(root, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "over-bound imported by a test", plan)
}

// TJAA-V0-108: an unowned config-like site widens only in the root or in an ancestor directory of
// the Playwright config or of a closure file; a sibling package's tsconfig is informational.
func TestPlaywrightConfigFrontierScopedToAncestors_GH1878(t *testing.T) {
	dirty := []string{"tests/support/fixture.ts"}
	root := outOfSuiteFixture(t)
	write(t, root, "packages/other/tsconfig.json", `{"compilerOptions":{"rootDirs":["src","gen"]}}`)
	write(t, root, "packages/other/src/index.ts", `export const other = 1`)
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertBoundedLogin(t, "sibling tsconfig", plan)
	if len(plan.Informational) == 0 || !strings.Contains(plan.Informational[0].Detail, "packages/other/tsconfig.json") {
		t.Fatalf("informational %+v", plan.Informational)
	}
	ancestor := outOfSuiteFixture(t)
	write(t, ancestor, "tsconfig.json", `{"compilerOptions":{"rootDirs":["src","gen"]}}`)
	plan, err = selectPlaywrightStatic(ancestor, "playwright.config.ts", dirty)
	if err != nil {
		t.Fatal(err)
	}
	assertFullSuite(t, "root tsconfig", plan)
	if len(plan.Informational) != 0 {
		t.Fatalf("root tsconfig informational: %+v", plan.Informational)
	}
}
