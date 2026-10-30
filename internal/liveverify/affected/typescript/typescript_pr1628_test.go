package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// pr1628RegisterConfig calls tsconfig-paths' register inside a non-identity `use` value, which
// changes runtime module resolution while the static graph still follows tsconfig.json.
func pr1628RegisterConfig(trace string) string {
	return "import { defineConfig, devices } from '@playwright/test';\nimport { register } from 'tsconfig-paths';\nexport default defineConfig({\n  testDir: './tests',\n  use: { trace: " + trace + " },\n  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],\n});\n"
}

// TJAA-V0-054 (PR #1628 review P1): a non-identity value that can run code keeps the recovered
// browser identity but is a selection-axis unknown, so a change the static graph does not connect
// (alternate/helper.ts, which register maps lib/* to) cannot exclude the importing spec under a
// bounded scope. A provably side-effect-free value is still ignored entirely.
func TestPlaywrightSideEffectUseValueKeepsSelectionUnknown_PR1628(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@playwright/test":"1.61.0","tsconfig-paths":"4"}}`)
	write(t, root, "tsconfig.json", `{ "compilerOptions": { "baseUrl": ".", "paths": { "lib/*": ["./lib/*"] } } }`)
	write(t, root, "lib/helper.ts", "export const h = 1;\n")
	write(t, root, "alternate/helper.ts", "export const h = 2;\n")
	write(t, root, "tests/a.spec.ts", "import { test } from '@playwright/test';\nimport { h } from 'lib/helper';\ntest('a', async () => { void h; });\n")
	write(t, root, "tests/b.spec.ts", "import { test } from '@playwright/test';\ntest('b', async () => {});\n")
	write(t, root, "playwright.config.ts", pr1628RegisterConfig("(register({ baseUrl: '.', paths: { 'lib/*': ['alternate/*'] } }), 'on')"))
	plan, err := selectPlaywrightStatic(root, "playwright.config.ts", []string{"alternate/helper.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeUnknown || plan.Fallback != PlaywrightFallbackFullSuite || !hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownUseValueUnverified) {
		t.Fatalf("side-effecting use value left the plan bounded: scope=%s fallback=%s unknown=%v", plan.Scope, plan.Fallback, plan.Unknown)
	}
	if hasPlaywrightUnknown(plan, PlaywrightAxisExecution, PlaywrightUnknownUseValueUnverified) || hasPlaywrightUnknown(plan, PlaywrightAxisSelection, PlaywrightUnknownBrowserIdentity) {
		t.Fatalf("unknown on the wrong axis or identity lost: %v", plan.Unknown)
	}
	if chromium := playwrightProject(plan, "chromium"); chromium.Browser != "chromium" || chromium.Device != "Desktop Chrome" {
		t.Fatalf("recovered identity lost: %+v", chromium)
	}
	for _, effect := range []string{"(globalThis.x = 1, 'on')", "(counter++, 'on')", "require('./hook') && 'on'"} {
		_, _, unknown := parsePlaywrightConfig("playwright.config.ts", "let counter = 0;\n"+pr1628RegisterConfig(effect))
		if !slices.ContainsFunc(unknown, func(entry PlaywrightUnknown) bool {
			return entry.Reason == PlaywrightUnknownUseValueUnverified && entry.Axis == PlaywrightAxisSelection
		}) {
			t.Fatalf("%s: no selection unknown: %+v", effect, unknown)
		}
	}
	// The value does not decide file membership, so a discovery listing check still reads the
	// project set, as it does for an unresolved browser identity.
	if projects, _, err := staticPlaywrightProjects("playwright.config.ts", []byte(pr1628RegisterConfig("(register({}), 'on')")), false); err != nil || len(projects) != 1 {
		t.Fatalf("listing check refused: %v %+v", err, projects)
	}
	// A side-effect-free value is ignored: the plan stays bounded.
	write(t, root, "playwright.config.ts", pr1628RegisterConfig("'on'"))
	plan, err = selectPlaywrightStatic(root, "playwright.config.ts", []string{"lib/helper.ts"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Scope != affected.ScopeBounded || len(plan.Selected) != 1 || plan.Selected[0].Test != "tests/a.spec.ts" {
		t.Fatalf("pure value: scope=%s selected=%+v unknown=%v", plan.Scope, plan.Selected, plan.Unknown)
	}
}

// TJAA-V0-060 (PR #1628 review P2): a `delete`, `++` or `--` confined to a non-identity `use`
// value that names no `devices` does not refuse the devices root, so the device spread keeps its
// identity and the mutating value is the selection unknown instead. A mutation outside such a
// value, one beside a `devices` occurrence, or any `\` escape still refuses every root.
func TestPlaywrightConfinedMutationKeepsDeviceIdentity_PR1628(t *testing.T) {
	config := func(setup, global string) string {
		return "import { defineConfig, devices } from '@playwright/test';\nlet counter = 0;\n" + setup + "\nexport default defineConfig({\n  use: " + global + ",\n  projects: [{ name: 'p', use: { ...devices['Desktop Chrome'] } }, { name: 'q', use: { ...devices['Desktop Firefox'], actionTimeout: counter-- } }],\n});\n"
	}
	for _, global := range []string{"{ navigationTimeout: counter++ }", "{ trace: (delete globalThis.x, 'on') }", "{ baseURL: process.env.URL, navigationTimeout: ++counter }"} {
		projects, _, unknown := parsePlaywrightConfig("playwright.config.ts", config("", global))
		if len(projects) != 2 || projects[0].Browser != "chromium" || projects[0].Device != "Desktop Chrome" || projects[1].Browser != "firefox" || hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("%s: confined mutation blocked identity: projects=%+v unknown=%+v", global, projects, unknown)
		}
		if !slices.ContainsFunc(unknown, func(entry PlaywrightUnknown) bool {
			return entry.Reason == PlaywrightUnknownUseValueUnverified && entry.Axis == PlaywrightAxisSelection && strings.HasPrefix(entry.Detail, "q use value") && strings.Contains(entry.Detail, "actionTimeout is not a side-effect-free value")
		}) {
			t.Fatalf("%s: mutating value is not a selection unknown: %+v", global, unknown)
		}
	}
	for _, row := range []struct{ setup, global string }{
		{"counter++;", "{ navigationTimeout: 1 }"},
		{"function f() { delete devices['Desktop Chrome'].defaultBrowserType; }", "{ navigationTimeout: 1 }"},
		{"", "{ trace: (delete devices['Desktop Chrome'].defaultBrowserType, 'on') }"},
		{"", "{ trace: (counter++, devices['Desktop Chrome'].userAgent) }"},
		{"", "{ trace: (counter++, devic\\u0065s) }"},
		{"", "{ viewport: { width: counter++ } }"},
		{"", "{ [counter++]: 1 }"},
	} {
		if _, _, unknown := parsePlaywrightConfig("playwright.config.ts", config(row.setup, row.global)); !hasPlaywrightUnknownReason(unknown, PlaywrightUnknownBrowserIdentity) {
			t.Fatalf("%q / %s was admitted: %+v", row.setup, row.global, unknown)
		}
	}
}

// TJAA-V0-059 (PR #1628 review P2): under a blocked alias gate (a Jest config here) a bare import
// of a workspace package that tsconfig also maps keeps its workspace link beside the speculative
// tsconfig edge, so a change inside the package still selects the importing test.
func TestBlockedGateKeepsWorkspaceLinks_PR1628(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"private":true,"workspaces":["packages/*"],"devDependencies":{"jest":"29"}}`)
	write(t, root, "jest.config.js", "module.exports = {};\n")
	write(t, root, "tsconfig.json", `{ "compilerOptions": { "baseUrl": ".", "paths": { "@acme/shared": ["types/shared.ts"], "lib/*": ["lib/*"] } } }`)
	write(t, root, "packages/shared/package.json", `{"name":"@acme/shared"}`)
	write(t, root, "packages/shared/index.ts", "export * from './runtime';\n")
	write(t, root, "packages/shared/runtime.ts", "export const r = 1;\n")
	write(t, root, "types/shared.ts", "export type S = {};\n")
	write(t, root, "lib/util.ts", "export const u = 1;\n")
	write(t, root, "app/shared.test.ts", "import { r } from '@acme/shared';\nimport { u } from 'lib/util';\ntest('s', () => { void r; void u; });\n")
	write(t, root, "app/other.test.ts", "test('o', () => {});\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"packages/shared/runtime.ts", "types/shared.ts", "lib/util.ts"} {
		plan := affected.Select(graph, []string{dirty})
		if got := selectedTests(plan); !slices.Equal(got, []string{"app/shared.test.ts"}) || plan.Scope != affected.ScopeUnknown {
			t.Fatalf("%s: selected %v scope %s (unknown %v)", dirty, got, plan.Scope, plan.Unknown)
		}
	}
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(result.Frontier, func(entry string) bool { return strings.Contains(entry, "app/shared.test.ts") }) {
		t.Fatalf("a resolved or linked import is a site: %v", result.Frontier)
	}
}
