// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// mustRunLanguage is a source-only plugin: an app unit whose spec covers
// src/app.ts, and a smoke unit nothing reaches.
type mustRunLanguage struct{}

func (mustRunLanguage) Name() string          { return "ts" }
func (mustRunLanguage) Owns(path string) bool { return strings.HasSuffix(path, ".ts") }
func (mustRunLanguage) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{
		{ID: "ts:app", Sources: []string{"src/app.ts"}, Tests: []string{"e2e/app.spec.ts"}},
		{ID: "ts:smoke", Tests: []string{"e2e/smoke/smoke.spec.ts"}},
	}}, nil
}

func mustRunSelector(t *testing.T, declaration string) (*e2eSelector, affected.Plan) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{"src/app.ts": "x", "e2e/app.spec.ts": "x", "e2e/smoke/smoke.spec.ts": "x"}
	if declaration != "" {
		files[affected.DeclaredSelectionPath] = declaration
	}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, mustRunLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/app.ts"})
	in := E2EInput{Provider: "e2e-provider.json", Revision: "HEAD", Graph: graph, Plan: plan}
	provider := SelectionProvider{Flows: "flows", RunnerConfig: "playwright.config.ts"}
	return newE2ESelector(context.Background(), root, in, provider, nil, nil), plan
}

// AFP-V0-2486: an e2e-safe selection keeps a test of a declared must-run unit,
// naming declared-must-run, while the must-run selection adds nothing to the
// obligation closure, so no other test is kept as static reach or linked to it.
func TestE2ESafeKeepsMustRunTests_AFPV02486(t *testing.T) {
	t.Parallel()
	const declaration = `{"profile":"corvint-test-selection/0","mustRun":[{"tests":"e2e/smoke/","reason":"release smoke suite"}]}`
	smoke := InventoryTest{TestKey: "smoke", Path: "e2e/smoke/smoke.spec.ts"}
	app := InventoryTest{TestKey: "app", Path: "e2e/app.spec.ts"}

	s, plan := mustRunSelector(t, declaration)
	if !slices.ContainsFunc(plan.MustRun, func(m affected.MustRun) bool { return m.UnitID == "ts:smoke" }) {
		t.Fatalf("plan.mustRun %+v does not name ts:smoke", plan.MustRun)
	}
	if got := s.reasons(smoke, nil); !slices.Equal(got, []string{"declared-must-run"}) {
		t.Fatalf("must-run test reasons %v, want [declared-must-run]", got)
	}
	if s.inClosure(smoke.Path) {
		t.Fatal("a must-run selection put its own test into the obligation closure")
	}
	linked := []EvaluatedLink{{Target: LinkTarget{Path: smoke.Path}}}
	if got := s.reasons(InventoryTest{TestKey: "other", Path: "e2e/other.spec.ts"}, linked); len(got) != 0 {
		t.Fatalf("a test linked only to the must-run test has reasons %v, want none", got)
	}
	if got := s.reasons(app, nil); !slices.Equal(got, []string{"static-reach"}) {
		t.Fatalf("reaching test reasons %v, want [static-reach]", got)
	}

	// Negative control: without the declaration the smoke test has no reason.
	bare, plan := mustRunSelector(t, "")
	if len(plan.MustRun) != 0 {
		t.Fatalf("plan.mustRun %+v without a declaration", plan.MustRun)
	}
	if got := bare.reasons(smoke, nil); len(got) != 0 {
		t.Fatalf("undeclared smoke test reasons %v, want none", got)
	}
}
