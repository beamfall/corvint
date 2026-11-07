// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"path/filepath"
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
