// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// AZU-V0-004, AZU-V0-8902: covered inbound sites select declared tests even
// when their server's import graph misses a runtime dependency. Participation
// of TypeScript itself must not bring the discharged site back as unknown.
func TestDeclaredInboundRuntime_AZU_V0_8902(t *testing.T) {
	const site = "typescript:e2e-runtime-dependency:e2e/a.spec.ts:1 `runner`"
	for _, declared := range []bool{false, true} {
		root := t.TempDir()
		if declared {
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, RuntimeEdgesPath), []byte(`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"e2e/a.spec.ts","runs":"g:server","reason":"declared server caller"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		ts := fake{name: "typescript", frontier: []string{site}, units: []Unit{
			{ID: "typescript:source", Sources: []string{"src.ts"}, Tests: []string{"src.test.ts"}},
			{ID: "typescript:e2e", Tests: []string{"e2e/a.spec.ts"}},
		}}
		g := fake{name: "g", units: []Unit{{ID: "g:server", Sources: []string{"server.g"}, Tests: []string{"server_test.g"}}}}
		graph, err := Build(context.Background(), root, g, ts)
		if err != nil {
			t.Fatal(err)
		}
		for _, dirty := range []string{"src.ts", "server.g"} {
			plan := Select(graph, []string{dirty})
			unknown := slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Detail == site })
			selected := slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == "typescript:e2e" })
			if declared && (unknown || !selected) {
				t.Fatalf("declared: selected=%v unknown=%v plan=%+v", selected, unknown, plan)
			}
			if !declared && !unknown && dirty == "src.ts" {
				t.Fatal("undeclared participating site lost its uncertainty")
			}
		}
		if declared && graph.inboundRuntimeSite("typescript:e2e-runtime-dependency:+10") != "" {
			t.Fatal("truncated site closed")
		}
	}
}
