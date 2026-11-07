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

// AFP-V0-9009, AZU-V0-8902 (V1-5026): a line-less site, `path `+"`token`"+`,
// reaches the directory runtime edge that declares its test, so the declared
// caller is selected and the entry closes; without the declaration the same
// entry stays unknown (the fault injection).
func TestLinelessInboundRuntimeSite_AFP_V0_9009(t *testing.T) {
	const site = "typescript:e2e-runtime-dependency:fixture/tests/items.spec.cjs `playwright`"
	for _, declared := range []bool{true, false} {
		root := t.TempDir()
		if declared {
			if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, RuntimeEdgesPath), []byte(`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"fixture/tests/","runs":"g:server","reason":"declared fixture server caller"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		ts := fake{name: "typescript", frontier: []string{site}, units: []Unit{
			{ID: "typescript:source", Sources: []string{"src.ts"}, Tests: []string{"src.test.ts"}},
			{ID: "typescript:fixture", Tests: []string{"fixture/tests/items.spec.cjs"}},
		}}
		g := fake{name: "g", units: []Unit{{ID: "g:server", Sources: []string{"server.g"}, Tests: []string{"server_test.g"}}}}
		graph, err := Build(context.Background(), root, g, ts)
		if err != nil {
			t.Fatal(err)
		}
		plan := Select(graph, []string{"src.ts"})
		unknown := slices.ContainsFunc(plan.Unknown, func(u Unknown) bool { return u.Detail == site })
		selected := slices.ContainsFunc(plan.Selected, func(s Selection) bool { return s.UnitID == "typescript:fixture" })
		if declared && (unknown || !selected) {
			t.Fatalf("declared: selected=%v unknown=%v plan=%+v", selected, unknown, plan)
		}
		if !declared && (!unknown || selected) {
			t.Fatalf("undeclared: selected=%v unknown=%v; the entry must stay unknown", selected, unknown)
		}
	}
}

// AFP-V0-9009: the site parse drops the backquoted token, then an all-digit
// line, and names no file for a truncated or malformed entry.
func TestInboundRuntimeSiteFile_AFP_V0_9009(t *testing.T) {
	for site, want := range map[string]string{
		"e2e/a.spec.ts":                  "e2e/a.spec.ts",
		"e2e/a.spec.ts:12":               "e2e/a.spec.ts",
		"e2e/a.spec.ts `runner`":         "e2e/a.spec.ts",
		"e2e/a.spec.ts:1 `runner`":       "e2e/a.spec.ts",
		"e2e/a:b.spec.ts:3 `@scope/x:y`": "e2e/a:b.spec.ts",
		"e2e/a.spec.ts:x":                "e2e/a.spec.ts:x",
		"e2e/a.spec.ts`":                 "",
		"+10":                            "+10",
		"e2e/a.spec.ts: `runner`":        "e2e/a.spec.ts:",
	} {
		if got := inboundRuntimeSiteFile(site); got != want {
			t.Errorf("inboundRuntimeSiteFile(%q) = %q, want %q", site, got, want)
		}
	}
}

// AFP-V0-9009: a test whose own name is the whole site text is that site, and
// a site whose whole text and parsed file are both tests stays unknown, so a
// strip can never move a site onto another declared test.
func TestInboundRuntimeSiteAmbiguity_AFP_V0_9009(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, RuntimeEdgesPath), []byte(`{"profile":"corvint-runtime-edges/0","edges":[{"tests":"declared/","runs":"g:server","reason":"declared caller"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	ts := fake{name: "typescript", units: []Unit{
		{ID: "typescript:plain", Tests: []string{"declared/a.spec.cjs"}},
		{ID: "typescript:named", Tests: []string{"declared/a.spec.cjs `x`"}},
		{ID: "typescript:other", Tests: []string{"elsewhere/b.spec.cjs `x`"}},
		{ID: "typescript:target", Tests: []string{"declared/b.spec.cjs"}},
		{ID: "typescript:whole", Tests: []string{"declared/c.spec.cjs `y`"}},
	}}
	g := fake{name: "g", units: []Unit{{ID: "g:server", Sources: []string{"server.g"}}}}
	graph, err := Build(context.Background(), root, g, ts)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "typescript:e2e-runtime-dependency:"
	for site, want := range map[string]string{
		prefix + "declared/a.spec.cjs `x`":  "",                  // both readings are tests
		prefix + "declared/a.spec.cjs `pw`": "typescript:plain",  // only the parsed file is
		prefix + "elsewhere/b.spec.cjs `x`": "",                  // the whole site is the test, undeclared
		prefix + "declared/c.spec.cjs `y`":  "typescript:whole",  // the whole site is the declared test
		prefix + "declared/b.spec.cjs:7":    "typescript:target", // a located site
	} {
		if got := graph.inboundRuntimeSite(site); got != want {
			t.Errorf("inboundRuntimeSite(%q) = %q, want %q", site, got, want)
		}
	}
}
