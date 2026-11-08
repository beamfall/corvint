// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"context"
	json "encoding/json/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// TestDriverFrontierBoundParity_AFP_V0_3124 holds this selector in parity with
// the planner's driver frontier bound (AFP-V0-3120 to AFP-V0-3123). The
// selector attributes Go packages only and has no driver unit or frontier-site
// claim of its own, so parity means the bound may change which make targets
// and workflow jobs a plan selects but never its Go selection, its scope or
// unknowns, or this selector's verdict on it. The fixture's Makefile holds an
// unreferenced `$(or ...)` (the injected fault), a computed invocation whose
// alternatives are literal, and one that cannot be bounded; each dirty set is
// planned with and without the driver claimant.
func TestDriverFrontierBoundParity_AFP_V0_3124(t *testing.T) {
	test := func(name string) string {
		return "package " + name + "\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"
	}
	root := writeFixture(t, map[string]string{
		"go.mod":            "module " + fixtureModule + "\n\ngo 1.22\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": test("core"),
		"tool/tool.go":      "package tool\n",
		"tool/tool_test.go": test("tool"),
		"Makefile": "CACHE = $(or $(XDG_CACHE_HOME),/tmp)/cache\n" +
			"RUN = $(if $(CI),go test,go vet)\n" +
			"check:\n\t$(RUN) ./core\n" +
			"verify:\n\t$$TOOL --verify ./tool\n",
		".github/workflows/ci.yml": "jobs:\n  unit:\n    steps:\n      - run: make check\n", //corvint:fixture-path
	})
	withDriver, err := affected.Build(context.Background(), root, golang.New(), affected.Driver{})
	if err != nil {
		t.Fatal(err)
	}
	withoutDriver, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	cem := ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	for _, dirty := range [][]string{{cem}, {"core/core.go"}, {"tool/tool.go"}, {cem, "core/core.go"}} {
		t.Run(strings.Join(dirty, "+"), func(t *testing.T) {
			bounded, plain := affected.Select(withDriver, dirty), affected.Select(withoutDriver, dirty)
			if got, want := goSelections(bounded), goSelections(plain); !reflect.DeepEqual(got, want) {
				t.Errorf("Go selections with the driver claimant %v, without %v", got, want)
			}
			if !reflect.DeepEqual(bounded.SelectedTests(), plain.SelectedTests()) {
				t.Errorf("selected tests with the driver claimant %v, without %v", bounded.SelectedTests(), plain.SelectedTests())
			}
			got, want := SelectStructural(receiptOf(t, bounded), fixtureModule, root), SelectStructural(receiptOf(t, plain), fixtureModule, root)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("selector with the driver claimant %q, without %q", got, want)
			}
			drivers := map[string]string{}
			for _, selection := range bounded.Selected {
				if strings.HasPrefix(selection.UnitID, affected.DriverNamespace+":") {
					drivers[selection.UnitID] = selection.Witness.Kind
				}
			}
			// The unbounded site keeps claiming every dirty path, by its
			// frontier where no edge reaches it; the bounded one is selected
			// only through an alternative's edge.
			verify, claimed := drivers["driver:make:Makefile#verify"]
			if !claimed || len(dirty) == 1 && dirty[0] == cem && verify != affected.WitnessFrontierSite {
				t.Errorf("verify witness %q (selected %v), want it selected, by FRONTIER_SITE for the CEM alone", verify, claimed)
			}
			_, checked := drivers["driver:make:Makefile#check"]
			if want := slices.Contains(dirty, "core/core.go"); checked != want {
				t.Errorf("check selected = %v, want %v (it runs only ./core)", checked, want)
			}
		})
	}
}

// goSelections is every non-driver selection of plan with its witness kind.
func goSelections(plan affected.Plan) map[string]string {
	out := map[string]string{}
	for _, selection := range plan.Selected {
		if !strings.HasPrefix(selection.UnitID, affected.DriverNamespace+":") {
			out[selection.UnitID] = selection.Witness.Kind
		}
	}
	return out
}

// receiptOf is the affected-plan receipt of plan as the selector reads it,
// with the Go provider's packages taken from the plan's Go selections.
func receiptOf(t *testing.T, plan affected.Plan) Receipt {
	t.Helper()
	body, err := json.Marshal(map[string]any{"plan": plan})
	if err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Provider.Go.State = "EMPTY_SELECTION"
	for _, selection := range plan.Selected {
		if pkg, ok := strings.CutPrefix(selection.UnitID, "go:"); ok {
			receipt.Provider.Go.Packages = append(receipt.Provider.Go.Packages, pkg)
			receipt.Provider.Go.State = "RUNNABLE"
		}
	}
	return receipt
}
