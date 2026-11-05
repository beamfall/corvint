package appmap

import (
	"slices"
	"strings"
	"testing"
)

// AMAP-V0-191: a declared router base path is stripped before a test URL is matched, a URL
// outside it reads outside-base-path, and projected URLs carry it.
func TestAMAPV0191BasePath(t *testing.T) {
	screens := []Screen{
		{ID: "s:root", Status: StatusResolved, Template: "/", Key: "/"},
		{ID: "s:a", Status: StatusResolved, Template: "/a/{id}", Key: "/a/{}", Params: []string{"id"}},
	}
	x := newScreenIndex("", "/web", screens)
	for raw, want := range map[string]string{
		"/web":                         "s:root",
		"/web/":                        "s:root",
		"/web?x=1":                     "s:root",
		"/web/a/7":                     "s:a",
		"https://example.test/web/a/7": "s:a",
		"/web/a/7#top":                 "s:a",
	} {
		if ids, reason := x.urlMatches(raw); len(ids) != 1 || ids[0] != want {
			t.Errorf("%s: ids %v reason %q, want %s", raw, ids, reason, want)
		}
	}
	for _, raw := range []string{"/a/7", "/webx/a/7", "/", "https://example.test/api/a/7"} {
		if ids, reason := x.urlMatches(raw); len(ids) != 0 || reason != "outside-base-path" {
			t.Errorf("%s: ids %v reason %q, want outside-base-path", raw, ids, reason)
		}
	}
	if ids, reason := x.urlMatches("a/7"); len(ids) != 0 || reason != "non-literal-url" {
		t.Errorf("relative: ids %v reason %q", ids, reason)
	}
	if ids, _ := newScreenIndex("", "", screens).urlMatches("/a/7"); len(ids) != 1 {
		t.Errorf("no base path: ids %v", ids)
	}
	m := &Map{BasePath: "/web", Screens: screens}
	if got := m.url(&screens[1]); got != "/web/a/{id}" {
		t.Errorf("url = %q", got)
	}
	if id, _, _ := m.findScreen("/web/a/{id}"); id != "s:a" {
		t.Errorf("findScreen with base path = %q", id)
	}
	m.HashPrefix = "#"
	if got := m.url(&screens[0]); got != "/web/#/" {
		t.Errorf("hash url = %q", got)
	}
	manifest := `{"schema": "application-map-manifest/0", "app": "shop", "base_path": BASE,
		"routers": [{"path": "src/App.tsx", "dialect": "react-router/0"}],
		"tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}}`
	if m, err := decodeManifest([]byte(strings.Replace(manifest, "BASE", `"/web/app"`, 1))); err != nil || m.BasePath != "/web/app" {
		t.Fatalf("valid base_path: %+v %v", m.BasePath, err)
	}
	for _, bad := range []string{`"web"`, `"/web/"`, `"/"`, `"/we b"`, `"/:x"`, `"/web//a"`} {
		if _, err := decodeManifest([]byte(strings.Replace(manifest, "BASE", bad, 1))); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: err = %v, want appmap-invalid-manifest", bad, err)
		}
	}
}

const mountManifest = `{"schema": "application-map-manifest/0", "app": "shop", "base_path": "/web",
	"routers": [{"path": "src/App.tsx", "dialect": "react-router/0"},
		{"path": "src/area.tsx", "dialect": "react-router/0", "mount": {"router": "src/App.tsx", "path": "MOUNT"}}],
	"tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}}`

const mountApp = `export function App() {
  return (
    <Routes>
      <Route path="/setup" element={<Setup />} />
      <Route path="/area/*" element={<Area />} />
      <Route path="*" element={<Navigate to="/setup" replace />} />
    </Routes>
  );
}
`

const mountArea = `export function Area() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="items/:id" element={<Item />} />
      <Route path="/abs" element={<Abs />} />
    </Routes>
  );
}
`

func mountRepo(t *testing.T, manifest, app string) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "src/App.tsx", app)
	writeFile(t, root, "src/area.tsx", mountArea)
	writeFile(t, root, "e2e/specs/area.spec.ts", "test(\"item\", async ({ page }) => {\n  await page.goto(\"/web/area/items/7\");\n});\n")
	writeFile(t, root, "e2e/pages/home.page.ts", "export class Home {}\n")
	gitTest(t, root, "init", "-q")
	return root, commitAll(t, root, "fixture")
}

// AMAP-V0-192: a mounted React router's routes are placed under the declared splat route of the
// router that renders it, relative to the mount path; the host splat becomes abstract and every
// placed state is bound to the host router's anchor. A mount the host does not declare exactly
// once, or that names no other React router, refuses the manifest.
// AMAP-V0-192: a router mounted into a mounted router names the route as its host declares it
// and is placed under the host route's placed path.
func TestAMAPV0192NestedMount(t *testing.T) {
	manifest := strings.Replace(strings.Replace(mountManifest, "MOUNT", "/area/*", 1), `"path": "/area/*"}}],`,
		`"path": "/area/*"}},
		{"path": "src/deep.tsx", "dialect": "react-router/0", "mount": {"router": "src/area.tsx", "path": "/deep/*"}}],`, 1)
	root, _ := mountRepo(t, manifest, mountApp)
	writeFile(t, root, "src/area.tsx", strings.Replace(mountArea, `<Route path="/abs"`, `<Route path="/deep/*" element={<Deep />} />
      <Route path="/abs"`, 1))
	writeFile(t, root, "src/deep.tsx", "export function Deep() {\n  return (\n    <Routes>\n      <Route path=\"leaf\" element={<Leaf />} />\n    </Routes>\n  );\n}\n")
	m := build(t, root, commitAll(t, root, "nested"))
	got := map[string]Screen{}
	for _, s := range m.Screens {
		got[s.Template] = s
	}
	if s, ok := got["/area/deep/leaf"]; !ok || s.Status != StatusResolved || s.Abstract {
		t.Errorf("nested leaf: %+v (present %v) in %v", s, ok, m.Screens)
	}
	if s := got["/area/deep/{*splat}"]; !s.Abstract {
		t.Errorf("nested mount host must be abstract: %+v", s)
	}
}

func TestAMAPV0192RouterMount(t *testing.T) {
	root, rev := mountRepo(t, strings.Replace(mountManifest, "MOUNT", "/area/*", 1), mountApp)
	m := build(t, root, rev)
	got := map[string]Screen{}
	for _, s := range m.Screens {
		got[s.Template] = s
	}
	for _, want := range []string{"/setup", "/area", "/area/items/{id}", "/area/abs"} {
		s, ok := got[want]
		if !ok || s.Status != StatusResolved || s.Abstract {
			t.Errorf("%s: %+v (present %v) in %v", want, s, ok, m.Screens)
		}
	}
	if s := got["/area/{*splat}"]; !s.Abstract {
		t.Errorf("mount host must be abstract: %+v", s)
	}
	for _, s := range m.Screens {
		if strings.HasPrefix(s.Template, "/area") && s.Template != "/area/{*splat}" && !slices.ContainsFunc(s.ParentFrom, func(a Anchor) bool { return a.Path == "src/App.tsx" }) {
			t.Errorf("%s: parent_from %+v lacks the host router", s.Template, s.ParentFrom)
		}
	}
	if m.BasePath != "/web" {
		t.Errorf("base_path = %q", m.BasePath)
	}
	x := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	if ids, reason := x.urlMatches("/web/area/items/7"); len(ids) != 1 || ids[0] != got["/area/items/{id}"].ID {
		t.Errorf("mounted url: %v %q", ids, reason)
	}
	for name, c := range map[string]struct{ mount, app string }{
		"undeclared path": {"/nope/*", mountApp},
		"duplicate path":  {"/area/*", strings.Replace(mountApp, `<Route path="/setup"`, `<Route path="/area/*" element={<B />} /><Route path="/setup"`, 1)},
	} {
		root, rev := mountRepo(t, strings.Replace(mountManifest, "MOUNT", c.mount, 1), c.app)
		if _, err := Build(t.Context(), root, "appmap.json", rev); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: err = %v, want appmap-invalid-manifest", name, err)
		}
	}
	for name, manifest := range map[string]string{
		"no splat":   strings.Replace(mountManifest, "MOUNT", "/area", 1),
		"relative":   strings.Replace(mountManifest, "MOUNT", "area/*", 1),
		"self":       strings.Replace(strings.Replace(mountManifest, "MOUNT", "/area/*", 1), `"router": "src/App.tsx"`, `"router": "src/area.tsx"`, 1),
		"undeclared": strings.Replace(strings.Replace(mountManifest, "MOUNT", "/area/*", 1), `"router": "src/App.tsx"`, `"router": "src/Other.tsx"`, 1),
		"non-react":  strings.Replace(strings.Replace(mountManifest, "MOUNT", "/area/*", 1), `"src/App.tsx", "dialect": "react-router/0"`, `"src/App.tsx", "dialect": "ui-router-states/0"`, 1),
		"cycle": strings.Replace(strings.Replace(mountManifest, "MOUNT", "/area/*", 1),
			`{"path": "src/App.tsx", "dialect": "react-router/0"}`, `{"path": "src/App.tsx", "dialect": "react-router/0", "mount": {"router": "src/area.tsx", "path": "/x/*"}}`, 1),
	} {
		if _, err := decodeManifest([]byte(manifest)); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: err = %v, want appmap-invalid-manifest", name, err)
		}
	}
}

// AMAP-V0-193: a Routes tree returned after guard statements, each `if (C) return|throw ...;` or
// `if (C) { ... }` with no else and no route element, is placed and every route carries the
// render-guarded entry constraint. An else branch or a guard holding routes still abstains; a guard
// without a terminating semicolon and a switch exit stay in TestAMAPV0190ReactRoutesInHooksComponent.
func TestAMAPV0193GuardedReturn(t *testing.T) {
	for name, src := range map[string]string{
		"early return":     `function A() { if (!enabled) return null; return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
		"early block exit": `const A = () => { if (!enabled) { return null; } return (<Routes><Route path="/a" /></Routes>); }; const b = on ? 1 : 2;`,
		"throw":            `function A() { if (!enabled) throw new Error("x"); return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
		"two guards":       `function A() { if (!a) return <Gate />; if (!b) { return <Loading />; } return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
	} {
		states, unknowns := parseReactRouter(blobEntry{path: "a.tsx"}, []byte(src))
		if len(unknowns) != 0 {
			t.Errorf("%s: unknowns %+v", name, unknowns)
		}
		screens, _ := resolveScreens("test", states)
		if len(screens) != 1 || screens[0].Status != StatusResolved || screens[0].Template != "/a" {
			t.Fatalf("%s: screens %+v", name, screens)
		}
		if e := screens[0].Entry; e == nil || !slices.Equal(e.Constraints, []string{"render-guarded"}) {
			t.Errorf("%s: entry %+v, want render-guarded", name, e)
		}
	}
	for name, src := range map[string]string{
		"else":            `function A() { if (!enabled) return null; else go(); return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
		"routes in guard": `function A() { if (x) { return (<Routes><Route path="/b" /></Routes>); } return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
	} {
		states, unknowns := parseReactRouter(blobEntry{path: "a.tsx"}, []byte(src))
		if !slices.ContainsFunc(unknowns, func(u Unknown) bool { return u.Reason == "dynamic-route-context" }) {
			t.Errorf("%s: unknowns %+v must keep dynamic-route-context", name, unknowns)
		}
		screens, _ := resolveScreens("test", states)
		for _, s := range screens {
			if s.Status == StatusResolved {
				t.Errorf("%s: resolved %+v", name, s)
			}
		}
	}
	states, _ := parseReactRouter(blobEntry{path: "a.tsx"}, []byte(`function A() { return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`))
	if screens, _ := resolveScreens("test", states); len(screens) != 1 || screens[0].Entry != nil {
		t.Errorf("unguarded tree must carry no entry constraint: %+v", screens)
	}
}

// AMAP-V0-194: a childless route whose element is exactly one `<Navigate ... />` is a redirect and
// declares no screen; any other element keeps the route a screen.
func TestAMAPV0194NavigateRedirect(t *testing.T) {
	src := `function A() { return (<Routes>
  <Route path="/a" element={<Page />} />
  <Route path="/old" element={<Navigate to="/a" replace />} />
  <Route path="*" element={<Navigate to="/a" />} />
  <Route path="/wrapped" element={<Shell><Navigate to="/a" /></Shell>} />
  <Route path="/panel" element={<Navigate.Panel />} />
</Routes>); }`
	states, unknowns := parseReactRouter(blobEntry{path: "a.tsx"}, []byte(src))
	if len(unknowns) != 0 {
		t.Fatalf("unknowns %+v", unknowns)
	}
	screens, _ := resolveScreens("test", states)
	abstract := map[string]bool{}
	for _, s := range screens {
		if s.Status != StatusResolved {
			t.Fatalf("not resolved: %+v", s)
		}
		abstract[s.Template] = s.Abstract
	}
	want := map[string]bool{"/a": false, "/old": true, "/{*splat}": true, "/wrapped": false, "/panel": false}
	if len(abstract) != len(want) {
		t.Fatalf("screens %v", abstract)
	}
	for k, v := range want {
		if a, ok := abstract[k]; !ok || a != v {
			t.Errorf("%s: abstract %v (present %v), want %v", k, a, ok, v)
		}
	}
	if ids, _ := newScreenIndex("", "", screens).urlMatches("/old"); len(ids) != 0 {
		t.Errorf("a redirect must not match a URL: %v", ids)
	}
}
