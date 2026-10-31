package gateselect

import (
	"context"
	"maps"
	"path"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestGatedRootLocatorsMatchThePlanner_TSP_V0_940 holds the selector's rule (d)
// equal to the planner's under the gated root-locator declaration (V1-3346).
// `loc` locates the root behind its gate and `user` imports it. Undeclared,
// both are unresolved; declared, `loc` keeps its own reason and `user` is no
// longer a root-locator dependent, in the selector as in the planner, and a
// change inside the nested module selects the same root-module units in both.
// An invalid declaration declares nothing, and a plan naming either
// declaration frontier falls back.
func TestGatedRootLocatorsMatchThePlanner_TSP_V0_940(t *testing.T) {
	test := func(name string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	}
	base := map[string]string{
		"go.mod":              "module " + fixtureModule + "\n\ngo 1.22\n",
		"loc/loc.go":          "package loc\n\nimport \"os\"\n\nconst Gate = \"/gate/root\"\n\nfunc Bind() string { dir, _ := os.Getwd(); return dir }\n",
		"loc/loc_test.go":     test("loc"),
		"user/user.go":        "package user\n\nimport _ \"" + fixtureModule + "/loc\"\n",
		"user/user_test.go":   test("user"),
		"other/other.go":      "package other\n",
		"other/other_test.go": test("other"),
		"nested/go.mod":       "module example.com/nested\n\ngo 1.22\n",
		"nested/n/n.go":       "package n\n",
		"nested/n/n_test.go":  test("n"),
	}
	const valid = `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"locates only behind /gate/root"}}}`
	for _, tc := range []struct {
		name, declaration string
		want              []string
	}{
		{name: "undeclared", want: []string{"loc", "user"}},
		{name: "declared", declaration: valid, want: []string{"loc"}},
		{name: "gate not carried", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/elsewhere","reason":"r"}}}`, want: []string{"loc", "user"}},
		{name: "not a locator", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r"},"other":{"gate":"/gate/root","reason":"r"}}}`, want: []string{"loc", "user"}},
		{name: "unknown member", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r","reads":[]}}}`, want: []string{"loc", "user"}},
		{name: "unparsed", declaration: `{`, want: []string{"loc", "user"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tc.declaration != "" {
				files[gatedLocatorsPath] = tc.declaration
			}
			root := writeFixture(t, files)
			index, err := indexRepository(root, fixtureModule)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			unresolved := sortedKeys(index.unresolved())
			if !reflect.DeepEqual(unresolved, tc.want) {
				t.Errorf("gateselect rule (d) = %v, want %v", unresolved, tc.want)
			}
			var planner []string
			for _, id := range graph.UnboundedReaders() {
				if rest, ok := strings.CutPrefix(id, "go:"+fixtureModule+"/"); ok {
					planner = append(planner, rest)
				}
			}
			sort.Strings(planner)
			if !reflect.DeepEqual(unresolved, planner) {
				t.Errorf("gateselect rule (d) = %v, planner rule (d) = %v", unresolved, planner)
			}

			// A nested-module change: the root-module units each side selects.
			const dirty = "nested/n/n.go"
			selected := map[string]bool{}
			for _, test := range affected.Select(graph, []string{dirty}).SelectedTests() {
				if directory := path.Dir(test); !strings.HasPrefix(directory, "nested/") && directory != "nested" {
					selected[directory] = true
				}
			}
			var plan Receipt
			plan.Provider.Go.State = "RUNNABLE"
			plan.Plan.Dirty = []string{dirty}
			want := "run"
			for _, directory := range sortedKeys(selected) {
				want += " " + fixtureModule + "/" + directory
			}
			if got := verdict(plan, root); got != want {
				t.Errorf("nested-module change: selector %q, planner %q", got, want)
			}
		})
	}
	root := writeFixture(t, base)
	for _, frontier := range []string{"go:gated-root-locators-invalid", "go:gated-root-locators-changed"} {
		var plan Receipt
		plan.Provider.Go.State = "RUNNABLE"
		plan.Plan.Dirty = []string{"other/other.go"}
		plan.Plan.Unknown = append(plan.Plan.Unknown, struct {
			Reason string `json:"reason"`
			Detail string `json:"detail"`
		}{"LANGUAGE_FRONTIER", frontier})
		if got := verdict(plan, root); got != "FALLBACK Go frontier is unknown at module level: "+frontier {
			t.Errorf("%s: verdict %q, want the module-level fallback", frontier, got)
		}
	}
}
