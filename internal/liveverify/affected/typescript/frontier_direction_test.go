// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TJAA-V0-1161 (AZU-V0-003, AZU-V0-008): every frontier reason has a
// direction, and only the reasons whose hidden edge starts at a reference in
// the site's own file are OUTBOUND or DISCOVERY; the runner reasons that hide
// only how a known test runs are PACKAGE_RUNNABLE; everything else, the
// resolution-table alias included, is INBOUND.
func TestFrontierDirections_TJAA_V0_1161(t *testing.T) {
	want := map[string]string{
		FrontierUnresolvedImport:       affected.FrontierOutbound,
		FrontierBareImportUndeclared:   affected.FrontierOutbound,
		FrontierDynamicImport:          affected.FrontierOutbound,
		FrontierConfig:                 affected.FrontierDiscovery,
		FrontierRuntimeFlags:           affected.FrontierPackageRunnable,
		FrontierUnknownRunner:          affected.FrontierPackageRunnable,
		FrontierPathAlias:              affected.FrontierInbound,
		FrontierE2ERuntimeDependency:   affected.FrontierInbound,
		FrontierAmbiguousRunner:        affected.FrontierInbound,
		FrontierPuppeteerHost:          affected.FrontierInbound,
		FrontierDetoxConfiguration:     affected.FrontierInbound,
		FrontierStorybookAddressing:    affected.FrontierInbound,
		FrontierCrossLanguageTestAsset: affected.FrontierInbound,
		FrontierUnreadableSource:       affected.FrontierInbound,
		FrontierUnparsedSource:         affected.FrontierInbound,
		FrontierScanUnresolved:         affected.FrontierInbound,
		FrontierOverBoundSource:        affected.FrontierInbound,
		"typescript:never-declared":    affected.FrontierInbound,
	}
	for reason, direction := range want {
		if got := New().FrontierDirection(reason); got != direction {
			t.Errorf("%s: %s, want %s", reason, got, direction)
		}
	}
}

// TJAA-V0-1161 (AZU-V0-008): a test unit of an identified runner is runnable;
// a source unit and a unit of the unknown runner are not.
func TestRunnableUnits_TJAA_V0_1161(t *testing.T) {
	for id, runnable := range map[string]bool{
		unitID(runnerVitest, "tests/a.test.ts"):    true,
		unitID(runnerNode, "extensions/t.test.ts"): true,
		unitID(runnerPlaywright, "e2e/a.spec.ts"):  true,
		unitID(runnerStorybook, "stories"):         true,
		unitID(runnerUnknown, "tests/mystery.ts"):  false,
		unitID("", "src/app.ts"):                   false,
		unitID("", "tests/a.test.ts"):              false,
		"go:example.com/pkg":                       false,
		"typescript:":                              false,
	} {
		if got := New().RunnableUnit(id); got != runnable {
			t.Errorf("%s: runnable %v, want %v", id, got, runnable)
		}
	}
}

// TJAA-V0-1161 (AZU-V0-003, AZU-V0-212): Result.FrontierSites records every
// located site of every plugin-wide reason, so the per-reason count equals the
// graph's frontierSites count, including the sites a `reason:+N` entry hides;
// a `+N` entry therefore closes by selecting every hidden site's unit.
func TestFrontierSiteRecordIsComplete_TJAA_V0_1161(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/app.ts", "export const app = 1\n")
	write(t, root, "tests/app.test.ts", "import { test } from \"vitest\"\nimport { app } from \"../src/app\"\n")
	for index := range frontierSiteBound + 4 {
		name := "tests/bare" + string(rune('a'+index/26)) + string(rune('a'+index%26)) + ".test.ts"
		write(t, root, name, "import { test } from \"vitest\"\nimport { x } from \"nowhere-"+name+"\"\n")
	}
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	recorded := result.FrontierSites[FrontierBareImportUndeclared]
	if len(recorded) != frontierSiteBound+4 {
		t.Fatalf("recorded %d bare-import sites, want %d: %v", len(recorded), frontierSiteBound+4, recorded)
	}
	overflow := FrontierBareImportUndeclared + ":+4"
	if !slices.Contains(result.Frontier, overflow) {
		t.Fatalf("frontier %v lacks %s", result.Frontier, overflow)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/app.ts"})
	if plan.FrontierSites[FrontierBareImportUndeclared] != len(recorded) {
		t.Errorf("frontierSites %v, want %d bare-import sites", plan.FrontierSites, len(recorded))
	}
	if got := frontierDetails(plan); len(got) != 0 {
		t.Errorf("located OUTBOUND sites stayed unknown: %v", got)
	}
	if selected := plan.SelectedTests(); len(selected) != frontierSiteBound+5 {
		t.Errorf("selected %d tests, want the importer and every bare-import site: %v", len(selected), selected)
	}
	if plan.Scope != affected.ScopeBounded {
		t.Errorf("scope %s", plan.Scope)
	}
}

// TJAA-V0-1161 (V1-4999): a fixture text copy of a runner configuration, such
// as `playwright.config.js.txt`, is no configuration: no runner loads it, so it
// raises no executable-configuration frontier.
func TestConfigurationTextCopiesAreNotConfigurations_TJAA_V0_1161(t *testing.T) {
	if prefixedConfig("playwright.config.js.txt", "playwright.config") || prefixedConfig("vitest.config.ts.bak", "vitest.config") {
		t.Fatal("a text copy counted as a configuration")
	}
	for _, name := range []string{"playwright.config.js", "playwright.config.cjs", "playwright.config.mts", "vitest.config.json"} {
		if !prefixedConfig(name, name[:len(name)-len(name[strings.LastIndex(name, "."):])]) {
			t.Errorf("%s is a configuration", name)
		}
	}
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "pkg/pkg.go", "package pkg\n")
	write(t, root, "pkg/pkg_test.go", "package pkg\n")
	write(t, root, "pkg/testdata/case/playwright.config.js.txt", "module.exports = { testDir: '.', projects: [{ name: 'chromium', use: { launchOptions: { executablePath: '<x>' } } }] };\n")
	write(t, root, "pkg/testdata/case/e2e.spec.js.txt", "test('x', () => {})\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range result.Frontier {
		if strings.HasPrefix(entry, FrontierConfig) {
			t.Errorf("a text copy raised %s", entry)
		}
	}
}

// V1-4683: a dirty JSON file a TypeScript module imports through a relative
// specifier, an exact tsconfig alias or a package.json `imports` mapping keeps
// or selects the importer: the specifier is a path token of the importing
// unit (TJAA-V0-126), so the importer is a reader of the dirty path even
// where the plugin resolves no edge to the JSON file.
func TestDirtyJSONImportKeepsTheImporter_V1_4683(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name":"app","imports":{"#data/*":"./data/*"},"devDependencies":{"vitest":"1"}}`)
	write(t, root, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"@cfg/settings":["config/settings.json"]}}}`)
	write(t, root, "data/items.json", `{"items":[1]}`)
	write(t, root, "config/settings.json", `{"retries":1}`)
	write(t, root, "fixtures/table.json", `[]`)
	write(t, root, "tests/relative.test.ts", "import { test } from \"vitest\"\nimport table from \"../fixtures/table.json\"\n")
	write(t, root, "tests/alias.test.ts", "import { test } from \"vitest\"\nimport settings from \"@cfg/settings\"\n")
	write(t, root, "tests/imports.test.ts", "import { test } from \"vitest\"\nimport items from \"#data/items.json\"\n")
	write(t, root, "tests/other.test.ts", "import { test } from \"vitest\"\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	for dirty, importer := range map[string]string{
		"fixtures/table.json":  "tests/relative.test.ts",
		"config/settings.json": "tests/alias.test.ts",
		"data/items.json":      "tests/imports.test.ts",
	} {
		plan := affected.Select(graph, []string{dirty})
		if !slices.Contains(plan.SelectedTests(), importer) {
			t.Errorf("%s: selected %v, want the importer %s", dirty, plan.SelectedTests(), importer)
		}
	}
}
