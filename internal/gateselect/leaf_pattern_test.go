package gateselect

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestLeafPackagePatternParity_AFPV02542 holds this index's rule (d) and the
// planner's equal over relative package patterns in test files (V1-4512). A
// "./..." or "./NAME/..." whose subtree holds no other package directory
// reaches what "." reaches and is no unbounded read: `leaf` (no subdirectory),
// `datum` (a data-only subdirectory) and `fixtured` (Go files only below
// testdata). A pattern that reaches another package, a nested module, the
// module's import path or a parent directory stays one: `parent`, `outer`,
// `imported` and `climber`.
func TestLeafPackagePatternParity_AFPV02542(t *testing.T) {
	pkg := func(name string) string { return "package " + name + "\n" }
	test := func(name, literal string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nvar patterns = []string{" + literal + "}\n\nfunc TestX(t *testing.T) { _ = patterns }\n"
	}
	root := writeFixture(t, map[string]string{
		"go.mod":                        "module example.com/fixture\n\ngo 1.22\n",
		"leaf/leaf.go":                  pkg("leaf"),
		"leaf/leaf_test.go":             test("leaf", `"./..."`),
		"datum/datum.go":                pkg("datum"),
		"datum/datum_test.go":           test("datum", `"./data/..."`),
		"datum/data/rows.txt":           "rows\n",
		"fixtured/fixtured.go":          pkg("fixtured"),
		"fixtured/fixtured_test.go":     test("fixtured", `"./..."`),
		"fixtured/testdata/m/m.go":      pkg("m"),
		"parent/parent.go":              pkg("parent"),
		"parent/parent_test.go":         test("parent", `"./..."`),
		"parent/sub/sub.go":             pkg("sub"),
		"outer/outer.go":                pkg("outer"),
		"outer/outer_test.go":           test("outer", `"./..."`),
		"outer/inner/go.mod":            "module example.com/inner\n\ngo 1.22\n",
		"outer/inner/inner.go":          pkg("inner"),
		"imported/imported.go":          pkg("imported"),
		"imported/imported_test.go":     test("imported", `"example.com/fixture/imported/..."`),
		"climber/climber.go":            pkg("climber"),
		"climber/climber_test.go":       test("climber", `"./../..."`),
		"deep/deep.go":                  pkg("deep"),
		"deep/deep_test.go":             test("deep", `"./data/.../x/..."`),
		"deep/data/notes.txt":           "notes\n",
		"wildcard/wildcard.go":          pkg("wildcard"),
		"wildcard/wildcard_test.go":     test("wildcard", `"./.../..."`),
		"production/production.go":      "package production\n\nvar Pattern = \"./...\"\n",
		"production/production_test.go": test("production", `""`),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	planner := []string{}
	for _, id := range graph.UnboundedReaders() {
		if rest, ok := strings.CutPrefix(id, "go:"+fixtureModule+"/"); ok {
			planner = append(planner, rest)
		}
	}
	sort.Strings(planner)
	unresolved := sortedKeys(index.unresolved())
	if want := []string{"climber", "deep", "imported", "outer", "parent", "wildcard"}; !reflect.DeepEqual(unresolved, want) {
		t.Errorf("gateselect rule (d) = %v, want %v", unresolved, want)
	}
	if !reflect.DeepEqual(unresolved, planner) {
		t.Errorf("gateselect rule (d) = %v, planner rule (d) = %v", unresolved, planner)
	}
}
