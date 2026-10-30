package typescript

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

// narrowingFixture is the GitHub #1294 shape: a shared test-support module imported by one spec,
// a statically resolved project, and a project entry whose own membership is dynamic.
func narrowingFixture(t *testing.T, mobile string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0"}}`)
	write(t, root, "playwright.config.ts", `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: "./tests",
  projects: [
    { name: "setup", testMatch: /.*\.setup\.ts/ },
    { name: "chromium", testIgnore: /.*\.setup\.ts/, dependencies: ["setup"] },
    `+mobile+`
  ],
})
`)
	write(t, root, "tests/support/fixture.ts", `export const user = "u"`)
	write(t, root, "tests/login.spec.ts", `
import { test } from "@playwright/test"
import { user } from "./support/fixture"
test("login", () => user)
`)
	write(t, root, "tests/cart.spec.ts", `import { test } from "@playwright/test"; test("cart", () => {})`)
	write(t, root, "tests/auth.setup.ts", `import { test } from "@playwright/test"; test("setup", () => {})`)
	write(t, root, "src/app.ts", `export const app = 1`)
	return root
}

const narrowingDynamicMobile = `{ name: "mobile", testDir: process.env.MOBILE_DIR, dependencies: ["setup"] },`

func narrowingCandidates(narrowing *PlaywrightNarrowing) []string {
	ids := []string{}
	for _, candidate := range narrowing.Candidates {
		ids = append(ids, candidate.Project+" "+candidate.Test)
	}
	return ids
}

func narrowingMemberships(narrowing *PlaywrightNarrowing) []string {
	memberships := []string{}
	for _, project := range narrowing.Projects {
		memberships = append(memberships, project.Name+" "+project.Membership)
	}
	return memberships
}

func assertNotNarrowed(t *testing.T, plan PlaywrightPlan, reason string) {
	t.Helper()
	if plan.Fallback != PlaywrightFallbackFullSuite || plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNotNarrowed || plan.Narrowed.Reason != reason || plan.Narrowed.Detail == "" || len(plan.Narrowed.Candidates) != 0 || len(plan.Narrowed.Projects) != 0 {
		t.Fatalf("want NOT_NARROWED %s, got fallback %s narrowed %+v", reason, plan.Fallback, plan.Narrowed)
	}
}

func TestPlaywrightNarrowingSharedFixture_TJAA_V0_030(t *testing.T) {
	root := narrowingFixture(t, narrowingDynamicMobile)
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The fallback and its claim are unchanged: zero selected units and FULL_RELEVANT_SUITE.
	if plan.Scope != "UNKNOWN" || plan.Fallback != PlaywrightFallbackFullSuite || len(plan.Selected) != 0 || len(plan.Excluded) != 0 {
		t.Fatalf("fallback changed: scope %s fallback %s selected %v", plan.Scope, plan.Fallback, playwrightSelectionIDs(plan))
	}
	narrowing := plan.Narrowed
	if narrowing == nil || narrowing.State != PlaywrightNarrowed || narrowing.Basis != PlaywrightNarrowingBasis || narrowing.Reason != "" {
		t.Fatalf("narrowing: %+v", narrowing)
	}
	if got, want := narrowingCandidates(narrowing), []string{"chromium tests/login.spec.ts", "setup tests/auth.setup.ts"}; !slices.Equal(got, want) {
		t.Fatalf("candidates %v, want %v", got, want)
	}
	if got, want := narrowingMemberships(narrowing), []string{"chromium STATIC_CONFIG", "mobile UNRESOLVED", "setup STATIC_CONFIG"}; !slices.Equal(got, want) {
		t.Fatalf("memberships %v, want %v", got, want)
	}
	for _, project := range narrowing.Projects {
		if project.Name == "mobile" && (project.Detail == "" || !slices.Contains(project.Argv, "--project=mobile")) {
			t.Fatalf("unresolved project must keep its argv and detail: %+v", project)
		}
	}
	if len(narrowing.Symbols) != 1 || narrowing.Symbols[0].State != PlaywrightSymbolsUnresolved || narrowing.Symbols[0].Path != "tests/support/fixture.ts" {
		t.Fatalf("symbols without a base reader: %+v", narrowing.Symbols)
	}

	first, err := json.Marshal(plan, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	again, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(again, json.Deterministic(true))
	if err != nil || string(first) != string(second) {
		t.Fatalf("plan bytes are not deterministic: %v", err)
	}
}

func TestPlaywrightNarrowingAbsentFromBoundedPlan_TJAA_V0_030(t *testing.T) {
	root := narrowingFixture(t, `{ name: "mobile", testIgnore: /.*\.setup\.ts/, dependencies: ["setup"] },`)
	plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, discoveryFixtureBytes(t, root, []PlaywrightDiscoveryUnit{
		{Project: "chromium", Test: "tests/cart.spec.ts"}, {Project: "chromium", Test: "tests/login.spec.ts"},
		{Project: "mobile", Test: "tests/cart.spec.ts"}, {Project: "mobile", Test: "tests/login.spec.ts"},
		{Project: "setup", Test: "tests/auth.setup.ts"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fallback == PlaywrightFallbackFullSuite || plan.Narrowed != nil {
		t.Fatalf("a bounded plan must not carry narrowing: fallback %s narrowed %+v discovery %+v", plan.Fallback, plan.Narrowed, plan.Discovery)
	}
	raw, err := json.Marshal(plan, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]jsontext.Value
	if err := json.Unmarshal(raw, &members); err != nil {
		t.Fatal(err)
	}
	if _, present := members["narrowed"]; present {
		t.Fatalf("bounded plan bytes carry narrowed: %s", raw)
	}
}

func TestPlaywrightNarrowingRefusals_TJAA_V0_031(t *testing.T) {
	for _, testCase := range []struct {
		name, mobile, reason string
		dirty                []string
		extra                map[string]string
	}{
		{name: "application module", mobile: narrowingDynamicMobile, reason: PlaywrightNarrowingNotTestSide, dirty: []string{"src/app.ts"}},
		{name: "fixture beside application module", mobile: narrowingDynamicMobile, reason: PlaywrightNarrowingNotTestSide, dirty: []string{"src/app.ts", "tests/support/fixture.ts"}},
		{name: "support module an application module imports", mobile: narrowingDynamicMobile, reason: PlaywrightNarrowingNotTestSide, dirty: []string{"tests/support/fixture.ts"},
			extra: map[string]string{"src/app.ts": `import { user } from "../tests/support/fixture"; export const app = user`}},
		{name: "unowned path", mobile: narrowingDynamicMobile, reason: PlaywrightNarrowingNotTestSide, dirty: []string{"README.md"}},
		{name: "global config unknown", mobile: `{ name: "mobile", dependencies: ["setup"] },`, reason: PlaywrightNarrowingConfigUnresolved, dirty: []string{"tests/support/fixture.ts"},
			extra: map[string]string{"playwright.config.ts": `
import { defineConfig } from "@playwright/test"
export default defineConfig({
  testDir: process.env.TEST_DIR,
  projects: [{ name: "chromium" }],
})
`}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := narrowingFixture(t, testCase.mobile)
			for relative, body := range testCase.extra {
				write(t, root, relative, body)
			}
			plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, testCase.dirty, nil)
			if err != nil {
				t.Fatal(err)
			}
			assertNotNarrowed(t, plan, testCase.reason)
		})
	}
	t.Run("dirty config", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"playwright.config.ts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertNotNarrowed(t, plan, PlaywrightNarrowingNotTestSide)
	})
}

func TestPlaywrightNarrowingPerProject_TJAA_V0_032(t *testing.T) {
	t.Run("unknown dependency reference stays scoped to its project", func(t *testing.T) {
		root := narrowingFixture(t, `{ name: "mobile", dependencies: ["missing"] },`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
			t.Fatalf("narrowing: fallback %s %+v", plan.Fallback, plan.Narrowed)
		}
		if got := narrowingMemberships(plan.Narrowed); !slices.Contains(got, "mobile UNRESOLVED") || !slices.Contains(got, "chromium STATIC_CONFIG") {
			t.Fatalf("memberships %v", got)
		}
		if got := narrowingCandidates(plan.Narrowed); !slices.Contains(got, "chromium tests/login.spec.ts") || slices.ContainsFunc(got, func(id string) bool { return id[:7] == "mobile " }) {
			t.Fatalf("candidates %v", got)
		}
	})
	t.Run("test-side dynamic import taints the tests that reach it", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		write(t, root, "tests/support/loader.ts", `export const load = (name: string) => import("./" + name)`)
		write(t, root, "tests/cart.spec.ts", `
import { test } from "@playwright/test"
import { load } from "./support/loader"
test("cart", () => load("fixture"))
`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
			t.Fatalf("narrowing: %+v", plan.Narrowed)
		}
		kinds := map[string]string{}
		for _, candidate := range plan.Narrowed.Candidates {
			kinds[candidate.Project+" "+candidate.Test] = candidate.Witness.Kind
		}
		if kinds["chromium tests/cart.spec.ts"] != playwrightWitnessDynamicImport || kinds["chromium tests/login.spec.ts"] == "" || kinds["chromium tests/login.spec.ts"] == playwrightWitnessDynamicImport {
			t.Fatalf("candidate witnesses %v", kinds)
		}
	})
	// TJAA-V0-108..110: a dynamic import in application code a test loads is a graph frontier;
	// one outside the Playwright closure that no dirty path reaches is informational and does not
	// block narrowing.
	t.Run("dynamic import in application code a test loads widens", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		write(t, root, "src/app.ts", `export const app = (name: string) => import("./" + name)`)
		write(t, root, "tests/cart.spec.ts", `import { test } from "@playwright/test"; import { app } from "../src/app"; test("cart", () => app)`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Fallback != PlaywrightFallbackFullSuite || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, FrontierDynamicImport) || len(plan.Informational) != 0 {
			t.Fatalf("closure dynamic import did not widen: %+v informational %+v", plan.Unknown, plan.Informational)
		}
	})
	t.Run("dynamic import outside the Playwright closure is informational", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		write(t, root, "src/app.ts", `export const app = (name: string) => import("./" + name)`)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
			t.Fatalf("narrowing %+v", plan.Narrowed)
		}
		if len(plan.Informational) != 1 || plan.Informational[0].Reason != FrontierDynamicImport || plan.Informational[0].Axis != PlaywrightAxisInformational || !strings.Contains(plan.Informational[0].Detail, "src/app.ts:1") {
			t.Fatalf("informational %+v", plan.Informational)
		}
		if hasPlaywrightUnknown(plan, PlaywrightAxisSelection, FrontierDynamicImport) || hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownDynamicSource) {
			t.Fatalf("informational frontier widened: %+v", plan.Unknown)
		}
	})
	listed := []PlaywrightDiscoveryUnit{
		{Project: "chromium", Test: "tests/cart.spec.ts"}, {Project: "chromium", Test: "tests/login.spec.ts"},
		{Project: "setup", Test: "tests/auth.setup.ts"},
	}
	t.Run("bound discovery confirms matching projects", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, discoveryFixtureBytes(t, root, listed))
		if err != nil {
			t.Fatal(err)
		}
		if plan.Fallback != PlaywrightFallbackFullSuite || plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
			t.Fatalf("discovery %+v narrowing %+v", plan.Discovery, plan.Narrowed)
		}
		if got, want := narrowingMemberships(plan.Narrowed), []string{"chromium DISCOVERY_MATCHED", "mobile UNRESOLVED", "setup DISCOVERY_MATCHED"}; !slices.Equal(got, want) {
			t.Fatalf("memberships %v, want %v (discovery %+v)", got, want, plan.Discovery)
		}
		if got := narrowingCandidates(plan.Narrowed); !slices.Contains(got, "chromium tests/login.spec.ts") {
			t.Fatalf("candidates %v", got)
		}
	})
	t.Run("bound discovery that differs unresolves the project", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		differs := []PlaywrightDiscoveryUnit{{Project: "chromium", Test: "tests/login.spec.ts"}, {Project: "setup", Test: "tests/auth.setup.ts"}}
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, discoveryFixtureBytes(t, root, differs))
		if err != nil {
			t.Fatal(err)
		}
		if plan.Narrowed == nil || plan.Narrowed.State != PlaywrightNarrowed {
			t.Fatalf("narrowing %+v", plan.Narrowed)
		}
		if got, want := narrowingMemberships(plan.Narrowed), []string{"chromium UNRESOLVED", "mobile UNRESOLVED", "setup DISCOVERY_MATCHED"}; !slices.Equal(got, want) {
			t.Fatalf("memberships %v, want %v", got, want)
		}
		if got := narrowingCandidates(plan.Narrowed); slices.ContainsFunc(got, func(id string) bool { return id[:9] == "chromium " }) {
			t.Fatalf("an unresolved project kept candidates: %v", got)
		}
	})
	t.Run("bound discovery listing an undeclared project", func(t *testing.T) {
		root := narrowingFixture(t, narrowingDynamicMobile)
		extra := append(slices.Clone(listed), PlaywrightDiscoveryUnit{Project: "webkit", Test: "tests/login.spec.ts"})
		plan, err := SelectPlaywright(root, "playwright.config.ts", discoveryFixtureRevision, []string{"tests/support/fixture.ts"}, discoveryFixtureBytes(t, root, extra))
		if err != nil {
			t.Fatal(err)
		}
		if plan.Fallback != PlaywrightFallbackFullSuite || plan.Narrowed == nil {
			t.Fatalf("discovery %+v narrowing %+v", plan.Discovery, plan.Narrowed)
		}
		if plan.Narrowed.State != PlaywrightNotNarrowed || plan.Narrowed.Reason != PlaywrightNarrowingDiscoveryProjects {
			t.Fatalf("discovery %+v narrowing %+v", plan.Discovery, plan.Narrowed)
		}
	})
}
