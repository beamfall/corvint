package appmap

import (
	"reflect"
	"testing"
)

// AMAP-V0-367 (V1-2967, #1953): a helper that passes its parameter as a template literal's one
// trailing substitution, or as the hash of a `new URL(...)` object it navigates to, is a
// navigation helper with that form; any other navigation form is a named gap with its line.
func TestAMAPV0367HelperForms(t *testing.T) {
	f := readFacts("export async function openRoute(page: Page, route: string) {\n  await page.goto(`/app#${route}`);\n}\n" +
		"export async function setHash(page, route) {\n  const t = new URL(page.url());\n  t.hash = route;\n  await page.goto(t.toString());\n}\n" +
		"function href(page, r) {\n  const u = new URL(page.url())\n  u.hash = r\n  return page.goto(u.href)\n}\n" +
		"function two(page, a, b) {\n  return page.goto(`${a}/${b}`);\n}\n" +
		"function moved(page, r) {\n  const u = new URL(page.url());\n  u.hash = r;\n  u.pathname = '/x';\n  return page.goto(u.toString());\n}\n" +
		"function called(page, r) {\n  return page.goto(build(r));\n}\n" +
		"function other(page, r) {\n  const u = new URL('/admin', page.url());\n  u.hash = r;\n  return page.goto(u.toString());\n}\n" +
		"function login(page) {\n  return page.goto('/login');\n}\n")
	if want := map[string]int{"openRoute": 1, "setHash": 1, "href": 1}; !reflect.DeepEqual(f.helpers, want) {
		t.Fatalf("helpers %v", f.helpers)
	}
	if want := map[string]helperForm{"openRoute": {prefix: "/app#"}, "setHash": {hash: true}, "href": {hash: true}}; !reflect.DeepEqual(f.helperForms, want) {
		t.Fatalf("forms %v", f.helperForms)
	}
	if want := map[string]string{"two": "15:template-form", "moved": "21:url-object", "called": "24:expression", "other": "29:url-object"}; !reflect.DeepEqual(f.helperGaps, want) {
		t.Fatalf("gaps %v", f.helperGaps)
	}
}

// AMAP-V0-367: in a map with a hash-based router, a page object navigating through a template or
// URL-hash helper is offered the state its literal reaches; a hash the router prefix does not
// start places no screen, and a helper the reader does not follow is named in the unbound
// unknown's detail with its file, line and reason.
func TestAMAPV0367HelperFormsInMap(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	writeFile(t, root, "e2e/support/nav.ts", "export async function openRoute(page, route) {\n  await page.goto(`/app#!${route}`);\n}\n"+
		"export async function setHash(page, route) {\n  const t = new URL(page.url());\n  t.hash = route;\n  await page.goto(t.toString());\n}\n"+
		"export async function plainHash(page, route) {\n  await page.goto(`/app#${route}`);\n}\n"+
		"export function two(page, a, b) {\n  return page.goto(`${a}/${b}`);\n}\n")
	writeFile(t, root, "e2e/pages/tpl.page.ts", "import { openRoute, setHash, plainHash } from '../support/nav';\n\nexport class TplPage {\n"+
		"  async open() {\n    await openRoute(this.page, '/home');\n  }\n  async again() {\n    await setHash(this.page, '!/home');\n  }\n"+
		"  async plain() {\n    await plainHash(this.page, '/home');\n  }\n}\n")
	writeFile(t, root, "e2e/pages/gap.page.ts", "import { two } from '../support/nav';\n\nexport class GapPage {\n"+
		"  async open() {\n    await two(this.page, 'a', 'b');\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "helper forms"))
	var tpl *TestFile
	for i := range m.Files {
		if m.Files[i].Path == "e2e/pages/tpl.page.ts" {
			tpl = &m.Files[i]
		}
	}
	if tpl == nil || tpl.Screen != "" || len(tpl.HelperGotos) != 3 {
		t.Fatalf("page object %+v", tpl)
	}
	got := []string{}
	for _, g := range tpl.HelperGotos {
		got = append(got, g.URL+" "+g.Screen)
	}
	home := screenID(m.App, "app.home")
	if want := []string{"/app#!/home " + home, "#!/home " + home, "/app#/home "}; !reflect.DeepEqual(got, want) {
		t.Fatalf("helper gotos %q, want %q", got, want)
	}
	for _, u := range m.Unknowns {
		if u.Path == "e2e/pages/gap.page.ts" {
			if u.Reason != "page-object-unbound" || u.Detail != "helper-unfollowed:e2e/support/nav.ts:13:two:template-form" {
				t.Fatalf("gap unknown %+v", u)
			}
			return
		}
	}
	t.Fatalf("no unknown for the gap page object: %+v", m.Unknowns)
}
