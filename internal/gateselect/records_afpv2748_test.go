// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2748 (V1-4933, review fix round 2, finding P1): a test that walks
// `../docs`, reading every Markdown file below it, and also calls
// `filepath.Glob("../docs/*/2026-*.md")` reads a new build-log entry outside
// that glob (`docs/build-log/2027-entry.md`). Neither selector selects its
// package for that entry: the planner's glob does not admit it and no literal
// joins `../docs` to `build-log`, and gate-affected-select has no glob reader.
// So the record-readers ratchet must keep the package a walker, which no
// record entry admits, rather than a reader its constrained glob suggests.
// Before AFP-V0-2748 Graph.RecordReaders returned it among the readers.
func TestRecordWalkerWithGlobSkippedByBothSelectors_AFPV2748(t *testing.T) {
	const outside = "docs/build-log/2027-entry.md"                                                     //corvint:fixture-path
	const inside = "docs/build-log/2026-11-06-selgo.md"                                                //corvint:fixture-path
	const glob = "../docs/*/2026-*.md"                                                                 //corvint:fixture-path
	const walk = `filepath.WalkDir("../docs", func(name string, entry fs.DirEntry, err error) error {` //corvint:fixture-path
	root := writeFixture(t, map[string]string{
		"go.mod": "module example.com/fixture\n\ngo 1.22\n",
		"globwalker/walk_test.go": "package globwalker\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" + //corvint:fixture-path
			"func TestWalk(t *testing.T) {\n\tif err := " + walk + "\n\t\tif err != nil || entry.IsDir() || !strings.HasSuffix(name, \".md\") {\n\t\t\treturn err\n\t\t}\n\t\t_, err = os.ReadFile(name)\n\t\treturn err\n\t}); err != nil {\n\t\tt.Fatal(err)\n\t}\n" +
			"\tif _, err := filepath.Glob(\"" + glob + "\"); err != nil {\n\t\tt.Fatal(err)\n\t}\n}\n",
		inside:  "# entry\n",
		outside: "# entry\n",
	})
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const id = "go:" + fixtureModule + "/globwalker"
	readers, walkers := graph.RecordReaders(affected.BuildLogDirectory)
	if slices.Contains(readers, id) || !slices.Contains(walkers, id) {
		t.Errorf("RecordReaders(%s) = readers %v, walkers %v; want %s a walker", affected.BuildLogDirectory, readers, walkers, id)
	}
	for _, selection := range affected.Select(graph, []string{outside}).Selected {
		if selection.UnitID == id {
			t.Errorf("the planner selects %s for %s (%s), outside its glob", id, outside, selection.Witness.Kind)
		}
	}
	var plan Receipt
	plan.Provider.Go.State = "RUNNABLE"
	plan.Plan.Dirty = []string{outside}
	for _, line := range selectPackages(plan, fixtureModule, root, true) {
		if strings.Contains(line, fixtureModule+"/globwalker") {
			t.Errorf("gate-affected-select selects the walker for %s: %s", outside, line)
		}
	}
	// The glob does admit a 2026 entry, so the planner selects the package
	// for it: a narrower match, not a read of every entry.
	if !slices.ContainsFunc(affected.Select(graph, []string{inside}).Selected, func(selection affected.Selection) bool {
		return selection.UnitID == id && selection.Witness.Kind == affected.WitnessPathGlobReader
	}) {
		t.Errorf("the planner does not select %s as a glob reader of %s", id, inside)
	}
}
