package typescript

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-710 (V1-2789): a package.json dependency whose file:, link:, portal: or workspace:
// specifier names repository source outside node_modules is no external package, so a bare import
// of it keeps the bare-import frontier and a change to the linked source is never excluded; that
// holds also when node_modules holds a copy of it. A local path that climbs out of the root and a
// version range stay declared dependencies with no frontier.
func TestLocalSpecifierDependencyKeepsBareImportFrontier_V12789(t *testing.T) {
	for _, tc := range []struct {
		specifier string
		copied    bool
		frontier  bool
	}{
		{"file:./src/lib", false, true},
		{"file:./src/lib", true, true},
		{"link:src/lib", false, true},
		{"portal:./src/lib", false, true},
		{"workspace:*", false, true},
		{"file:../outside/lib", false, false},
		{"1.0.0", false, false},
	} {
		root := t.TempDir()
		write(t, root, "package.json", `{"name":"app","private":true,"devDependencies":{"vitest":"1.6.0","alias":"`+tc.specifier+`"}}`)
		write(t, root, "vitest.config.ts", "export default {}\n")
		write(t, root, "src/lib/package.json", `{"name":"other-name","main":"logic.js"}`)
		write(t, root, "src/lib/logic.js", "export const value = 1\n")
		write(t, root, "test/alias.test.js", "import { test } from 'vitest'\nimport { value } from 'alias'\ntest('alias', () => { if (value !== 1) throw new Error('value') })\n")
		if tc.copied {
			write(t, root, ".gitignore", "node_modules\n")
			write(t, root, "node_modules/alias/package.json", `{"name":"alias","main":"logic.js"}`)
			write(t, root, "node_modules/alias/logic.js", "export const value = 1\n")
		}
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"src/lib/logic.js"})
		got := plan.FrontierSites[FrontierBareImportUndeclared] > 0
		if got != tc.frontier {
			t.Errorf("%s (copied=%v): bare-import frontier = %v, want %v; scope=%s selected=%v", tc.specifier, tc.copied, got, tc.frontier, plan.Scope, selectedTests(plan))
		}
		if tc.frontier && plan.Scope == affected.ScopeBounded && !slices.Contains(selectedTests(plan), "test/alias.test.js") {
			t.Errorf("%s: a BOUNDED plan excluded the importing test", tc.specifier)
		}
	}
	if !repositorySpecifier("pkgs/app", "link:../lib") || repositorySpecifier("pkgs/app", "file:../../../x") ||
		repositorySpecifier(".", "file:./node_modules/x") || repositorySpecifier(".", "file:/abs/x") || repositorySpecifier(".", "^1.2.3") {
		t.Fatal("repositorySpecifier classifies a nested, an escaping, a node_modules, an absolute or a range specifier wrongly")
	}
}

// TJAA-V0-710 (Codex P2): a nameless manifest whose only dependency is a local specifier is still a
// package scope, so a node:test file importing the alias keeps the bare-import frontier even with a
// copied node_modules install, and a change to the linked source never yields a bounded plan
// without the importing test.
func TestNamelessManifestLocalSpecifierKeepsFrontier_V12789(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"private":true,"dependencies":{"alias":"file:./src/lib"}}`)
	write(t, root, "src/lib/package.json", `{"name":"other-name","main":"logic.js"}`)
	write(t, root, "src/lib/logic.js", "export const value = 1\n")
	write(t, root, "test/alias.test.mjs", "import test from 'node:test'\nimport { value } from 'alias'\ntest('alias', () => { if (value !== 1) throw new Error('value') })\n")
	write(t, root, ".gitignore", "node_modules\n")
	write(t, root, "node_modules/alias/package.json", `{"name":"alias","main":"logic.js"}`)
	write(t, root, "node_modules/alias/logic.js", "export const value = 1\n")
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/lib/logic.js"})
	if plan.FrontierSites[FrontierBareImportUndeclared] == 0 && !(plan.Scope != affected.ScopeBounded || slices.Contains(selectedTests(plan), "test/alias.test.mjs")) {
		t.Fatalf("nameless manifest: scope=%s selected=%v frontier=%v", plan.Scope, selectedTests(plan), plan.FrontierSites)
	}
	if plan.FrontierSites[FrontierBareImportUndeclared] == 0 {
		t.Fatalf("nameless manifest: the copied install suppressed the bare-import frontier: %v scope=%s selected=%v unknown=%+v units=%v", plan.FrontierSites, plan.Scope, selectedTests(plan), plan.Unknown, graph.UnitIDs())
	}
}

// TJAA-V0-710 (Codex P2): a test runner declared by a local specifier still names that runner, so a
// convention-only Jest test (a __tests__ file using the global test API, no import) stays a test
// candidate of its package.
func TestLocalSpecifierRunnerStillRecognised_V12789(t *testing.T) {
	for _, specifier := range []string{"29.7.0", "file:./vendor/jest"} {
		root := t.TempDir()
		write(t, root, "package.json", `{"name":"app","devDependencies":{"jest":"`+specifier+`"}}`)
		write(t, root, "vendor/jest/package.json", `{"name":"jest"}`)
		write(t, root, "src/value.js", "module.exports = 1\n")
		write(t, root, "__tests__/case.js", "const value = require('../src/value')\ntest('case', () => { expect(value).toBe(1) })\n")
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		candidate := false
		for _, id := range graph.UnitIDs() {
			unit, _ := graph.Unit(id)
			if slices.Contains(unit.Tests, "__tests__/case.js") {
				candidate = true
			}
		}
		if !candidate {
			t.Errorf("jest %q: __tests__/case.js is no test of any unit; units %v", specifier, graph.UnitIDs())
		}
	}
}
