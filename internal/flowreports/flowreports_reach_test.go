// SPDX-License-Identifier: Apache-2.0

package flowreports

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
)

// everyVariation is every flow and variation of the shop fixture, in report order.
var everyVariation = []string{"checkout/checkout.basic", "search/search.basic", "search/search.none"}

const (
	cartPath  = "src/components/Cart.vue"
	pricePath = "src/lib/price.ts"
)

// sourceLink adds an unreviewed declared source link from the flow's step to path.
func sourceLink(flow, step, path string) func(*appflows.FlowIntent) {
	return func(intent *appflows.FlowIntent) {
		if intent.FlowID == flow {
			intent.Links = append(intent.Links, appflows.FlowLink{From: step, Basis: "declared", Target: appflows.LinkTarget{Type: "source", Path: path}})
		}
	}
}

// impactOf is flows impact for base at HEAD.
func impactOf(t *testing.T, root, base string) appflows.ImpactReport {
	t.Helper()
	code, out, diagnostic := report(t, root, "impact", "--flows", "flows", "--base", base)
	if code != 0 {
		t.Fatalf("impact %d %s", code, diagnostic)
	}
	var impact appflows.ImpactReport
	if err := json.Unmarshal([]byte(out), &impact); err != nil {
		t.Fatal(err)
	}
	return impact
}

// wantEveryFlowReachUnknown checks the two reports limited by base: flows impact names every flow
// and variation with reason reach-unknown, lists unknown in reach_unknown under an UNKNOWN scope,
// and flows status --base keeps exactly those flows and variations with the reason and the graph.
func wantEveryFlowReachUnknown(t *testing.T, root, base string, unknown []string) appflows.ImpactReport {
	t.Helper()
	impact := impactOf(t, root, base)
	var impacted []string
	for _, f := range impact.Flows {
		if f.Reason != appflows.ReasonReachUnknown {
			t.Errorf("impact flow %s has reason %q, want %s", f.FlowID, f.Reason, appflows.ReasonReachUnknown)
		}
		for _, v := range f.Variations {
			impacted = append(impacted, f.FlowID+"/"+v)
		}
	}
	if !slices.Equal(impacted, everyVariation) {
		t.Errorf("impact names %v, want every flow and variation %v", impacted, everyVariation)
	}
	if !slices.Equal(impact.ReachUnknown, unknown) || impact.Graph.Scope != "UNKNOWN" {
		t.Errorf("impact reach_unknown %v scope %s, want %v UNKNOWN; graph unknown %q", impact.ReachUnknown, impact.Graph.Scope, unknown, impact.Graph.Unknown)
	}
	r, _ := status(t, root, "--base", base)
	var reported []string
	for _, f := range r.Flows {
		if f.Reason != appflows.ReasonReachUnknown {
			t.Errorf("status flow %s has reason %q, want %s", f.FlowID, f.Reason, appflows.ReasonReachUnknown)
		}
		for _, v := range f.Variations {
			reported = append(reported, f.FlowID+"/"+v.VariationID)
		}
	}
	if !slices.Equal(reported, everyVariation) || r.Totals.Variations != len(everyVariation) {
		t.Errorf("status --base %v totals %+v, want %v", reported, r.Totals, everyVariation)
	}
	if r.ImpactGraph == nil || r.ImpactGraph.Scope != "UNKNOWN" || !slices.Equal(r.ImpactGraph.Unknown, impact.Graph.Unknown) {
		t.Errorf("status --base graph %+v, impact graph %+v", r.ImpactGraph, impact.Graph)
	}
	return impact
}

// graphNames reports whether a graph unknown line starts with reason and names path.
func graphNames(impact appflows.ImpactReport, reason, path string) bool {
	return slices.ContainsFunc(impact.Graph.Unknown, func(line string) bool {
		return strings.HasPrefix(line, reason+": ") && strings.Contains(line, path)
	})
}

// AFL-V1-007 (reach-unknown, review finding B1): a changed path with no owning unit in the impact
// graph has unknown reach whether or not a link targets it. Cart.vue is linked by checkout and
// imported by src/search.ts through an edge the graph does not hold; a commit changing only
// Cart.vue names every flow, not checkout alone, and status --base drops none.
func TestAFLV1007LinkedUnownedChangedPathIsReachUnknown_V15623(t *testing.T) {
	fx := newShop(t, sourceLink("checkout", "pay", cartPath))
	write(t, fx.root, map[string]string{
		cartPath:        "<template><div/></template>\n",
		"src/search.ts": "import Cart from \"./components/Cart.vue\";\nexport function find(q: string): string {\n  return q + String(Cart);\n}\n",
	})
	base := commit(t, fx.root, "D: Cart.vue linked by checkout, imported by search")
	write(t, fx.root, map[string]string{cartPath: "<template><span/></template>\n"})
	commit(t, fx.root, "E: change only Cart.vue")
	impact := wantEveryFlowReachUnknown(t, fx.root, base, []string{cartPath})
	for _, f := range impact.Flows {
		if f.FlowID == "checkout" && (len(f.Hits) != 1 || f.Hits[0].Target.Path != cartPath || !slices.Equal(f.Hits[0].Via, []string{cartPath})) {
			t.Errorf("checkout keeps the hit that did resolve: %+v", f.Hits)
		}
		if f.FlowID == "search" && len(f.Hits) != 0 {
			t.Errorf("search has no resolved hit: %+v", f.Hits)
		}
	}
}

// AFL-V1-007 (reach-unknown, review finding B2): a link target with no owning unit in the impact
// graph (its language has no adapter) leaves every changed path's reach to it unresolved. Cart.vue
// is linked by checkout and imports src/lib/price.ts; a commit changing only price.ts names every
// flow with the reason and the target, never an empty list.
func TestAFLV1007UnownedLinkTargetIsReachUnknown_V15623(t *testing.T) {
	fx := newShop(t, sourceLink("checkout", "pay", cartPath))
	write(t, fx.root, map[string]string{
		cartPath:  "<script setup lang=\"ts\">\nimport { price } from \"../lib/price\";\n</script>\n<template><span>{{ price }}</span></template>\n",
		pricePath: "export const price = 1;\n",
	})
	base := commit(t, fx.root, "D: Cart.vue linked by checkout imports price.ts")
	write(t, fx.root, map[string]string{pricePath: "export const price = 2;\n"})
	commit(t, fx.root, "E: change only price.ts")
	impact := wantEveryFlowReachUnknown(t, fx.root, base, []string{pricePath})
	if !graphNames(impact, appflows.ImpactLinkTargetUnowned, cartPath) {
		t.Errorf("graph unknown %q does not name the unowned link target %s", impact.Graph.Unknown, cartPath)
	}
}

// AFL-V1-007 (reach-unknown): a link target that no longer exists has no owning unit either, so a
// later change of any other path names every flow.
func TestAFLV1007MissingLinkTargetIsReachUnknown_V15623(t *testing.T) {
	fx := newShop(t)
	git(t, fx.root, "rm", "-q", "src/search.ts")
	base := commit(t, fx.root, "D: delete the search link target")
	// The deleting commit itself: the deleted path has no owning unit at HEAD.
	wantEveryFlowReachUnknown(t, fx.root, fx.head, []string{"src/search.ts"})
	write(t, fx.root, map[string]string{"src/checkout.ts": "export function pay(n: number): number {\n  return n + 2;\n}\n"})
	commit(t, fx.root, "E: change only checkout.ts")
	impact := wantEveryFlowReachUnknown(t, fx.root, base, []string{"src/checkout.ts"})
	if !graphNames(impact, appflows.ImpactLinkTargetUnowned, "src/search.ts") {
		t.Errorf("graph unknown %q does not name the missing link target", impact.Graph.Unknown)
	}
	for _, f := range impact.Flows {
		if f.FlowID == "checkout" && len(f.Hits) != 1 {
			t.Errorf("checkout keeps its resolved hit: %+v", f.Hits)
		}
	}
}

// AFL-V1-007 (reach-unknown): only a flow's own intent file and the flow declaration are flow
// inputs; any other unowned file under the intents directory is not exempt from the reach rule.
func TestAFLV1007UnownedFileBesideIntentsIsReachUnknown_V15623(t *testing.T) {
	fx := newShop(t)
	write(t, fx.root, map[string]string{"flows/seed.sql": "insert into t values (1);\n", "flows/fixtures/cart.json": "{}\n"})
	commit(t, fx.root, "D: unowned files under the intents directory")
	wantEveryFlowReachUnknown(t, fx.root, fx.head, []string{"flows/fixtures/cart.json", "flows/seed.sql"})
}

// AFL-V1-007: with every changed path and every link target owned, the report stays narrowed and
// carries neither reach_unknown nor a reason, so earlier reports keep their bytes; an intent edit
// alone is a flow input and names no flow.
func TestAFLV1007ResolvedReachStaysNarrow_V15623(t *testing.T) {
	fx := newShop(t)
	impact := impactOf(t, fx.root, fx.b)
	if len(impact.Flows) != 1 || impact.Flows[0].FlowID != "checkout" || impact.Flows[0].Reason != "" || impact.ReachUnknown != nil {
		t.Fatalf("resolved reach: flows %+v reach_unknown %v", impact.Flows, impact.ReachUnknown)
	}
	if strings.Contains(strings.Join(impact.Graph.Unknown, "\n"), appflows.ImpactLinkTargetUnowned) {
		t.Fatalf("owned link targets are reported unowned: %q", impact.Graph.Unknown)
	}
	impact = impactOf(t, fx.root, fx.a)
	if len(impact.Flows) != 1 || impact.Flows[0].FlowID != "checkout" || impact.Flows[0].Reason != "" || impact.ReachUnknown != nil {
		t.Fatalf("an intent edit beside a resolved change: flows %+v reach_unknown %v", impact.Flows, impact.ReachUnknown)
	}
}
