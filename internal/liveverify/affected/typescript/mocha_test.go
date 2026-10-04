package typescript

import (
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"testing"
)

func TestMochaLiteralEvidenceAndUnknowns(t *testing.T) {
	for _, tc := range []struct{ name, manifest, body, runner, frontier string }{
		{"import", `{"devDependencies":{"mocha":"12"}}`, `import {it} from "mocha"`, runnerMocha, ""},
		{"require", `{"devDependencies":{"mocha":"12"}}`, `const {it}=require("mocha")`, runnerMocha, ""},
		{"dependency", `{"devDependencies":{"mocha":"12"}}`, `describe("a",()=>{})`, runnerUnknown, FrontierUnknownRunner},
		{"scripts", `{"scripts":{"test":"mocha"}}`, `describe("a",()=>{})`, runnerUnknown, FrontierUnknownRunner},
		{"chai", `{"devDependencies":{"chai":"1"}}`, `const {expect}=require("chai")`, runnerUnknown, FrontierUnknownRunner},
		{"ambiguous imports", `{}`, `import {it} from "mocha"; import {test} from "@jest/globals"`, runnerUnknown, FrontierAmbiguousRunner},
		{"package config", `{"mocha":{"spec":"other/**"}}`, `describe("a",()=>{})`, runnerMocha, FrontierConfig},
		{"null config", `{"mocha":null}`, `describe("a",()=>{})`, runnerUnknown, FrontierUnknownRunner},
		{"opaque literal", `{}`, `const s="require('mocha')"`, runnerUnknown, FrontierUnknownRunner},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", tc.manifest)
			write(t, root, "tests/a.test.js", tc.body)
			got, e := New().Units(root)
			if e != nil {
				t.Fatal(e)
			}
			if u := testUnit(got, "tests/a.test.js"); u.ID != unitID(tc.runner, "tests/a.test.js") {
				t.Fatalf("unit=%+v", u)
			}
			if tc.frontier != "" {
				assertFrontier(t, got, tc.frontier)
			}
		})
	}
}

func TestMochaConfigsAlwaysRetainDiscoveryFrontier(t *testing.T) {
	for _, ext := range []string{"js", "cjs", "mjs", "json", "jsonc", "yaml", "yml"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			name := ".mocharc." + ext
			write(t, root, name, "{}")
			write(t, root, "tests/a.test.js", `describe("a",()=>{})`)
			got, e := New().Units(root)
			if e != nil {
				t.Fatal(e)
			}
			if testUnit(got, "tests/a.test.js").ID != unitID(runnerMocha, "tests/a.test.js") {
				t.Fatal(got)
			}
			assertFrontier(t, got, FrontierConfig)
			graph, e := affected.Build(root, New())
			if e != nil {
				t.Fatal(e)
			}
			if plan := affected.Select(graph, []string{name}); plan.Scope != affected.ScopeUnknown {
				t.Fatalf("config scope=%s", plan.Scope)
			}
		})
	}
	root := t.TempDir()
	write(t, root, ".mocharc.json", "{}")
	write(t, root, "jest.config.js", "module.exports={}")
	write(t, root, "tests/a.test.js", `describe("a",()=>{})`)
	got, e := New().Units(root)
	if e != nil {
		t.Fatal(e)
	}
	assertFrontier(t, got, FrontierAmbiguousRunner)
	if testUnit(got, "tests/a.test.js").ID != unitID(runnerUnknown, "tests/a.test.js") {
		t.Fatal(got)
	}
}

func TestMochaTypeScriptRetainsRuntimeFlags(t *testing.T) {
	for _, ext := range []string{"ts", "tsx"} {
		root := t.TempDir()
		write(t, root, "tests/a.test."+ext, `import {it} from "mocha"`)
		got, e := New().Units(root)
		if e != nil {
			t.Fatal(e)
		}
		assertFrontier(t, got, FrontierRuntimeFlags)
	}
}

func TestMochaNestedYAMLConfigurationIsRecognized(t *testing.T) {
	root := t.TempDir()
	write(t, root, "e2e/.mocharc.yaml", "spec: other/**")
	write(t, root, "e2e/a.test.js", `describe("a",()=>{})`)
	got, e := New().Units(root)
	if e != nil {
		t.Fatal(e)
	}
	assertFrontier(t, got, FrontierConfig)
	if testUnit(got, "e2e/a.test.js").ID != unitID(runnerMocha, "e2e/a.test.js") {
		t.Fatal(got)
	}
}

func TestMochaSourceSelectionRetainsUnrelatedExclusion(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"devDependencies":{"mocha":"12"}}`)
	write(t, root, "src/a.cjs", "module.exports=1")
	write(t, root, "tests/a.test.cjs", `const {it}=require("mocha");require("../src/a.cjs")`)
	write(t, root, "tests/b.test.cjs", `const {it}=require("mocha")`)
	graph, e := affected.Build(root, New())
	if e != nil {
		t.Fatal(e)
	}
	plan := affected.Select(graph, []string{"src/a.cjs"})
	if plan.Scope != affected.ScopeBounded || len(plan.SelectedTests()) != 1 || plan.SelectedTests()[0] != "tests/a.test.cjs" || len(plan.Excluded) != 1 {
		t.Fatalf("plan %+v", plan)
	}
}

func TestMochaCompetingImportAndConfigurationRetainsEdges(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		configs    []string
	}{
		{"mocha import jest config", `const {it}=require("mocha")`, []string{"jest.config.js"}},
		{"jest import mocha config", `import {test} from "@jest/globals"`, []string{".mocharc.cjs"}},
		{"mocha import both configs", `const {it}=require("mocha")`, []string{".mocharc.cjs", "jest.config.js"}},
		{"jest import both configs", `import {test} from "@jest/globals"`, []string{".mocharc.cjs", "jest.config.js"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "package.json", `{"devDependencies":{"mocha":"12","@jest/globals":"30"}}`)
			for _, c := range tc.configs {
				write(t, root, c, "module.exports={}")
			}
			write(t, root, "tests/a.test.js", tc.body)
			result, e := New().Units(root)
			if e != nil {
				t.Fatal(e)
			}
			assertFrontier(t, result, FrontierAmbiguousRunner)
			unit := testUnit(result, "tests/a.test.js")
			if unit.ID != unitID(runnerUnknown, "tests/a.test.js") {
				t.Fatal(unit)
			}
			graph, e := affected.Build(root, New())
			if e != nil {
				t.Fatal(e)
			}
			for _, c := range tc.configs {
				plan := affected.Select(graph, []string{c})
				if plan.Scope != affected.ScopeUnknown || len(plan.SelectedTests()) != 1 || plan.SelectedTests()[0] != "tests/a.test.js" {
					t.Fatalf("configuration %s lost dependency: %+v", c, plan)
				}
			}
		})
	}
}
