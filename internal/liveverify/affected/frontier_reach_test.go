// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// reaching is a fake plugin that classifies its frontier reasons (AFP-V0-511).
type reaching struct {
	fake
	sourceBound   []string
	fixtureClosed []string
	inputs        func(string) bool
}

func (language reaching) SourceBoundFrontiers() []string   { return language.sourceBound }
func (language reaching) FixtureClosedFrontiers() []string { return language.fixtureClosed }
func (language reaching) ResolutionInput(path string) bool {
	return language.inputs != nil && language.inputs(path)
}

// reading is a reaching plugin whose units read any path (PathReader).
type reading struct{ reaching }

func (reading) ReadsAnyPath() bool { return true }

// bystander is the plugin y of the AFP-V0-511 tests: one path-open reason, two
// source-bound ones (one located, one bounded) and one fixture-closed reason
// with a bounded entry and a located site outside every fixture.
func bystander(extra ...string) reaching {
	frontier := append([]string{
		"y:dynamic-load:app.y:3 `load(expr)`",
		"y:import-unresolved:app.y:1 `./gone`",
		"y:path-alias:+2",
		"y:runner-unknown:+3",
		"y:runner-unknown:app_test.y",
	}, extra...)
	return reaching{
		fake: fake{name: "y", frontier: sortedKeys(toSet(frontier)), units: []Unit{
			{ID: "y:app", Sources: []string{"app.y"}, Tests: []string{"app_test.y"}},
		}},
		sourceBound:   []string{"y:import-unresolved", "y:path-alias"},
		fixtureClosed: []string{"y:runner-unknown"},
	}
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func languageFrontier(plan Plan) []string {
	frontier := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier {
			frontier = append(frontier, unknown.Detail)
		}
	}
	return frontier
}

// AFP-V0-511: a dirty path no plugin owns raises, from a plugin that neither owns a dirty path nor
// reaches a changed unit, only the frontier entries whose hidden edge could bind that path. A
// plugin without FrontierReach stays path-open in full; before AFP-V0-511 every plugin did.
func TestUnownedPathRaisesOnlyPathOpenFrontiers_AFPV0511(t *testing.T) {
	const (
		dynamic   = "y:dynamic-load:app.y:3 `load(expr)`"
		unresolve = "y:import-unresolved:app.y:1 `./gone`"
		alias     = "y:path-alias:+2"
		bound     = "y:runner-unknown:+3"
		site      = "y:runner-unknown:app_test.y"
		inside    = "y:runner-unknown:pkg/testdata/suite/t_test.y"
		plain     = "z:import-unresolved:app.z:1 `./gone`"
	)
	goPlugin := fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}}
	plainPlugin := fake{name: "z", frontier: []string{plain}, units: []Unit{{ID: "z:app", Sources: []string{"app.z"}, Tests: []string{"app_test.z"}}}}
	steered := bystander()
	steered.inputs = func(path string) bool { return strings.HasSuffix(path, "y.config.json") }
	listing := bystander()
	listing.units = append(listing.units, Unit{ID: "y:fixture", Tests: []string{"pkg/testdata/suite/t_test.y"}})
	cases := []struct {
		name      string
		languages []Language
		dirty     []string
		want      []string
	}{
		{"an unowned path raises only y's path-open entries and all of z's", []Language{goPlugin, bystander(), plainPlugin}, []string{"Makefile"}, []string{dynamic, bound, site, plain}},
		{"y's own path raises every y entry", []Language{goPlugin, bystander(), plainPlugin}, []string{"app.y"}, []string{dynamic, unresolve, alias, bound, site}},
		{"an unowned path beside y's own raises every y entry", []Language{goPlugin, bystander(), plainPlugin}, []string{"Makefile", "app.y"}, []string{dynamic, unresolve, alias, bound, site, plain}},
		{"a plugin whose units read any path takes part in full", []Language{goPlugin, reading{bystander()}, plainPlugin}, []string{"Makefile"}, []string{dynamic, unresolve, alias, bound, site, plain}},
		{"a Go-hosted fixture drops the fixture-closed site outside it, not the bound", []Language{goPlugin, bystander(), plainPlugin}, []string{"pkg/testdata/cases.json"}, []string{dynamic, bound, plain}},
		{"an unhosted fixture keeps the fixture-closed site", []Language{goPlugin, bystander(), plainPlugin}, []string{"other/testdata/cases.json"}, []string{dynamic, bound, site, plain}},
		{"a fixture-closed site inside the fixture is kept", []Language{goPlugin, bystander(inside), plainPlugin}, []string{"pkg/testdata/cases.json"}, []string{dynamic, bound, inside, plain}},
		{"a plugin listing a path below the fixture keeps its fixture-closed sites", []Language{goPlugin, listing, plainPlugin}, []string{"pkg/testdata/cases.json"}, []string{dynamic, bound, site, plain}},
		{"one unowned path outside a fixture turns the fixture rule off", []Language{goPlugin, bystander(), plainPlugin}, []string{"Makefile", "pkg/testdata/cases.json"}, []string{dynamic, bound, site, plain}},
		{"an unowned path the plugin reads as a resolution input raises every entry", []Language{goPlugin, steered, plainPlugin}, []string{"tools/y.config.json"}, []string{dynamic, unresolve, alias, bound, site, plain}},
		{"an empty dirty set raises nothing", []Language{goPlugin, bystander(), plainPlugin}, nil, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := Build(context.Background(), t.TempDir(), tc.languages...)
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, tc.dirty)
			if got := languageFrontier(plan); fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("dirty %v: frontier %v, want %v", tc.dirty, got, tc.want)
			}
			if len(tc.dirty) != 0 && plan.Scope != ScopeUnknown {
				t.Errorf("dirty %v: scope %s, want UNKNOWN", tc.dirty, plan.Scope)
			}
		})
	}
}

// AFP-V0-511: a FrontierReach declaration classifies plugin code, not the observed repository,
// so it changes no graph digest; a plan's exclusion certificates stay comparable across it.
func TestFrontierReachLeavesTheDigestUnchanged_AFPV0511(t *testing.T) {
	declared := bystander()
	undeclared := declared.fake
	with, err := Build(context.Background(), t.TempDir(), declared)
	if err != nil {
		t.Fatal(err)
	}
	without, err := Build(context.Background(), t.TempDir(), undeclared)
	if err != nil {
		t.Fatal(err)
	}
	if with.Digest() != without.Digest() {
		t.Fatalf("digest changed with the declaration: %s vs %s", with.Digest(), without.Digest())
	}
}

// AFP-V0-511: a declaration that is not `<plugin>:<category>` in the plugin's own namespace, or
// that names one reason on both lists, fails Build, so it can never narrow a plan.
func TestFrontierReachDeclarationsAreValidatedAtBuild_AFPV0511(t *testing.T) {
	cases := map[string]reaching{
		"foreign namespace":    {fake: fake{name: "y"}, sourceBound: []string{"z:import-unresolved"}},
		"empty category":       {fake: fake{name: "y"}, sourceBound: []string{"y:"}},
		"located entry":        {fake: fake{name: "y"}, fixtureClosed: []string{"y:runner-unknown:app_test.y"}},
		"no namespace":         {fake: fake{name: "y"}, fixtureClosed: []string{"runner-unknown"}},
		"reason on both lists": {fake: fake{name: "y"}, sourceBound: []string{"y:runner-unknown"}, fixtureClosed: []string{"y:runner-unknown"}},
	}
	for name, language := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Build(context.Background(), t.TempDir(), language)
			if !errors.Is(err, ErrInvalidLanguage) {
				t.Fatalf("err = %v, want ErrInvalidLanguage", err)
			}
		})
	}
	valid := bystander()
	if _, err := Build(context.Background(), t.TempDir(), valid); err != nil {
		t.Fatalf("a well-formed declaration failed: %v", err)
	}
}
