package appmap

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// AMAP-V0-342 (#1934): a page object that navigates only through a helper whose URL parameter
// flows unchanged into `page.goto` (alone or after a base prefix) stays unbound, and its remedy
// offers each literal URL passed to the helper as a learned candidate, never a binding.
func TestAMAPV0342HelperGotoCandidates(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	writeFile(t, root, "e2e/support/nav.ts", "import { Page } from '@playwright/test';\nconst base = '';\n"+
		"export async function openRoute(page: Page, path: string, timeout: number) {\n  await page.goto(base + path, { timeout });\n}\n"+
		"export const changed = async (page, path) => {\n  path = path + '?x';\n  await page.goto(path);\n};\n")
	writeFile(t, root, "e2e/pages/via.page.ts", "import { openRoute, changed } from '../support/nav';\n\nexport class ViaPage {\n"+
		"  async open() {\n    await openRoute(this.page, '/#!/home', 10);\n  }\n  async other() {\n    await changed(this.page, '/#!/clubs/1/settings');\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "helper navigation"))
	if !hasUnknown(m, "page-object", fileID("e2e/pages/via.page.ts"), "page-object-unbound") {
		t.Fatalf("helper navigation bound the page object: %+v", m.Unknowns)
	}
	var u Unknown
	for _, x := range m.Unknowns {
		if x.Path == "e2e/pages/via.page.ts" {
			u = x
		}
	}
	r := UnknownRemedy(m, u)
	if r.Key != RemedyKeyPageObjectScreens || !reflect.DeepEqual(r.Candidates, []string{`{"path":"e2e/pages/via.page.ts","state":"app.home"}`}) ||
		!strings.HasPrefix(r.Basis, "learned:") || !strings.Contains(r.Uncertainty, "binds no screen") {
		t.Fatalf("remedy %+v", r)
	}

	f := readFacts("function twice(p) { page.goto(p) }\nfunction twice(p) { page.goto(p) }\nfunction two(a, b) { page.goto(a); page.waitForURL(b) }\n" +
		"const one = (x, p): Promise<void> => { return page.waitForURL(p); };\nfunction lit(p) { page.goto('/a' + p) }\n" +
		"async function typed(page: Page, p: string): Promise<void> { await page.goto(p) }\nfunction based(page, base, p) { page.goto(base + p) }\n")
	if !reflect.DeepEqual(f.helpers, map[string]int{"one": 1, "lit": 0, "typed": 1}) {
		t.Fatalf("helpers %v", f.helpers)
	}
	if c := readFacts("go(page, '/a', `/b`, x, '/c' + y, { a: 1 }, '/d')").calls; len(c) != 1 || !reflect.DeepEqual(c[0].lits, []string{"", "/a", "/b", "", "", "", "/d"}) {
		t.Fatalf("calls %+v", c)
	}
}

// AMAP-V0-343 (#1934): stats --remedies lists every keyed remedy of the map's unknowns, grouped by
// file, key and candidates with the unknowns each names, and counts the unknowns no declaration
// names; without it the report is unchanged.
func TestAMAPV0343StatsRemedies(t *testing.T) {
	m := &Map{Manifest: Anchor{Path: "appmap.json"}, Screens: []Screen{{ID: "screen:a:home", State: "home"}},
		Files: []TestFile{{Path: "e2e/pages/a.page.ts", HelperGotos: []Goto{{Line: 3, Screen: "screen:a:home"}}}, {Path: "e2e/pages/b.page.ts"}},
		Unknowns: []Unknown{{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/a.page.ts"},
			{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/b.page.ts"},
			{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/b.page.ts"},
			{Kind: "screen", Reason: "non-literal-url", Path: "app/routes.js"}}}
	rows, keyless := mapRemedies(m)
	if keyless != 3 || len(rows) != 1 || rows[0].(statsRemedy).Unknowns != 1 ||
		rows[0].(statsRemedy).Remedy.Candidates[0] != `{"path":"e2e/pages/a.page.ts","state":"home"}` {
		t.Fatalf("remedies %+v %d", rows, keyless)
	}
	lost := &Map{Manifest: Anchor{Path: "appmap.json"}, Files: []TestFile{{Path: "e2e/pages/c.page.ts", HelperGotos: []Goto{{Line: 7}}}}}
	if r := UnknownRemedy(lost, Unknown{Kind: "page-object", Reason: "page-object-unbound", Path: "e2e/pages/c.page.ts"}); len(r.Candidates) != 0 ||
		!strings.Contains(r.Uncertainty, "only through helper calls at lines 7") {
		t.Fatalf("unresolved helper remedy %+v", r)
	}
	with, err := ProjectStatsWith(context.Background(), m, Options{Full: true}, true)
	plain, err2 := ProjectStats(context.Background(), m, Options{Full: true})
	if err != nil || err2 != nil || !strings.Contains(string(with), `"remedies":[`) || !strings.Contains(string(with), `"unknowns_without_remedy_key":3`) ||
		strings.Contains(string(plain), "remedies") {
		t.Fatalf("stats:\n%s\n%s %v %v", with, plain, err, err2)
	}
}
