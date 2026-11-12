package appmap

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// gotoFixture commits a react-router app whose specs navigate by a literal goto, a for-of loop
// over a same-file array and a same-file helper, plus any extra files, and builds its map.
func gotoFixture(t *testing.T, extra map[string]string) (string, *Map) {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	files := map[string]string{
		"appmap.json": `{"schema": "application-map-manifest/0", "app": "web", "base_path": "/web",
  "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
  "tests": {"root": "e2e", "specs": ["e2e/specs"]}}
`,
		"src/routes.tsx": `import { createBrowserRouter } from 'react-router-dom';
export const router = createBrowserRouter([
  { path: '/', element: <Home /> },
  { path: '/manage/users', element: <Users /> },
  { path: '/manage/roles', element: <Roles /> },
  { path: '/reports', element: <Reports /> },
  { path: '/settings', element: <Settings /> },
  { path: '/audit', element: <Audit /> },
]);
`,
		"e2e/specs/plain.spec.ts": `import { test } from '@playwright/test';
test('home', async ({ page }) => {
  await page.goto('/web/');
});
`,
		"e2e/specs/loop.spec.ts": `import { test, expect } from '@playwright/test';

const PAGES = ['/web/reports', '/web/settings'];

for (const r of PAGES) {
  test(` + "`visits ${r}`" + `, async ({ page }) => {
    await page.goto(r);
    await expect(page.locator('h1')).toBeVisible();
  });
}
`,
		"e2e/specs/helper.spec.ts": `import { test, expect } from '@playwright/test';

async function gotoManage(page, sub) {
  await page.goto(` + "`/web/manage/${sub}`" + `);
}

test('users', async ({ page }) => {
  await gotoManage(page, 'users');
  await expect(page.locator('h1')).toBeVisible();
});

test('roles', async ({ page }) => {
  await gotoManage(page, 'roles');
  await expect(page.locator('h1')).toBeVisible();
});
`,
	}
	for name, text := range extra {
		files[name] = text
	}
	for name, text := range files {
		writeFile(t, root, name, text)
	}
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=", "commit", "-qm", "fixture")
	return root, build(t, root, gitTest(t, root, "rev-parse", "HEAD"))
}

// specFile returns the map's test file p.
func specFile(t *testing.T, m *Map, p string) TestFile {
	t.Helper()
	for _, f := range m.Files {
		if f.Path == p {
			return f
		}
	}
	t.Fatalf("no test file %s", p)
	return TestFile{}
}

// reachedTemplates returns the templates of the screens a spec reaches and the basis per template.
func reachedTemplates(m *Map) map[string]string {
	out := map[string]string{}
	for _, s := range m.Screens {
		if len(s.Specs) > 0 {
			out[s.Template] = s.Specs[0].Basis
		}
	}
	return out
}

// inferStarts returns the templates of the start screens of infer's candidate flows and the
// titles of the tests infer read.
func inferStarts(t *testing.T, root string, m *Map) ([]string, []string) {
	t.Helper()
	k, flows, _, _, _, err := inferRead(context.Background(), m, []string{"e2e"}, nil, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, s := range m.Screens {
		byID[s.ID] = s.Template
	}
	starts := []string{}
	for _, f := range flows {
		starts = append(starts, byID[f.Start])
	}
	titles := []string{}
	for _, tt := range k.Tests {
		titles = append(titles, tt.Title)
	}
	slices.Sort(starts)
	slices.Sort(titles)
	return slices.Compact(starts), titles
}

func gotoURLs(f TestFile) []string {
	out := []string{}
	for _, g := range f.Gotos {
		out = append(out, g.URL)
	}
	return out
}

func TestAMAPV0972LoopGotos(t *testing.T) {
	root, m := gotoFixture(t, map[string]string{
		"e2e/specs/inline.spec.ts": "import { test } from '@playwright/test';\n" +
			"test('inline', async ({ page }) => {\n" +
			"  for (const p of ['audit', 'reports']) {\n" +
			"    await page.goto(`/web/${p}`);\n" +
			"  }\n" +
			"});\n",
		"e2e/specs/shadow.spec.ts": "import { test } from '@playwright/test';\n" +
			"const ROUTES = ['/web/audit'];\n" +
			"for (const r of ROUTES) {\n" +
			"  test('shadow', async ({ page }) => {\n" +
			"    const r = other();\n" +
			"    await page.goto(r);\n" +
			"  });\n" +
			"}\n",
		"e2e/specs/let.spec.ts": "import { test } from '@playwright/test';\n" +
			"test('let', async ({ page }) => {\n" +
			"  for (let r of ['/web/audit']) {\n" +
			"    await page.goto(r);\n" +
			"  }\n" +
			"});\n",
		"e2e/specs/call.spec.ts": "import { test } from '@playwright/test';\n" +
			"test('call', async ({ page }) => {\n" +
			"  for (const r of loadPages()) {\n" +
			"    await page.goto(r);\n" +
			"  }\n" +
			"});\n",
	})
	if got := gotoURLs(specFile(t, m, "e2e/specs/loop.spec.ts")); !slices.Equal(got, []string{"/web/reports", "/web/settings"}) {
		t.Fatalf("const-array loop gotos = %v", got)
	}
	inline := specFile(t, m, "e2e/specs/inline.spec.ts")
	if got := gotoURLs(inline); !slices.Equal(got, []string{"/web/audit", "/web/reports"}) {
		t.Fatalf("inline template loop gotos = %v", got)
	}
	if inline.Gotos[0].Line != inline.Gotos[1].Line || inline.Gotos[0].Line != 4 {
		t.Errorf("loop gotos lines = %d, %d, want 4", inline.Gotos[0].Line, inline.Gotos[1].Line)
	}
	for _, p := range []string{"e2e/specs/shadow.spec.ts", "e2e/specs/let.spec.ts", "e2e/specs/call.spec.ts"} {
		f := specFile(t, m, p)
		if len(f.Gotos) != 1 || f.Gotos[0].Screen != "" || f.Gotos[0].Reason != "non-literal-url" {
			t.Errorf("%s gotos = %+v, want one non-literal-url", p, f.Gotos)
		}
	}
	if got := reachedTemplates(m); got["/reports"] != "goto" || got["/settings"] != "goto" {
		t.Errorf("loop screens reached = %v", got)
	}
	_, titles := inferStarts(t, root, m)
	if !slices.Contains(titles, "visits /web/reports") || !slices.Contains(titles, "visits /web/settings") {
		t.Errorf("infer tests = %v, want one per loop value", titles)
	}
}

func TestAMAPV0973SpecHelperAttribution(t *testing.T) {
	_, m := gotoFixture(t, map[string]string{
		"e2e/specs/dyn.spec.ts": "import { test } from '@playwright/test';\n" +
			"async function open(page, where) {\n" +
			"  await page.goto(`/web/${where}`);\n" +
			"}\n" +
			"test('dyn', async ({ page }) => {\n" +
			"  await open(page, pick());\n" +
			"});\n",
	})
	f := specFile(t, m, "e2e/specs/helper.spec.ts")
	if len(f.HelperGotos) != 2 {
		t.Fatalf("helper_gotos = %+v, want the two literal calls", f.HelperGotos)
	}
	for _, tmpl := range []string{"/manage/users", "/manage/roles"} {
		var s *Screen
		for i := range m.Screens {
			if m.Screens[i].Template == tmpl {
				s = &m.Screens[i]
			}
		}
		if s == nil || len(s.Specs) != 1 || s.Specs[0].Basis != "helper-goto" || s.Specs[0].File != "e2e/specs/helper.spec.ts" ||
			len(s.Specs[0].Via) != 1 || !strings.HasPrefix(s.Specs[0].Via[0], "e2e/specs/helper.spec.ts:") {
			t.Errorf("%s specs = %+v, want one helper-goto attribution at the call", tmpl, s)
		}
	}
	if d := specFile(t, m, "e2e/specs/dyn.spec.ts"); len(d.HelperGotos) != 0 {
		for _, g := range d.HelperGotos {
			if g.Screen != "" {
				t.Errorf("a non-literal helper argument placed %+v", g)
			}
		}
	}
	for _, s := range m.Screens {
		for _, a := range s.Specs {
			if a.File == "e2e/specs/dyn.spec.ts" {
				t.Errorf("a non-literal helper argument attributed %s", s.Template)
			}
		}
	}
}

func TestAMAPV0974CoverageAndInferAgree(t *testing.T) {
	root, m := gotoFixture(t, map[string]string{
		"e2e/specs/dynamic.spec.ts": "import { test } from '@playwright/test';\n" +
			"for (const r of loadPages()) {\n" +
			"  test(`dyn ${r}`, async ({ page }) => {\n" +
			"    await page.goto(r);\n" +
			"  });\n" +
			"}\n",
		// A literal loop inside a test body: coverage and infer both read each value's visit.
		"e2e/specs/inbody.spec.ts": "import { test } from '@playwright/test';\n" +
			"test('in body', async ({ page }) => {\n" +
			"  for (const p of ['audit', 'settings']) {\n" +
			"    await page.goto(`/web/${p}`);\n" +
			"  }\n" +
			"});\n",
	})
	raw, err := ProjectModuleCoverage(context.Background(), m, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var cov struct {
		Reached int `json:"screens_reached"`
		Total   int `json:"screens_total"`
	}
	if err := json.Unmarshal(raw, &cov); err != nil {
		t.Fatal(err)
	}
	reached := []string{}
	for tmpl := range reachedTemplates(m) {
		reached = append(reached, tmpl)
	}
	slices.Sort(reached)
	want := []string{"/", "/audit", "/manage/roles", "/manage/users", "/reports", "/settings"}
	if !slices.Equal(reached, want) || cov.Reached != len(want) || cov.Total != 6 {
		t.Fatalf("coverage reached %v (%d of %d), want %v of 6", reached, cov.Reached, cov.Total, want)
	}
	starts, _ := inferStarts(t, root, m)
	if !slices.Equal(starts, reached) {
		t.Errorf("infer start screens %v differ from coverage reached %v", starts, reached)
	}
	k, _, _, _, _, err := inferRead(context.Background(), m, []string{"e2e"}, nil, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var visits []string
	for _, tt := range k.Tests {
		if tt.Title != "in body" {
			continue
		}
		for _, st := range tt.Steps {
			if st.Kind == StepVisit {
				visits = append(visits, st.Target)
			}
		}
	}
	if !slices.Equal(visits, []string{"/web/audit", "/web/settings"}) {
		t.Errorf("in-body loop visits = %v, want one goto per loop value", visits)
	}
	d := specFile(t, m, "e2e/specs/dynamic.spec.ts")
	if len(d.Gotos) != 1 || d.Gotos[0].Reason != "non-literal-url" || d.Gotos[0].Screen != "" {
		t.Errorf("dynamic loop gotos = %+v, want one non-literal-url", d.Gotos)
	}
}

// AMAP-V0-971/972: a `for` header that binds the loop variable again, destructured or not, leaves
// the goto non-literal; so do an array that forEach or a length write may change, and a const
// declared in another function's block. The reference check reads loop gotos through the same
// resolver.
func TestAMAPV0972LoopRebindsAndMutation(t *testing.T) {
	spec := func(body string) string {
		return "import { test } from '@playwright/test';\n" + body
	}
	inner := func(header string) string {
		return spec("test('t', async ({ page }) => {\n" +
			"  for (const u of ['/web/audit', '/web/reports']) {\n" +
			"    for (" + header + ") {\n" +
			"      await page.goto(u);\n" +
			"    }\n" +
			"  }\n" +
			"});\n")
	}
	loopOver := func(prelude string) string {
		return spec(prelude + "\ntest('t', async ({ page }) => {\n" +
			"  for (const r of PAGES) {\n" +
			"    await page.goto(r);\n" +
			"  }\n" +
			"});\n")
	}
	cases := map[string]string{
		"e2e/specs/entries.spec.ts":   inner("const [k, u] of Object.entries(m)"),
		"e2e/specs/object.spec.ts":    inner("const {u} of rows"),
		"e2e/specs/assign.spec.ts":    inner("u of rows"),
		"e2e/specs/forin.spec.ts":     inner("const u in table"),
		"e2e/specs/foreach.spec.ts":   loopOver("const PAGES = ['/web/audit'];\nPAGES.forEach((p, i, all) => { all[i] = '/web/x'; });"),
		"e2e/specs/truncate.spec.ts":  loopOver("const PAGES = ['/web/audit'];\nPAGES.length = 0;"),
		"e2e/specs/increment.spec.ts": loopOver("const PAGES = ['/web/audit'];\nPAGES.length++;"),
		"e2e/specs/third.spec.ts":     loopOver("const PAGES = ['/web/audit'];\nPAGES.map((p, i, all) => all);"),
		"e2e/specs/scope.spec.ts":     loopOver("function setup() {\n  const PAGES = ['/web/audit'];\n}"),
		"e2e/specs/reads.spec.ts":     loopOver("const PAGES = ['/web/audit'];\nlog(PAGES.map((p, i) => p));"),
	}
	_, m := gotoFixture(t, cases)
	for p := range cases {
		f := specFile(t, m, p)
		if p == "e2e/specs/reads.spec.ts" {
			if got := gotoURLs(f); !slices.Equal(got, []string{"/web/audit"}) {
				t.Errorf("%s: a two-parameter map that only reads keeps the proof, gotos %v", p, got)
			}
			continue
		}
		if len(f.Gotos) != 1 || f.Gotos[0].Screen != "" || f.Gotos[0].Reason != "non-literal-url" {
			t.Errorf("%s gotos = %+v, want one non-literal-url", p, f.Gotos)
		}
	}

	routes := []string{}
	for _, r := range scanRefs(spec("const PAGES = ['/web/reports', '/web/settings'];\n" +
		"for (const r of PAGES) {\n  test('t', async ({ page }) => { await page.goto(r); });\n}\n")) {
		if r.kind == refRoute {
			routes = append(routes, r.v.mode+" "+r.v.value+r.v.reason)
		}
	}
	if !slices.Equal(routes, []string{"exact /web/reports", "exact /web/settings"}) {
		t.Errorf("refs loop routes = %v", routes)
	}
}

// AMAP-V0-972: a loop body that may bind the loop variable again keeps the goto unknown: a nested
// loop header, braced or not (`for await`), and a declaration list naming it; every `.length`
// use of a const array leaves its loop unproven.
func TestAMAPV0972ConservativeBodies(t *testing.T) {
	spec := func(body string) string {
		return "import { test } from '@playwright/test';\n" + body
	}
	outer := func(body string) string {
		return spec("test('t', async ({ page }) => {\n" +
			"  for (const u of ['/web/audit', '/web/reports']) {\n" +
			"    " + body + "\n" +
			"    await page.goto(u);\n" +
			"  }\n" +
			"});\n")
	}
	loopOver := func(prelude string) string {
		return spec(prelude + "\ntest('t', async ({ page }) => {\n" +
			"  for (const r of PAGES) {\n" +
			"    await page.goto(r);\n" +
			"  }\n" +
			"});\n")
	}
	cases := map[string]string{
		"e2e/specs/forawait.spec.ts":   outer("for await (u of rows) foo(u);"),
		"e2e/specs/unbraced.spec.ts":   outer("for (u of rows) foo(u);"),
		"e2e/specs/letlist.spec.ts":    outer("let a, u;"),
		"e2e/specs/asyncarrow.spec.ts": outer("['/b'].map(async (u) => foo());"),
		"e2e/specs/predec.spec.ts":     loopOver("const PAGES = ['/web/audit'];\n--PAGES.length;"),
		"e2e/specs/pattern.spec.ts":    loopOver("const PAGES = ['/web/audit'];\n[PAGES.length] = [0];"),
		"e2e/specs/forlength.spec.ts":  loopOver("const PAGES = ['/web/audit'];\nfor (PAGES.length of [0]) {}"),
	}
	_, m := gotoFixture(t, cases)
	for p := range cases {
		f := specFile(t, m, p)
		if len(f.Gotos) != 1 || f.Gotos[0].Screen != "" || f.Gotos[0].Reason != "non-literal-url" {
			t.Errorf("%s gotos = %+v, want one non-literal-url", p, f.Gotos)
		}
	}
}
