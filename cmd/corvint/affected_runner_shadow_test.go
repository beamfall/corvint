// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/runnerselection"
)

func TestRSPV0040AffectedReportsShadowWithoutEnforcement(t *testing.T) {
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	before := affectedGit(t, root, "status", "--porcelain")
	receipt, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	shadow := receipt.Advice.RunnerSelectionShadow
	if shadow == nil || !shadow.Shadow || shadow.GoverningPlan != "static-unqualified" || len(shadow.Runners) == 0 {
		t.Fatalf("missing shadow: %+v", shadow)
	}
	for _, r := range shadow.Runners {
		if r.Mode != "full" || r.Counts.InventoryKnown || len(r.Excluded) != 0 {
			t.Fatalf("absent runner narrowed: %+v", r)
		}
	}
	plan, _ := json.Marshal(receipt.Plan)
	provider, _ := json.Marshal(receipt.Provider)
	compact, err := compactAffected(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if compact.Advice.RunnerSelectionShadow != shadow {
		t.Fatal("compact advice lost shadow")
	}
	// No shadow field is part of the governing plan or provider.
	if bytes.Contains(plan, []byte("shadow")) || bytes.Contains(provider, []byte("shadow")) {
		t.Fatal("shadow leaked into execution inputs")
	}
	if after := affectedGit(t, root, "status", "--porcelain"); after != before {
		t.Fatalf("read changed repository: %q -> %q", before, after)
	}
}

func TestRSPV0040AffectedUnknownInputsRemainFull(t *testing.T) {
	p := affectedRunnerShadow("", "", []string{"opaque path"})
	if !p.Shadow || len(p.Runners) == 0 {
		t.Fatal("missing shadow refusal")
	}
	for _, r := range p.Runners {
		if r.Mode != "full" || len(r.Reasons) == 0 || len(r.Excluded) != 0 {
			t.Fatalf("unknown input narrowed: %+v", r)
		}
	}
	a, _ := json.Marshal(p)
	b, _ := json.Marshal(affectedRunnerShadow("", "", []string{"opaque path"}))
	if !bytes.Equal(a, b) {
		t.Fatal("declaration-only shadow is not replay stable")
	}
}

func TestRSPV0016AffectedShadowJSONProjection(t *testing.T) {
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for _, extra := range [][]string{nil, {"--full"}} {
		args := append([]string{"--base", base}, extra...)
		receipt, _, stderr, code := runAffectedArguments(t, root, args...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		raw, err := json.Marshal(receipt["advice"].(map[string]any)["runner_selection_shadow"])
		if err != nil {
			t.Fatal(err)
		}
		var shadow runnerselection.Plan
		if err := json.Unmarshal(raw, &shadow); err != nil || !shadow.Shadow || len(shadow.Runners) == 0 {
			t.Fatalf("shadow projection %s: %v", raw, err)
		}
	}
}

func TestRSPV0016AffectedUnknownCountsStayUnknown(t *testing.T) {
	shadow := affectedRunnerShadow(strings.Repeat("a", 40), strings.Repeat("b", 40), nil)
	for _, r := range shadow.Runners {
		if r.Counts.InventoryKnown || r.Counts.Selected != 0 || r.Mode != "full" {
			t.Fatalf("static test files invented inventory counts: %+v", r)
		}
	}
	if shadow.LoadingNS != nil || shadow.SelectionNS != nil {
		t.Fatal("declaration report invented retained timings")
	}
}
