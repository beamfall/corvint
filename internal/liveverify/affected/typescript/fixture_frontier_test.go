// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// writeFixtureRepository lays out a Go module with a package whose testdata holds a TypeScript
// fixture suite with an unresolved relative import beside an ordinary vitest suite.
func writeFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "internal/appmap/appmap.go", "package appmap\n")
	write(t, root, "internal/appmap/appmap_test.go", "package appmap\n\nvar fixture = \"testdata/fixture\"\n")
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1"}}`)
	write(t, root, "src/dep.ts", "export const dep = 1\n")
	write(t, root, "tests/dep.test.ts", "import { test } from \"vitest\"\nimport { dep } from \"../src/dep\"\n")
	write(t, root, "internal/appmap/testdata/fixture/specs/broken.test.ts", "import { test } from \"vitest\"\nimport { page } from \"../pages/missing.page\"\nimport { helper } from \"./helper\"\n")
	write(t, root, "internal/appmap/testdata/fixture/specs/helper.ts", "export const helper = 1\n")
	write(t, root, "internal/appmap/testdata/fixture/specs/apart.ts", "export const apart = 1\n")
	return root
}

func frontierDetails(plan affected.Plan) []string {
	details := []string{}
	for _, unknown := range plan.Unknown {
		if unknown.Reason == affected.UnknownLanguageFrontier {
			details = append(details, unknown.Detail)
		}
	}
	return details
}

// AFP-V0-446: an unresolved import inside a Go package's testdata fixture is the fixture unit's
// own frontier: it is absent from the plugin frontier, it widens a plan that reaches the fixture,
// and it leaves a plan over an unrelated TypeScript source bounded.
func TestFixtureFrontierIsTheFixtureUnitsOwn_AFPV0446(t *testing.T) {
	root := writeFixtureRepository(t)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	site := FrontierUnresolvedImport + ":internal/appmap/testdata/fixture/specs/broken.test.ts:2 `../pages/missing.page`"
	if frontierHas(result, FrontierUnresolvedImport) {
		t.Fatalf("plugin frontier still carries the fixture site: %v", result.Frontier)
	}
	fixture := unitByID(result, unitID(runnerVitest, "internal/appmap/testdata/fixture/specs/broken.test.ts"))
	if fmt.Sprint(fixture.Frontier) != fmt.Sprint([]string{site}) {
		t.Fatalf("fixture unit frontier = %v, want [%s]", fixture.Frontier, site)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dirty []string
		want  string
	}{
		{[]string{"src/dep.ts"}, "BOUNDED []"},
		{[]string{"internal/appmap/appmap.go"}, "BOUNDED []"},
		{[]string{"internal/appmap/testdata/fixture/specs/broken.test.ts"}, "UNKNOWN [" + site + "]"},
	}
	for _, tc := range cases {
		plan := affected.Select(graph, tc.dirty)
		if got := plan.Scope + " " + fmt.Sprint(frontierDetails(plan)); got != tc.want {
			t.Errorf("dirty %v: %s, want %s", tc.dirty, got, tc.want)
		}
	}
	if plan := affected.Select(graph, []string{"src/dep.ts"}); plan.FrontierSites[FrontierUnresolvedImport] != 1 {
		t.Errorf("frontier sites = %v, want the fixture site counted once", plan.FrontierSites)
	}
}

// AFP-V0-446: an import edge across the testdata boundary, in either direction, keeps every
// site of the fixture units it connects to plugin-wide, and with no Go unit above it a testdata
// directory is ordinary, so its sites stay plugin-wide too.
func TestFixtureFrontierStaysPluginWideWhenEscapedOrUnhosted_AFPV0446(t *testing.T) {
	site := FrontierUnresolvedImport + ":internal/appmap/testdata/fixture/specs/broken.test.ts:2 `../pages/missing.page`"
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{"a suite outside imports a fixture source", func(t *testing.T, root string) {
			write(t, root, "tests/uses.test.ts", "import { test } from \"vitest\"\nimport { helper } from \"../internal/appmap/testdata/fixture/specs/helper\"\n")
		}},
		{"a fixture imports a source outside", func(t *testing.T, root string) {
			write(t, root, "internal/appmap/testdata/fixture/specs/broken.test.ts", "import { test } from \"vitest\"\nimport { page } from \"../pages/missing.page\"\nimport { dep } from \"../../../../../src/dep\"\n")
		}},
		{"a fixture file the suite imports imports a source outside", func(t *testing.T, root string) {
			write(t, root, "internal/appmap/testdata/fixture/specs/helper.ts", "import { dep } from \"../../../../../src/dep\"\nexport const helper = dep\n")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFixtureRepository(t)
			tc.setup(t, root)
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if !frontierHas(result, FrontierUnresolvedImport) {
				t.Fatalf("plugin frontier lost the escaped fixture site: %v", result.Frontier)
			}
			fixture := unitByID(result, unitID(runnerVitest, "internal/appmap/testdata/fixture/specs/broken.test.ts"))
			if len(fixture.Frontier) != 0 {
				t.Fatalf("escaped fixture unit carries its own frontier: %v", fixture.Frontier)
			}
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"src/dep.ts"})
			if got := fmt.Sprint(frontierDetails(plan)); plan.Scope != affected.ScopeUnknown || !strings.Contains(got, site) {
				t.Errorf("scope=%s frontier=%s, want UNKNOWN with %s", plan.Scope, got, site)
			}
		})
	}
	t.Run("a suite outside imports a fixture file the suite is not connected to", func(t *testing.T) {
		root := writeFixtureRepository(t)
		write(t, root, "tests/apart.test.ts", "import { test } from \"vitest\"\nimport { apart } from \"../internal/appmap/testdata/fixture/specs/apart\"\n")
		graph, err := affected.Build(context.Background(), root, golang.New(), New())
		if err != nil {
			t.Fatal(err)
		}
		if plan := affected.Select(graph, []string{"src/dep.ts"}); plan.Scope != affected.ScopeBounded {
			t.Errorf("scope=%s frontier=%v, want BOUNDED", plan.Scope, frontierDetails(plan))
		}
	})
	t.Run("no Go unit above testdata", func(t *testing.T) {
		root := writeFixtureRepository(t)
		graph, err := affected.Build(context.Background(), root, New())
		if err != nil {
			t.Fatal(err)
		}
		plan := affected.Select(graph, []string{"src/dep.ts"})
		if got := fmt.Sprint(frontierDetails(plan)); plan.Scope != affected.ScopeUnknown || got != "["+site+"]" {
			t.Errorf("scope=%s frontier=%s, want UNKNOWN [%s]", plan.Scope, got, site)
		}
		if plan.FrontierSites[FrontierUnresolvedImport] != 1 {
			t.Errorf("frontier sites = %v, want the promoted site counted once", plan.FrontierSites)
		}
	})
}

// AFP-V0-446: only a relative specifier whose path stays inside the fixture's `testdata`
// directory is the fixture unit's own; one that leaves the directory and a computed import,
// whose target nothing bounds, stay plugin-wide and keep a plan over an unrelated source
// `UNKNOWN`.
func TestFixtureFrontierKeepsUnboundedSpecifiersPluginWide_AFPV0446(t *testing.T) {
	file := "internal/appmap/testdata/fixture/specs/broken.test.ts"
	inside := FrontierUnresolvedImport + ":" + file + ":2 `../pages/missing.page`"
	cases := []struct {
		name       string
		body       string
		pluginWide string
		own        []string
	}{
		{"a relative specifier leaving the testdata directory", "import { test } from \"vitest\"\nimport { gone } from \"../../../gone\"\n",
			FrontierUnresolvedImport + ":" + file + ":2 `../../../gone`", nil},
		{"a relative specifier leaving the repository", "import { test } from \"vitest\"\nimport { gone } from \"../../../../../../gone\"\n",
			FrontierUnresolvedImport + ":" + file + ":2 `../../../../../../gone`", nil},
		{"a computed import beside an in-fixture site", "import { test } from \"vitest\"\nimport { page } from \"../pages/missing.page\"\nexport const value = import(process.env.TARGET)\n",
			FrontierDynamicImport + ":" + file + ":3 `import`", []string{inside}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeFixtureRepository(t)
			write(t, root, file, tc.body)
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(result.Frontier, tc.pluginWide) {
				t.Fatalf("plugin frontier %v lacks %s", result.Frontier, tc.pluginWide)
			}
			if fixture := unitByID(result, unitID(runnerVitest, file)); fmt.Sprint(fixture.Frontier) != fmt.Sprint(tc.own) {
				t.Fatalf("fixture unit frontier = %v, want %v", fixture.Frontier, tc.own)
			}
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"src/dep.ts"})
			if details := frontierDetails(plan); plan.Scope != affected.ScopeUnknown || !slices.Contains(details, tc.pluginWide) {
				t.Errorf("scope=%s frontier=%v, want UNKNOWN with %s", plan.Scope, details, tc.pluginWide)
			}
		})
	}
}

func TestInsideFixture_AFPV0446(t *testing.T) {
	file := "pkg/testdata/suite/specs/a.test.ts"
	cases := map[string]bool{
		"./helper": true, "../pages/missing.page": true, "../../other/x?raw": true, "../../x#frag": true,
		"../../../gone": false, "../../../testdata/x": true, "../../../../../../gone": false,
		"helper": false, "@app/helper": false, "vitest": false, "/abs/x": false,
	}
	for specifier, want := range cases {
		if got := insideFixture(file, specifier, "pkg/testdata"); got != want {
			t.Errorf("insideFixture(%q) = %v, want %v", specifier, got, want)
		}
	}
}

func TestSiteFile_AFPV0446(t *testing.T) {
	cases := map[string]string{"a/b.ts:12": "a/b.ts", "a/b.ts": "a/b.ts", "a/b.ts:": "a/b.ts:", "a/b:c.ts:3": "a/b:c.ts", "a/b:x": "a/b:x"}
	for site, want := range cases {
		if got := siteFile(site); got != want {
			t.Errorf("siteFile(%q) = %q, want %q", site, got, want)
		}
	}
}

func unitByID(result affected.Result, id string) affected.Unit {
	for _, unit := range result.Units {
		if unit.ID == id {
			return unit
		}
	}
	return affected.Unit{}
}
