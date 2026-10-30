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

// TestReachMatchesThePlannerGraph_V12727 holds this index's rule (a) and rule
// (d) reach equal to the planner's over one fixture (V1-2727, STR-V0-061).
// `tester` imports `leaf` from its test files alone, so `tester` is reached and
// `far`, which imports `tester`, is not: its test binary links neither `leaf`
// nor `walker`. `namer` names `./walker` in a non-test literal, a path token
// that links no code, so it is not a rule (d) dependent of the root locator.
// `user` imports `mid` from its tests alone, so a change to `core` reaches
// `user` and stops before `beyond`. `framer` reads only runtime frame metadata
// and `framed` imports it: neither is a rule (d) unit (AFP-V0-810, V1-2758).
func TestReachMatchesThePlannerGraph_V12727(t *testing.T) {
	test := func(name string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	}
	root := writeFixture(t, map[string]string{
		"go.mod":                "module example.com/fixture\n\ngo 1.22\n",
		"core/core.go":          "package core\n",
		"core/core_test.go":     test("core"),
		"mid/mid.go":            "package mid\n\nimport _ \"example.com/fixture/core\"\n",
		"mid/mid_test.go":       test("mid"),
		"user/user.go":          "package user\n",
		"user/user_test.go":     "package user\n\nimport (\n\t\"testing\"\n\n\t_ \"example.com/fixture/mid\"\n)\n\nfunc TestX(t *testing.T) {}\n",
		"beyond/beyond.go":      "package beyond\n\nimport _ \"example.com/fixture/user\"\n",
		"beyond/beyond_test.go": test("beyond"),
		"walker/walker.go":      "package walker\n\nimport \"os\"\n\nvar _, _ = os.Getwd()\n",
		"walker/walker_test.go": test("walker"),
		"leaf/leaf.go":          "package leaf\n\nimport _ \"example.com/fixture/walker\"\n",
		"leaf/leaf_test.go":     test("leaf"),
		"tester/tester.go":      "package tester\n",
		"tester/tester_test.go": "package tester\n\nimport (\n\t\"testing\"\n\n\t_ \"example.com/fixture/leaf\"\n)\n\nfunc TestX(t *testing.T) {}\n",
		"far/far.go":            "package far\n\nimport _ \"example.com/fixture/tester\"\n",
		"far/far_test.go":       test("far"),
		"namer/namer.go":        "package namer\n\nvar Target = \"./walker\"\n",
		"namer/namer_test.go":   test("namer"),
		"framer/framer.go":      "package framer\n\nimport \"runtime\"\n\nvar _, file, _, _ = runtime.Caller(0)\nvar n = runtime.Callers(0, nil)\nvar f = runtime.FuncForPC(0)\nvar frames = runtime.CallersFrames(nil)\n",
		"framer/framer_test.go": test("framer"),
		"framed/framed.go":      "package framed\n\nimport _ \"example.com/fixture/framer\"\n",
		"framed/framed_test.go": test("framed"),
	})
	index, err := indexRepository(root, fixtureModule)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	directories := func(ids []string) []string {
		out := []string{}
		for _, id := range ids {
			if rest, ok := strings.CutPrefix(id, "go:"+fixtureModule+"/"); ok {
				out = append(out, rest)
			}
		}
		sort.Strings(out)
		return out
	}

	unresolved := sortedKeys(index.unresolved())
	if want := []string{"leaf", "tester", "walker"}; !reflect.DeepEqual(unresolved, want) {
		t.Errorf("gateselect rule (d) = %v, want %v", unresolved, want)
	}
	if planner := directories(graph.UnboundedReaders()); !reflect.DeepEqual(unresolved, planner) {
		t.Errorf("gateselect rule (d) = %v, planner rule (d) = %v", unresolved, planner)
	}

	// Rule (a): the dirty package plus its dependents, against the planner's
	// reach less its rule (d) units, which a dirty path selects everywhere.
	frontier := append([]string{"core"}, index.dependents("core")...)
	sort.Strings(frontier)
	if want := []string{"core", "mid", "user"}; !reflect.DeepEqual(frontier, want) {
		t.Errorf("gateselect rule (a) = %v, want %v", frontier, want)
	}
	unbounded := map[string]bool{}
	for _, directory := range unresolved {
		unbounded[directory] = true
	}
	planner := []string{}
	for _, directory := range directories(graph.ReachedUnitIDs([]string{"core/core.go"})) {
		if !unbounded[directory] {
			planner = append(planner, directory)
		}
	}
	if !reflect.DeepEqual(frontier, planner) {
		t.Errorf("gateselect rule (a) = %v, planner reach = %v", frontier, planner)
	}
}
