// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// AZU-V0-210 (V1-4999): a declared runtime caller is selected for its tests
// alone on a change in another language, so its plugin does not take part
// and the plugin-wide TypeScript frontier is reported in frontierExcluded, not
// as LANGUAGE_FRONTIER. The fault injection reaches the same caller through an
// import edge from a dirty TypeScript source: then the plugin takes part and
// every plugin-wide entry bears again.
func TestDeclaredRuntimeCallerIsTerminal_AZU_V0_210(t *testing.T) {
	const site = "typescript:e2e-runtime-dependency:e2e/a.spec.ts:1 `runner`"
	const bare = "typescript:bare-import-undeclared:src/widget.ts:1 `vscode`"
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, RuntimeEdgesPath), []byte(`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/a.spec.ts","runs":"g:server","reason":"declared server caller"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := fake{name: "typescript", frontier: []string{bare, site}, units: []Unit{
		{ID: "typescript:widget", Sources: []string{"src/widget.ts"}, Tests: []string{"src/widget.test.ts"}},
		{ID: "typescript:helper", Sources: []string{"src/helper.ts"}},
		{ID: "typescript:e2e", Tests: []string{"e2e/a.spec.ts"}, Imports: []string{"typescript:helper"}},
	}}
	g := fake{name: "g", units: []Unit{
		{ID: "g:server", Sources: []string{"server.g"}, Tests: []string{"server_test.g"}},
		{ID: "g:other", Sources: []string{"other.g"}, Tests: []string{"other_test.g"}},
	}}
	graph, err := Build(context.Background(), root, g, ts)
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"other.g"})
	frontier := languageFrontier(plan)
	if slices.Contains(frontier, bare) || slices.Contains(frontier, site) {
		t.Errorf("a change in another language raised the TypeScript frontier: %v", frontier)
	}
	if plan.Scope != ScopeBounded {
		t.Errorf("scope %s, unknown %+v", plan.Scope, plan.Unknown)
	}
	e2e := slices.IndexFunc(plan.Selected, func(s Selection) bool { return s.UnitID == "typescript:e2e" })
	if e2e < 0 || plan.Selected[e2e].Witness.Kind != WitnessFrontierSite {
		t.Fatalf("declared caller not selected as a frontier site: %+v", plan.Selected)
	}
	excluded := slices.IndexFunc(plan.FrontierExcluded, func(e FrontierExclusion) bool { return e.Language == "typescript" })
	if excluded < 0 || !slices.Contains(plan.FrontierExcluded[excluded].Frontier, bare) || !slices.Contains(plan.FrontierExcluded[excluded].Units, "typescript:e2e") {
		t.Errorf("TypeScript frontier not reported beside the plan: %+v", plan.FrontierExcluded)
	}
	for _, bearer := range plan.FrontierBearers {
		if bearer.Language == "typescript" && bearer.Via == FrontierBearerParticipating {
			t.Errorf("TypeScript bears as a participating plugin: %+v", bearer)
		}
	}
	// Fault injection: the same caller reached through an import edge from a
	// dirty TypeScript source is no terminal selection, so the plugin takes
	// part and the plugin-wide entry bears.
	plan = Select(graph, []string{"src/helper.ts"})
	if !slices.Contains(languageFrontier(plan), bare) || plan.Scope != ScopeUnknown {
		t.Errorf("a reached TypeScript dependent must keep the plugin-wide frontier: %+v", plan.Unknown)
	}
}
