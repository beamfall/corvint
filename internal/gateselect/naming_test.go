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
		// `..\x` (AFP-V0-506): since round 8 both lexicons read the
		// climbing `../notes` (parentBackslash).
		"backslash/backslash_test.go": "package backslash\n\nvar note = `..\\notes`\n",
		"mixed/mixed_test.go":         "package mixed\n\nvar note = `a/..\\notes`\n",
		// The closing-tag lexicon difference (AFP-V0-505/840): the planner
		// reads the unanchored `repo/page.html`, whose `repo` may end a longer
		// name; this index keeps that token and the anchored one.
		"tag/tag_test.go":       "package tag\n\nvar page = \"</repo/page.html>\"\n",
		"esctag/esctag_test.go": "package esctag\n\nvar page = \"&lt;/repo/page.html&gt;\"\n",
		// Round 8: a runtime prefix joined to a backslash-separated
		// literal. Both lexicons read the one token `foo../notes`
		// (parentBackslash), whose run names app/prefoo../notes.
		"prefix/prefix_test.go": "package prefix\n\nimport (\n\t\"os\"\n\t\"strings\"\n)\n\nvar notes = os.Getenv(\"PREFIX\") + strings.ReplaceAll(\"foo..\\\\notes\", \"\\\\\", \"/\")\n",
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
	"app/prefoo../notes",
}

// TestRuleCAnchoredTokens_AFPV02240 pins rule (c)'s retention of anchored
// tokens (round 8): a path literal may be joined to a root that no literal
// of its package spells (an environment variable, a flag, a walked
// directory), so an anchored run names every path holding it at any depth
// and placement drops no reader. The fixture repository's testdata/repo is
// named by "/repo" as the root's repo/ is; a bare "/" still names nothing
// (the AFP-V0-765 residue, slash is in no row).
func TestRuleCAnchoredTokens_AFPV02240(t *testing.T) {
	index, err := indexRepository(namingFixture(t), fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty string
		want  []string
	}{
		{"internal/x/testdata/repo/lib.js", []string{"climbs", "fake", "modpath", "multi", "placed"}},
		{"repo/lib.js", []string{"climbs", "fake"}},
		{"nested/data/f.txt", []string{"nested/inner"}},
		{"other/data/f.txt", []string{"nested/inner"}},
		{"internal/jsprobe/testdata/repo/lib.js", []string{"climbs", "fake", "multi", "placed"}},
		{"internal/x/y.txt", []string{"modpath", "placed"}},
		{"app/xrepo/page.html", []string{"esctag", "tag"}},
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
// package that encloses the path. So does the lone "/lib.js" of another
// package, which is the path's file name.
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
// counterexample, closed for the whole class in round 8: a test that reads
// `os.Getenv("FIXTURES") + "/lib.js"`, or one that opens the directory
// `os.Getenv("FIXTURES") + "/repo"`, with FIXTURES naming app/x, reaches
// app/x/repo/lib.js though no literal of its package spells app or x. Both
// stay readers. Control: an anchored run naming another file is no reader.
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
	if got := index.readers("app/x/repo/lib.js"); !slices.Equal(got, []string{"dir", "reader"}) {
		t.Errorf("readers(app/x/repo/lib.js) = %v, want [dir reader]", got)
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"app/x/repo/lib.js"}
	if got, want := verdict(plan, root), "run example.com/fixture/app/x example.com/fixture/dir example.com/fixture/reader"; got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
}

// TestRuleCRuntimeRootedDirectoryWalk_AFPV02240 is round 8's review
// counterexample: a test walks `os.Getenv("FIXTURES") + "/repo"` (or the
// filepath.Join form, whose "repo" is unanchored) with FIXTURES=app/x, so it
// reads every file below app/x/repo though no literal of its package spells
// app/x. Rule (c), its verdict and -bounds all keep both walkers. The
// planner half is golang's TestRuntimeRootedDirectoryWalkStaysSelected.
func TestRuleCRuntimeRootedDirectoryWalk_AFPV02240(t *testing.T) {
	const walk = "package %s\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestWalk(t *testing.T) {\n\t_ = os.Getenv\n\t_ = filepath.WalkDir(%s, func(string, fs.DirEntry, error) error { return nil })\n}\n"
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"app/x/x.go":            "package x\n",
		"app/x/x_test.go":       "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n",
		"app/x/repo/lib.js":     "export const x = 1\n",
		"walker/walker_test.go": fmt.Sprintf(walk, "walker", `os.Getenv("FIXTURES")+"/repo"`),
		"joined/joined_test.go": fmt.Sprintf(walk, "joined", `filepath.Join(os.Getenv("FIXTURES"), "repo")`),
		"other/other_test.go":   fmt.Sprintf(walk, "other", `os.Getenv("FIXTURES")+"/elsewhere"`),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	if got := index.readers("app/x/repo/lib.js"); !slices.Equal(got, []string{"joined", "walker"}) {
		t.Errorf("readers(app/x/repo/lib.js) = %v, want [joined walker]", got)
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{"app/x/repo/lib.js"}
	if got, want := verdict(plan, root), "run example.com/fixture/app/x example.com/fixture/joined example.com/fixture/walker"; got != want {
		t.Fatalf("verdict = %q, want %q", got, want)
	}
	lines, err := packageBounds(fixtureModule, root, []string{"app/x/repo/lib.js"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "")
	for _, want := range []string{"bound example.com/fixture/walker reader app/x/repo/lib.js", "bound example.com/fixture/joined reader app/x/repo/lib.js"} {
		if !strings.Contains(got, want) {
			t.Errorf("-bounds misses %q:\n%s", want, got)
		}
	}
}

// TestRuleCLoneTokens_AFPV02241 pins rule (c)'s retention of unanchored
// tokens (round 8): a lone component names any path component of that name
// at any depth, because the literal may be joined to a runtime root, so
// neither V1-0290's file-name-only rule nor AFP-V0-072's placement drops a
// Go reader. Round 8's backslash case: `os.Getenv("PREFIX") +
// strings.ReplaceAll("foo..\\notes", "\\", "/")` lexes as the one token
// `foo../notes` here as in the planner, and -bounds keeps app/prefoo../notes.
func TestRuleCLoneTokens_AFPV02241(t *testing.T) {
	index, err := indexRepository(namingFixture(t), fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty string
		want  []string
	}{
		{"docs/specs/INDEX.json", []string{"lone", "lonepl"}},
		{"notes/x.md", []string{"backslash", "climb", "dirshape", "mixed"}},
		{"out/calc.js", []string{"outdir"}},
		{"internal/jsprobe/testdata/repo/out/calc.js", []string{"climbs", "fake", "outdir", "placed"}},
		{"app/prefoo../notes", []string{"backslash", "climb", "dirshape", "mixed", "prefix"}},
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
	lines, err := packageBounds(fixtureModule, namingFixture(t), []string{"internal/jsprobe/testdata/repo/out/calc.js", "out/calc.js", "app/prefoo../notes"})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(lines, "")
	for _, want := range []string{
		"bound example.com/fixture/outdir reader out/calc.js",
		"bound example.com/fixture/outdir reader internal/jsprobe/testdata/repo/out/calc.js",
		"bound example.com/fixture/prefix reader app/prefoo../notes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("-bounds misses %q:\n%s", want, got)
		}
	}
}

// TestRuleCMatchesThePlanner_AFPV02242 holds rule (c) equal to the planner's
// path-literal readers on the shared fixture, path by path, so a change to
// either relation that the other does not follow fails here. Since round 8
// both lexicons normalise `..\` alike (parentBackslash) and both relations
// are the any-depth component-run match, so there is no intended difference:
// the `..\notes` packages and the runtime-prefixed `foo..\\notes` package
// read the same tokens in both, and a package the planner selects and rule
// (c) does not is the failure this test exists for.
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
