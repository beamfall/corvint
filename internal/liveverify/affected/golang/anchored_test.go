package golang_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// pathLiteralReaders returns the tests of every unit the plan for dirty
// selects by a PATH_LITERAL_READER witness, and the ids of every other
// selection.
func pathLiteralReaders(t *testing.T, graph *affected.Graph, dirty string) (readers, others []string) {
	t.Helper()
	for _, selection := range affected.Select(graph, []string{dirty}).Selected {
		if selection.Witness.Kind == affected.WitnessPathLiteralReader {
			readers = append(readers, selection.Tests...)
		} else {
			others = append(others, selection.UnitID)
		}
	}
	return readers, others
}

// AFP-V0-2203 (V1-4173), round 8: an anchored token of a Go unit (a fake
// absolute root such as "/repo", or a fragment joined to a root variable)
// keeps the component-run relation at any depth, because the root it is
// joined to may come from the environment, a flag or a walked directory,
// which no literal of the unit places. So "/repo" names a fixture
// repository's testdata/repo as it names the root's repo/, and "/data/"
// names any data/ directory. A bare "/" names nothing.
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
		{"internal/x/testdata/repo/lib.js", "[climbs/climbs_test.go fake/fake_test.go placed/placed_test.go]"},
		{"repo/lib.js", "[climbs/climbs_test.go fake/fake_test.go]"},
		{"nested/data/f.txt", "[nested/inner/inner_test.go]"},
		{"other/data/f.txt", "[nested/inner/inner_test.go]"},
		{"internal/jslinecov/testdata/repo/lib.js", "[climbs/climbs_test.go fake/fake_test.go placed/placed_test.go]"},
		// Control: a path naming none of the runs selects nothing.
		{"elsewhere/lib.js", "[]"},
	}
	for _, tc := range cases {
		readers, _ := pathLiteralReaders(t, graph, tc.dirty)
		if got := fmt.Sprint(readers); got != tc.want && !(tc.want == "[]" && len(readers) == 0) {
			t.Errorf("%s: path-literal readers = %s, want %s", tc.dirty, got, tc.want)
		}
	}
}

// AFP-V0-2203: a reader of `os.Getenv("FIXTURES") + "/repo/lib.js"` stays
// selected for app/x/repo/lib.js alongside app/x, the package that encloses
// the path, and so does the lone "/lib.js" of another unit, the path's file
// name.
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
	readers, others := pathLiteralReaders(t, graph, "app/x/repo/lib.js")
	if got := fmt.Sprint(readers); got != "[lone/lone_test.go reader/reader_test.go]" {
		t.Errorf("path-literal readers = %s, want [lone/lone_test.go reader/reader_test.go]", got)
	}
	if got := fmt.Sprint(others); got != "[go:example.test/m/app/x]" {
		t.Errorf("other selections = %s, want [go:example.test/m/app/x]", got)
	}
}

// AFP-V0-2203 round 7, closed for the class in round 8: tests that read
// `os.Getenv("FIXTURES") + "/lib.js"` or open `os.Getenv("FIXTURES") +
// "/repo"`, with FIXTURES naming app/x, reach app/x/repo/lib.js though none
// of their literals spells app or x, so both stay selected. Control: an
// anchored lone run naming another file selects nothing.
func TestRuntimeRootedFileNamesStaySelected_AFPV02203(t *testing.T) {
	const read = "package %s\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestRead(t *testing.T) {\n\tif _, err := os.ReadFile(os.Getenv(\"FIXTURES\") + %q); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"reader/reader_test.go": fmt.Sprintf(read, "reader", "/lib.js"),
		"other/other_test.go":   fmt.Sprintf(read, "other", "/other.js"),
		"dir/dir_test.go":       fmt.Sprintf(read, "dir", "/repo"),
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	readers, others := pathLiteralReaders(t, graph, "app/x/repo/lib.js")
	if got := fmt.Sprint(readers); got != "[dir/dir_test.go reader/reader_test.go]" {
		t.Errorf("path-literal readers = %s, want [dir/dir_test.go reader/reader_test.go]", got)
	}
	if got := fmt.Sprint(others); got != "[go:example.test/m/app/x]" {
		t.Errorf("other selections = %s, want [go:example.test/m/app/x]", got)
	}
}

// AFP-V0-2203 round 8 review counterexample: a test walks
// `os.Getenv("FIXTURES") + "/repo"`, or `filepath.Join(os.Getenv("FIXTURES"),
// "repo")` whose lone "repo" is unanchored, with FIXTURES=app/x, so it reads
// every file below app/x/repo though no literal of its unit spells app/x.
// Neither placement (AFP-V0-072) nor the file-name-only rule (V1-0290) may
// drop either walker. Control: a walk of another directory selects nothing.
// The selector half is gateselect's TestRuleCRuntimeRootedDirectoryWalk.
func TestRuntimeRootedDirectoryWalkStaysSelected_AFPV02203(t *testing.T) {
	const walk = "package %s\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestWalk(t *testing.T) {\n\t_ = os.Getenv\n\t_ = filepath.WalkDir(%s, func(string, fs.DirEntry, error) error { return nil })\n}\n"
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":                "module example.test/m\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"walker/walker_test.go": fmt.Sprintf(walk, "walker", `os.Getenv("FIXTURES")+"/repo"`),
		"joined/joined_test.go": fmt.Sprintf(walk, "joined", `filepath.Join(os.Getenv("FIXTURES"), "repo")`),
		"other/other_test.go":   fmt.Sprintf(walk, "other", `os.Getenv("FIXTURES")+"/elsewhere"`),
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	readers, others := pathLiteralReaders(t, graph, "app/x/repo/lib.js")
	if got := fmt.Sprint(readers); got != "[joined/joined_test.go walker/walker_test.go]" {
		t.Errorf("path-literal readers = %s, want [joined/joined_test.go walker/walker_test.go]", got)
	}
	if got := fmt.Sprint(others); got != "[go:example.test/m/app/x]" {
		t.Errorf("other selections = %s, want [go:example.test/m/app/x]", got)
	}
}
