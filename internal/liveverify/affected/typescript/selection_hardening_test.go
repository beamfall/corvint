// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// withGate turns one SLF-V1-006 gate on for one test.
func withGate(t *testing.T, gate *bool) {
	t.Helper()
	previous := *gate
	*gate = true
	t.Cleanup(func() { *gate = previous })
}

func buildPlan(t *testing.T, root string, dirty ...string) affected.Plan {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return affected.Select(graph, dirty)
}

func sourceUnit(result affected.Result, relative string) affected.Unit {
	for _, unit := range result.Units {
		if slices.Contains(unit.Sources, relative) || slices.Contains(unit.Tests, relative) {
			return unit
		}
	}
	return affected.Unit{}
}

func unitsOf(t *testing.T, root string) affected.Result {
	t.Helper()
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func frontierAt(result affected.Result, reason, site string) bool {
	return slices.ContainsFunc(result.Frontier, func(entry string) bool {
		return strings.HasPrefix(entry, reason+":"+site)
	})
}

// viteGlobProject is the SLF-V1-003 fixture: a route table loads its pages by
// a literal glob and a test imports the route table.
func viteGlobProject(t *testing.T, call string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/routes.ts", "export const pages = "+call+"\n")
	write(t, root, "src/pages/about.tsx", "export default function About() { return null }\n")
	write(t, root, "src/pages/home.tsx", "export default function Home() { return null }\n")
	write(t, root, "src/pages/nested/deep.tsx", "export const deep = 1\n")
	write(t, root, "tests/routes.test.ts", "import { test } from \"vitest\"\nimport { pages } from \"../src/routes\"\ntest(\"routes\", () => pages)\n")
	return root
}

// SLF-V1-003 with its gate held (SLF-V1-006): a glob call, read as no import at
// all before this change, raises typescript:dynamic-import, so the plan
// selects the importer's tests through the OUTBOUND site closure.
func TestSLFV1003HeldGlobRaisesDynamicImport(t *testing.T) {
	for _, call := range []string{
		`import.meta.glob("./pages/*.tsx")`,
		`import.meta.glob<{ default: unknown }>("./pages/*.tsx", { eager: true })`,
		`import.meta.globEager("./pages/*.tsx")`,
		`require.context("./pages", false, /\.tsx$/)`,
	} {
		root := viteGlobProject(t, call)
		if result := unitsOf(t, root); !frontierAt(result, FrontierDynamicImport, "src/routes.ts:1") {
			t.Fatalf("%s: frontier = %v", call, result.Frontier)
		}
		plan := buildPlan(t, root, "src/pages/about.tsx")
		if !slices.Contains(plan.SelectedTests(), "tests/routes.test.ts") {
			t.Fatalf("%s: selected=%v unknown=%+v", call, plan.SelectedTests(), plan.Unknown)
		}
	}
}

// SLF-V1-003 with its gate on: a literal glob expands to one edge per matching
// file and a membership reader, and raises no frontier; the "Literal glob" and
// "New file matching a glob" scenarios.
func TestSLFV1003LiteralGlobExpands(t *testing.T) {
	withGate(t, &globExpansionNarrowing)
	root := viteGlobProject(t, `import.meta.glob("./pages/*.tsx")`)
	result := unitsOf(t, root)
	if frontierHas(result, FrontierDynamicImport) {
		t.Fatalf("literal glob raised %v", result.Frontier)
	}
	routes := sourceUnit(result, "src/routes.ts")
	if fmt.Sprint(routes.Imports) != "[typescript:src/pages/about.tsx typescript:src/pages/home.tsx]" || fmt.Sprint(routes.PathGlobs) != "[/src/pages/*.tsx]" {
		t.Fatalf("routes imports=%v globs=%v", routes.Imports, routes.PathGlobs)
	}
	plan := buildPlan(t, root, "src/pages/about.tsx")
	if !slices.Contains(plan.SelectedTests(), "tests/routes.test.ts") || plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s selected=%v unknown=%+v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
	// A file added after the graph was built reaches the test through the
	// membership reader alone.
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "src/pages/contact.tsx", "export default 1\n")
	added := affected.Select(graph, []string{"src/pages/contact.tsx"})
	if !slices.Contains(added.SelectedTests(), "tests/routes.test.ts") {
		t.Fatalf("new matching file: selected=%v unknown=%+v", added.SelectedTests(), added.Unknown)
	}
}

// SLF-V1-003: require.context reads its directory, recursion flag and regular
// expression; a `**` glob's membership reader is its literal directory.
func TestSLFV1003RequireContextAndRecursiveGlob(t *testing.T) {
	withGate(t, &globExpansionNarrowing)
	root := viteGlobProject(t, `require.context("./pages", false, /\.tsx$/)`)
	routes := sourceUnit(unitsOf(t, root), "src/routes.ts")
	if fmt.Sprint(routes.Imports) != "[typescript:src/pages/about.tsx typescript:src/pages/home.tsx]" || fmt.Sprint(routes.PathGlobs) != "[/src/pages/*]" {
		t.Fatalf("context imports=%v globs=%v", routes.Imports, routes.PathGlobs)
	}
	root = viteGlobProject(t, `import.meta.glob(["./pages/**/*.tsx", "!./pages/home.tsx"])`)
	routes = sourceUnit(unitsOf(t, root), "src/routes.ts")
	want := "[typescript:src/pages/about.tsx typescript:src/pages/home.tsx typescript:src/pages/nested/deep.tsx]"
	if fmt.Sprint(routes.Imports) != want || fmt.Sprint(routes.PathGlobs) != "[/src/pages]" {
		t.Fatalf("recursive imports=%v globs=%v", routes.Imports, routes.PathGlobs)
	}
}

// SLF-V1-003: a computed argument, a root-relative or aliased pattern, a base
// option or an unreadable regular expression keeps typescript:dynamic-import
// even with the gate on (the "Computed glob" scenario).
func TestSLFV1003ComputedGlobKeepsTheFrontier(t *testing.T) {
	withGate(t, &globExpansionNarrowing)
	for _, call := range []string{
		`import.meta.glob(base + "/*.ts")`,
		`import.meta.glob("/src/pages/*.tsx")`,
		`import.meta.glob("@/pages/*.tsx")`,
		`import.meta.glob("./*.tsx", { base: "./pages" })`,
		"import.meta.glob(`./${dir}/*.tsx`)",
		`require.context(dir)`,
		`require.context("./pages", true, /(?<=x)\.tsx$/)`,
		`import.meta.glob`,
	} {
		root := viteGlobProject(t, call)
		if result := unitsOf(t, root); !frontierAt(result, FrontierDynamicImport, "src/routes.ts:1") {
			t.Fatalf("%s: frontier = %v", call, result.Frontier)
		}
	}
}

// SLF-V1-003: an expansion over the path budget is computed.
func TestSLFV1003ExpansionBound(t *testing.T) {
	files := make([]string, 0, globExpansionBound+1)
	for index := range globExpansionBound + 1 {
		files = append(files, fmt.Sprintf("src/pages/p%04d.ts", index))
	}
	call := loaderCall{kind: loaderGlob, args: []string{`"./pages/*.ts"`}}
	if _, literal := expandLoader("src/routes.ts", call, files, New().Owns); literal {
		t.Fatal("expansion over the bound was not computed")
	}
	if expansion, literal := expandLoader("src/routes.ts", call, files[:globExpansionBound], New().Owns); !literal || len(expansion.refs) != globExpansionBound {
		t.Fatalf("expansion at the bound: literal=%v refs=%d", literal, len(expansion.refs))
	}
}

// globRegexp reads the picomatch forms Vite, Vitest and Jest configs use.
func TestSLFGlobRegexpDialect(t *testing.T) {
	for _, tc := range []struct {
		glob, path string
		match      bool
	}{
		{"src/**/*.test.ts", "src/a/b/c.test.ts", true},
		{"src/**/*.test.ts", "src/c.test.ts", true},
		{"src/*.ts", "src/a/b.ts", false},
		{"**/?(*.)+(spec|test).[jt]s?(x)", "a/b/x.spec.tsx", true},
		{"**/?(*.)+(spec|test).[jt]s?(x)", "a/test.js", true},
		{"**/?(*.)+(spec|test).[jt]s?(x)", "a/x.ts", false},
		{"src/{a,b}/*.ts", "src/b/x.ts", true},
		{"src/[!x]*.ts", "src/y.ts", true},
	} {
		expression, ok := globRegexp(tc.glob)
		if !ok || expression.MatchString(tc.path) != tc.match {
			t.Errorf("%s ~ %s: ok=%v want %v", tc.glob, tc.path, ok, tc.match)
		}
	}
	if _, ok := globRegexp("src/!(skip).ts"); ok {
		t.Error("a negated extglob was translated")
	}
}

// jestMockProject is the SLF-V1-004 fixture: an adjacent manual mock, a root
// package mock and a module that imports both.
func jestMockProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"jest":"29","@jest/globals":"29"},"dependencies":{"axios":"1"}}`)
	write(t, root, "src/api.ts", "export const get = () => 1\n")
	write(t, root, "src/__mocks__/api.ts", "export const get = () => 2\n")
	write(t, root, "__mocks__/axios.js", "module.exports = { get: () => 3 }\n")
	write(t, root, "src/client.ts", "import axios from \"axios\"\nexport const client = axios\n")
	write(t, root, "src/service.ts", "import { get } from \"./api\"\nexport const run = get\n")
	write(t, root, "tests/api.test.ts", "import { test } from \"@jest/globals\"\nimport { get } from \"../src/api\"\ntest(\"get\", () => get())\n")
	write(t, root, "tests/service.test.ts", "import { test } from \"@jest/globals\"\nimport { run } from \"../src/service\"\ntest(\"run\", () => run())\n")
	write(t, root, "tests/client.test.ts", "import { test } from \"@jest/globals\"\nimport { client } from \"../src/client\"\ntest(\"client\", () => client)\n")
	write(t, root, "tests/literal.test.ts", "import { test, jest } from \"@jest/globals\"\njest.mock(\"../src/api\")\ntest(\"literal\", () => 1)\n")
	write(t, root, "tests/other.test.ts", "import { test } from \"@jest/globals\"\ntest(\"other\", () => 1)\n")
	return root
}

// SLF-V1-004: a change to an adjacent manual mock selects every test that
// imports its module, directly, through another module or through a literal
// jest.mock; a root package mock is read by every importer of the package.
func TestSLFV1004ManualMocks(t *testing.T) {
	root := jestMockProject(t)
	plan := buildPlan(t, root, "src/__mocks__/api.ts")
	if got := strings.Join(plan.SelectedTests(), " "); got != "tests/api.test.ts tests/literal.test.ts tests/service.test.ts" {
		t.Fatalf("adjacent mock selected=%s unknown=%+v", got, plan.Unknown)
	}
	plan = buildPlan(t, root, "__mocks__/axios.js")
	if got := strings.Join(plan.SelectedTests(), " "); got != "tests/client.test.ts" {
		t.Fatalf("package mock selected=%s unknown=%+v", got, plan.Unknown)
	}
	plan = buildPlan(t, root, "src/api.ts")
	if !slices.Contains(plan.SelectedTests(), "tests/literal.test.ts") {
		t.Fatalf("literal jest.mock target: selected=%v", plan.SelectedTests())
	}
}

// SLF-V1-004: a computed mock target raises typescript:mock-target-unresolved,
// DISCOVERY at the calling file (the "Computed mock target" scenario).
func TestSLFV1004ComputedMockTarget(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/db.ts", "export const db = 1\n")
	write(t, root, "tests/db.test.ts", "import { test, vi } from \"vitest\"\nvi.mock(pathFor(\"db\"))\ntest(\"db\", () => 1)\n")
	result := unitsOf(t, root)
	if !frontierAt(result, FrontierMockTarget, "tests/db.test.ts:2") {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	if got := New().FrontierDirection(FrontierMockTarget); got != affected.FrontierDiscovery {
		t.Fatalf("direction = %s", got)
	}
	if slices.Contains(New().SourceBoundFrontiers(), FrontierMockTarget) {
		t.Fatal("a mock target may name any module, so it must stay path-open")
	}
}

// SLF-V1-004: a changed manual mock no modelled consumer reads is unknown
// NO_SELECTABLE_TEST, never Uncovered.
func TestSLFV1004UnconsumedMockIsNeverUncovered(t *testing.T) {
	root := jestMockProject(t)
	write(t, root, "src/__mocks__/orphan.ts", "export const orphan = 1\n")
	plan := buildPlan(t, root, "src/__mocks__/orphan.ts")
	if slices.ContainsFunc(plan.Uncovered, func(entry affected.Unknown) bool { return strings.Contains(entry.Detail, "orphan") }) {
		t.Fatalf("orphan mock is Uncovered: %+v", plan.Uncovered)
	}
	if !slices.ContainsFunc(plan.Unknown, func(entry affected.Unknown) bool {
		return entry.Reason == affected.UnknownNoSelectableTest && strings.Contains(entry.Detail, "orphan")
	}) {
		t.Fatalf("orphan mock is not NO_SELECTABLE_TEST: %+v", plan.Unknown)
	}
}

// vitestProject is the SLF-V1-005 fixture: a Vitest config, a module and the
// test that reaches it, and an unrelated test.
func vitestProject(t *testing.T, config string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "vitest.config.js", config)
	write(t, root, "src/math.ts", "export const add = (a: number, b: number) => a + b\n")
	write(t, root, "src/other.ts", "export const other = 1\n")
	write(t, root, "src/math.test.ts", "import { test } from \"vitest\"\nimport { add } from \"./math\"\ntest(\"add\", () => add(1, 2))\n")
	write(t, root, "src/other.test.ts", "import { test } from \"vitest\"\nimport { other } from \"./other\"\ntest(\"other\", () => other)\n")
	write(t, root, "setup/env.ts", "export {}\n")
	return root
}

const literalVitestConfig = "import { defineConfig } from \"vitest/config\"\n" +
	"export default defineConfig({\n  test: {\n    include: [\"src/**/*.test.ts\", \"src/**/*.check.ts\"],\n" +
	"    exclude: [\"dist/**\"],\n    setupFiles: [\"./setup/env.ts\"],\n    coverage: { include: [\"src/**\"] },\n  },\n})\n"

// SLF-V1-005 and SLF-V1-006 ("Rule held until proved"): with the gate held, a
// literal config still raises the frontier.
func TestSLFV1005HeldLiteralConfigKeepsTheFrontier(t *testing.T) {
	root := vitestProject(t, literalVitestConfig)
	if !frontierAt(unitsOf(t, root), FrontierConfig, "vitest.config.js:") {
		t.Fatal("held rule dropped the executable-config frontier")
	}
	if plan := buildPlan(t, root, "src/math.ts"); plan.Scope != affected.ScopeUnknown {
		t.Fatalf("held rule narrowed: scope=%s unknown=%+v", plan.Scope, plan.Unknown)
	}
}

// SLF-V1-005 "Literal include narrows": with the gate on, a fully literal
// config is evaluated; the plan selects the test that reaches the change,
// names no executable-config frontier, adds the literal-include test and the
// setup-file edge, and ignores coverage.include.
func TestSLFV1005LiteralIncludeNarrows(t *testing.T) {
	withGate(t, &literalConfigNarrowing)
	root := vitestProject(t, literalVitestConfig)
	write(t, root, "src/extra.check.ts", "import { test } from \"vitest\"\ntest(\"extra\", () => 1)\n")
	result := unitsOf(t, root)
	if frontierHas(result, FrontierConfig) {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	if testUnit(result, "src/extra.check.ts").ID != "typescript:vitest:src/extra.check.ts" {
		t.Fatal("a literal include match is not a test")
	}
	if testUnit(result, "src/math.ts").ID != "" {
		t.Fatal("coverage.include made a source file a test")
	}
	plan := buildPlan(t, root, "src/math.ts")
	if got := strings.Join(plan.SelectedTests(), " "); got != "src/math.test.ts" || plan.Scope != affected.ScopeBounded {
		t.Fatalf("scope=%s selected=%s unknown=%+v", plan.Scope, got, plan.Unknown)
	}
	plan = buildPlan(t, root, "setup/env.ts")
	if got := strings.Join(plan.SelectedTests(), " "); got != "src/extra.check.ts src/math.test.ts src/other.test.ts" {
		t.Fatalf("setup file selected=%s unknown=%+v", got, plan.Unknown)
	}
}

// SLF-V1-005 "Dirty config": a dirty literal config still runs its runner
// instance in full.
func TestSLFV1005DirtyConfigRunsInFull(t *testing.T) {
	withGate(t, &literalConfigNarrowing)
	root := vitestProject(t, literalVitestConfig)
	plan := buildPlan(t, root, "vitest.config.js")
	if got := strings.Join(plan.SelectedTests(), " "); got != "src/math.test.ts src/other.test.ts" {
		t.Fatalf("dirty config selected=%s unknown=%+v", got, plan.Unknown)
	}
}

// SLF-V1-005: a computed marker value, a spread, a relative import, a marker
// in an unknown object, a negated glob or a capturing mapper whose key may be
// a package keeps the frontier with the gate on ("Computed value keeps the
// frontier", "Jest roots is a recognised marker").
func TestSLFV1005ComputedConfigKeepsTheFrontier(t *testing.T) {
	withGate(t, &literalConfigNarrowing)
	for name, tc := range map[string]struct{ file, config string }{
		"computed include": {"vitest.config.js", "export default { test: { include: [process.env.SUITE] } }\n"},
		"jest roots":       {"jest.config.js", "module.exports = { roots: [process.env.SRC] }\n"},
		"spread":           {"vitest.config.js", "const base = {}\nexport default { ...base, test: { include: [\"src/**/*.test.ts\"] } }\n"},
		"relative import":  {"vitest.config.js", "import shared from \"./shared.config\"\nexport default { test: { include: [\"src/**/*.test.ts\"] } }\n"},
		"unknown parent":   {"vitest.config.js", "const options = { include: [\"src/**/*.test.ts\"] }\nexport default { test: options }\n"},
		"negated glob":     {"vitest.config.js", "export default { test: { include: [\"src/!(skip)/*.test.ts\"] } }\n"},
		"package mapper":   {"jest.config.js", "module.exports = { moduleNameMapper: { \"^lodash/(.*)$\": \"<rootDir>/shim/$1\" } }\n"},
		"unevaluated key":  {"vitest.config.js", "export default { test: { include: [\"src/**/*.test.ts\"], browser: { capabilities: {} } } }\n"},
	} {
		root := t.TempDir()
		write(t, root, "package.json", `{"devDependencies":{"vitest":"1","jest":"29"}}`)
		write(t, root, tc.file, tc.config)
		write(t, root, "src/math.ts", "export const add = 1\n")
		if !frontierAt(unitsOf(t, root), FrontierConfig, tc.file+":") {
			t.Errorf("%s: frontier kept nothing", name)
		}
	}
}

// SLF-V1-005: the five key-shaped markers are recognised, a non-key spelling
// is not, and a fully literal Jest config with them is evaluated with its
// mapper and setup targets as loads.
func TestSLFV1005KeyShapedMarkers(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{"module.exports = { roots: ['<rootDir>/src'] }", true},
		{"module.exports = { 'testPathIgnorePatterns': ['/node_modules/'] }", true},
		{"module.exports = { moduleNameMapper: {} }", true},
		{"export default { test: { dir: 'src' } }", true},
		{"export default { root: 'app' }", true},
		{"module.exports = { rootDir: '.', outDir: 'x' }", false},
		{"const __dir = 1; const roots = []", false},
	} {
		if got := dynamicConfigMarker(tc.body) >= 0; got != tc.want {
			t.Errorf("%q: marker=%v want %v", tc.body, got, tc.want)
		}
	}
	withGate(t, &literalConfigNarrowing)
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"jest":"29","@jest/globals":"29"}}`)
	write(t, root, "jest.config.js", "module.exports = {\n  roots: ['<rootDir>/src'],\n  testPathIgnorePatterns: ['/node_modules/'],\n"+
		"  setupFilesAfterEnv: ['<rootDir>/setup/env.ts', '@testing-library/jest-dom'],\n"+
		"  moduleNameMapper: { '^@/(.*)$': '<rootDir>/src/$1', '\\\\.css$': '<rootDir>/setup/styleMock.js' },\n"+
		"  testRegex: '(/__tests__/.*|\\\\.spec)\\\\.ts$',\n}\n")
	write(t, root, "setup/env.ts", "export {}\n")
	write(t, root, "setup/styleMock.js", "module.exports = {}\n")
	write(t, root, "src/math.ts", "export const add = 1\n")
	write(t, root, "src/__tests__/math.ts", "import { test } from \"@jest/globals\"\nimport { add } from \"../math\"\ntest(\"add\", () => add)\n")
	result := unitsOf(t, root)
	if frontierHas(result, FrontierConfig) {
		t.Fatalf("frontier = %v", result.Frontier)
	}
	unit := testUnit(result, "src/__tests__/math.ts")
	for _, want := range []string{"typescript:setup/env.ts", "typescript:setup/styleMock.js", "typescript:src/math.ts"} {
		if !slices.Contains(unit.Imports, want) {
			t.Fatalf("test unit %s imports=%v, want %s", unit.ID, unit.Imports, want)
		}
	}
}
