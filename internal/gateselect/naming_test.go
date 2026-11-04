// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// namingFixture holds the shared fixture of AFP-V0-2240 to AFP-V0-2242. Every
// token sits in a test file, so no package is unresolved, and every dirty path
// the tests probe lies outside every package, so a package reaches it only by
// rule (c).
func namingFixture(t testing.TB) string {
	return writeFixture(t, map[string]string{
		"go.mod": "module example.com/fixture\n\ngo 1.22\n",
		// AFP-V0-2240: anchored tokens.
		"fake/fake_test.go":          "package fake\n\nvar git, slash = \"/repo/.git\", \"/\"\n\nfunc dir(root string) string { return root + \"/repo\" }\n",
		"nested/inner/inner_test.go": "package inner\n\nfunc data(root string) string { return root + \"/data/\" }\n",
		"placed/placed_test.go":      "package placed\n\nvar sub = \"jsprobe\"\nvar parent = \"internal\"\n\nfunc dir(root string) string { return root + \"/testdata/repo\" }\n",
		"climbs/climbs_test.go":      "package climbs\n\nvar outside = \"/../repo\"\n",
		"slash/slash_test.go":        "package slash\n\nvar separator = \"/\"\n",
		"modpath/modpath_test.go":    "package modpath\n\nvar dir = \"example.com/fixture/internal/x\"\n",
		"own/own_test.go":            "package own\n\nfunc dir(root string) string { return root + \"/fixture\" }\n",
		// AFP-V0-2241: unanchored tokens.
		"lone/lone_test.go":         "package lone\n\nvar name = \"INDEX.json\"\n",
		"lonepl/lonepl_test.go":     "package lonepl\n\nvar name, a, b = \"INDEX.json\", \"docs\", \"specs\"\n",
		"dirshape/dirshape_test.go": "package dirshape\n\nvar dir = \"notes/\"\n",
		"climb/climb_test.go":       "package climb\n\nvar note = \"../notes\"\n",
		"multi/multi_test.go":       "package multi\n\nvar lib = \"testdata/repo/lib.js\"\n",
		"outdir/outdir_test.go":     "package outdir\n\nvar out, calc = \"out\", \"calc.js\"\n",
		// The `..\x` lexicon difference (AFP-V0-765): the planner reads the
		// climbing `../notes`, this index `..` and a lone `notes`.
		"backslash/backslash_test.go": "package backslash\n\nvar note = `..\\notes`\n",
		"mixed/mixed_test.go":         "package mixed\n\nvar note = `a/..\\notes`\n",
		// The closing-tag lexicon difference (AFP-V0-505/840): the planner
		// reads the unanchored `repo/page.html`, whose `repo` may end a longer
		// name; this index keeps that token and the anchored one.
		"tag/tag_test.go":       "package tag\n\nvar page = \"</repo/page.html>\"\n",
		"esctag/esctag_test.go": "package esctag\n\nvar page = \"&lt;/repo/page.html&gt;\"\n",
	})
}

// namingPaths are the dirty paths the tests probe, none inside a package.
var namingPaths = []string{
	"internal/x/testdata/repo/lib.js",
	"internal/jsprobe/testdata/repo/lib.js",
	"internal/jsprobe/testdata/repo/out/calc.js",
	"repo/lib.js",
	"app/xrepo/page.html",
	"nested/data/f.txt",
	"other/data/f.txt",
	"internal/x/y.txt",
	"docs/specs/INDEX.json",
	"notes/x.md",
	"out/calc.js",
}

// TestRuleCAnchoredTokens_AFPV02240 replays the planner's AFP-V0-2203 cases
// on rule (c): an anchored token no longer names a same-named directory
// anywhere in the tree (a fixture repository's testdata/repo), but still
// names the root's repo/, an ancestor's data/, a path placed by the
// package's own literals, a module-path directory, and anything in the
// holder's own subtree; a bare "/" names nothing. The controls each keep a
// reader the narrowing must not drop.
func TestRuleCAnchoredTokens_AFPV02240(t *testing.T) {
	index, err := indexRepository(namingFixture(t), fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty string
		want  []string
	}{
		// Narrowed: no base places the lone "/repo" in a fixture
		// repository; placed's two-component "/testdata/repo" keeps the
		// component-run relation (its root may come from the environment).
		{"internal/x/testdata/repo/lib.js", []string{"modpath", "multi", "placed"}},
		// Controls: the root's repo/ ("/repo", and "/../repo" from a module
		// rooted at climbs/), an ancestor's data/, placed components.
		{"repo/lib.js", []string{"climbs", "fake"}},
		{"nested/data/f.txt", []string{"nested/inner"}},
		{"other/data/f.txt", nil},
		{"internal/jsprobe/testdata/repo/lib.js", []string{"multi", "placed"}},
		{"internal/x/y.txt", []string{"modpath"}},
		// A closing tag's unanchored token (AFP-V0-2242): `repo` may end
		// `xrepo`.
		{"app/xrepo/page.html", []string{"esctag", "tag"}},
		// Control: the holder's own subtree keeps every token.
		{"own/testdata/fixture/f.txt", []string{"own"}},
	}
	for _, tc := range cases {
		got := index.readers(tc.dirty)
		if !slices.Equal(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
			t.Errorf("readers(%s) = %v, want %v", tc.dirty, got, tc.want)
		}
	}
}

// TestRuleCRuntimeRootedRuns_AFPV02240 is the review counterexample to
// placing every anchored run: a reader of `os.Getenv("FIXTURES") +
// "/repo/lib.js"` joins its run to a root no literal spells, so it stays a
// reader of app/x/repo/lib.js wherever that run lies, next to app/x, the
// package that encloses the path. Round 7: so does the lone "/lib.js" of
// another package, which is the path's file name.
func TestRuleCRuntimeRootedRuns_AFPV02240(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"reader/reader_test.go": "package reader\n\nimport \"os\"\n\nvar lib = os.Getenv(\"FIXTURES\") + \"/repo/lib.js\"\n",
		"lone/lone_test.go":     "package lone\n\nimport \"os\"\n\nvar lib = os.Getenv(\"FIXTURES\") + \"/lib.js\"\n",
	})
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"app/x/repo/lib.js"}
	if got, want := verdict(plan, root), "run example.com/fixture/app/x example.com/fixture/lone example.com/fixture/reader"; got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// TestRuleCRuntimeRootedFileNames_AFPV02240 is round 7's review
// counterexample: a test that reads `os.Getenv("FIXTURES") + "/lib.js"`, with
// FIXTURES naming app/x/repo, reads app/x/repo/lib.js though no literal of
// its package spells app, x or repo, so placement cannot bound its lone
// anchored run once that run is the file name. The reader stays in the
// verdict beside app/x, which encloses the path. Controls: an anchored lone
// run naming another file, and one naming only a directory component the
// package does not place (`"/repo"`, the AFP-V0-2240 narrowing), are no
// readers.
func TestRuleCRuntimeRootedFileNames_AFPV02240(t *testing.T) {
	const read = "package %s\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestRead(t *testing.T) {\n\tif _, err := os.ReadFile(os.Getenv(\"FIXTURES\") + %q); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n"
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"reader/reader_test.go": fmt.Sprintf(read, "reader", "/lib.js"),
		"other/other_test.go":   fmt.Sprintf(read, "other", "/other.js"),
		"dir/dir_test.go":       fmt.Sprintf(read, "dir", "/repo"),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	if got := index.readers("app/x/repo/lib.js"); !slices.Equal(got, []string{"reader"}) {
		t.Errorf("readers(app/x/repo/lib.js) = %v, want [reader]", got)
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"app/x/repo/lib.js"}
	if got, want := verdict(plan, root), "run example.com/fixture/app/x example.com/fixture/reader"; got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// TestRuleCLoneTokens_AFPV02241 replays V1-0290 and AFP-V0-072 on rule (c):
// a lone component of an unanchored token names only a file of that name, and
// only where the package's literals place its directories; a directory-shaped
// token names no path. Controls: a placed file name, a climbing token, a
// multi-component run, and a package whose `..\notes` this lexicon splits
// (it stays a reader of notes/x.md, as the planner's climbing `../notes` is).
func TestRuleCLoneTokens_AFPV02241(t *testing.T) {
	index, err := indexRepository(namingFixture(t), fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty string
		want  []string
	}{
		{"docs/specs/INDEX.json", []string{"lonepl"}},
		{"notes/x.md", []string{"backslash", "climb", "mixed"}},
		// "out" and "calc.js" name out/calc.js from the root, but not the
		// same names below a fixture directory none of its literals holds;
		// placed's anchored "/testdata/repo" still reaches it.
		{"out/calc.js", []string{"outdir"}},
		{"internal/jsprobe/testdata/repo/out/calc.js", []string{"placed"}},
	}
	for _, tc := range cases {
		got := index.readers(tc.dirty)
		if !slices.Equal(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
			t.Errorf("readers(%s) = %v, want %v", tc.dirty, got, tc.want)
		}
	}
	batched := index.pathReaders(namingPaths)
	for i, p := range namingPaths {
		if want := index.readers(p); fmt.Sprint(batched[i]) != fmt.Sprint(want) {
			t.Errorf("%s: pathReaders %v, readers %v", p, batched[i], want)
		}
	}
	lines, err := packageBounds(fixtureModule, namingFixture(t), []string{"internal/jsprobe/testdata/repo/out/calc.js", "out/calc.js"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(lines, ""); strings.Contains(got, "outdir reader internal/") || !strings.Contains(got, "bound example.com/fixture/outdir reader out/calc.js") {
		t.Errorf("-bounds does not follow rule (c):\n%s", got)
	}
}

// TestRuleCMatchesThePlanner_AFPV02242 holds rule (c) equal to the planner's
// path-literal readers on the shared fixture, path by path, so a change to
// either relation that the other does not follow fails here. The one
// intended difference is the `..\notes` package, which this index keeps as a
// reader of a path the planner's climbing token also names, so it is equal
// there too; a package the planner selects and rule (c) does not is the
// failure this test exists for. The closing-tag packages (`</repo/page.html>`,
// `&lt;/repo/page.html&gt;`) are readers of app/xrepo/page.html only through the
// planner's unanchored token, which this index must keep too.
func TestRuleCMatchesThePlanner_AFPV02242(t *testing.T) {
	root := namingFixture(t)
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, dirty := range namingPaths {
		var planner []string
		for _, selection := range affected.Select(graph, []string{dirty}).Selected {
			if selection.Witness.Kind != affected.WitnessPathLiteralReader {
				continue
			}
			if directory, ok := strings.CutPrefix(selection.UnitID, "go:"+fixtureModule+"/"); ok {
				planner = append(planner, directory)
			}
		}
		sort.Strings(planner)
		gate := index.readers(dirty)
		compared += len(gate)
		if !slices.Equal(planner, gate) && !(len(planner) == 0 && len(gate) == 0) {
			t.Errorf("%s: planner path-literal readers %v, rule (c) readers %v", dirty, planner, gate)
		}
	}
	if compared == 0 {
		t.Fatal("the fixture names no reader; the comparison proves nothing")
	}
}
