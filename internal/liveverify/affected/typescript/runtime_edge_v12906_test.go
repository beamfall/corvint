package typescript

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// runtimeEdgeRepository is a Go server, the command that builds it, a Go
// tool whose test names the server's file, and a Playwright spec that drives
// the built server at run time, an edge neither plugin models.
func runtimeEdgeRepository(t *testing.T, edges string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module example.test/app\n\ngo 1.27\n")
	write(t, root, "server/server.go", "package server\n\nfunc Handle() string { return \"ok\" }\n")
	write(t, root, "server/server_test.go", "package server\n\nimport \"testing\"\n\nfunc TestHandle(t *testing.T) { _ = Handle() }\n")
	write(t, root, "cmd/server/main.go", "package main\n\nimport \"example.test/app/server\"\n\nfunc main() { println(server.Handle()) }\n")
	write(t, root, "package.json", `{"name":"app","private":true,"devDependencies":{"@playwright/test":"1.48.0"}}`+"\n")
	write(t, root, "playwright.config.ts", "import { defineConfig } from '@playwright/test';\nexport default defineConfig({ testDir: './e2e' });\n")
	write(t, root, "e2e/login.spec.ts", "import { test, expect } from '@playwright/test';\ntest('login', async ({ page }) => {\n  await page.goto('http://localhost:8080/');\n  await expect(page).toHaveTitle('ok');\n});\n")
	if edges != "" {
		write(t, root, affected.RuntimeEdgesPath, edges)
	}
	return root
}

// TestGoServerChangeSelectsDeclaredE2ESpec_V12906 pins AFP-V0-818 and
// AFP-V0-819 over the real Go and TypeScript plugins: with a declared runtime
// edge from the Playwright spec to the server command, a change to the Go
// server selects the spec through that command with a BINARY_EXEC witness;
// without the declaration the spec is not selected, no TypeScript unit is
// reached, and the e2e-runtime-dependency entry is never recorded as excluded.
func TestGoServerChangeSelectsDeclaredE2ESpec_V12906(t *testing.T) {
	declared := `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/","runs":"go:example.test/app/cmd/server","reason":"the Playwright specs drive the server cmd/server builds"}]}`
	for _, c := range []struct {
		name, edges string
		selects     bool
	}{{"declared", declared, true}, {"undeclared", "", false}} {
		t.Run(c.name, func(t *testing.T) {
			root := runtimeEdgeRepository(t, c.edges)
			graph, err := affected.Build(context.Background(), root, golang.New(), New())
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{"server/server.go"})
			var spec *affected.Selection
			for index, selection := range plan.Selected {
				if slices.Contains(selection.Tests, "e2e/login.spec.ts") {
					spec = &plan.Selected[index]
				}
			}
			if c.selects != (spec != nil) {
				t.Fatalf("spec selected=%v, want %v: %+v", spec != nil, c.selects, plan.Selected)
			}
			if spec != nil && (spec.Witness.Kind != affected.WitnessBinaryExec || spec.Witness.DirtyPath != "server/server.go") {
				t.Fatalf("spec witness %+v, want BINARY_EXEC from server/server.go", spec.Witness)
			}
			// Undeclared, the entry is never recorded as excluded; declared, the
			// edge models its site, so AFP-V0-560 may leave it out.
			for _, excluded := range plan.FrontierExcluded {
				for _, entry := range excluded.Frontier {
					if strings.HasPrefix(entry, FrontierE2ERuntimeDependency+":") && c.edges == "" {
						t.Fatalf("the e2e runtime entry was excluded: %+v", plan.FrontierExcluded)
					}
				}
			}
		})
	}
}

const runtimeEdgeGoDeclaration = `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/","runs":"go:example.test/app/cmd/server","reason":"the Playwright specs drive the server cmd/server builds"}]}`

// AFP-V0-819: a graph built from a subset of the plugins skips an edge whose
// other side it does not hold, while the full build still refuses a stale
// edge and every build refuses an invalid declaration.
func TestPartialGraphSkipsOutOfGraphRuntimeEdges_AFP_V0_819(t *testing.T) {
	root := runtimeEdgeRepository(t, runtimeEdgeGoDeclaration)
	for name, languages := range map[string][]affected.Language{"go only": {golang.New()}, "typescript only": {New()}} {
		if _, err := affected.BuildPartial(context.Background(), root, languages...); err != nil {
			t.Errorf("%s partial graph: %v", name, err)
		}
	}
	if _, err := affected.Build(context.Background(), root, golang.New()); err == nil || !strings.Contains(err.Error(), "holds no test file") {
		t.Fatalf("full Go-only build err=%v, want the stale-edge refusal", err)
	}
	write(t, root, affected.RuntimeEdgesPath, `{"profile":"corvint-runtime-edges/1","edges":[]}`)
	if _, err := affected.BuildPartial(context.Background(), root, golang.New()); err == nil || !strings.Contains(err.Error(), "profile must be") {
		t.Fatalf("partial build of an invalid declaration err=%v", err)
	}
}

// AFP-V0-819: the Playwright graph, built from the TypeScript plugin alone,
// accepts a valid Go-to-Playwright edge.
func TestPlaywrightGraphAcceptsGoRuntimeEdge_AFP_V0_819(t *testing.T) {
	root := runtimeEdgeRepository(t, runtimeEdgeGoDeclaration)
	if _, err := preparePlaywrightGraph(root, "playwright.config.ts", nil); err != nil {
		t.Fatalf("preparePlaywrightGraph: %v", err)
	}
}

// AFP-V0-819: the Playwright source identity binds the runtime-edges
// declaration's presence and bytes, so an edge retargeted mid-plan fails the
// final source check.
func TestPlaywrightSourcesBindRuntimeEdges_AFP_V0_819(t *testing.T) {
	root := runtimeEdgeRepository(t, "")
	absent, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, affected.RuntimeEdgesPath, runtimeEdgeGoDeclaration)
	present, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil || present == absent {
		t.Fatalf("presence not bound: absent=%s present=%s err=%v", absent, present, err)
	}
	write(t, root, affected.RuntimeEdgesPath, strings.Replace(runtimeEdgeGoDeclaration, `"e2e/"`, `"e2e/login.spec.ts"`, 1))
	retargeted, err := ObservePlaywrightSources(root, "playwright.config.ts")
	if err != nil || retargeted == present {
		t.Fatalf("bytes not bound: present=%s retargeted=%s err=%v", present, retargeted, err)
	}
}
