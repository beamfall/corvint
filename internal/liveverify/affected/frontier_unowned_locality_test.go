// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type localRunner struct{ reaching }

func (localRunner) AncestorBoundFrontiers() []string { return []string{"y:runner-unknown"} }

// AFP-V0-2600: a located runner's hidden edges cannot reach an unrelated
// sibling data path; adding such a path to a hosted fixture must not undo
// the fixture proof either. Path-open dynamic loads retain their uncertainty.
func TestUnownedRunnerSiteLocality_AFPV02600(t *testing.T) {
	const runner = "y:runner-unknown:app/app_test.y"
	language := reaching{
		fake: fake{name: "y", frontier: []string{runner}, units: []Unit{
			{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
		}},
		fixtureClosed: []string{"y:runner-unknown"},
	}
	goPlugin := fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}}
	for _, dirty := range [][]string{
		{"data/payload.json"},
		{"data/payload.json", "pkg/testdata/payload.json"},
	} {
		graph, err := Build(context.Background(), t.TempDir(), goPlugin, localRunner{language})
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, dirty)
		if slices.Contains(languageFrontier(plan), runner) {
			t.Errorf("%v: unrelated runner bears: %+v", dirty, plan.Unknown)
		}
		if plan.Scope != ScopeUnknown {
			t.Errorf("%v: unowned data must remain UNKNOWN: %+v", dirty, plan)
		}
	}
}

// AFP-V0-2600: project-declared output can be BOUNDED after the unrelated
// runner is removed. The declaration alone cannot close dynamic loads.
func TestDeclaredOutputOutsideRunnerSite_AFPV02600(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, NonSourcePathsPath), []byte(`{"profile":"corvint-non-source-paths/0","entries":[{"path":"evidence/","reason":"retained output"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dynamic := range []bool{false, true} {
		language := reaching{
			fake: fake{name: "y", frontier: []string{"y:runner-unknown:app/app_test.y"}, units: []Unit{
				{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
			}},
			fixtureClosed: []string{"y:runner-unknown"},
		}
		if dynamic {
			language.frontier = append(language.frontier, "y:dynamic-load:app/app.y:1")
			slices.Sort(language.frontier)
		}
		graph, err := Build(context.Background(), root, localRunner{language})
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{"evidence/result.json"})
		want := ScopeBounded
		if dynamic {
			want = ScopeUnknown
		}
		if plan.Scope != want || len(plan.Selected) != 0 || len(plan.NonSource) != 1 {
			t.Errorf("dynamic=%v: want %s, got %+v", dynamic, want, plan)
		}
	}
}

// AFP-V0-2600: absent edge evidence is never a proof. Ancestor configs,
// unbounded drivers, unresolved/summarized sites, resolution inputs and real
// readers/dependents all retain UNKNOWN and their runner frontier.
func TestUnboundedRunnerInputsFailClosed_AFPV02600(t *testing.T) {
	const runner = "y:runner-unknown:app/app_test.y"
	for _, tc := range []struct {
		name, dirty, entry string
		edit               func(*reaching)
	}{
		{name: "CI driver", dirty: ".github/workflows/ci.yml", entry: runner},
		{name: "ancestor configuration", dirty: "app/config.json", entry: runner},
		{name: "root configuration", dirty: "config.json", entry: runner},
		{name: "unhosted fixture", dirty: "other/testdata/data.json", entry: runner},
		{name: "unknown site", dirty: "data/payload.json", entry: "y:runner-unknown:missing.y"},
		{name: "summarized site", dirty: "data/payload.json", entry: "y:runner-unknown:+2"},
		{name: "bare reason", dirty: "data/payload.json", entry: "y:runner-unknown"},
		{name: "resolution input", dirty: "data/config.json", entry: runner, edit: func(l *reaching) { l.inputs = func(string) bool { return true } }},
		{name: "literal reader", dirty: "data/payload.json", entry: runner, edit: func(l *reaching) { l.units[0].PathTokens = []string{"data/payload.json"} }},
		{name: "dependency", dirty: "lib/lib.y", entry: runner, edit: func(l *reaching) {
			l.units[0].Imports = []string{"y:lib"}
			l.units = append(l.units, Unit{ID: "y:lib", Sources: []string{"lib/lib.y"}})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			language := reaching{
				fake: fake{name: "y", frontier: []string{tc.entry}, units: []Unit{
					{ID: "y:app", Sources: []string{"app/app.y"}, Tests: []string{"app/app_test.y"}},
				}},
				fixtureClosed: []string{"y:runner-unknown"},
			}
			if tc.edit != nil {
				tc.edit(&language)
			}
			graph, err := Build(context.Background(), t.TempDir(), localRunner{language})
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{tc.dirty})
			if plan.Scope != ScopeUnknown || !slices.Contains(languageFrontier(plan), tc.entry) {
				t.Fatalf("uncertainty lost: %+v", plan)
			}
			if tc.name == "literal reader" || tc.name == "dependency" {
				if !slices.Contains(plan.SelectedTests(), "app/app_test.y") {
					t.Fatal("real edge lost its selected reader")
				}
			}
		})
	}
}

// AFP-V0-2600: the older fixture-closed declaration alone is insufficient to
// bound sibling paths. Malformed stronger declarations cannot narrow a plan.
func TestRunnerLocalityNeedsExplicitValidatedDeclaration_AFPV02600(t *testing.T) {
	language := reaching{
		fake: fake{name: "y", frontier: []string{"y:runner-unknown:app/app_test.y"}, units: []Unit{
			{ID: "y:app", Tests: []string{"app/app_test.y"}},
		}},
		fixtureClosed: []string{"y:runner-unknown"},
	}
	graph, err := Build(context.Background(), t.TempDir(), language)
	if err != nil {
		t.Fatal(err)
	}
	if plan := Select(graph, []string{"data/payload.json"}); len(languageFrontier(plan)) != 1 || plan.Scope != ScopeUnknown {
		t.Fatalf("undeclared narrowing: %+v", plan)
	}
	declared, err := Build(context.Background(), t.TempDir(), localRunner{language})
	if err != nil {
		t.Fatal(err)
	}
	if graph.Digest() != declared.Digest() {
		t.Fatal("plugin code classification changed graph identity")
	}
	// Cache restoration admits the recorded units through the same declaration
	// validation as a fresh build, rather than trusting serialized policy.
	restored, err := restoreGraph(context.Background(), cacheEntry{Languages: []cachedLanguage{{
		Name: "y", Frontier: language.frontier, Units: []cachedUnit{cachedUnit(language.units[0])},
	}}}, []Language{localRunner{language}})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Digest() != declared.Digest() || len(languageFrontier(Select(restored, []string{"data/payload.json"}))) != 0 {
		t.Fatal("cache restore lost runner locality")
	}
	language.fixtureClosed = nil
	if _, err := Build(context.Background(), t.TempDir(), localRunner{language}); !errors.Is(err, ErrInvalidLanguage) {
		t.Fatalf("invalid locality accepted: %v", err)
	}
}
