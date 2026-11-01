package appmap

import (
	"testing"
)

// v13932Repo is the VanillaTwisted shape: a react-router/0 App.tsx whose specific routes sit
// beside a `/*` splat, one of them rendered only after a guard return, and specs that navigate
// with their own literal page.goto.
func v13932Repo(t *testing.T) (*Map, string) {
	t.Helper()
	root := t.TempDir()
	gitTest(t, root, "init", "-q")
	writeFile(t, root, "package.json", `{"name":"web","devDependencies":{"@playwright/test":"1.50.0"}}`)
	writeFile(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"web",`+
		`"routers":[{"path":"apps/web/src/App.tsx","dialect":"react-router/0"}],`+
		`"tests":{"root":"apps/web","specs":["apps/web/e2e"],"page_objects":[]}}`)
	writeFile(t, root, "apps/web/src/App.tsx", `import { Routes, Route } from "react-router-dom";
export function App() {
  if (!ready) return null;
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/new" element={<New />} />
      <Route path="/dynamic/new" element={<DynamicNew />} />
      <Route path="/dynamic/:id" element={<Dynamic />} />
      <Route path="/settings" element={<Settings />} />
      <Route path="/*" element={<NotFound />} />
    </Routes>
  );
}
`)
	writeFile(t, root, "apps/web/e2e/journey-admission.spec.ts", `import { test } from "@playwright/test";
test("admission", async ({ page, browser }) => {
  await page.goto("/new");
  const other = await browser.newPage();
  await other.goto("/dynamic/new");
});
`)
	writeFile(t, root, "apps/web/e2e/journey-kindred.spec.ts", `import { test } from "@playwright/test";
test("kindred", async ({ page }) => {
  await page.goto("/kindred");
  await page.goto("/dynamic/42");
});
`)
	return build(t, root, commitAll(t, root, "fixture")), root
}

func specFiles(s *Screen) map[string]string {
	out := map[string]string{}
	for _, a := range s.Specs {
		out[a.File] = a.Basis
	}
	return out
}

// AMAP-V0-600 (V1-3932): in a react-router/0 app a literal goto to a specific route attributes the
// spec to that route's screen, not to the splat, render guard or not; a URL only the splat matches
// stays with the splat.
func TestAMAPV0600SpecificRouteBeatsSplat_V13932(t *testing.T) {
	m, _ := v13932Repo(t)
	byTemplate := map[string]*Screen{}
	for i := range m.Screens {
		s := &m.Screens[i]
		if s.Entry == nil || len(s.Entry.Constraints) != 1 || s.Entry.Constraints[0] != "render-guarded" {
			t.Fatalf("%s: entry %+v, want render-guarded", s.Template, s.Entry)
		}
		byTemplate[s.Template] = s
	}
	admission, kindred := "apps/web/e2e/journey-admission.spec.ts", "apps/web/e2e/journey-kindred.spec.ts"
	for template, want := range map[string]map[string]string{
		"/new":          {admission: "goto"},
		"/dynamic/new":  {admission: "goto"},
		"/dynamic/{id}": {kindred: "goto"},
		"/{*splat}":     {kindred: "goto"},
		"/settings":     {},
	} {
		s := byTemplate[template]
		if s == nil {
			t.Fatalf("no screen %s in %+v", template, byTemplate)
		}
		got := specFiles(s)
		if len(got) != len(want) {
			t.Fatalf("%s: specs %+v, want %+v", template, s.Specs, want)
		}
		for file, basis := range want {
			if got[file] != basis {
				t.Fatalf("%s: specs %+v, want %+v", template, s.Specs, want)
			}
		}
	}
}

// AMAP-V0-600: ranking applies only when every candidate is a react-router/0 screen and one score
// is highest; a tie or a candidate from another dialect keeps ambiguous-template.
func TestAMAPV0600RankingStaysWithinReact_V13932(t *testing.T) {
	raws := []rawState{{name: "new", url: "/new"}, {name: "splat", url: "/*"}, {name: "x-b", url: "/:x/b"}, {name: "a-y", url: "/a/:y"}, {name: "ui-new", url: "/n/:id"}, {name: "react-n", url: "/n/*"}}
	screens, u := resolveScreens("test", raws)
	if len(u) > 0 {
		t.Fatal(u)
	}
	routers := []RouterFile{{Path: "App.tsx", Dialect: DialectReact}, {Path: "routes.js", Dialect: DialectUIRouter}, {Path: "More.tsx", Dialect: DialectReact}}
	markDialects("test", routers, raws, [][2]int{{0, 4}, {4, 5}, {5, 6}}, screens)
	idx := newScreenIndex("", "", screens)
	if id, reason := idx.byURL("/new"); id != "screen:test:new" || reason != "" {
		t.Fatalf("/new: %q %q", id, reason)
	}
	if id, reason := idx.byURL("/elsewhere"); id != "screen:test:splat" || reason != "" {
		t.Fatalf("/elsewhere: %q %q", id, reason)
	}
	if _, reason := idx.byURL("/a/b"); reason != "ambiguous-template" {
		t.Fatalf("tied /a/b: %q", reason)
	}
	// /n/7 matches the UI-Router state, the react /n/* splat and the root splat: not ranked.
	if _, reason := idx.byURL("/n/7"); reason != "ambiguous-template" {
		t.Fatalf("mixed /n/7: %q", reason)
	}
	// Screens without a recorded dialect (a map an earlier builder wrote) rank nothing (AMAP-V0-881).
	for i := range screens {
		screens[i].Dialect = ""
	}
	if _, reason := newScreenIndex("", "", screens).byURL("/new"); reason != "ambiguous-template" {
		t.Fatalf("unranked index: %q", reason)
	}
}
