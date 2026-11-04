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
	})
}

// namingPaths are the dirty paths the tests probe, none inside a package.
var namingPaths = []string{
	"internal/x/testdata/repo/lib.js",
	"internal/jsprobe/testdata/repo/lib.js",
	"internal/jsprobe/testdata/repo/out/calc.js",
	"repo/lib.js",
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
		// Narrowed: no base places "/repo" in a fixture repository.
		{"internal/x/testdata/repo/lib.js", []string{"modpath", "multi"}},
		// Controls: the root's repo/ ("/repo", and "/../repo" from a module
		// rooted at climbs/), an ancestor's data/, placed components.
		{"repo/lib.js", []string{"climbs", "fake"}},
		{"nested/data/f.txt", []string{"nested/inner"}},
		{"other/data/f.txt", nil},
		{"internal/jsprobe/testdata/repo/lib.js", []string{"multi", "placed"}},
		{"internal/x/y.txt", []string{"modpath"}},
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
// failure this test exists for.
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
