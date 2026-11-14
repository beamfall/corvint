// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// slfv1006Fault is one pre-registered fault for a narrowing rule
// (SLF-V1-006): a project, the file whose line the fault changes and its
// faulted body, and the test the plan must select, which would kill the
// fault. killerNotProduced, when set, says why that test cannot kill the
// fault: the case then checks the selection side only and claims no killer.
type slfv1006Fault struct {
	name              string
	files             map[string]string
	path              string
	mutant            string
	killer            string
	killerNotProduced string
}

// slfv1006PagesTest is the route table's killer for an import.meta.glob
// table: it loads every page the glob matched and compares each export's
// value (a function's return value) with want, a literal of the unfaulted
// pages, so a fault in a page or in a file a page imports changes a value
// and fails it, while the unfaulted tree passes.
func slfv1006PagesTest(want string) string {
	return "import { test, expect } from \"vitest\"\nimport { pages } from \"../src/routes\"\n" +
		"test(\"routes\", async () => {\n" +
		"  const loaded: Record<string, Record<string, unknown>> = {}\n" +
		"  for (const [path, load] of Object.entries(pages)) {\n" +
		"    const module = await (load as () => Promise<Record<string, unknown>>)()\n" +
		"    loaded[path] = Object.fromEntries(Object.entries(module).map(([name, value]) => [name, typeof value === \"function\" ? value() : value]))\n" +
		"  }\n" +
		"  expect(loaded).toEqual(" + want + ")\n" +
		"})\n"
}

// The unfaulted values of the pages each glob matches.
const (
	slfv1006FlatPages = `{ "./pages/about.tsx": { default: "ABOUT" }, "./pages/home.tsx": { default: "home" } }`
	slfv1006AllPages  = `{ "./pages/about.tsx": { default: "ABOUT" }, "./pages/home.tsx": { default: "home" }, "./pages/nested/deep.tsx": { deep: 1 } }`
)

// slfv1006KeysTest snapshots only the route table's keys; it is the
// require.context cases' selected test, which kills nothing (below).
const slfv1006KeysTest = "import { test, expect } from \"vitest\"\nimport { pages } from \"../src/routes\"\ntest(\"routes\", () => expect(Object.keys(pages)).toMatchSnapshot())\n"

// slfv1006NoRequireContext is why no require.context case has a killer.
const slfv1006NoRequireContext = "NOT_PRODUCED: the project's runner is Vitest, which has no require.context (a webpack API), " +
	"so no test of the route table runs on the unfaulted tree; a killer needs a webpack- or Jest-run fixture"

// slfv1006GlobProject is a Vite route table that loads its pages through
// call, with test as the test of the route table.
func slfv1006GlobProject(call, test string) map[string]string {
	return map[string]string{
		"package.json":              `{"devDependencies":{"vitest":"1"}}`,
		"src/routes.ts":             "export const pages = " + call + "\n",
		"src/pages/about.tsx":       "import { title } from \"../lib/format\"\nexport default function About() { return title(\"about\") }\n",
		"src/pages/home.tsx":        "export default function Home() { return \"home\" }\n",
		"src/pages/nested/deep.tsx": "export const deep = 1\n",
		"src/lib/format.ts":         "export const title = (s: string) => s.toUpperCase()\n",
		"tests/routes.test.ts":      test,
	}
}

// slfv1006GlobFaults are the SLF-V1-003 cases: each literal glob form the
// rule expands, a recursive match, and a file reached through a globbed one.
var slfv1006GlobFaults = []slfv1006Fault{
	{name: "import.meta.glob star", files: slfv1006GlobProject(`import.meta.glob("./pages/*.tsx")`, slfv1006PagesTest(slfv1006FlatPages)),
		path: "src/pages/home.tsx", mutant: "export default function Home() { return \"Home\" }\n", killer: "tests/routes.test.ts"},
	{name: "import.meta.glob recursive", files: slfv1006GlobProject(`import.meta.glob("./pages/**/*.tsx")`, slfv1006PagesTest(slfv1006AllPages)),
		path: "src/pages/nested/deep.tsx", mutant: "export const deep = 2\n", killer: "tests/routes.test.ts"},
	{name: "import.meta.glob array", files: slfv1006GlobProject(`import.meta.glob(["./pages/*.tsx", "./pages/nested/*.tsx"])`, slfv1006PagesTest(slfv1006AllPages)),
		path: "src/pages/nested/deep.tsx", mutant: "export const deep = 0\n", killer: "tests/routes.test.ts"},
	{name: "require.context flat", files: slfv1006GlobProject(`require.context("./pages", false, /\.tsx$/)`, slfv1006KeysTest),
		path: "src/pages/home.tsx", mutant: "export default function Home() { return \"\" }\n", killer: "tests/routes.test.ts", killerNotProduced: slfv1006NoRequireContext},
	{name: "require.context recursive", files: slfv1006GlobProject(`require.context("./pages", true, /\.tsx$/)`, slfv1006KeysTest),
		path: "src/pages/nested/deep.tsx", mutant: "export const deep = -1\n", killer: "tests/routes.test.ts", killerNotProduced: slfv1006NoRequireContext},
	{name: "file reached through a globbed page", files: slfv1006GlobProject(`import.meta.glob("./pages/*.tsx")`, slfv1006PagesTest(slfv1006FlatPages)),
		path: "src/lib/format.ts", mutant: "export const title = (s: string) => s.toLowerCase()\n", killer: "tests/routes.test.ts"},
}

const slfv1006VitestConfig = "import { defineConfig } from \"vitest/config\"\n" +
	"export default defineConfig({\n  test: {\n    include: [\"src/**/*.test.ts\"],\n    setupFiles: [\"./setup/env.ts\"],\n" +
	"    globalSetup: \"./setup/global.ts\",\n  },\n})\n"

const slfv1006JestConfig = "module.exports = {\n  roots: ['<rootDir>/src'],\n  testMatch: ['**/__tests__/**/*.ts'],\n" +
	"  moduleNameMapper: { '^@app/config$': '<rootDir>/src/config.ts' },\n}\n"

// slfv1006VitestProject is a Vitest project whose config is fully literal.
func slfv1006VitestProject() map[string]string {
	return map[string]string{
		"package.json":      `{"devDependencies":{"vitest":"1"}}`,
		"vitest.config.js":  slfv1006VitestConfig,
		"src/math.ts":       "import { round } from \"./round\"\nexport const add = (a: number, b: number) => round(a + b)\n",
		"src/round.ts":      "export const round = (x: number) => Math.round(x)\n",
		"src/math.test.ts":  "import { test, expect } from \"vitest\"\nimport { add } from \"./math\"\ntest(\"add\", () => expect(add(1, 2)).toBe(3))\n",
		"src/other.ts":      "export const other = 1\n",
		"src/other.test.ts": "import { test, expect } from \"vitest\"\nimport { other } from \"./other\"\ntest(\"other\", () => expect(other).toBe(1))\n",
		"setup/env.ts":      "globalThis.ENV = \"test\"\n",
		"setup/global.ts":   "export default function setup() { process.env.TZ = \"UTC\" }\n",
	}
}

// slfv1006JestProject is a Jest project whose config is fully literal.
func slfv1006JestProject() map[string]string {
	return map[string]string{
		"package.json":          `{"devDependencies":{"jest":"29","@jest/globals":"29"}}`,
		"jest.config.js":        slfv1006JestConfig,
		"src/sum.ts":            "export const sum = (a: number, b: number) => a + b\n",
		"src/config.ts":         "export const limit = 10\n",
		"src/__tests__/sum.ts":  "import { test, expect } from \"@jest/globals\"\nimport { sum } from \"../sum\"\ntest(\"sum\", () => expect(sum(1, 2)).toBe(3))\n",
		"src/__tests__/conf.ts": "import { test, expect } from \"@jest/globals\"\nimport { limit } from \"@app/config\"\ntest(\"limit\", () => expect(limit).toBe(10))\n",
	}
}

// slfv1006ConfigFaults are the SLF-V1-005 cases: a source a test imports
// below a literal Vitest include, a file reached through it, the literal
// setupFiles and globalSetup targets, a source below a literal Jest roots
// and testMatch, and a literal moduleNameMapper target.
var slfv1006ConfigFaults = []slfv1006Fault{
	{name: "vitest include source", files: slfv1006VitestProject(),
		path: "src/math.ts", mutant: "import { round } from \"./round\"\nexport const add = (a: number, b: number) => round(a - b)\n", killer: "src/math.test.ts"},
	{name: "vitest transitive source", files: slfv1006VitestProject(),
		path: "src/round.ts", mutant: "export const round = (x: number) => Math.floor(x) + 1\n", killer: "src/math.test.ts"},
	{name: "vitest setupFiles target", files: slfv1006VitestProject(),
		path: "setup/env.ts", mutant: "globalThis.ENV = \"prod\"\nthrow new Error(\"setup\")\n", killer: "src/other.test.ts"},
	{name: "vitest globalSetup target", files: slfv1006VitestProject(),
		path: "setup/global.ts", mutant: "export default function setup() { throw new Error(\"global\") }\n", killer: "src/math.test.ts"},
	{name: "jest roots and testMatch source", files: slfv1006JestProject(),
		path: "src/sum.ts", mutant: "export const sum = (a: number, b: number) => a * b\n", killer: "src/__tests__/sum.ts"},
	{name: "jest moduleNameMapper target", files: slfv1006JestProject(),
		path: "src/config.ts", mutant: "export const limit = 11\n", killer: "src/__tests__/conf.ts"},
}

// slfv1006Plan writes the faulted tree, the head of a fault-carrying change,
// and returns the plan of the faulted path and the plugin's result.
func slfv1006Plan(t *testing.T, fault slfv1006Fault) (affected.Plan, affected.Result) {
	t.Helper()
	root := t.TempDir()
	for relative, body := range fault.files {
		write(t, root, relative, body)
	}
	write(t, root, fault.path, fault.mutant)
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return affected.Select(graph, []string{fault.path}), unitsOf(t, root)
}

// slfv1006Run checks one rule's cases, the selection side of each FIH-V1
// case: with the gate held the rule's frontier is raised and the plan
// selects the killer or is UNKNOWN; with the gate on the frontier is gone
// and the plan is BOUNDED and selects the test that kills the fault. That the
// killer fails on the faulted tree is the FIH-V1 kill, which needs the
// harness's Vitest and Jest adapters and is not measured here. A case whose
// killer is NOT_PRODUCED must say why, and a case that claims a killer must
// not select the keys-only route test, which observes no page's value.
func slfv1006Run(t *testing.T, rule string, gate *bool, frontier string, faults []slfv1006Fault) {
	t.Helper()
	if len(faults) < 5 {
		t.Fatalf("SLF-V1-006 needs at least five faults for %s, have %d", rule, len(faults))
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			if killer := fault.files[fault.killer]; fault.killerNotProduced == "" && (killer == "" || killer == slfv1006KeysTest) {
				t.Fatalf("the case claims killer %s, which observes no faulted value: %q", fault.killer, killer)
			} else if fault.killerNotProduced != "" && !strings.HasPrefix(fault.killerNotProduced, "NOT_PRODUCED: ") {
				t.Fatalf("killerNotProduced %q names no NOT_PRODUCED reason", fault.killerNotProduced)
			}
			held, result := slfv1006Plan(t, fault)
			if !frontierHas(result, frontier) {
				t.Fatalf("held rule: no %s frontier, so the case does not exercise %s: %v", frontier, rule, result.Frontier)
			}
			if !slices.Contains(held.SelectedTests(), fault.killer) && held.Scope != affected.ScopeUnknown {
				t.Fatalf("held rule: killer %s neither selected nor UNKNOWN: scope=%s selected=%v", fault.killer, held.Scope, held.SelectedTests())
			}
			withGate(t, gate)
			enabled, result := slfv1006Plan(t, fault)
			if frontierHas(result, frontier) || enabled.Scope != affected.ScopeBounded || !slices.Contains(enabled.SelectedTests(), fault.killer) {
				t.Fatalf("enabled rule: frontier=%v scope=%s selected=%v unknown=%+v, want BOUNDED with killer %s", result.Frontier, enabled.Scope, enabled.SelectedTests(), enabled.Unknown, fault.killer)
			}
		})
	}
}

// SLF-V1-006 for SLF-V1-003 (literal glob expansion).
func TestSLFV1006GlobFaultCases(t *testing.T) {
	slfv1006Run(t, "SLF-V1-003", &globExpansionNarrowing, FrontierDynamicImport, slfv1006GlobFaults)
}

// SLF-V1-006 for SLF-V1-005 (literal Vitest and Jest configuration).
func TestSLFV1006LiteralConfigFaultCases(t *testing.T) {
	slfv1006Run(t, "SLF-V1-005", &literalConfigNarrowing, FrontierConfig, slfv1006ConfigFaults)
}
