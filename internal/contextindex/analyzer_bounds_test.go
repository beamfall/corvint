package contextindex

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// rubyNestingOverflowFiles nests a reference past rubyNestingBound inside the
// namespaces that name it: the namespaces opened past the bound are not
// tracked, so the reference to Outer::Mid::Deep::Leaf cannot be resolved.
func rubyNestingOverflowFiles() map[string]string {
	depth := rubyNestingBound.limit - 1
	return map[string]string{
		"app/models/outer/mid/deep/leaf.rb": "module Outer\n  module Mid\n    module Deep\n      class Leaf\n      end\n    end\n  end\nend\n",
		"app/models/user.rb": "module Outer\n" + strings.Repeat("begin\n", depth) + "module Mid\nmodule Deep\nLeaf.new\nend\nend\n" +
			strings.Repeat("end\n", depth) + "end\n",
	}
}

// TestRubyNestingOverflowIsBounded: TSA-V0-461, NGI-V0-040. A reference whose
// enclosing namespaces opened past the nesting bound is either indexed or
// counted in ruby-reference-index-bounded, never silently dropped.
func TestRubyNestingOverflowIsBounded(t *testing.T) {
	_, receipt := rubyBoundsImpact(t, rubyNestingOverflowFiles(), "app/models/outer/mid/deep/leaf.rb")
	if rubyResultRows(receipt)["app/models/user.rb"] == nil && rubyUnknownCount(receipt, rubyNestingBound.outcome) == 0 {
		t.Fatalf("a reference nested past the bound was dropped silently: unknowns %#v", receipt["unknowns"])
	}
}

// selectSetupOf builds files and runs setup-select for spec.
func selectSetupOf(t *testing.T, files map[string]string, spec string) SetupSelection {
	t.Helper()
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	return SelectSetup(index, []string{spec})
}

// TestSharedResolutionCacheSkipsCycleMarkers: TSA-V0-462. An `export *` cycle
// reached first through its other member must not leave that member's
// in-progress (cycle-marked) miss in the cache: resolving it directly still
// finds the constant.
func TestSharedResolutionCacheSkipsCycleMarkers(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"lib/shared.ts": "export const SHARED = { scenario_a: 1 };\n",
		"lib/a.ts":      "export * from './b';\nexport * from './shared';\n",
		"lib/b.ts":      "export * from './a';\n",
	})
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	s := &setupSelector{index: index, resolver: NewWebImportResolver(index), read: map[string]string{}, shared: newSharedState()}
	if got := s.resolveConst("lib/a.ts", "SHARED"); got.kind != sharedFound {
		t.Fatalf("lib/a.ts SHARED = %+v, want found", got)
	}
	for key, cached := range s.shared.resolved {
		if cached.kind != sharedFound {
			t.Errorf("cached %v = %+v: a cycle-marked miss is not a permanent answer", key, cached)
		}
	}
	if got := s.resolveConst("lib/b.ts", "SHARED"); got.kind != sharedFound {
		t.Fatalf("lib/b.ts SHARED after the cycle = %+v, want found", got)
	}
}

// TestSharedExhaustedBranchOutranksLaterFind: TSA-V0-461, TSA-V0-462. An
// `export *` branch the re-export bound stopped may bind the name too, so a
// shorter later branch that finds it does not make the answer exact: the
// resolution stays unknown, is not cached, and setup-select returns ALL.
func TestSharedExhaustedBranchOutranksLaterFind(t *testing.T) {
	files := map[string]string{
		"lib/shared.ts":    "export const SHARED = { scenario_a: 1 };\n",
		"lib/fork.ts":      "export * from './hop0';\nexport * from './short';\n",
		"lib/short.ts":     "export * from './shared';\n",
		"setup/x.setup.ts": "import { SHARED } from '../lib/fork';\nexport default async () => db.insert(SHARED.scenario_a);\n",
		"tests/x.spec.ts":  "import { SHARED } from '../lib/shared';\ntest('x', () => SHARED.scenario_a);\n",
	}
	sharedHops(files, 0) // fork -> hop0 .. hop8 -> shared is past the 8-hop bound
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	s := &setupSelector{index: index, resolver: NewWebImportResolver(index), read: map[string]string{}, shared: newSharedState()}
	if got := s.resolveConst("lib/fork.ts", "SHARED"); got.kind != sharedUnknown {
		t.Fatalf("lib/fork.ts SHARED = %+v, want unknown: the stopped branch is unresolved", got)
	}
	if cached, ok := s.shared.resolved[[2]string{"lib/fork.ts", "SHARED"}]; ok {
		t.Fatalf("cached lib/fork.ts SHARED = %+v: a bound-shaped result is not a permanent answer", cached)
	}
	selection := SelectSetup(index, []string{"tests/x.spec.ts"})
	if selection.Mode != "all" || !slices.Contains(reasonCodes(selection), sharedReexportBound.outcome) {
		t.Fatalf("mode=%s reasons=%+v, want ALL with %s", selection.Mode, selection.Reasons, sharedReexportBound.outcome)
	}
}

// TestSharedLookupBoundAtProductionValue: TSA-V0-460, TSA-V0-461 (V1-1729).
// The fail-closed case lowers shared-lookup to 2; this one runs the declared
// 4,096. Each of n modules does `export *` from the other n-1 and none binds
// SHARED, so every simple path is a lookup whose cycle-marked miss is not
// cached: 13,700 lookups for 8 modules, 326 for 6, all within the 8-hop
// re-export bound. Under the bound the answer is the exact "not a constant";
// past it the answer is unknown, names the bound and is not cached. Removing
// the bound turns the 8-module answer into "not a constant" and fails.
func TestSharedLookupBoundAtProductionValue(t *testing.T) {
	t.Parallel()
	if sharedLookupBound.limit != 4096 {
		t.Fatalf("shared-lookup limit = %d, want the declared 4096 (TSA-V0-460)", sharedLookupBound.limit)
	}
	for _, tc := range []struct {
		modules int
		want    sharedKind
	}{{6, sharedNone}, {8, sharedUnknown}} {
		files := map[string]string{}
		for from := range tc.modules {
			var body strings.Builder
			for to := range tc.modules {
				if to != from {
					fmt.Fprintf(&body, "export * from './m%d';\n", to)
				}
			}
			files[fmt.Sprintf("lib/m%d.ts", from)] = body.String()
		}
		root := impactRepositoryWithFiles(t, files)
		index, err := BuildRevisionContext(context.Background(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
		if err != nil {
			t.Fatal(err)
		}
		s := &setupSelector{index: index, resolver: NewWebImportResolver(index), read: map[string]string{}, shared: newSharedState()}
		got := s.resolveConst("lib/m0.ts", "SHARED")
		if got.kind != tc.want {
			t.Fatalf("%d modules: SHARED = %+v after %d lookups, want kind %d", tc.modules, got, s.shared.steps, tc.want)
		}
		if tc.want == sharedUnknown && (s.shared.steps != sharedLookupBound.limit+1 || !strings.Contains(got.why, "shared-lookup bound of 4096 lookups")) {
			t.Fatalf("%d modules: stopped after %d lookups with %q, want the shared-lookup bound at 4097", tc.modules, s.shared.steps, got.why)
		}
		if cached, ok := s.shared.resolved[[2]string{"lib/m0.ts", "SHARED"}]; ok {
			t.Fatalf("%d modules: cached %+v: a cycle- or bound-shaped result is not a permanent answer", tc.modules, cached)
		}
	}
}

// TestOversizedRegistryManifestFailsClosed: TSA-V0-461. A manifest past the
// index's source admission is absent from index.Sources, yet it still counts as
// an invalid registries declaration, so setup-select returns ALL; an oversized
// `.json` that is no manifest leaves the selection narrow.
func TestOversizedRegistryManifestFailsClosed(t *testing.T) {
	t.Parallel()
	files := setupSelectFiles()
	files["data/big.json"] = `{"rows": "` + strings.Repeat("x", maxSourceBytes) + `"}`
	if selection := selectSetupOf(t, files, "tests/alpha.spec.ts"); selection.Mode != "selected" {
		t.Fatalf("an oversized non-manifest: mode=%s reasons=%+v, want selected", selection.Mode, selection.Reasons)
	}
	files["corvint/big-appmap.json"] = files["corvint/appmap.json"] + strings.Repeat(" ", maxSourceBytes)
	selection := selectSetupOf(t, files, "tests/alpha.spec.ts")
	if selection.Mode != "all" || !slices.Contains(reasonCodes(selection), registryManifestBound.outcome) {
		t.Fatalf("an oversized manifest: mode=%s reasons=%+v, want ALL with %s", selection.Mode, selection.Reasons, registryManifestBound.outcome)
	}
}
