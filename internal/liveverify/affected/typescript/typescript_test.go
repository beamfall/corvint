package typescript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsOnlyDeclaredSourceExtensions(t *testing.T) {
	language := New()
	for _, relative := range []string{"a.ts", "a.tsx", "a.js", "a.jsx", "a.mjs", "a.cjs"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{"a.mts", "a.cts", "a.json", "a.feature", "flow.yaml", "story.mdx", "a.py"} {
		if language.Owns(relative) {
			t.Fatalf("%s must remain unowned", relative)
		}
	}
}

func TestScanImportsReadsStaticFormsAndFlagsComputedLoading(t *testing.T) {
	refs, dynamic, err := scanImports("", `
import value from "./value";
import type { Kind } from './kind';
export { other } from "./other.js";
const common = require("./common.cjs");
const lazy = import("./lazy.mjs");
const unknown = import(target);
// import "./forged";
const text = "require('./also-forged')";
`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./common.cjs", "./kind", "./other.js", "./value"}
	if strings.Join(refs, " ") != strings.Join(want, " ") {
		t.Fatalf("refs=%v want=%v", refs, want)
	}
	if !dynamic {
		t.Fatal("computed import did not raise the dynamic flag")
	}
}

func TestDynamicImportWithCommentBeforeParenthesisRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "vitest.config.ts", `export default {}`)
	write(t, root, "src/lazy.ts", `export const value = 1`)
	write(t, root, "src/lazy.test.ts", `import { test } from "vitest"; test("lazy", async () => { await import /* chunk */ ("./lazy") })`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	assertFrontier(t, result, FrontierDynamicImport)
}

func TestUnitsDetectsEverySupportedRunnerFromImportsOrConfiguration(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		config   string
		path     string
		body     string
		runner   string
	}{
		{name: "vitest", manifest: `{"devDependencies":{"vitest":"1"}}`, path: "tests/a.test.ts", body: `import { test } from "vitest"`, runner: runnerVitest},
		{name: "jest", manifest: `{"devDependencies":{"jest":"1"}}`, path: "tests/a.test.js", body: `import { test } from "@jest/globals"`, runner: runnerJest},
		{name: "node", path: "checks/a.js", body: `import test from "node:test"`, runner: runnerNode},
		{name: "playwright", path: "e2e/login.ts", body: `import { test } from "@playwright/test"`, runner: runnerPlaywright},
		{name: "bun", path: "tests/a.ts", body: `import { test } from "bun:test"`, runner: runnerBun},
		{name: "deno", path: "tests/a.ts", body: `Deno.test("a", () => {})`, runner: runnerDeno},
		{name: "cypress config membership", config: "cypress.config.ts", path: "cypress/e2e/login.ts", body: `describe("login", () => {})`, runner: runnerCypress},
		{name: "webdriverio", path: "e2e/login.ts", body: `import { browser } from "@wdio/globals"`, runner: runnerWebdriverIO},
		{name: "testcafe", path: "e2e/login.ts", body: `import { Selector } from "testcafe"`, runner: runnerTestCafe},
		{name: "nightwatch config membership", config: "nightwatch.conf.js", path: "e2e/login.js", body: `module.exports = { Login() {} }`, runner: runnerNightwatch},
		{name: "detox", path: "e2e/login.js", body: `import { device } from "detox"`, runner: runnerDetox},
		{name: "storybook vitest", path: "src/button.stories.ts", body: `import { storybookTest } from "@storybook/addon-vitest/vitest-plugin"`, runner: runnerStorybookVitest},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			if testCase.manifest != "" {
				write(t, root, "package.json", testCase.manifest)
			}
			if testCase.config != "" {
				write(t, root, testCase.config, "export default {}\n")
			}
			write(t, root, testCase.path, testCase.body)
			result, err := New().Units(root)
			if err != nil {
				t.Fatal(err)
			}
			unit := testUnit(result, testCase.path)
			if unit.ID != unitID(testCase.runner, testCase.path) {
				t.Fatalf("id=%s want=%s frontier=%v", unit.ID, unitID(testCase.runner, testCase.path), result.Frontier)
			}
		})
	}
}

func TestPuppeteerUsesItsHostRunnerAndUnknownWithoutOne(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"puppeteer":"1","jest":"1"}}`)
	write(t, root, "e2e/hosted.test.js", `import { test } from "@jest/globals"; import puppeteer from "puppeteer"`)
	write(t, root, "e2e/script.js", `import puppeteer from "puppeteer"`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "e2e/hosted.test.js").ID; got != unitID(runnerJest, "e2e/hosted.test.js") {
		t.Fatalf("hosted id=%s", got)
	}
	if got := testUnit(result, "e2e/script.js").ID; got != unitID(runnerUnknown, "e2e/script.js") {
		t.Fatalf("script id=%s", got)
	}
	assertFrontier(t, result, FrontierPuppeteerHost)
}

func TestPackageScriptsDoNotAssignRunnerOwnership(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"test":"vitest run"}}`)
	write(t, root, "tests/orphan.test.ts", `test("a", () => {})`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "tests/orphan.test.ts").ID; got != unitID(runnerUnknown, "tests/orphan.test.ts") {
		t.Fatalf("id=%s frontier=%v", got, result.Frontier)
	}
	assertFrontier(t, result, FrontierUnknownRunner)
}

func TestFilenameWithoutRunnerEvidenceIsDiscoveredAsUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "tests/orphan.test.ts", `test("orphan", () => {})`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "tests/orphan.test.ts").ID; got != unitID(runnerUnknown, "tests/orphan.test.ts") {
		t.Fatalf("id=%s", got)
	}
	assertFrontier(t, result, FrontierUnknownRunner)
}

func TestTopLevelTestDirectoryWithoutRunnerEvidenceIsDiscoveredAsUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "src/value.js", `export const value = 1`)
	write(t, root, "test/value.js", `import { value } from "../src/value.js"; test("value", () => value)`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "test/value.js").ID; got != unitID(runnerUnknown, "test/value.js") {
		t.Fatalf("id=%s frontier=%v", got, result.Frontier)
	}
	assertFrontier(t, result, FrontierUnknownRunner)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/value.js"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestJestDependencyKeepsTestsDirectoryVisibleAsUnknown(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"jest":"29"}}`)
	write(t, root, "src/sum.js", `module.exports = (a, b) => a + b`)
	write(t, root, "src/__tests__/sum.js", `const sum = require("../sum"); test("adds", () => expect(sum(1, 2)).toBe(3))`)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/sum.js"})
	if plan.Scope != affected.ScopeUnknown || len(plan.SelectedTests()) != 1 || plan.SelectedTests()[0] != "src/__tests__/sum.js" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestAvaImportInTopLevelTestDirectoryIsAddressable(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"ava":"1"}}`)
	write(t, root, "lib/escape.js", `export const escape = value => value`)
	write(t, root, "test/arguments/escape.js", `import test from "ava"; import { escape } from "../../lib/escape.js"; test("escape", t => t.is(escape("a"), "a"))`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "test/arguments/escape.js").ID; got != unitID("ava", "test/arguments/escape.js") {
		t.Fatalf("id=%s want=%s frontier=%v", got, unitID("ava", "test/arguments/escape.js"), result.Frontier)
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"lib/escape.js"})
	if got := strings.Join(plan.SelectedTests(), " "); got != "test/arguments/escape.js" {
		t.Fatalf("selected=%s unknown=%v", got, plan.Unknown)
	}
}

func TestOverlappingRunnerEvidenceWidensRatherThanChoosing(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1","jest":"1"}}`)
	write(t, root, "vitest.config.ts", "export default {}")
	write(t, root, "jest.config.js", "module.exports = {}")
	write(t, root, "tests/shared.test.ts", `describe("shared", () => {})`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := testUnit(result, "tests/shared.test.ts").ID; got != unitID(runnerUnknown, "tests/shared.test.ts") {
		t.Fatalf("id=%s", got)
	}
	assertFrontier(t, result, FrontierAmbiguousRunner)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"tests/shared.test.ts"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestImportsBuildAConservativeDependencyClosure(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/core.ts", `export const core = 1`)
	write(t, root, "src/mid.ts", `export { core } from "./core"`)
	write(t, root, "src/leaf.ts", `import { core } from "./mid.js"; export { core }`)
	write(t, root, "tests/leaf.test.ts", `import { test } from "vitest"; import { core } from "../src/leaf"`)
	write(t, root, "tests/solo.test.ts", `import { test } from "vitest"`)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/core.ts"})
	if got := plan.SelectedTests(); len(got) != 1 || got[0] != "tests/leaf.test.ts" {
		t.Fatalf("selected=%v unknown=%v", got, plan.Unknown)
	}
	if plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestFrameworkConfigIsAnAffectingSourceUnit(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "vitest.config.ts", `export default {}`)
	write(t, root, "tests/a.test.ts", `import { test } from "vitest"`)
	write(t, root, "tests/b.test.ts", `import { test } from "vitest"`)
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"vitest.config.ts"})
	if got := strings.Join(plan.SelectedTests(), " "); got != "tests/a.test.ts tests/b.test.ts" {
		t.Fatalf("selected=%s unknown=%v", got, plan.Unknown)
	}
}

func TestVitestSpecificViteConfigIsSourceNotTest(t *testing.T) {
	root := t.TempDir()
	write(t, root, "vite.config.ts", `import { defineConfig } from "vitest/config"; export default defineConfig({ test: {} })`)
	write(t, root, "tests/a.test.ts", `import { test } from "vitest"`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		for _, testPath := range unit.Tests {
			if testPath == "vite.config.ts" {
				t.Fatalf("Vite config classified as test: %+v", unit)
			}
		}
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"vite.config.ts"})
	if got := strings.Join(plan.SelectedTests(), " "); got != "tests/a.test.ts" {
		t.Fatalf("selected=%s unknown=%v", got, plan.Unknown)
	}
}

func TestLegacyStorybookUsesOneAddressableStorySet(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"@storybook/test-runner":"1"}}`)
	write(t, root, "src/a.stories.ts", `import "./a"`)
	write(t, root, "src/b.stories.ts", `import "./b"`)
	write(t, root, "src/a.ts", `export const a = 1`)
	write(t, root, "src/b.ts", `export const b = 1`)
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, unit := range result.Units {
		if unit.ID != "typescript:storybook-test-runner:." {
			continue
		}
		count++
		if got := strings.Join(unit.Tests, " "); got != "src/a.stories.ts src/b.stories.ts" {
			t.Fatalf("stories=%s", got)
		}
	}
	if count != 1 {
		t.Fatalf("storybook units=%d result=%+v", count, result.Units)
	}
	assertFrontier(t, result, FrontierStorybookAddressing)
}

func TestDynamicAndCrossLanguageInputsRemainExplicitFrontiers(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"test":"node --test"}}`)
	write(t, root, "tests/dynamic.test.js", `const target = "../src/a.js"; import(target)`)
	write(t, root, "features/login.feature", "Feature: login\n")
	write(t, root, "maestro/login.yaml", "appId: example\n")
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	assertFrontier(t, result, FrontierDynamicImport)
	assertFrontier(t, result, FrontierCrossLanguageTestAsset)
	if New().Owns("features/login.feature") || New().Owns("maestro/login.yaml") {
		t.Fatal("ambiguous assets were claimed")
	}
}

func testUnit(result affected.Result, relative string) affected.Unit {
	for _, unit := range result.Units {
		for _, test := range unit.Tests {
			if test == relative {
				return unit
			}
		}
	}
	return affected.Unit{}
}

func assertFrontier(t *testing.T, result affected.Result, want string) {
	t.Helper()
	for _, reason := range result.Frontier {
		if reason == want {
			return
		}
	}
	t.Fatalf("frontier=%v want=%s", result.Frontier, want)
}

func write(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestQuoteInRegexLiteralHidesNoRequire(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.js", `module.exports = 1`)
	write(t, root, "src/other.js", `module.exports = 2`)
	write(t, root, "tests/dep.test.js", "import { test } from \"vitest\"\nconst quote = /\"/;\nconst dep = require('./../src/dep')\nconst text = \"x\".concat(/\"/.source)\n")
	write(t, root, "tests/division.test.js", "import { test } from \"vitest\"\nconst half = total / 2; const other = require('../src/other') / \"1\"\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	for dirty, want := range map[string]string{"src/dep.js": "tests/dep.test.js", "src/other.js": "tests/division.test.js"} {
		plan := affected.Select(graph, []string{dirty})
		if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != want {
			t.Errorf("%s: scope=%s selected=%v unknown=%v", dirty, plan.Scope, got, plan.Unknown)
		}
	}
}

func TestTemplateSubstitutionRequireBuildsDependencyEdge(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst value = `${require('../src/dep')}`\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "tests/a.test.tsx" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
}

func TestMultilineJSXQuoteAmbiguityRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst first = <p>it's</p>\nconst dep = require('../src/dep')\nconst last = <p>Bob's</p>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}
	untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:src/dep.ts"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want || plan.Unknown[1] != untested {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestSameLineJSXApostropheAmbiguityRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst view = <><p>it's</p>{require('../src/dep')}<p>Bob's</p></>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}
	untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:src/dep.ts"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want || plan.Unknown[1] != untested {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestStandaloneJSXApostrophesRaiseFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst view = <><p>'</p>{require('../src/dep')}<p>'</p></>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}
	untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:src/dep.ts"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want || plan.Unknown[1] != untested {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestJSXTextCannotImitateALiteralOpeningContext(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst view = <><p>return '</p>{require('../src/dep')}<p>'</p></>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}
	untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:src/dep.ts"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want || plan.Unknown[1] != untested {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestKeywordEndingJSXTextCannotHideRequireWithoutBraces(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "return.ts", `export const value = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst a=<p>return '</p>; const dep=require('../return'); const b=<p>'</p>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"return.ts"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}
	untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:return.ts"}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want || plan.Unknown[1] != untested {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestJSXAttributeAndExpressionStringsRemainParsed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", `export const dep = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst view = <p title=\"it's\">{require('../src/dep')}</p>\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/dep.ts"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "tests/a.test.tsx" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
}

func TestOrdinaryTSXStringsAndJSXExpressionLiteralsRemainParsed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "return.ts", `export const value = 1`)
	write(t, root, "tests/a.test.tsx", "import { test } from \"vitest\"\nconst before = \"ordinary\"\nconst view = <p title=\"it's\">{require('../return')}</p>\nconst after = 'still ordinary'\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"return.ts"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "tests/a.test.tsx" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
}

func TestUnownedModuleExtensionTestWidensItsImportedSource(t *testing.T) {
	for _, test := range []string{"tests/a.test.mts", "tests/a.test.CTS"} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
		write(t, root, "src/a.ts", `export const a = 1`)
		write(t, root, test, "import { test } from \"vitest\"\nimport { a } from \"../src/a\"\n")
		graph, err := affected.Build(root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"src/a.ts"})
		want := []affected.Unknown{{Reason: affected.UnknownLanguageFrontier, Detail: FrontierUnparsedSource}}
		untested := affected.Unknown{Reason: affected.UnknownNoSelectableTest, Detail: "typescript:src/a.ts"}
		if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 2 || plan.Unknown[0] != want[0] || plan.Unknown[1] != untested {
			t.Errorf("%s: scope=%s unknown=%v", test, plan.Scope, plan.Unknown)
		}
	}
}

func TestTripleSlashReferencePathIsARelativeEdge(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "lib/globals.d.ts", "declare var G: number\n")
	write(t, root, "src/types/local.d.ts", "declare var L: number\n")
	write(t, root, "src/a.test.ts", "/// <reference path=\"../lib/globals.d.ts\" />\n///<reference path='types/local.d.ts'/>\n/// <reference types=\"node\" />\nimport { test } from \"vitest\"\n")
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"lib/globals.d.ts", "src/types/local.d.ts"} {
		plan := affected.Select(graph, []string{dirty})
		if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "src/a.test.ts" {
			t.Errorf("%s: scope=%s selected=%v unknown=%v", dirty, plan.Scope, got, plan.Unknown)
		}
	}
}
