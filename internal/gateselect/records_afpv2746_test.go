// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-2746 (V1-4933, review finding P1): a test that reads a record entry
// through separately joined literals, `os.ReadFile(filepath.Join("../docs",
// "build-log", "2026-11-06-selgo.md"))`, holds no single literal that
// resolves to the record directory, yet reads the entry. Both selectors keep
// it, from its own directory (`reader`) and two levels down (`deep/dir`),
// for a build-log entry and a sealed CEM. A package whose ancestor literal is
// joined to another directory (`docs/build`, `.corvint/change`) or file
// (`docs/README.md`) reads no record entry and stays narrowed, as
// AFP-V0-2743 and AFP-V0-2744 narrow it; one that computes the directory's
// name at run time is AFP-V0-2747's walker. The
// planner's selection stays a subset of gate-affected-select's. Removing the
// join from either selector drops `reader` and `deep/dir` from it.
func TestRecordEntryJoinedReadersSelectedByBothSelectors_AFPV2746(t *testing.T) {
	const sealedName = "0123456789abcdef0123456789abcdef01234567.cem.json"
	const sealed = ".corvint/changes/" + sealedName    //corvint:fixture-path
	const entry = "docs/build-log/2026-11-06-selgo.md" //corvint:fixture-path
	test := func(name, body string) string {
		return "package " + name + "\n\nimport (\n\t\"os\"\n\t\"path/filepath\"\n\t\"testing\"\n)\n\nfunc TestRead(t *testing.T) {\n" + body + "}\n"
	}
	read := func(call string) string {
		return "\tif _, err := " + call + "; err != nil {\n\t\tt.Fatal(err)\n\t}\n"
	}
	root := writeFixture(t, map[string]string{
		"go.mod": "module example.com/fixture\n\ngo 1.22\n",
		"reader/reader_test.go": test("reader", //corvint:fixture-path
			read(`os.ReadFile(filepath.Join("../docs", "build-log", "2026-11-06-selgo.md"))`)+ //corvint:fixture-path
				read(`os.ReadFile(filepath.Join("../.corvint", "changes", "`+sealedName+`"))`)), //corvint:fixture-path
		"deep/dir/dir_test.go": test("dir", //corvint:fixture-path
			read(`os.ReadDir(filepath.Join("../../docs", "build-log"))`)+ //corvint:fixture-path
				read(`os.ReadDir(filepath.Join("../../.corvint", "changes"))`)), //corvint:fixture-path
		"fragment/fragment_test.go": test("fragment", //corvint:fixture-path
			read(`os.ReadDir(filepath.Join("../docs", "build"))`)+ //corvint:fixture-path
				read(`os.ReadDir(filepath.Join("../.corvint", "change"))`)), //corvint:fixture-path
		"walker/walker_test.go": test("walker", //corvint:fixture-path
			read(`os.ReadFile(filepath.Join("../docs", "README.md"))`)+ //corvint:fixture-path
				read(`os.ReadFile(filepath.Join("../.corvint", "config.json"))`)), //corvint:fixture-path
		entry:                  "# entry\n",
		sealed:                 "{}\n",
		"docs/README.md":       "# docs\n",
		"docs/build/x.md":      "# x\n",
		".corvint/config.json": "{}\n",
	})
	// The joined literals of reader name the entries that exist on disk.
	for _, joined := range []string{
		filepath.Join(root, "reader", "../docs", "build-log", "2026-11-06-selgo.md"), //corvint:fixture-path
		filepath.Join(root, "reader", "../.corvint", "changes", sealedName),          //corvint:fixture-path
	} {
		if _, err := os.Stat(joined); err != nil {
			t.Fatalf("the fixture's joined read does not exist: %v", err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{entry, sealed} {
		t.Run(dirty, func(t *testing.T) {
			planner := []string{}
			for _, selection := range affected.Select(graph, []string{dirty}).Selected {
				if selection.Witness.Kind == affected.WitnessPathLiteralReader {
					planner = append(planner, strings.TrimPrefix(selection.UnitID, "go:"+fixtureModule+"/"))
				}
			}
			slices.Sort(planner)
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{dirty}
			lines := selectPackages(plan, fixtureModule, root, true)
			driver := []string{}
			for _, line := range lines {
				if rest, ok := strings.CutPrefix(line, "reader "+fixtureModule+"/"); ok {
					driver = append(driver, strings.Fields(rest)[0])
				}
			}
			slices.Sort(driver)
			for name, selected := range map[string][]string{"planner": planner, "gate-affected-select": driver} {
				for _, want := range []string{"deep/dir", "reader"} {
					if !slices.Contains(selected, want) {
						t.Errorf("%s: %s reads %s through joined literals but is not a reader: %v", name, want, dirty, selected)
					}
				}
				for _, not := range []string{"fragment", "walker"} {
					if slices.Contains(selected, not) {
						t.Errorf("%s: %s names no record directory component, yet reads %s: %v", name, not, dirty, selected)
					}
				}
			}
			for _, id := range planner {
				if !slices.Contains(driver, id) {
					t.Errorf("the planner reader %s is not a gate-affected-select reader: planner %v, driver %v", id, planner, driver)
				}
			}
			bounds, err := packageBounds(fixtureModule, root, []string{dirty})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(bounds, ""); !strings.Contains(got, "bound "+fixtureModule+"/reader reader "+dirty) {
				t.Errorf("bounds omit the joined reader:\n%s", got)
			}
		})
	}
}
