// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

const fixtureE2ESpec = "internal/appmap/testdata/e2e/login.spec.ts"

// writeE2EFixtureRepository lays out a Go package whose testdata holds a Playwright suite with
// its own config beside an ordinary vitest suite.
func writeE2EFixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/fixture\n\ngo 1.22\n")
	write(t, root, "internal/appmap/appmap.go", "package appmap\n")
	write(t, root, "package.json", `{"devDependencies":{"vitest":"1","@playwright/test":"1.61.0"}}`)
	write(t, root, "src/dep.ts", "export const dep = 1\n")
	write(t, root, "tests/dep.test.ts", "import { test } from \"vitest\"\nimport { dep } from \"../src/dep\"\n")
	write(t, root, "internal/appmap/testdata/e2e/playwright.config.ts", "import { defineConfig } from \"@playwright/test\"\nexport default defineConfig({})\n")
	write(t, root, fixtureE2ESpec, "import { test } from \"@playwright/test\"\ntest(\"login\", async () => {})\n")
	return root
}

func e2eEntries(entries []string) []string {
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry, FrontierE2ERuntimeDependency+":") {
			found = append(found, entry)
		}
	}
	return found
}

// AFP-V0-725: an end-to-end runner site inside a hosted Go testdata fixture is the fixture
// unit's own frontier, so a plan that reaches TypeScript elsewhere does not name it, and a plan
// that reaches the fixture spec does.
func TestFixtureE2ESiteIsTheFixtureUnitsOwn_AFPV0725(t *testing.T) {
	root := writeE2EFixtureRepository(t)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if found := e2eEntries(result.Frontier); len(found) != 0 {
		t.Fatalf("plugin frontier still carries the fixture e2e site: %v", found)
	}
	fixture := unitByID(result, unitID(runnerPlaywright, fixtureE2ESpec))
	site := e2eEntries(fixture.Frontier)
	if len(site) != 1 || !strings.Contains(site[0], fixtureE2ESpec) {
		t.Fatalf("fixture unit %s frontier = %v, want its e2e site", fixture.ID, fixture.Frontier)
	}
	graph, err := affected.Build(context.Background(), root, golang.New(), New())
	if err != nil {
		t.Fatal(err)
	}
	if plan := affected.Select(graph, []string{"src/dep.ts"}); plan.Scope != affected.ScopeBounded || len(frontierDetails(plan)) != 0 {
		t.Errorf("src/dep.ts: %s %v, want BOUNDED with no frontier", plan.Scope, frontierDetails(plan))
	}
	if plan := affected.Select(graph, []string{fixtureE2ESpec}); !slices.Contains(frontierDetails(plan), site[0]) {
		t.Errorf("%s: %s %v, want its e2e site named", fixtureE2ESpec, plan.Scope, frontierDetails(plan))
	}
}

// AFP-V0-725: the e2e site stays plugin-wide when a runner config outside the fixture encloses
// the spec, or when the spec escapes the fixture by import; and the graph raises it back when no
// Go unit hosts the testdata directory (AFP-V0-446).
func TestFixtureE2ESiteStaysPluginWide_AFPV0725(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, root string)
		localize bool
	}{
		{"a config outside the fixture encloses the spec", func(t *testing.T, root string) {
			write(t, root, "playwright.config.ts", "import { defineConfig } from \"@playwright/test\"\nexport default defineConfig({})\n")
		}, false},
		{"the spec imports a source outside the fixture", func(t *testing.T, root string) {
			write(t, root, fixtureE2ESpec, "import { test } from \"@playwright/test\"\nimport { dep } from \"../../../../src/dep\"\ntest(\"login\", async () => {})\n")
		}, false},
		{"no Go unit hosts the testdata directory", func(t *testing.T, root string) {
			write(t, root, "internal/appmap/appmap.go", "")
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeE2EFixtureRepository(t)
			tc.setup(t, root)
			result, err := New().Units(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if pluginWide := len(e2eEntries(result.Frontier)) != 0; pluginWide == tc.localize {
				t.Fatalf("plugin frontier e2e entries %v, localized=%v", e2eEntries(result.Frontier), tc.localize)
			}
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			if found := e2eEntries(frontierDetails(affected.Select(graph, []string{"src/dep.ts"}))); len(found) == 0 {
				t.Errorf("src/dep.ts plan names no e2e site; want the plugin-wide site")
			}
		})
	}
}
