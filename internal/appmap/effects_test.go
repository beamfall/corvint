package appmap

import (
	"sort"
	"strings"
	"testing"
)

const effectHomePage = `import { Page } from '@playwright/test';

export class HomePage {
  constructor(private readonly page: Page) {}

  async open() {
    await this.page.goto('/#!/home');
  }

  async goToTeeSheets() {
    await this.page.getByTestId('nav-teesheets').click();
  }

  async openSheets() {
    await this.page.getByRole('link', { name: 'Tee sheets' }).first().click();
    await this.page.waitForLoadState();
  }

  async search(term: string) {
    await this.page.getByLabel('Search').fill(term);
  }

  async loop() {
    await this.loop();
  }
}
`

const effectSheetPage = `import { Page } from '@playwright/test';

export class TeeSheetPage {
  constructor(private readonly page: Page) {}

  async backHome() {
    await this.helper();
  }

  private async helper() {
    await expect(this.page.getByText('Sheet')).toBeVisible();
    await this.page.goBack();
  }
}
`

func effectSpec(step string) string {
	return "import { test, expect } from '@playwright/test';\nimport { HomePage } from '../pages/home.page';\nimport { TeeSheetPage } from '../pages/teesheet.page';\n\n" +
		"test('steps', async ({ page, request }) => {\n  const home = new HomePage(page);\n  " + step + "\n  const sheet: TeeSheetPage = new TeeSheetPage(page);\n  await sheet.backHome();\n  const again = new HomePage(page);\n});\n"
}

func stepEdges(m *Map, path string) []Edge {
	out := []Edge{}
	for _, f := range m.Files {
		if f.Path != path {
			continue
		}
		for _, e := range m.Edges {
			if e.Basis == "test-sequence" && e.Source == f.ID {
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Anchor.Start < out[j].Anchor.Start })
	return out
}

// ANL-V0-035: each test-sequence step is classified from the spec span and the page-object
// methods it calls; only provable navigation, waits and reads are navigation-only.
func TestANLV0035StepEffects(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "e2e/pages/home.page.ts", effectHomePage)
	writeFile(t, root, "e2e/pages/teesheet.page.ts", effectSheetPage)
	cases := map[string]struct{ step, effect, ref, call string }{
		"nav":       {"await home.openSheets();", EffectNavigation, "", ""},
		"fill":      {"await home.search('x');", EffectState, "e2e/pages/home.page.ts:20", "fill"},
		"request":   {"await request.post('/api/slots');", EffectState, "e2e/specs/request.spec.ts:7", "post"},
		"storage":   {"await page.evaluate(() => localStorage.setItem('a', 'b'));", EffectState, "e2e/specs/storage.spec.ts:7", "setItem"},
		"testid":    {"await home.goToTeeSheets();", EffectUnknown, "e2e/pages/home.page.ts:11", "click"},
		"helper":    {"await login(page);", EffectUnknown, "e2e/specs/helper.spec.ts:7", "login"},
		"cycle":     {"await home.loop();", EffectUnknown, "e2e/pages/home.page.ts:24", "loop"},
		"missing":   {"await home.nothing();", EffectUnknown, "e2e/specs/missing.spec.ts:7", "nothing"},
		"evaluate":  {"await page.evaluate(() => 1);", EffectUnknown, "e2e/specs/evaluate.spec.ts:7", "evaluate"},
		"rawclick":  {"await page.click('a.sheets');", EffectUnknown, "e2e/specs/rawclick.spec.ts:7", "click"},
		"template":  {"await page.goto(`/x/${slug()}`);", EffectUnknown, "e2e/specs/template.spec.ts:7", "template-substitution"},
		"unhandled": {"await page.keyboard.insertText('x');", EffectUnknown, "e2e/specs/unhandled.spec.ts:7", "insertText"},
	}
	for name, c := range cases {
		writeFile(t, root, "e2e/specs/"+name+".spec.ts", effectSpec(c.step))
	}
	rev := commitAll(t, root, "effects")
	m := build(t, root, rev)
	for name, c := range cases {
		edges := stepEdges(m, "e2e/specs/"+name+".spec.ts")
		if len(edges) != 2 {
			t.Fatalf("%s: %+v", name, edges)
		}
		first, back := edges[0], edges[1]
		if first.Effect != c.effect || first.EffectRef != c.ref || first.EffectCall != c.call {
			t.Errorf("%s: got %s %s %s", name, first.Effect, first.EffectRef, first.EffectCall)
		}
		// sheet.backHome() reaches goBack through a private helper and reads an assertion.
		if back.Effect != EffectNavigation || len(back.EffectFrom) != 2 {
			t.Errorf("%s back: %+v", name, back)
		}
	}
	nav := stepEdges(m, "e2e/specs/nav.spec.ts")[0]
	if len(nav.EffectFrom) != 1 || nav.EffectFrom[0].Path != "e2e/pages/home.page.ts" || nav.EffectFrom[0].Start != 14 {
		t.Fatalf("nav anchors: %+v", nav.EffectFrom)
	}
	click := stepEdges(m, "e2e/specs/teesheet-click.spec.ts")
	if len(click) != 1 || click[0].Effect != EffectUnknown || click[0].EffectCall != "click" {
		t.Fatalf("fixture click spec: %+v", click)
	}
	other := 0
	for _, e := range m.Edges {
		if e.Basis != "test-sequence" && (e.Effect != "" || e.EffectFrom != nil) {
			other++
		}
	}
	if other != 0 {
		t.Fatal("effect on a non-sequence edge")
	}
}

// ANL-V0-036: a navigation-only chain flags without a sidecar; a state-effect or unknown step
// abstains naming the step and call, and a read-only sidecar declaration cannot upgrade it; a
// changed page-object method makes the step not fresh.
func TestANLV0036PrefixesUseStepEffects(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "e2e/pages/home.page.ts", effectHomePage)
	writeFile(t, root, "e2e/pages/teesheet.page.ts", effectSheetPage)
	writeFile(t, root, "e2e/specs/nav.spec.ts", effectSpec("await home.openSheets();"))
	writeFile(t, root, "e2e/specs/fill.spec.ts", effectSpec("await home.search('x');"))
	writeFile(t, root, "e2e/specs/testid.spec.ts", effectSpec("await home.goToTeeSheets();"))
	rev := commitAll(t, root, "effects")
	m := build(t, root, rev)
	nav := stepEdges(m, "e2e/specs/nav.spec.ts")
	f := prefixFixture{m: m, root: root, rev: rev}
	l := provenLedger(f, [2]string{nav[0].From, nav[0].To}, [2]string{nav[1].From, nav[1].To})
	upgrade := &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{
		{Path: "e2e/specs/fill.spec.ts", ReadOnlyEdges: []string{prefixEdgeID(nav[0].From, nav[0].To), prefixEdgeID(nav[1].From, nav[1].To)}},
		{Path: "e2e/specs/testid.spec.ts", ReadOnlyEdges: []string{prefixEdgeID(nav[0].From, nav[0].To), prefixEdgeID(nav[1].From, nav[1].To)}},
	}}
	r := runPrefixes(t, f, l, upgrade, Options{})
	if len(r.Flagged) != 1 || r.Flagged[0].Test != "e2e/specs/nav.spec.ts" || r.Flagged[0].SavedSteps != 1 {
		t.Fatalf("%+v", r)
	}
	want := map[string]PrefixAbstention{
		"e2e/specs/fill.spec.ts":   {Reason: "step-state-effect", Ref: "e2e/pages/home.page.ts:20", Call: "fill"},
		"e2e/specs/testid.spec.ts": {Reason: "step-effect-unknown", Ref: "e2e/pages/home.page.ts:11", Call: "click"},
	}
	for _, a := range r.Abstained {
		if w, ok := want[a.Test]; ok {
			if a.Reason != w.Reason || a.Ref != w.Ref || a.Call != w.Call || a.Edge != prefixEdgeID(nav[0].From, nav[0].To) {
				t.Errorf("%s: %+v", a.Test, a)
			}
			delete(want, a.Test)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing abstentions %v in %+v", want, r.Abstained)
	}
	// A declared assertion still outranks a navigation-only step.
	assert := &PrefixFacts{Schema: PrefixFactsSchema, Tests: []PrefixFact{{Path: "e2e/specs/nav.spec.ts", AssertionEdges: []string{prefixEdgeID(nav[1].From, nav[1].To)}}}}
	if a := runPrefixes(t, f, l, assert, Options{}); len(a.Flagged) != 0 || reasonFor(a, "e2e/specs/nav.spec.ts") != "assertion-target" {
		t.Fatalf("%+v", a)
	}
	writeFile(t, root, "e2e/pages/home.page.ts", strings.Replace(effectHomePage, "await this.page.waitForLoadState();", "await this.page.getByLabel('Club').fill('1');", 1))
	changed := commitAll(t, root, "method changed")
	moved := *l
	moved.Revision = changed
	stale := runPrefixes(t, f, &moved, nil, Options{Revision: changed})
	if len(stale.Flagged) != 0 || reasonFor(stale, "e2e/specs/nav.spec.ts") != "step-effect-not-fresh" {
		t.Fatalf("%+v", stale)
	}
}
