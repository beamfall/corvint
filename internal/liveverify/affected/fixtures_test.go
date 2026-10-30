// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"fmt"
	"testing"
)

// AFP-V0-446: a fixture unit's own frontier bears only on a plan that reaches
// the unit when a Go unit encloses its `testdata` directory; with no Go unit
// above it, the site is the plugin's again and widens every plan the plugin
// takes part in, and the site count names it once either way.
func TestFixtureFrontierBearsOnlyWhereAGoUnitHostsIt_AFPV0446(t *testing.T) {
	plugin := func() fake {
		return fake{name: "x", units: []Unit{
			{ID: "x:app", Sources: []string{"app.x"}, Tests: []string{"app_test.x"}},
			{ID: "x:fixture", Tests: []string{"pkg/testdata/suite/broken_test.x"}, Frontier: []string{"x:import-unresolved:pkg/testdata/suite/broken_test.x:1 `../pages/missing`"}},
			{ID: "x:plain", Tests: []string{"other/testdata/plain_test.x"}, Frontier: []string{"x:import-unresolved:other/testdata/plain_test.x:1 `./gone`"}},
		}}
	}
	goPlugin := fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}}
	cases := []struct {
		name      string
		languages []Language
		dirty     []string
		want      string
	}{
		{"no Go unit: both fixture sites widen a plan over another path", []Language{plugin()}, []string{"app.x"}, "UNKNOWN [x:import-unresolved:other/testdata/plain_test.x:1 `./gone` x:import-unresolved:pkg/testdata/suite/broken_test.x:1 `../pages/missing`]"},
		{"a Go unit hosts pkg/testdata: only the unhosted site widens another path's plan", []Language{goPlugin, plugin()}, []string{"app.x"}, "UNKNOWN [x:import-unresolved:other/testdata/plain_test.x:1 `./gone`]"},
		{"a hosted fixture's own change raises its own site", []Language{goPlugin, plugin()}, []string{"pkg/testdata/suite/broken_test.x"}, "UNKNOWN [x:import-unresolved:other/testdata/plain_test.x:1 `./gone` x:import-unresolved:pkg/testdata/suite/broken_test.x:1 `../pages/missing`]"},
		{"a hosted fixture does not widen a plan over its Go package, which x takes no part in", []Language{goPlugin, plugin()}, []string{"pkg/pkg.go"}, "BOUNDED []"},
		{"a hosted fixture does not widen a plan over its Go package and an x source", []Language{goPlugin, plugin()}, []string{"pkg/pkg.go", "app.x"}, "UNKNOWN [x:import-unresolved:other/testdata/plain_test.x:1 `./gone`]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := Build(context.Background(), t.TempDir(), tc.languages...)
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, tc.dirty)
			frontier := []string{}
			for _, unknown := range plan.Unknown {
				if unknown.Reason == UnknownLanguageFrontier {
					frontier = append(frontier, unknown.Detail)
				}
			}
			if got := plan.Scope + " " + fmt.Sprint(frontier); got != tc.want {
				t.Errorf("dirty %v: %s, want %s", tc.dirty, got, tc.want)
			}
			if plan.FrontierSites["x:import-unresolved"] != 2 {
				t.Errorf("frontier sites = %v, want x:import-unresolved counted twice", plan.FrontierSites)
			}
		})
	}
}

// AFP-V0-446: a promoted unit's sites are listed in full on the plugin's frontier; the plugin's
// own per-reason `+N` bound applies only to the sites it names itself.
func TestPromotedFixtureSitesAreListedInFull_AFPV0446(t *testing.T) {
	sites := make([]string, 20)
	for index := range sites {
		sites[index] = fmt.Sprintf("x:import-unresolved:pkg/testdata/suite/t%02d_test.x:1 `./gone`", index)
	}
	plugin := fake{name: "x", units: []Unit{
		{ID: "x:app", Sources: []string{"app.x"}, Tests: []string{"app_test.x"}},
		{ID: "x:fixture", Tests: []string{"pkg/testdata/suite/t00_test.x"}, Frontier: sites},
	}}
	graph, err := Build(context.Background(), t.TempDir(), plugin)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"app.x"})
	listed := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == UnknownLanguageFrontier {
			listed = append(listed, unknown.Detail)
		}
	}
	if plan.Scope != ScopeUnknown || fmt.Sprint(listed) != fmt.Sprint(sites) {
		t.Errorf("scope=%s listed %d sites %v, want UNKNOWN with all 20 in order", plan.Scope, len(listed), listed)
	}
	if plan.FrontierSites["x:import-unresolved"] != 20 {
		t.Errorf("frontier sites = %v, want 20", plan.FrontierSites)
	}
}

func TestFixtureDirectory_AFPV0446(t *testing.T) {
	cases := map[string]string{
		"pkg/testdata/suite/a.ts":          "pkg/testdata",
		"testdata/a.ts":                    "testdata",
		"pkg/testdata/inner/testdata/a.ts": "pkg/testdata",
		"pkg/a.ts":                         "",
		"pkg/testdata":                     "",
		"testdata":                         "",
		"pkg/testdata-old/a.ts":            "",
	}
	for relative, want := range cases {
		if got := fixtureDirectory(relative); got != want {
			t.Errorf("fixtureDirectory(%q) = %q, want %q", relative, got, want)
		}
	}
}
