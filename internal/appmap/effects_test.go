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

  async save() {
    const gateway = this.page;
    await gateway.reload();
  }

  async openWith(label = ` + "`${seedTeam()}`" + `) {
    await this.page.reload();
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

// effectResetPage's constructor issues a request: constructing it is a state effect.
const effectResetPage = `import { Page } from '@playwright/test';

export class ResetWorkflow {
  readonly ready: Promise<unknown>;

  constructor(private readonly page: Page) {
    this.ready = page.request.post('/api/reset');
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
		"optional":  {"await fetch?.('/api');", EffectUnknown, "e2e/specs/optional.spec.ts:7", "computed-call"},
		"computed":  {"await localStorage['setItem']('a', 'b');", EffectUnknown, "e2e/specs/computed.spec.ts:7", "computed-call"},
		"assign":    {"Object.assign(localStorage, {a: 'b'});", EffectUnknown, "e2e/specs/assign.spec.ts:7", "assign"},
		"member":    {"localStorage.token = 'x'; await home.openSheets();", EffectUnknown, "e2e/specs/member.spec.ts:7", "member-assignment"},
		"delete":    {"delete localStorage.token;", EffectUnknown, "e2e/specs/delete.spec.ts:7", "delete"},
		"rebind":    {"await home.openSheets(); home = other;", EffectUnknown, "e2e/specs/rebind.spec.ts:7", "openSheets"},
		"generic":   {"await home.search<string>('x');", EffectUnknown, "e2e/specs/generic.spec.ts:7", "generic-call"},
		"tagged":    {"mutate`value`;", EffectUnknown, "e2e/specs/tagged.spec.ts:7", "tagged-template"},
		"destruct":  {"({home} = replacement); await home.openSheets();", EffectUnknown, "e2e/specs/destruct.spec.ts:7", "openSheets"},
		"substset":  {"await page.goto(`/x/${localStorage.token = 'x'}`);", EffectUnknown, "e2e/specs/substset.spec.ts:7", "template-substitution"},
		"increment": {"localStorage.counter++;", EffectUnknown, "e2e/specs/increment.spec.ts:7", "member-update"},
		"receiver":  {"await home.save();", EffectUnknown, "e2e/pages/home.page.ts:29", "reload"},
		"otherroot": {"await api.goto('/x');", EffectUnknown, "e2e/specs/otherroot.spec.ts:7", "goto"},
		"rebound":   {"page = other; await page.goto('/x');", EffectUnknown, "e2e/specs/rebound.spec.ts:7", "goto"},
		"role":      {"await page.getByRole('link' && 'button').click();", EffectUnknown, "e2e/specs/role.spec.ts:7", "click"},
		"nonnull":   {"await home.search!('x');", EffectUnknown, "e2e/specs/nonnull.spec.ts:7", "computed-call"},
		"bang":      {"await helper!();", EffectUnknown, "e2e/specs/bang.spec.ts:7", "computed-call"},
		"hover":     {"await page.hover('a.sheets');", EffectUnknown, "e2e/specs/hover.spec.ts:7", "hover"},
		"default":   {"await home.openWith();", EffectUnknown, "e2e/pages/home.page.ts:32", "default-parameter"},
		"stepnamed": {"await test.step('reset', reset);", EffectUnknown, "e2e/specs/stepnamed.spec.ts:7", "callback"},
		"pollnamed": {"await expect.poll(check).toBe(1);", EffectUnknown, "e2e/specs/pollnamed.spec.ts:7", "callback"},
		"passnamed": {"await expect(check).toPass();", EffectUnknown, "e2e/specs/passnamed.spec.ts:7", "callback"},
		"stepfunc":  {"await test.step('reset', function() {} && reset);", EffectUnknown, "e2e/specs/stepfunc.spec.ts:7", "callback"},
		"pollfunc":  {"await expect.poll(function() {} && check).toBe(1);", EffectUnknown, "e2e/specs/pollfunc.spec.ts:7", "callback"},
		"passfunc":  {"await expect(function() {} && check).toPass();", EffectUnknown, "e2e/specs/passfunc.spec.ts:7", "callback"},
		"throwfunc": {"expect(function() {} && reset).toThrowError();", EffectUnknown, "e2e/specs/throwfunc.spec.ts:7", "callback"},
		"throwtwo":  {"expect(function() {} && reset).toThrow();", EffectUnknown, "e2e/specs/throwtwo.spec.ts:7", "callback"},
		"funcexpr":  {"await test.step('open', async function () { await home.openSheets(); }); expect(function () { JSON.parse('x'); }).toThrow();", EffectNavigation, "", ""},
		"inline":    {"await test.step('open', async () => { await home.openSheets(); }); await expect.poll(async () => page.url()).toBe('/'); await expect(() => { JSON.parse('1'); }).toPass();", EffectNavigation, "", ""},
		"query":     {"await page.goto(`/x/${slug}?tab=a`);", EffectNavigation, "", ""},
		"pure":      {"await Promise.all([home.openSheets()]); JSON.stringify({a: 1 == 1});", EffectNavigation, "", ""},
	}
	for name, c := range cases {
		writeFile(t, root, "e2e/specs/"+name+".spec.ts", effectSpec(c.step))
	}
	writeFile(t, root, "e2e/pages/reset.page.ts", effectResetPage)
	writeFile(t, root, "e2e/specs/ctor.spec.ts", strings.Replace(effectSpec("await new ResetWorkflow(page).ready;"),
		"import { TeeSheetPage }", "import { ResetWorkflow } from '../pages/reset.page';\nimport { TeeSheetPage }", 1))
	cases["ctor"] = struct{ step, effect, ref, call string }{"", EffectState, "e2e/pages/reset.page.ts:7", "post"}
	// A class that extends another or declares an accessor runs code this reader does not follow.
	writeFile(t, root, "e2e/pages/child.page.ts", "import { Page } from '@playwright/test';\nimport { BasePage } from './base.page';\n\nexport class ChildPage extends BasePage {\n  async open() {\n    await this.page.reload();\n  }\n}\n")
	writeFile(t, root, "e2e/pages/gate.page.ts", "import { Page } from '@playwright/test';\n\nexport class GatePage {\n  constructor(private readonly page: Page) {}\n\n  set ready(v: boolean) {\n    localStorage.setItem('ready', String(v));\n  }\n}\n")
	for name, class := range map[string]string{"child": "ChildPage", "gate": "GatePage"} {
		writeFile(t, root, "e2e/specs/"+name+".spec.ts", strings.Replace(effectSpec("new "+class+"(page);"),
			"import { TeeSheetPage }", "import { "+class+" } from '../pages/"+name+".page';\nimport { TeeSheetPage }", 1))
	}
	cases["child"] = struct{ step, effect, ref, call string }{"", EffectUnknown, "e2e/specs/child.spec.ts:8", "extends"}
	cases["gate"] = struct{ step, effect, ref, call string }{"", EffectUnknown, "e2e/specs/gate.spec.ts:8", "accessor"}
	writeFile(t, root, "e2e/specs/args.spec.ts", strings.Replace(effectSpec("await home.openSheets();"),
		"const again = new HomePage(page);", "const again = new HomePage(page, await request.post('/api/reset'));", 1))
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
		// sheet.backHome() reaches goBack through a private helper and reads an assertion; both
		// constructions bounding the step are read.
		if back.Effect != EffectNavigation || len(back.EffectFrom) != 3 {
			t.Errorf("%s back: %+v", name, back)
		}
	}
	nav := stepEdges(m, "e2e/specs/nav.spec.ts")[0]
	// The whole spec and page-object files are cited: imports and bindings decide the methods read.
	if len(nav.EffectFrom) != 3 || nav.EffectFrom[0].Path != "e2e/specs/nav.spec.ts" || nav.EffectFrom[1].Path != "e2e/pages/home.page.ts" || nav.EffectFrom[1].Start != 1 || nav.EffectFrom[2].Path != "e2e/pages/teesheet.page.ts" {
		t.Fatalf("nav anchors: %+v", nav.EffectFrom)
	}
	// The closing construction's own arguments run in the step that ends at it.
	if back := stepEdges(m, "e2e/specs/args.spec.ts")[1]; back.Effect != EffectState || back.EffectRef != "e2e/specs/args.spec.ts:10" || back.EffectCall != "post" {
		t.Fatalf("closing construction arguments: %+v", back)
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
	// An import-only change outside every step span also leaves the derived effect not fresh.
	writeFile(t, root, "e2e/pages/home.page.ts", effectHomePage)
	writeFile(t, root, "e2e/pages/home2.page.ts", strings.Replace(effectHomePage, "await this.page.waitForLoadState();", "await this.page.getByLabel('Club').fill('1');", 1))
	writeFile(t, root, "e2e/specs/nav.spec.ts", strings.Replace(effectSpec("await home.openSheets();"), "'../pages/home.page'", "'../pages/home2.page'", 1))
	imported := *l
	imported.Revision = commitAll(t, root, "import changed")
	if r := runPrefixes(t, f, &imported, nil, Options{Revision: imported.Revision}); len(r.Flagged) != 0 || reasonFor(r, "e2e/specs/nav.spec.ts") != "step-effect-not-fresh" {
		t.Fatalf("%+v", r)
	}
}
