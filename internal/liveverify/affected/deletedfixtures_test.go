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

// AFP-V0-2740 (V1-4931): a non-Go fixture deleted or moved away from below a
// Go-hosted `testdata` directory is fixture data, not an unindexed source. The
// shape is beamfall/corvint#114's: the TypeScript plugin still lists files of
// a sibling fixture below the same `testdata`, so the AFP-V0-2440 claim on the
// whole directory does not hold, but nothing of the plugin survives in the
// moved fixture's own subtree. The plan raises no unknown for the path, still
// selects the hosting package (rule (b)) and a Go package elsewhere that names
// the path (rule (c)), so no reader of the deleted bytes is dropped. Each
// control keeps UNINDEXED_SOURCE_PATH: the file present on disk, a plugin unit
// left in the same fixture subtree, no Go host, a path outside `testdata`, a
// sparse checkout and a path inside a directory the plugin does not observe.
// With deletedFixture removed from seed the first case is
// UNINDEXED_SOURCE_PATH again, which forces FULL in both selectors.
func TestDeletedFixtureIsFixtureData_AFPV2740(t *testing.T) {
	const deleted = "pkg/testdata/flows/app/web/profile.x"
	goUnits := fake{name: "go", units: []Unit{
		{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
		{ID: "go:reader", Sources: []string{"reader/reader.go"}, Tests: []string{"reader/reader_test.go"}, PathTokens: []string{"../pkg/testdata/flows/app/web/profile.x"}},
		{ID: "go:other", Sources: []string{"other/other.go"}, Tests: []string{"other/other_test.go"}},
	}}
	sibling := fake{name: "x", units: []Unit{{ID: "x:kept", Tests: []string{"pkg/testdata/kept/suite/a_test.x"}}}}
	sameSubtree := fake{name: "x", units: []Unit{{ID: "x:flows", Tests: []string{"pkg/testdata/flows/e2e/a_test.x"}}}}
	unobserving := fake{name: "x", units: sibling.units, unobserved: []string{"pkg/testdata/flows"}}
	cases := []struct {
		name      string
		languages []Language
		dirty     string
		onDisk    bool
		sparse    bool
		want      string // "" for no unknown on the path
	}{
		{"deleted, sibling fixture still indexed", []Language{goUnits, sibling}, deleted, false, false, ""},
		{"deleted file directly in testdata, a fixture below it indexed", []Language{goUnits, sibling}, "pkg/testdata/gone.x", false, false, UnknownUnindexedSourcePath},
		{"present on disk", []Language{goUnits, sibling}, deleted, true, false, UnknownUnindexedSourcePath},
		{"the plugin lists a unit in the same fixture subtree", []Language{goUnits, sameSubtree}, deleted, false, false, UnknownUnindexedSourcePath},
		{"no Go unit hosts the testdata directory", []Language{sibling}, deleted, false, false, UnknownUnindexedSourcePath},
		{"outside testdata", []Language{goUnits, sibling}, "pkg/flows/app/web/profile.x", false, false, UnknownUnindexedSourcePath},
		{"sparse checkout", []Language{goUnits, sibling}, deleted, false, true, UnknownUnindexedSourcePath},
		{"inside a directory the plugin does not observe", []Language{goUnits, unobserving}, deleted, false, false, UnknownLanguageFrontier},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.onDisk {
				if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tc.dirty)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, tc.dirty), []byte("fixture\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			graph, err := Build(context.Background(), root, tc.languages...)
			if err != nil {
				t.Fatal(err)
			}
			graph.sparse = tc.sparse
			plan := Select(graph, []string{tc.dirty})
			got := ""
			for _, unknown := range plan.Unknown {
				if unknown.Detail == tc.dirty || (unknown.Reason == UnknownLanguageFrontier && filepath.Base(unknown.Detail) == filepath.Base(tc.dirty)) {
					got = unknown.Reason
				}
			}
			if got != tc.want {
				t.Fatalf("unknowns %+v, want %q on %s", plan.Unknown, tc.want, tc.dirty)
			}
			if tc.want != "" {
				return
			}
			if len(plan.Unknown) != 0 {
				t.Fatalf("unknowns %+v, want none", plan.Unknown)
			}
			selected := map[string]bool{}
			for _, selection := range plan.Selected {
				selected[selection.UnitID] = true
			}
			if !selected["go:pkg"] || !selected["go:reader"] {
				t.Fatalf("selected %v, want the hosting package go:pkg and the literal reader go:reader", selected)
			}
			if selected["go:other"] {
				t.Fatalf("selected %v: go:other neither hosts nor names the path", selected)
			}
		})
	}
}

// AFP-V0-2740 (review fix round 3): the deleted fixture stays unowned for
// every frontier rule although its claimant's Owns still claims it. A Python
// fixture deleted from below a Go-hosted `testdata` directory, with a Python
// unit left in a sibling fixture, raises no UNINDEXED_SOURCE_PATH. A third
// plugin, Ruby-shaped, owns no dirty path and reaches no changed unit; its spec
// reads fixture files through a computed name, which it reports as a path-open
// implicit-test-dependency entry. That entry still bears through the unowned
// path (AFP-V0-511), with the hint naming the deleted file, so the plan stays
// UNKNOWN and the computed reader's suite is not skipped. Its source-bound and
// fixture-closed entries do not bear, as for any unowned fixture path, and the
// claimant still takes part with its own entry. With participants reading
// claimantsOf alone, Python claims the path, the path leaves the unowned-path
// checks and the Ruby entry is lost: a clean plan that skips the reader.
func TestDeletedFixtureStaysUnownedForOtherPlugins_AFPV2740(t *testing.T) {
	const (
		deleted   = "pkg/testdata/flows/app/input.py"
		ambiguous = "rb:ambiguous-framework:spec/reader_spec.rb"
		implicit  = "rb:implicit-test-dependency:spec/reader_spec.rb"
		relative  = "rb:relative-load:spec/reader_spec.rb:2 `require_relative(name)`"
		claimant  = "py:dynamic-import:pkg/testdata/kept/suite/test_a.py:1 `importlib.import_module(name)`"
	)
	goUnits := fake{name: "go", units: []Unit{
		{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
		{ID: "go:other", Sources: []string{"other/other.go"}, Tests: []string{"other/other_test.go"}},
	}}
	python := fake{name: "py", frontier: []string{claimant}, units: []Unit{
		{ID: "py:kept", Tests: []string{"pkg/testdata/kept/suite/test_a.py"}},
	}}
	ruby := reaching{
		fake: fake{name: "rb", frontier: []string{ambiguous, implicit, relative}, units: []Unit{
			{ID: "rb:spec", Sources: []string{"lib/app.rb"}, Tests: []string{"spec/reader_spec.rb"}},
		}},
		sourceBound:   []string{"rb:relative-load"},
		fixtureClosed: []string{"rb:ambiguous-framework"},
	}
	graph, err := Build(context.Background(), t.TempDir(), goUnits, python, ruby)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{deleted})
	for _, unknown := range plan.Unknown {
		if unknown.Detail == deleted {
			t.Fatalf("unknowns %+v: the deleted fixture raises no entry of its own", plan.Unknown)
		}
	}
	frontier := map[string]Unknown{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier {
			frontier[unknown.Detail] = unknown
		}
	}
	entry, kept := frontier[implicit]
	if !kept {
		t.Fatalf("frontier %v: the computed reader's %s must bear through the unowned deleted fixture", languageFrontier(plan), implicit)
	}
	if !strings.Contains(entry.Hint, strconv.Quote(deleted)) {
		t.Fatalf("hint %q, want the deleted fixture %q named", entry.Hint, deleted)
	}
	if _, raised := frontier[relative]; raised {
		t.Fatalf("frontier %v: a source-bound entry cannot bind the fixture", languageFrontier(plan))
	}
	if _, raised := frontier[ambiguous]; raised {
		t.Fatalf("frontier %v: a fixture-closed entry outside the fixture cannot bind it", languageFrontier(plan))
	}
	if _, raised := frontier[claimant]; !raised {
		t.Fatalf("frontier %v: the claimant still takes part with its own entry", languageFrontier(plan))
	}
	bore := false
	for _, bearer := range plan.FrontierBearers {
		if bearer.Detail == implicit && bearer.Via == FrontierBearerUnownedPath && bearer.Language == "rb" && bearer.DirtyPath == deleted {
			bore = true
		}
	}
	if !bore {
		t.Fatalf("bearers %+v, want %s borne by the unowned path %s", plan.FrontierBearers, implicit, deleted)
	}
	if plan.Scope != ScopeUnknown {
		t.Fatalf("scope %s, want %s while the computed reader's entry bears", plan.Scope, ScopeUnknown)
	}
	selected := map[string]bool{}
	for _, selection := range plan.Selected {
		selected[selection.UnitID] = true
	}
	if !selected["go:pkg"] || selected["go:other"] {
		t.Fatalf("selected %v, want the hosting package go:pkg and not go:other", selected)
	}
}
