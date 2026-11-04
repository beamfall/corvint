// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			// AFP-V0-2406: x's two sites count only when the plan involves x
			// (it names one of x's frontiers); otherwise x adds no count.
			want := 0
			if strings.Contains(tc.want, "x:") {
				want = 2
			}
			if plan.FrontierSites["x:import-unresolved"] != want {
				t.Errorf("frontier sites = %v, want x:import-unresolved counted %d times", plan.FrontierSites, want)
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

// AFP-V0-2440 (V1-4446): a non-Go plugin's claim on a file below a `testdata` directory a Go unit
// hosts, below which the plugin lists no path, is a fixture-only claim. With no other claimant
// the path is UNOWNED_DIRTY_PATH, whose hint names the claim, never UNINDEXED_SOURCE_PATH; the
// hosting Go package is still selected. Every other shape keeps UNINDEXED_SOURCE_PATH: no hosting
// Go unit, the plugin listing a unit below the directory (the injected fault: a fixture the plugin
// does index), a path outside testdata, a claim by the Go plugin, and a second claimant that is
// not fixture-only.
func TestFixtureOnlyClaimIsNoUnindexedSource_AFPV2440(t *testing.T) {
	goUnit := fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}}
	x := fake{name: "x", units: []Unit{{ID: "x:app", Sources: []string{"app.x"}, Tests: []string{"app_test.x"}}}}
	xIndexed := fake{name: "x", units: []Unit{
		{ID: "x:app", Sources: []string{"app.x"}, Tests: []string{"app_test.x"}},
		{ID: "x:fixture", Tests: []string{"pkg/testdata/suite/t_test.x"}},
	}}
	y := fake{name: "y", owns: func(path string) bool { return strings.HasSuffix(path, ".x") }, units: []Unit{
		{ID: "y:fixture", Sources: []string{"pkg/testdata/lib/kept.y"}},
	}}
	cases := []struct {
		name      string
		languages []Language
		dirty     string
		reason    string
		hint      bool
	}{
		{"hosted fixture, plugin lists nothing below it", []Language{goUnit, x}, "pkg/testdata/lib/calc.x", UnknownUnownedDirtyPath, true},
		{"nested testdata keeps the outermost hosted directory", []Language{goUnit, x}, "pkg/testdata/a/testdata/b.x", UnknownUnownedDirtyPath, true},
		{"no Go unit hosts the directory", []Language{x}, "pkg/testdata/lib/calc.x", UnknownUnindexedSourcePath, false},
		{"the plugin lists a unit below the directory", []Language{goUnit, xIndexed}, "pkg/testdata/lib/calc.x", UnknownUnindexedSourcePath, false},
		{"a path outside testdata", []Language{goUnit, x}, "pkg/lib/calc.x", UnknownUnindexedSourcePath, false},
		{"a Go source below hosted testdata", []Language{goUnit, x}, "pkg/testdata/gen/main.go", UnknownUnindexedSourcePath, false},
		{"a second claimant indexes below the directory", []Language{goUnit, x, y}, "pkg/testdata/lib/calc.x", UnknownUnindexedSourcePath, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The dirty file exists, so no deleted-file rule (AFP-V0-620, AFP-V0-821) takes it.
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(tc.dirty)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, tc.dirty), []byte("fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			graph, err := Build(context.Background(), root, tc.languages...)
			if err != nil {
				t.Fatal(err)
			}
			plan := Select(graph, []string{tc.dirty})
			var found *Unknown
			for index, unknown := range plan.Unknown {
				if unknown.Detail == tc.dirty {
					found = &plan.Unknown[index]
				}
				if unknown.Reason == UnknownUnindexedSourcePath && tc.reason != UnknownUnindexedSourcePath {
					t.Errorf("unexpected %s %s", unknown.Reason, unknown.Detail)
				}
			}
			if found == nil || found.Reason != tc.reason {
				t.Fatalf("dirty %s: unknown %+v, want reason %s", tc.dirty, plan.Unknown, tc.reason)
			}
			if hinted := strings.Contains(found.Hint, "AFP-V0-2440"); hinted != tc.hint {
				t.Errorf("hint %q, want AFP-V0-2440 hint %v", found.Hint, tc.hint)
			}
			if tc.hint && !strings.Contains(found.Hint, "x") {
				t.Errorf("hint %q does not name the claiming plugin", found.Hint)
			}
		})
	}
}
