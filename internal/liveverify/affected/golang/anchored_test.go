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
		// A fixture repository's file: no base places the lone "/repo"
		// there, but placed's two-component "/testdata/repo" keeps the
		// component-run relation, since its root may come from the
		// environment (AFP-V0-2203, review round 6).
		{"internal/x/testdata/repo/lib.js", "[placed/placed_test.go]"},
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

// AFP-V0-2203: an anchored run of two or more components keeps the
// component-run relation at any depth, because the root it is joined to may
// come from the environment, which no literal of the unit places. A reader
// of `os.Getenv("FIXTURES") + "/repo/lib.js"` stays selected for
// app/x/repo/lib.js alongside app/x, the package that encloses the path. The
// lone "/lib.js" of another unit is still placed (AFP-V0-072's residual).
func TestRuntimeRootedRunsKeepComponentRuns_AFPV02203(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"reader/reader_test.go": "package reader\n\nimport \"os\"\n\nvar lib = os.Getenv(\"FIXTURES\") + \"/repo/lib.js\"\n",
		"lone/lone_test.go":     "package lone\n\nimport \"os\"\n\nvar lib = os.Getenv(\"FIXTURES\") + \"/lib.js\"\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"app/x/repo/lib.js"})
	var readers, others []string
	for _, selection := range plan.Selected {
		if selection.Witness.Kind == affected.WitnessPathLiteralReader {
			readers = append(readers, selection.Tests...)
		} else {
			others = append(others, selection.UnitID)
		}
	}
	if got := fmt.Sprint(readers); got != "[reader/reader_test.go]" {
		t.Errorf("path-literal readers = %s, want [reader/reader_test.go]", got)
	}
	if got := fmt.Sprint(others); got != "[go:example.test/m/app/x]" {
		t.Errorf("other selections = %s, want [go:example.test/m/app/x]", got)
	}
}
