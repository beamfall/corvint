package golang_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2203 (V1-4173): an anchored token of a Go unit (a fake absolute root
// such as "/repo", or a fragment joined to a root variable) names a dirty
// path only from the repository root, from the unit's own directory or one
// of its ancestors (where its module's directory lies), or after directory
// components a literal of the unit could join. It no longer names a
// same-named directory anywhere in the tree, such as a fixture repository's
// testdata/repo. A bare "/" names nothing.
func TestAnchoredTokensResolveFromModuleBases_AFPV02203(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                          "module example.test/m\n",
		"fake/fake_test.go":               "package fake\n\nvar git, slash = \"/repo/.git\", \"/\"\n\nfunc dir(root string) string { return root + \"/repo\" }\n",
		"nested/inner/inner_test.go":      "package inner\n\nfunc data(root string) string { return root + \"/data/\" }\n",
		"placed/placed_test.go":           "package placed\n\nvar sub = \"jslinecov\"\nvar parent = \"internal\"\n\nfunc dir(root string) string { return root + \"/testdata/repo\" }\n",
		"climbs/climbs_test.go":           "package climbs\n\nvar outside = \"/../repo\"\n",
		"slash/slash_test.go":             "package slash\n\nvar separator = \"/\"\n",
		"internal/x/testdata/repo/lib.js": "export const x = 1\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ dirty, want string }{
		// A fixture repository's file: no base places "/repo" there.
		{"internal/x/testdata/repo/lib.js", "[]"},
		// The repository root's repo/ directory is what "/repo" names, and
		// what "/../repo" names from a module rooted at climbs/.
		{"repo/lib.js", "[climbs/climbs_test.go fake/fake_test.go]"},
		// An ancestor of the unit's directory may be its module's.
		{"nested/data/f.txt", "[nested/inner/inner_test.go]"},
		{"other/data/f.txt", "[]"},
		// Every component before the run is a literal the unit could join.
		{"internal/jslinecov/testdata/repo/lib.js", "[placed/placed_test.go]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, []string{tc.dirty})
		var readers []string
		for _, selection := range plan.Selected {
			if selection.Witness.Kind == affected.WitnessPathLiteralReader {
				readers = append(readers, selection.Tests...)
			}
		}
		if got := fmt.Sprint(readers); got != tc.want && !(tc.want == "[]" && len(readers) == 0) {
			t.Errorf("%s: path-literal readers = %s, want %s", tc.dirty, got, tc.want)
		}
	}
}
