// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"fmt"
	"testing"
)

// TSP-V0-942 (V1-3349): every LANGUAGE_FRONTIER entry names what made it bear
// (the plugin owning a dirty path, the reached unit and its witness, the
// plugin raised by an unowned dirty path), no bearer names an entry the plan
// does not, a clean plan names none, and unit sources past the bound are
// counted in one OMITTED entry. Selection and scope are unchanged.
func TestFrontierBearersNameTheSource_TSP_V0_942(t *testing.T) {
	units := []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}, Frontier: []string{"go:unit-entry"}}}
	for index := range MaxFrontierBearerUnits + 2 {
		directory := fmt.Sprintf("many%02d", index)
		units = append(units, Unit{ID: "go:" + directory, Sources: []string{directory + "/a.go"}, Tests: []string{directory + "/a_test.go"}, Frontier: []string{"go:shared"}})
	}
	goPlugin := fake{name: "go", units: units}
	plainPlugin := fake{name: "z", frontier: []string{"z:import-unresolved:app.z:1 `./gone`"}, units: []Unit{{ID: "z:app", Sources: []string{"app.z"}, Tests: []string{"app_test.z"}}}}
	graph, err := Build(context.Background(), t.TempDir(), goPlugin, bystander(), plainPlugin)
	if err != nil {
		t.Fatal(err)
	}
	if plan := Select(graph, nil); len(plan.FrontierBearers) != 0 {
		t.Fatalf("clean plan bearers %+v", plan.FrontierBearers)
	}
	dirty := []string{"Makefile", "app.y", "pkg/pkg.go"}
	for index := range MaxFrontierBearerUnits + 2 {
		dirty = append(dirty, fmt.Sprintf("many%02d/a.go", index))
	}
	plan := Select(graph, dirty)
	named := make(map[string]bool)
	for _, entry := range languageFrontier(plan) {
		named[entry] = true
	}
	borne := make(map[string]bool)
	has := func(want FrontierBearer) bool {
		for _, bearer := range plan.FrontierBearers {
			if bearer == want {
				return true
			}
		}
		return false
	}
	for _, bearer := range plan.FrontierBearers {
		if !named[bearer.Detail] {
			t.Errorf("bearer %+v names an entry the plan does not", bearer)
		}
		borne[bearer.Detail] = true
	}
	for entry := range named {
		if !borne[entry] {
			t.Errorf("frontier %s has no bearer", entry)
		}
	}
	for _, want := range []FrontierBearer{
		{Detail: "go:unit-entry", Via: FrontierBearerReachedUnit, Unit: "go:pkg", Witness: WitnessDirectSource, DirtyPath: "pkg/pkg.go"},
		{Detail: "y:path-alias:+2", Via: FrontierBearerParticipating, Language: "y"},
		{Detail: "z:import-unresolved:app.z:1 `./gone`", Via: FrontierBearerUnownedPath, Language: "z", DirtyPath: "Makefile"},
		{Detail: "go:shared", Via: FrontierBearerOmitted, Count: 2},
	} {
		if !has(want) {
			t.Errorf("no bearer %+v in %+v", want, plan.FrontierBearers)
		}
	}
	shared := 0
	for _, bearer := range plan.FrontierBearers {
		if bearer.Detail == "go:shared" && bearer.Unit != "" {
			shared++
		}
	}
	if shared != MaxFrontierBearerUnits {
		t.Fatalf("go:shared lists %d units, want %d", shared, MaxFrontierBearerUnits)
	}
}
