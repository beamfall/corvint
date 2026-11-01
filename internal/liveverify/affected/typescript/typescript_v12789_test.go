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
