package changee2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
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

const irBindingSpec = `import { test, expect } from '@playwright/test';
import { TeeSheetPage } from '../pages/teesheet.page';

test('banners through the site-knowledge IR', async ({ page }) => {
  await page.goto('/#!/home');
  await expect(page.getByTestId('home-banner')).toHaveText('Welcome');
  const sheet = new TeeSheetPage(page);
  await sheet.selectSlot();
  await expect(page.getByTestId('sheet-banner')).toHaveText('Ready');
  await page.goto('/#!/nowhere');
  await expect(page.getByTestId('lost-banner')).toHaveText('Gone');
});
`

// CCE-V0-310: with --source declared, the site-knowledge IR binds a proposal the map does not bind
// by its own line or flow: a visit to the screen its URL resolves to, an assertion to the screen
// of the last visit or joined page-object action before it. The map's own line and flow bindings
// are kept, an unresolved visit forgets the screen, and without sources nothing changes.
func TestCCEV0310HarvestBindingSiteKnowledge(t *testing.T) {
	root, _ := fixtureRepo(t)
	// The map resolves this file's gotos but places none of its outcomes on a flow or screen.
	rev := commitFiles(t, root, map[string]*string{"e2e/extra/zz-ir.spec.ts": text(irBindingSpec),
		"e2e/specs/zz-binding.spec.ts": text(bindingSpec)}, "ir binding spec")
	bindings := func(sources []string) map[string]Binding {
		t.Helper()
		p, err := Harvest(bg, HarvestOptions{Root: root, Rev: rev, Sources: sources})
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
		return got
	}
	without, with := bindings(nil), bindings([]string{"e2e"})
	if len(with) != len(without) {
		t.Fatalf("sources changed the proposal set: %d vs %d", len(with), len(without))
	}
	for key, b := range without {
		want := b
		switch key {
		case KindOutcome + " home-banner":
			want = Binding{Screen: "screen:admin:app.home", Basis: appmap.SKLineBasisStep}
		case KindOutcome + " sheet-banner":
			// The joined page object's action moved the test to the tee sheet (SKH-V0-185).
			want = Binding{Screen: "screen:admin:app.clubs.teesheets", Basis: appmap.SKLineBasisStep}
		}
		if b.Basis == appmap.SKLineBasisStep || b.Basis == appmap.SKLineBasisVisit {
			t.Fatalf("%s bound through the IR without sources: %+v", key, b)
		}
		if with[key] != want {
			t.Fatalf("%s: with sources %+v, want %+v (without %+v)", key, with[key], want, b)
		}
	}
	// Kept: the map's own goto and flow bindings and their reasons; the unresolved visit forgot
	// the screen, so the outcome after it stays unbound.
	for key, want := range map[string]Binding{
		KindPath + " /#!/home":       {Screen: "screen:admin:app.home", Basis: "app-map-goto"},
		KindOutcome + " slot-status": {Flow: "book-tee-time", Basis: "flow-outcome-locator"},
		KindPath + " /#!/nowhere":    {Unbound: "goto-unresolved"},
		KindOutcome + " lost-banner": {Unbound: "no-flow-or-screen"},
	} {
		if with[key] != want {
			t.Fatalf("%s: %+v, want %+v", key, with[key], want)
		}
	}
	// A declared source the IR cannot read refuses rather than silently binding through the map.
	if _, err := Harvest(bg, HarvestOptions{Root: root, Rev: rev, Sources: []string{"no-such-dir"}}); refusalCode(err) != "e2e-site-knowledge-unavailable" {
		t.Fatalf("unreadable source: %v", err)
	}
}

// CCE-V0-310: a path proposal the map does not bind takes the IR's visit screen for its line, an
// outcome only an assertion's; a line kind mismatch, or no IR line, keeps the map's reason.
func TestCCEV0310BindProposalIRPrecedence(t *testing.T) {
	m := &appmap.Map{}
	at := func(line int) appmap.SKLine { return appmap.SKLine{Path: "e2e/a.spec.ts", Line: line} }
	lines := map[appmap.SKLine]appmap.SKLineScreen{
		at(3): {Screen: "screen:x", Basis: appmap.SKLineBasisVisit},
		at(4): {Screen: "screen:y", Basis: appmap.SKLineBasisStep},
	}
	block := func(kind string, line int) Block {
		return Block{Kind: kind, Span: Span{Path: "e2e/a.spec.ts", Start: line, End: line}}
	}
	for _, c := range []struct {
		b    Block
		want Binding
	}{
		{block(KindPath, 3), Binding{Screen: "screen:x", Basis: appmap.SKLineBasisVisit}},
		{block(KindPath, 4), Binding{Unbound: "test-file-not-mapped"}},
		{block(KindPath, 5), Binding{Unbound: "test-file-not-mapped"}},
		{block(KindOutcome, 4), Binding{Screen: "screen:y", Basis: appmap.SKLineBasisStep}},
		{block(KindOutcome, 3), Binding{Unbound: "no-flow-or-screen"}},
	} {
		if got := bindProposal(m, lines, c.b); got != c.want {
			t.Fatalf("%s line %d: %+v, want %+v", c.b.Kind, c.b.Span.Start, got, c.want)
		}
		if got := bindProposal(nil, lines, c.b); got != (Binding{Unbound: "app-map-unavailable"}) {
			t.Fatalf("no map: %+v", got)
		}
	}
}
