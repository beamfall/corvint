package contextindex

import (
	"context"
	"reflect"
	"slices"
	"strconv"
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

// lowerBound sets bound's limit for the rest of the test, so a fixture one past
// it stays small.
func lowerBound(t *testing.T, bound *analyzerBound, limit int) {
	t.Helper()
	saved := bound.limit
	bound.limit = limit
	t.Cleanup(func() { bound.limit = saved })
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

// webConfigChain links tsconfig.json through count extends to a base whose
// baseUrl resolves `setup/x.setup` (the spec's only import).
func webConfigChain(count int) map[string]string {
	files := map[string]string{
		"setup/x.setup.ts": "export default async () => 1;\n",
		"tests/x.spec.ts":  "import seed from 'setup/x.setup';\ntest('x', () => seed());\n",
		"tsconfig.json":    `{"extends": "./c1.json"}`,
	}
	for link := 1; link <= count; link++ {
		files["c"+strconv.Itoa(link)+".json"] = `{"extends": "./c` + strconv.Itoa(link+1) + `.json"}`
	}
	files["c"+strconv.Itoa(count)+".json"] = `{"compilerOptions": {"baseUrl": "."}}`
	return files
}

// TestAnalyzerBoundsFailClosed: TSA-V0-460, TSA-V0-461. Every declared
// analyzer bound, driven one past its limit, yields its typed outcome and no
// complete answer: setup-select returns ALL naming the outcome, the Ruby index
// counts the ruby-reference-index-bounded unknown, and the Ruby verification
// command is withheld. A case per declared bound is required, so a new bound
// cannot be added without one.
func TestAnalyzerBoundsFailClosed(t *testing.T) {
	setupAll := func(t *testing.T, bound *analyzerBound, files map[string]string, spec string) {
		t.Helper()
		selection := selectSetupOf(t, files, spec)
		if selection.Mode != "all" || !slices.Contains(reasonCodes(selection), bound.outcome) {
			t.Fatalf("mode=%s reasons=%+v, want ALL with %s", selection.Mode, selection.Reasons, bound.outcome)
		}
	}
	rubyBounded := func(t *testing.T, bound *analyzerBound, files map[string]string, changed string) {
		t.Helper()
		if _, receipt := rubyBoundsImpact(t, files, changed); rubyUnknownCount(receipt, bound.outcome) == 0 {
			t.Fatalf("unknowns %#v, want %s", receipt["unknowns"], bound.outcome)
		}
	}
	sharedRead := func(from string) string {
		return "import { SHARED } from '" + from + "';\nexport default async () => db.insert(SHARED.scenario_a);\n"
	}
	sharedSpec := "import { SHARED } from '../lib/shared';\ntest('x', () => SHARED.scenario_a);\n"
	cases := map[string]func(t *testing.T, bound *analyzerBound){
		"setup-closure": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 2)
			setupAll(t, bound, map[string]string{
				"tests/x.spec.ts": "import './a';\ntest('x', () => 1);\n",
				"tests/a.ts":      "import './b';\n",
				"tests/b.ts":      "import './c';\n",
				"tests/c.ts":      "export const c = 1;\n",
			}, "tests/x.spec.ts")
		},
		"shared-reexport": func(t *testing.T, bound *analyzerBound) {
			files := map[string]string{
				"lib/shared.ts":    "export const SHARED = { scenario_a: 1 };\n",
				"setup/x.setup.ts": sharedRead("../lib/hop0"),
				"tests/x.spec.ts":  sharedSpec,
			}
			sharedHops(files, 8-bound.limit) // bound.limit+1 hops
			setupAll(t, bound, files, "tests/x.spec.ts")
		},
		"shared-lookup": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 2)
			files := map[string]string{
				"lib/shared.ts":    "export const SHARED = { scenario_a: 1 };\n",
				"setup/x.setup.ts": sharedRead("../lib/hop6"),
				"tests/x.spec.ts":  sharedSpec,
			}
			sharedHops(files, 6)
			setupAll(t, bound, files, "tests/x.spec.ts")
		},
		"shared-template": func(t *testing.T, bound *analyzerBound) {
			setupAll(t, bound, map[string]string{
				"lib/shared.ts":    "export const SHARED = { scenario_a: 1 };\n",
				"setup/x.setup.ts": sharedRead("../lib/shared"),
				"tests/x.spec.ts":  "import { SHARED } from '../lib/shared';\ntest('t', () => " + sharedTemplate("SHARED.scenario_a", bound.limit+1) + ");\n",
			}, "tests/x.spec.ts")
		},
		"web-config-bytes": func(t *testing.T, bound *analyzerBound) {
			files := webConfigChain(1)
			lowerBound(t, bound, len(files["c1.json"])-1)
			setupAll(t, bound, files, "tests/x.spec.ts")
		},
		"web-config-depth": func(t *testing.T, bound *analyzerBound) {
			setupAll(t, bound, webConfigChain(bound.limit+1), "tests/x.spec.ts")
		},
		"web-config-visits": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 2)
			setupAll(t, bound, webConfigChain(3), "tests/x.spec.ts")
		},
		"registry-manifest": func(t *testing.T, bound *analyzerBound) {
			files := setupSelectFiles()
			lowerBound(t, bound, len(files["corvint/appmap.json"])-1)
			setupAll(t, bound, files, "tests/alpha.spec.ts")
		},
		"registry-declarations": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 0)
			setupAll(t, bound, setupSelectFiles(), "tests/alpha.spec.ts")
		},
		"registry-accessors": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 0)
			setupAll(t, bound, setupSelectFiles(), "tests/alpha.spec.ts")
		},
		"registry-key": func(t *testing.T, bound *analyzerBound) {
			files := setupSelectFiles()
			entry := "  " + strings.Repeat("k", bound.limit+1) + ": Beta,\n"
			files["tests/data/registry.ts"] = strings.Replace(files["tests/data/registry.ts"], "};\n", entry+"};\n", 1)
			setupAll(t, bound, files, "tests/alpha.spec.ts")
		},
		"ruby-nesting": func(t *testing.T, bound *analyzerBound) {
			rubyBounded(t, bound, rubyNestingOverflowFiles(), "app/models/outer/mid/deep/leaf.rb")
		},
		"ruby-resolutions": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 2)
			rubyBounded(t, bound, rubyCraftedNamespaces(bound.limit+1), "app/models/ns0/x.rb")
		},
		"ruby-reference-sites": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 1)
			rubyBounded(t, bound, map[string]string{
				"app/models/a.rb": "class A\nend\n",
				"app/models/b.rb": "class B\n  def x = [A, A]\nend\n",
			}, "app/models/a.rb")
		},
		"ruby-manifest": func(t *testing.T, bound *analyzerBound) {
			lowerBound(t, bound, 64)
			commands, uncertainty := impactVerificationOf(t, rubyVerificationFiles(map[string]string{
				"Gemfile": "gem \"rspec\"\n" + strings.Repeat("#", bound.limit),
			}), "app/api_clients/foo_client.rb")
			if !reflect.DeepEqual(commands, []any{"git diff --check"}) || !anyContains(uncertainty, unknownLanguageVerification) {
				t.Fatalf("verification = %v, uncertainty = %v, want the command withheld", commands, uncertainty)
			}
		},
		"source-bytes": func(t *testing.T, bound *analyzerBound) {
			setupAll(t, bound, map[string]string{
				"setup/x.setup.ts": "export default async () => 1;\n",
				"tests/x.spec.ts":  "import './big';\nimport '../setup/x.setup';\ntest('x', () => 1);\n",
				"tests/big.ts":     strings.Repeat("/", bound.limit+1),
			}, "tests/x.spec.ts")
		},
	}
	for _, bound := range analyzerBounds {
		run, ok := cases[bound.name]
		if !ok {
			t.Errorf("declared bound %s has no fail-closed case", bound.name)
			continue
		}
		delete(cases, bound.name)
		t.Run(bound.name, func(t *testing.T) { run(t, bound) })
	}
	for name := range cases {
		t.Errorf("case %s names no declared bound", name)
	}
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
