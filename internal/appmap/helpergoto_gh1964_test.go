package appmap

import (
	"reflect"
	"strings"
	"testing"
)

// AMAP-V0-370 (V1-3057, #1964): a helper whose other parameters carry literal defaults
// (`timeout: number = 60_000`) is a navigation helper; a default the reader does not read is the
// named gap `default-parameter` at the function's line, never a silently skipped function.
func TestAMAPV0370HelperDefaults(t *testing.T) {
	f := readFacts("export async function openRoute(page: Page, route: string, timeout: number = 60_000) {\n  await page.goto(`/app#${route}`, { timeout });\n}\n" +
		"export function plain(page, route, wait = 'load', strict = true) {\n  return page.goto(route);\n}\n" +
		"export function computed(page, route, t = budget()) {\n  return page.goto(route);\n}\n" +
		"export function quiet(page, t = budget()) {\n  return page.click('x');\n}\n")
	if want := map[string]int{"openRoute": 1, "plain": 1}; !reflect.DeepEqual(f.helpers, want) {
		t.Fatalf("helpers %v", f.helpers)
	}
	if want := map[string]string{"computed": "7:default-parameter"}; !reflect.DeepEqual(f.helperGaps, want) {
		t.Fatalf("gaps %v", f.helperGaps)
	}
}

// AMAP-V0-370 (V1-3057, #1964): under a declared base_path a template or URL-hash helper's
// candidate places its screen; a page object calling an inherited `this.openRoute(...)` that
// forwards to the helper is followed one hop, and a template argument's whole-segment
// substitution matches a parameterised state. A page object with neither navigation nor a gap
// still carries a detail, and an overriding method that does not navigate hides the base one.
func TestAMAPV0370HelperHopsInMap(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	manifest := strings.Replace(readText(t, root, "appmap.json"), `"hash_prefix": "#!",`, `"hash_prefix": "#!",`+"\n  \"base_path\": \"/app\",", 1)
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "e2e/support/nav.ts", "export async function openRoute(page: Page, route: string, timeout: number = 60_000) {\n  await page.goto(`/app#!${route}`, { timeout });\n}\n"+
		"export async function setHash(page, route) {\n  const t = new URL(page.url());\n  t.hash = route;\n  await page.goto(t.toString());\n}\n"+
		"export async function late(page, route, t = budget()) {\n  await page.goto(route);\n}\n")
	writeFile(t, root, "e2e/pages/base.page.ts", "import { Page } from '@playwright/test';\nimport { openRoute } from '../support/nav';\n\nexport class BasePage {\n"+
		"  constructor(readonly page: Page) {}\n  async openRoute(route: string) {\n    await openRoute(this.page, route);\n  }\n}\n")
	writeFile(t, root, "e2e/pages/items.page.ts", "import { BasePage } from './base.page';\nimport { setHash, late } from '../support/nav';\n\nexport class ItemsPage extends BasePage {\n"+
		"  async open(id: string) {\n    await this.openRoute(`/clubs/${id}/settings`);\n  }\n  async home() {\n    await setHash(this.page, '!/home');\n  }\n"+
		"  async later() {\n    await late(this.page, '/home');\n  }\n}\n")
	writeFile(t, root, "e2e/pages/override.page.ts", "import { BasePage } from './base.page';\n\nexport class OverridePage extends BasePage {\n"+
		"  async openRoute(route: string) {\n    await this.audit(route);\n  }\n  async open() {\n    await this.openRoute('/home');\n  }\n}\n")
	writeFile(t, root, "e2e/pages/idle.page.ts", "export class IdlePage {\n  async tap() {\n    await this.page.click('#x');\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "helper hops"))
	var items *TestFile
	for i := range m.Files {
		if m.Files[i].Path == "e2e/pages/items.page.ts" {
			items = &m.Files[i]
		}
	}
	if items == nil || items.Screen != "" {
		t.Fatalf("page object %+v", items)
	}
	got := []string{}
	for _, g := range items.HelperGotos {
		got = append(got, g.URL+" "+g.Screen+" "+g.Reason)
	}
	settings, home := screenID(m.App, "app.clubs.settings"), screenID(m.App, "app.home")
	want := []string{"/app#!/clubs/${}/settings " + settings + " helper-candidate:e2e/support/nav.ts:this.openRoute:0",
		"#!/home " + home + " helper-candidate:e2e/support/nav.ts:setHash:1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("helper gotos %q, want %q", got, want)
	}
	details := map[string]string{}
	for _, u := range m.Unknowns {
		if u.Reason == "page-object-unbound" {
			details[u.Path] = u.Detail
		}
	}
	if d := details["e2e/pages/items.page.ts"]; d != "helper-unfollowed:e2e/support/nav.ts:9:late:default-parameter" {
		t.Errorf("items detail %q", d)
	}
	if d := details["e2e/pages/idle.page.ts"]; d != "no-navigation" {
		t.Errorf("idle detail %q", d)
	}
	if d := details["e2e/pages/override.page.ts"]; d != "no-navigation" { // its own openRoute hides the base one
		t.Errorf("override detail %q", d)
	}
	if d := details["e2e/pages/base.page.ts"]; d != "no-navigation" {
		t.Errorf("base detail %q", d)
	}
}

// AMAP-V0-370 (batch review): a template helper whose explicit page lies outside the declared
// base_path places no screen (AMAP-V0-191), and a method declared twice, the second time with a
// default the reader does not read, follows neither declaration.
func TestAMAPV0370HelperReviewHoles(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	manifest := strings.Replace(readText(t, root, "appmap.json"), `"hash_prefix": "#!",`, `"hash_prefix": "#!",`+"\n  \"base_path\": \"/app\",", 1)
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "e2e/support/nav.ts", "export async function openRoute(page, route) {\n  await page.goto(`/app#!${route}`);\n}\n"+
		"export async function otherRoute(page, route) {\n  await page.goto(`/other#!${route}`);\n}\n")
	writeFile(t, root, "e2e/pages/outside.page.ts", "import { otherRoute } from '../support/nav';\n\nexport class OutsidePage {\n"+
		"  async open() {\n    await otherRoute(this.page, '/home');\n  }\n}\n")
	writeFile(t, root, "e2e/pages/twice.base.ts", "import { openRoute } from '../support/nav';\n\nexport class TwiceBase {\n"+
		"  open(r) {\n    openRoute(this.page, r);\n  }\n  open(r = compute()) {\n    this.page.click(r);\n  }\n}\n")
	writeFile(t, root, "e2e/pages/twice.page.ts", "import { TwiceBase } from './twice.base';\n\nexport class TwicePage extends TwiceBase {\n"+
		"  async home() {\n    await this.open('/home');\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "helper review holes"))
	for _, f := range m.Files {
		switch f.Path {
		case "e2e/pages/outside.page.ts":
			if len(f.HelperGotos) != 1 || f.HelperGotos[0].URL != "/other#!/home" || f.HelperGotos[0].Screen != "" {
				t.Errorf("outside base path: %+v", f.HelperGotos)
			}
		case "e2e/pages/twice.page.ts":
			if len(f.HelperGotos) != 0 {
				t.Errorf("duplicate method followed: %+v", f.HelperGotos)
			}
		}
	}
}
