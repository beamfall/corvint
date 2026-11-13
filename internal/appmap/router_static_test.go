package appmap

import (
	"slices"
	"strings"
	"testing"
)

const staticManifest = `{"schema": "application-map-manifest/0", "app": "site",
	"routers": [{"path": "src/App.tsx", "dialect": "react-router/0"},
		{"path": "src/shared/exposedRoutes.ts", "dialect": "static-routes/0", "export": "PRERENDER_ROUTES"}],
	"tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}}`

const staticApp = `export function App() {
  return (
    <Routes>
      <Route path="/app/*" element={<Shell />} />
      <Route path="/lab" element={<Lab />} />
      <Route path="/pricing" element={<Pricing />} />
      <Route path="*" element={<Navigate to="/app" replace />} />
    </Routes>
  );
}
`

const staticList = `import { EXPOSED_LOCALES, localePath } from './i18n.ts'

export const PRERENDER_ROUTES = [
  '/',
  '/pricing',
  '/terms',
  '/terms',
  'relative',
  '/item/:id',
  ...MORE,
] as const

export const EXPOSED_WEB_ROUTES = [
  ...EXPOSED_LOCALES.flatMap((locale) => PRERENDER_ROUTES.map((route) => localePath(locale, route))),
  '/app',
] as const
`

func staticRepo(t *testing.T, manifest, list string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "src/App.tsx", staticApp)
	writeFile(t, root, "src/shared/exposedRoutes.ts", list)
	writeFile(t, root, "e2e/pages/terms.page.ts", "export class TermsPage {}\n")
	writeFile(t, root, "e2e/specs/terms.spec.ts", "test('terms', async ({ page }) => {\n  await page.goto('/terms');\n});\n")
	gitTest(t, root, "init", "-q")
	commitAll(t, root, "fixture")
	return root
}

// AMAP-V0-980: each literal absolute path of a static-routes/0 export is a resolved screen of
// dialect static-routes/0 that test navigation reaches; a repeated path is one screen; a path the
// application router also declares stays that router's one screen; a relative, parameterised or
// non-literal element reads its reason; other exports of the file are not read.
func TestAMAPV0980PrerenderedRoutes(t *testing.T) {
	m := build(t, staticRepo(t, staticManifest, staticList), "HEAD")
	got := templates(m)
	for _, want := range []string{"/", "/terms"} {
		s, ok := got[want]
		if !ok || s.Status != StatusResolved || s.Abstract || s.Dialect != DialectStatic || s.State != "static:"+want {
			t.Errorf("%s: %+v (present %v)", want, s, ok)
		}
	}
	if s := got["/pricing"]; s.Dialect != DialectReact {
		t.Errorf("/pricing must stay the React router's screen: %+v", s)
	}
	if _, ok := got["/app"]; ok {
		t.Errorf("another export of the file was read")
	}
	n := 0
	for _, s := range m.Screens {
		if s.Dialect == DialectStatic {
			n++
		}
	}
	if n != 2 {
		t.Errorf("static screens = %d, want 2: %v", n, slices.Collect(mapsKeys(got)))
	}
	reasons := map[string]int{}
	for _, u := range m.Unknowns {
		if u.Path == "src/shared/exposedRoutes.ts" {
			reasons[u.Reason]++
		}
	}
	if reasons["unsupported-static-route"] != 2 || reasons["non-literal-static-route"] != 1 {
		t.Errorf("unknown reasons = %v", reasons)
	}
	reached := false
	for _, a := range got["/terms"].Specs {
		reached = reached || a.File == "e2e/specs/terms.spec.ts"
	}
	if !reached {
		t.Errorf("/terms not reached by its spec: %+v", got["/terms"])
	}
}

// AMAP-V0-980: the export member is required for static-routes/0 and refused elsewhere; a list
// that is missing, declared twice, mutated or computed declares no screen and reads its reason.
func TestAMAPV0980StaticRoutesRefusals(t *testing.T) {
	for _, bad := range []string{
		strings.Replace(staticManifest, `, "export": "PRERENDER_ROUTES"`, ``, 1),
		strings.Replace(staticManifest, `"export": "PRERENDER_ROUTES"`, `"export": "not-an-identifier"`, 1),
		strings.Replace(staticManifest, `"dialect": "react-router/0"}`, `"dialect": "react-router/0", "export": "X"}`, 1),
	} {
		if _, err := decodeManifest([]byte(bad)); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: err = %v", bad, err)
		}
	}
	for list, reason := range map[string]string{
		"export const OTHER = ['/a']\n":                                         "missing-static-routes",
		"export const PRERENDER_ROUTES = ['/a']\nPRERENDER_ROUTES.push('/b')\n": "computed-static-routes",
		"export const PRERENDER_ROUTES = build()\n":                             "computed-static-routes",
		"export const PRERENDER_ROUTES = ['/a'].concat(more)\n":                 "computed-static-routes",
		"export const PRERENDER_ROUTES = [`/${x}`]\n":                           "computed-static-routes",
	} {
		m := build(t, staticRepo(t, staticManifest, list), "HEAD")
		for _, s := range m.Screens {
			if s.Dialect == DialectStatic {
				t.Errorf("%q declared %s", list, s.Template)
			}
		}
		if !slices.ContainsFunc(m.Unknowns, func(u Unknown) bool { return u.Reason == reason && u.Path == "src/shared/exposedRoutes.ts" }) {
			t.Errorf("%q: no %s in %+v", list, reason, m.Unknowns)
		}
	}
	typed := "export const PRERENDER_ROUTES: readonly string[] = ['/typed'] satisfies ReadonlyArray<string>;\n"
	if s, ok := templates(build(t, staticRepo(t, staticManifest, typed), "HEAD"))["/typed"]; !ok || s.Status != StatusResolved {
		t.Errorf("typed list: %+v %v", s, ok)
	}
}
