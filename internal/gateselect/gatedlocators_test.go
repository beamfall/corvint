// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestGatedRootLocatorParity_TSP_V0_862 holds Select's rule (d) and run set
// equal to the planner's on a change in a nested module that imports a root
// locator, with and without the TSP-V0-003 declaration: declared, neither
// selects the locator's dependents; undeclared, an invalid declaration, or the
// declaration itself dirty, both (or Select alone, for the dirty one) keep them.
func TestGatedRootLocatorParity_TSP_V0_862(t *testing.T) {
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
		"nested/go.mod":       "module example.com/nested\n\ngo 1.22\n\nrequire " + fixtureModule + " v0.0.0\n\nreplace " + fixtureModule + " => ../\n",
		"nested/n.go":         "package nested\n\nimport _ \"" + fixtureModule + "/loc\"\n",
		"nested/n_test.go":    test("nested"),
	}
	const valid = `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"locates only behind /gate/root"}}}`
	for _, tc := range []struct {
		name, declaration string
		dirty             []string
		userChosen        bool
		plannerUser       bool
	}{
		{name: "undeclared", dirty: []string{"nested/n.go"}, userChosen: true, plannerUser: true},
		{name: "declared", declaration: valid, dirty: []string{"nested/n.go"}},
		{name: "declared, unrelated change", declaration: valid, dirty: []string{"other/other.go"}},
		{name: "gate not carried", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/elsewhere","reason":"r"}}}`, dirty: []string{"nested/n.go"}, userChosen: true, plannerUser: true},
		{name: "not a locator", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r"},"other":{"gate":"/gate/root","reason":"r"}}}`, dirty: []string{"nested/n.go"}, userChosen: true, plannerUser: true},
		{name: "unknown member", declaration: `{"profile":"corvint-gated-root-locators/0","packages":{"loc":{"gate":"/gate/root","reason":"r","reads":[]}}}`, dirty: []string{"nested/n.go"}, userChosen: true, plannerUser: true},
		{name: "unparsed", declaration: `{`, dirty: []string{"nested/n.go"}, userChosen: true, plannerUser: true},
		// The planner reads the head declaration and names the changed frontier;
		// Select keeps the wider closure for the edit itself.
		{name: "dirty declaration", declaration: valid, dirty: []string{affected.GatedRootLocatorsPath, "nested/n.go"}, userChosen: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := maps.Clone(base)
			if tc.declaration != "" {
				files[affected.GatedRootLocatorsPath] = tc.declaration
			}
			root := writeFixture(t, files)
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, tc.dirty)
			var receipt Receipt
			receipt.Plan.Dirty = tc.dirty
			receipt.Plan.Scope = string(plan.Scope)
			receipt.Provider.Go.State = "RUNNABLE"
			planned := []string{}
			for _, selection := range plan.Selected {
				if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok && (pkg == fixtureModule || strings.HasPrefix(pkg, fixtureModule+"/")) {
					planned = append(planned, pkg)
				}
			}
			sort.Strings(planned)
			receipt.Provider.Go.Packages = planned
			lines := selectPackages(receipt, fixtureModule, root, true)
			last := strings.Fields(lines[len(lines)-1])
			if len(last) == 0 || last[0] != "run" {
				t.Fatalf("verdict %q", lines[len(lines)-1])
			}
			run := slices.Sorted(slices.Values(last[1:]))
			user := fixtureModule + "/user"
			if got := slices.Contains(run, user); got != tc.userChosen {
				t.Errorf("Select chose user %v, want %v (lines %q)", got, tc.userChosen, lines)
			}
			if got := slices.Contains(planned, user); got != tc.plannerUser {
				t.Errorf("planner chose user %v, want %v (planned %v)", got, tc.plannerUser, planned)
			}
			if !slices.Contains(run, fixtureModule+"/loc") {
				t.Errorf("the locator's own tests stay selected: %v", run)
			}
			if tc.userChosen == tc.plannerUser && !reflect.DeepEqual(run, planned) {
				t.Errorf("Select run %v, planner %v", run, planned)
			}
		})
	}
}
