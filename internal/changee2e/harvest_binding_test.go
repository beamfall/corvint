package changee2e

import (
	"os"
	"path/filepath"
	"testing"
)

const bindingSpec = `import { test, expect } from '@playwright/test';

test('unbound outcome and route', async ({ page }) => {
  await page.goto('/#!/nowhere');
  await expect(page.getByTestId('orphan-banner')).toHaveText('Hello');
});
`

// CCE-V0-050: every harvested proposal names its flow or screen binding from the application map
// at the harvested revision, or an explicit unbound reason.
func TestCCEV0050HarvestBinding(t *testing.T) {
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{"e2e/specs/zz-binding.spec.ts": text(bindingSpec)}, "binding spec")
	p, err := Harvest(bg, HarvestOptions{Root: root, Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Binding{}
	for _, pr := range p.Proposals {
		b := pr.Binding
		if (b.Unbound == "") == (b.Basis == "") || (b.Basis != "" && b.Flow == "" && b.Screen == "") {
			t.Fatalf("proposal %s has neither exactly one binding nor an unbound reason: %+v", pr.ID, b)
		}
		got[pr.Block.Kind+" "+pr.Block.Locator+pr.Block.URL] = b
	}
	if b := got[KindOutcome+" slot-status"]; b.Flow != "book-tee-time" || b.Basis != "flow-outcome-locator" {
		t.Fatalf("flow outcome binding %+v", b)
	}
	if b := got[KindPath+" /#!/login"]; b.Screen == "" || b.Basis != "app-map-goto" {
		t.Fatalf("goto binding %+v (all %+v)", b, got)
	}
	if b := got[KindOutcome+" orphan-banner"]; b.Unbound != "no-flow-or-screen" {
		t.Fatalf("orphan outcome %+v", b)
	}
	if b := got[KindPath+" /#!/nowhere"]; b.Unbound != "goto-unresolved" {
		t.Fatalf("unresolved goto %+v", b)
	}

	// Without an application map every proposal is unbound, and harvesting still succeeds.
	noMap := commitFiles(t, root, map[string]*string{"appmap.json": nil}, "drop map")
	p, err = Harvest(bg, HarvestOptions{Root: root, Rev: noMap})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Proposals) == 0 {
		t.Fatal("no proposals without a map")
	}
	for _, pr := range p.Proposals {
		if pr.Binding != (Binding{Unbound: "app-map-unavailable"}) {
			t.Fatalf("proposal %s without a map: %+v", pr.ID, pr.Binding)
		}
	}
}

func TestCCEV0088HarvestConfiguredManifest(t *testing.T) {
	root, _ := fixtureRepo(t)
	data, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	const manifest = "config/club.appmap.json"
	rev := commitFiles(t, root, map[string]*string{"appmap.json": nil, manifest: text(string(data)),
		"e2e/specs/zz-binding.spec.ts": text(bindingSpec)}, "map at a configured path")
	outcome := func(p *Proposals) Binding {
		for _, pr := range p.Proposals {
			if pr.Block.Kind == KindOutcome && pr.Block.Locator == "slot-status" {
				return pr.Binding
			}
		}
		t.Fatalf("no slot-status proposal in %+v", p.Proposals)
		return Binding{}
	}
	p, err := Harvest(bg, HarvestOptions{Root: root, Rev: rev, Manifest: manifest})
	if err != nil {
		t.Fatal(err)
	}
	if b := outcome(p); b.Flow != "book-tee-time" || b.Basis != "flow-outcome-locator" {
		t.Fatalf("configured manifest binding %+v", b)
	}
	p, err = Harvest(bg, HarvestOptions{Root: root, Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	if b := outcome(p); b != (Binding{Unbound: "app-map-unavailable"}) {
		t.Fatalf("default manifest binding %+v", b)
	}
}
