// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// computedRubyReader is a Ruby-shaped plugin that owns no dirty path and
// reaches no changed unit. Its spec reads repository files through a computed
// name, which it reports as a path-open implicit-test-dependency entry; its
// relative-load entry is source-bound and cannot bind a non-Ruby path.
func computedRubyReader() reaching {
	return reaching{
		fake: fake{name: "rb", frontier: []string{
			"rb:implicit-test-dependency:spec/reader_spec.rb",
			"rb:relative-load:spec/reader_spec.rb:2 `require_relative(name)`",
		}, units: []Unit{
			{ID: "rb:spec", Sources: []string{"lib/app.rb"}, Tests: []string{"spec/reader_spec.rb"}},
		}},
		sourceBound: []string{"rb:relative-load"},
	}
}

// AFP-V0-2749 (review fix round 4): a deleted Go source of a surviving package
// (AFP-V0-2610) and a wholly deleted Go package (AFP-V0-821) stay unowned for
// every other plugin's frontier checks, as a deleted fixture does
// (AFP-V0-2740). Before those rules admitted the paths, their
// UNINDEXED_SOURCE_PATH entry made the plan FULL, which ran the Ruby spec. Now
// each path raises no entry of its own, so the Ruby spec that reads files by a
// computed name is only covered while its path-open entry bears through the
// unowned deleted path (AFP-V0-511): the plan stays UNKNOWN and names the
// deleted file. With participants reading claimantsOf alone, Go claims the
// path, it leaves the unowned-path checks, and the plan is a clean selection
// that skips the reader. An edit to a Go file a unit still declares is owned
// and is no such path: the Ruby entry does not bear on it.
func TestDeletedGoSourceStaysUnownedForOtherPlugins_AFPV2749(t *testing.T) {
	const implicit = "rb:implicit-test-dependency:spec/reader_spec.rb"
	const relative = "rb:relative-load:spec/reader_spec.rb:2 `require_relative(name)`"
	cases := map[string]struct {
		dirty    string
		bears    bool
		selected []string
	}{
		"deleted source of a surviving package": {dirty: "pkg/gone.go", bears: true, selected: []string{"go:pkg", "go:user"}},
		"deleted test of a surviving package":   {dirty: "pkg/gone_test.go", bears: true, selected: []string{"go:pkg"}},
		"wholly deleted package":                {dirty: "removed/removed.go", bears: true},
		"edited declared source (control)":      {dirty: "pkg/pkg.go", bears: false, selected: []string{"go:pkg", "go:user"}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for _, path := range []string{"pkg/pkg.go", "pkg/pkg_test.go", "user/user.go", "user/user_test.go", "other/other.go", "other/other_test.go"} {
				full := filepath.Join(root, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			goUnits := fake{name: "go", units: []Unit{
				{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
				{ID: "go:user", Sources: []string{"user/user.go"}, Tests: []string{"user/user_test.go"}, Imports: []string{"go:pkg"}},
				{ID: "go:other", Sources: []string{"other/other.go"}, Tests: []string{"other/other_test.go"}},
			}}
			graph, err := Build(context.Background(), root, goUnits, computedRubyReader())
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{test.dirty})
			for _, unknown := range plan.Unknown {
				if unknown.Detail == test.dirty {
					t.Fatalf("unknowns %+v: the path raises no entry of its own", plan.Unknown)
				}
			}
			frontier := map[string]Unknown{}
			for _, unknown := range plan.Unknown {
				if unknown.Reason == UnknownLanguageFrontier {
					frontier[unknown.Detail] = unknown
				}
			}
			if _, raised := frontier[relative]; raised {
				t.Fatalf("frontier %v: a source-bound entry cannot bind a Go path", languageFrontier(plan))
			}
			bore := false
			for _, bearer := range plan.FrontierBearers {
				if bearer.Detail == implicit && bearer.Via == FrontierBearerUnownedPath && bearer.Language == "rb" && bearer.DirtyPath == test.dirty {
					bore = true
				}
			}
			entry, kept := frontier[implicit]
			if !test.bears {
				if kept || bore {
					t.Fatalf("frontier %v, bearers %+v: an owned edit is no unowned path", languageFrontier(plan), plan.FrontierBearers)
				}
				if plan.Scope == ScopeUnknown {
					t.Fatalf("scope %s, unknowns %+v: an owned edit stays a clean selection", plan.Scope, plan.Unknown)
				}
			} else {
				if !kept || !bore {
					t.Fatalf("frontier %v, bearers %+v: the computed reader's %s must bear through the unowned deleted path %s", languageFrontier(plan), plan.FrontierBearers, implicit, test.dirty)
				}
				if !strings.Contains(entry.Hint, strconv.Quote(test.dirty)) {
					t.Fatalf("hint %q, want the deleted path %q named", entry.Hint, test.dirty)
				}
				if plan.Scope != ScopeUnknown {
					t.Fatalf("scope %s, want %s while the computed reader's entry bears", plan.Scope, ScopeUnknown)
				}
			}
			selected := map[string]bool{}
			for _, selection := range plan.Selected {
				selected[selection.UnitID] = true
			}
			for _, id := range test.selected {
				if !selected[id] {
					t.Fatalf("selected %v, want %s", selected, id)
				}
			}
			if selected["go:other"] {
				t.Fatalf("selected %v: go:other neither declares nor imports the path", selected)
			}
		})
	}
}

// AFP-V0-2749 fault injection: every path seed admits as a deletion is listed
// among the unowned paths participants returns, and a kept case of each rule
// is not. The three rules each admit one path of this graph; the injected
// faults are the present twin of each path, a sparse checkout, which none of
// the package or fixture rules may read absence from, and a graph with no disk
// root. A participants that tests only one rule, or a seed that admits a path
// participants drops, fails here.
func TestAdmittedDeletionsStayUnownedInParticipants_AFPV2749(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"pkg/pkg.go", "pkg/present.go", "pkg/testdata/flows/app/present.py", "lib/app.py"} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goUnits := fake{name: "go", units: []Unit{
		{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
	}}
	python := fake{name: "py", units: []Unit{{ID: "py:app", Sources: []string{"lib/app.py"}}}}
	built, err := Build(context.Background(), root, goUnits, python)
	if err != nil {
		t.Fatal(err)
	}
	admitted := []string{"pkg/gone.go", "removed/removed.go", "pkg/testdata/flows/app/gone.py"}
	kept := []string{"pkg/present.go", "pkg/testdata/flows/app/present.py", "lib/app.py", "pkg/pkg.go"}
	cases := map[string]struct {
		graph    *Graph
		admitted []string
	}{
		"disk checkout":   {graph: built, admitted: admitted},
		"sparse checkout": {graph: built.WithSparseCheckout(), admitted: []string{"pkg/gone.go"}},
	}
	rootless := *built
	rootless.root = ""
	cases["no disk root"] = struct {
		graph    *Graph
		admitted []string
	}{graph: &rootless}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			graph := test.graph
			packages := graph.goPackageDirectories()
			dirty := append(append([]string{}, admitted...), kept...)
			_, unowned, _ := graph.participants(map[string]Witness{}, nil, nil, dirty, nil)
			listed := map[string]bool{}
			for _, path := range unowned {
				listed[path] = true
			}
			want := map[string]bool{}
			for _, path := range test.admitted {
				want[path] = true
			}
			for _, path := range dirty {
				if graph.admittedDeletion(path, packages) != want[path] {
					t.Fatalf("admittedDeletion(%s) = %v, want %v", path, !want[path], want[path])
				}
				if listed[path] != want[path] {
					t.Fatalf("unowned %v: %s listed = %v, want %v (seed admits it: %v)", unowned, path, listed[path], want[path], want[path])
				}
			}
			_, unknown := graph.seed(dirty)
			for _, entry := range unknown {
				if want[entry.Detail] {
					t.Fatalf("seed unknowns %+v: admitted %s raises an entry", unknown, entry.Detail)
				}
			}
		})
	}
}
