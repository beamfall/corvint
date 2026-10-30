package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func dialectData(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "dialects", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func dialectManifest(t *testing.T, root string, routers []RouterFile, globs []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err = json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m.Routers = routers
	m.RouterGlobs = globs
	m.HashPrefix = ""
	m.Flows = ""
	m.PageObjectScreens = nil
	raw, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "appmap.json", string(raw))
}

func dialectScreens(t *testing.T, states []rawState, unknowns []Unknown) map[string]Screen {
	t.Helper()
	screens, more := resolveScreens("test", states)
	if len(unknowns)+len(more) > 0 {
		t.Fatalf("unexpected unknowns: %+v %+v", unknowns, more)
	}
	out := map[string]Screen{}
	for _, s := range screens {
		out[s.State] = s
	}
	return out
}

// AMAP-V0-041: unique helper/path pairs for nested, scoped and shallow resources.
func TestAMAPV0041RailsRoutes(t *testing.T) {
	p := &railsParser{draw: func(string, railsScope, int) {}}
	p.parse(blobEntry{path: "config/routes.rb", oid: "rails"}, dialectData(t, "routes.rb"), railsScope{}, 0)
	screens := dialectScreens(t, p.states, p.unknowns)
	for name, want := range map[string]string{
		"root": "/", "welcome": "/welcome", "submit": "/submit",
		"admin_v2_posts": "/admin/v2/posts", "admin_v2_post": "/admin/v2/posts/{id}",
		"admin_v2_post_comments":    "/admin/v2/posts/{post_id}/comments",
		"admin_v2_post_comment":     "/admin/v2/posts/{post_id}/comments/{id}",
		"new_admin_v2_post_comment": "/admin/v2/posts/{post_id}/comments/new",
		"preview_admin_v2_post":     "/admin/v2/posts/{id}/preview",
		"search_admin_v2_posts":     "/admin/v2/posts/search",
		"article_notes":             "/articles/{article_id}/notes", "note": "/notes/{id}",
		"new_article_note": "/articles/{article_id}/notes/new", "edit_note": "/notes/{id}/edit",
		"profile": "/profile", "new_profile": "/profile/new", "files": "/files/{*path}", "mounted": "/mounted",
	} {
		if s, ok := screens[name]; !ok || s.Template != want {
			t.Errorf("%s: %+v, want %s", name, s, want)
		}
	}
}

// AMAP-V0-042: dynamic Ruby and deeper draws never supply guessed screens.
func TestAMAPV0042RailsUnknownAndDraw(t *testing.T) {
	for _, src := range []string{
		"get `whoami`, as: :bad", `get "*", as: :bad`, `get route_path, as: :bad`, `get "#{prefix}/x", as: :bad`,
		"constraints host: 'x' do\nget 'secret', as: :bad\nend",
		"defaults format: 'json' do\nget 'secret', as: :bad\nend",
		"constraints ->(req) do\nget 'secret', as: :bad\nend",
		"constraints ->(req) { req.valid? } do\nget 'secret', as: :bad\nend",
		"if enabled\nget 'secret', as: :bad\nend",
		"scope dynamic do\nget 'secret', as: :bad\nend",
		`resources :people`, `get 'x', constraints: pattern, as: :bad`,
		"custom do |router|\nget 'secret', as: :bad\nend",
		"content = <<~TEXT\nget 'secret', as: :bad\nTEXT",
	} {
		t.Run(src, func(t *testing.T) {
			p := &railsParser{}
			p.parse(blobEntry{path: "routes.rb"}, []byte(src), railsScope{}, 0)
			if len(p.states) > 0 || len(p.unknowns) == 0 {
				t.Fatalf("states=%+v unknowns=%+v", p.states, p.unknowns)
			}
		})
	}
	root, _ := fixtureRepo(t)
	writeFile(t, root, "config/routes.rb", "Rails.application.routes.draw do\nnamespace :admin do\ndraw :extra\nend\nend\n")
	writeFile(t, root, "config/routes/extra.rb", "get 'extra', as: :extra\ndraw :deeper\n")
	dialectManifest(t, root, []RouterFile{{Path: "config/routes.rb", Dialect: DialectRails}}, nil)
	rev := commitAll(t, root, "draw")
	m := build(t, root, rev)
	s := screenByID(t, m, "admin_extra")
	if s.Template != "/admin/extra" || s.Anchor.Path != "config/routes/extra.rb" || len(s.NameFrom) == 0 {
		t.Fatalf("draw: %+v", s)
	}
	if !hasUnknown(m, "router", "config/routes/extra.rb:2", "draw-depth-exceeded") {
		t.Fatal(m.Unknowns)
	}
	writeFile(t, root, "config/routes.rb", "Rails.application.routes.draw do\nnamespace :staff do\ndraw :extra\nend\nend\n")
	changed := commitAll(t, root, "scope changed")
	if got := LineageFreshness(context.Background(), root, changed, m, []string{s.ID})[s.ID]; got != Stale {
		t.Fatalf("draw scope freshness: %s", got)
	}
	writeFile(t, root, "config/routes.rb", "draw :missing\ndraw '../escape'\n")
	rev = commitAll(t, root, "missing")
	m = build(t, root, rev)
	if !hasUnknown(m, "router", "config/routes.rb:1", "draw-unavailable") || !hasUnknown(m, "router", "config/routes.rb:2", "non-literal-draw") {
		t.Fatal(m.Unknowns)
	}
}

// AMAP-V0-043: object and JSX trees agree on hierarchy, index and splat semantics.
func TestAMAPV0043ReactRoutes(t *testing.T) {
	for _, file := range []string{"objects.tsx", "routes.jsx"} {
		states, unknowns := parseReactRouter(blobEntry{path: file}, dialectData(t, file))
		screens := dialectScreens(t, states, unknowns)
		wants := map[string]string{"dashboard.index": "/dashboard", "dashboard.project": "/dashboard/projects/{projectId}", "dashboard.files": "/dashboard/files/{*splat}", "dashboard.absolute": "/dashboard/absolute", "dashboard.settings": "/dashboard/settings"}
		if file == "routes.jsx" {
			wants = map[string]string{"shop.index": "/shop", "shop.product": "/shop/products/{id}", "shop.files": "/shop/files/{*splat}"}
		}
		for name, want := range wants {
			if s := screens[name]; s.Template != want {
				t.Errorf("%s: %+v want %s", name, s, want)
			}
		}
	}
	states, u := parseReactRouter(blobEntry{path: "ui.jsx"}, []byte(`<Routes><Route path="/a" element={<A />} /><Route path={'/b'} /></Routes>`))
	if len(dialectScreens(t, states, u)) != 2 {
		t.Fatal(states)
	}
}

// AMAP-V0-044: dynamic route data and JSX expressions cannot produce resolved screens.
func TestAMAPV0044ReactUnknown(t *testing.T) {
	for _, src := range []string{
		`createBrowserRouter([{path: routePath}])`,
		`createBrowserRouter([{path:'/x',lazy:()=>import('./x')}])`,
		`createBrowserRouter([{path:'/x',...rest}])`,
		`createBrowserRouter([{[key]:'/x'}])`,
		`createBrowserRouter([{path:'/x',path:'/y'}])`,
		`createBrowserRouter([{path:'/x',children:children}])`,
		`createBrowserRouter([{path:'/x',index:enabled}])`,
		`createBrowserRouter(routes)`,
		`enabled && <Routes><Route id="secret" path="/secret" /></Routes>; <Routes>{children}</Routes>`,
		`<Routes><Route id="amp" path="/a&amp;b" /></Routes>`,
		`<Routes><Route id="escape" path="/a\u0062" /></Routes>`,
		`enabled && createBrowserRouter([{id:'secret',path:'/secret'}])`,
		`const ui = enabled && <Routes><Route id="secret" path="/secret" /></Routes>`,
		`createBrowserRouter([{path:'/files/*rest'}])`,
		`createBrowserRouter([{path:'/:user-id'}])`,
		`createRoutesFromElements(<Route {...props} path="/x" />)`,
		`createRoutesFromElements(<Route path={routePath} />)`,
		`createRoutesFromElements(<Route path="/x">{enabled && <Route path="secret" />}</Route>)`,
		`createBrowserRouter([{path:'/x/:id?'}])`,
		`createBrowserRouter([{id:'bad',path:prefix,children:[{id:'child',path:'child'}]}])`,
	} {
		t.Run(src, func(t *testing.T) {
			states, u := parseReactRouter(blobEntry{path: "routes.tsx"}, []byte(src))
			screens, more := resolveScreens("test", states)
			if len(u)+len(more) == 0 {
				t.Fatal("no unknown")
			}
			for _, s := range screens {
				if s.Status == StatusResolved {
					t.Fatalf("guessed %+v", s)
				}
			}
		})
	}
	// A regular expression's contents are not router calls.
	states, u := parseReactRouter(blobEntry{path: "routes.ts"}, []byte(`const pattern = /createBrowserRouter([{path:'fake'}])/;`))
	if len(states) != 0 || len(u) == 0 {
		t.Fatalf("regex mined: %+v %+v", states, u)
	}
}

// AMAP-V0-040 AMAP-V0-048: all dialects feed the existing test join and stable artifact encoding.
func TestAMAPV0040MixedDialects(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "app/rails.rb", "get '/legacy/:id', to: 'legacy#show', as: :legacy\n")
	writeFile(t, root, "app/react.tsx", `createBrowserRouter([{id:'island',path:'/island/:slug',Component:Island}])`)
	writeFile(t, root, "e2e/specs/mixed.spec.ts", `test('mixed',async({page})=>{await page.goto('/legacy/7');await page.goto('/island/one');});`)
	dialectManifest(t, root, []RouterFile{{Path: "app/routes.js", Dialect: DialectUIRouter}, {Path: "app/rails.rb", Dialect: DialectRails}, {Path: "app/react.tsx", Dialect: DialectReact}}, nil)
	rev := commitAll(t, root, "mixed")
	m := build(t, root, rev)
	for _, name := range []string{"legacy", "island"} {
		if s := screenByID(t, m, name); s.Status != StatusResolved || len(s.Specs) != 1 {
			t.Fatalf("join: %+v", s)
		}
	}
	a, err := Encode(m)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Encode(build(t, root, rev))
	if !bytes.Equal(a, b) {
		t.Fatal("non deterministic")
	}
	file := filepath.Join(t.TempDir(), "map.json")
	if err = os.WriteFile(file, a, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadMap(file)
	if err != nil || loaded.Digest != m.Digest {
		t.Fatalf("round trip: %v", err)
	}
}

// AMAP-V0-045: discovery uses committed paths, de-duplicates globs and reports undeclared routers.
func TestAMAPV0045RouterCoverage(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "app/other/routes.rb", "get 'other'\n")
	dialectManifest(t, root, []RouterFile{{Path: "app/routes.js", Dialect: DialectUIRouter}}, []string{"app/**/routes.*", "app/*.js"})
	rev := commitAll(t, root, "coverage")
	writeFile(t, root, "app/untracked/routes.rb", "get 'untracked'\n")
	m := build(t, root, rev)
	c := m.AppCoverage
	if c == nil || c.Status != StatusUnknown || len(c.Matched) != 2 || len(c.Undeclared) != 1 || c.Undeclared[0] != "app/other/routes.rb" {
		t.Fatalf("coverage: %+v", c)
	}
	if !hasUnknown(m, "router", "app/other/routes.rb", "router-not-declared") {
		t.Fatal(m.Unknowns)
	}
	a, _ := Encode(m)
	file := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(file, a, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadMap(file); err != nil {
		t.Fatal(err)
	}
}

// AMAP-V0-046: common keys and ambiguity work across dialects; splats match several segments.
func TestAMAPV0046SharedTemplates(t *testing.T) {
	raw := []rawState{{name: "ui", url: "/mixed/:id"}, {name: "rails", url: "/mixed/:slug"}, {name: "react", url: "/mixed/{key}"}, {name: "files", url: "/files/*path"}}
	screens, u := resolveScreens("test", raw)
	if len(u) > 0 {
		t.Fatal(u)
	}
	idx := newScreenIndex("", "", screens)
	if _, reason := idx.byURL("/mixed/7"); reason != "ambiguous-template" {
		t.Fatal(reason)
	}
	if id, reason := idx.byURL("/files/a/b"); id != "screen:test:files" || reason != "" {
		t.Fatal(id, reason)
	}
	if a, b := templateKey("/files/{*path}"), templateKey("/files/{*splat}"); a != b {
		t.Fatal(a, b)
	}
}

// AMAP-V0-047: explicit bounds and malformed inputs refuse without evaluating source.
func TestAMAPV0047Bounds(t *testing.T) {
	for _, globs := range [][]string{{}, {"../routes.*"}, {"**x/routes.js"}, {"["}, {"app/*", "app/*"}} {
		if validateRouterGlobs(globs) == nil {
			t.Fatalf("accepted %v", globs)
		}
	}
	states, u := parseReactRouter(blobEntry{path: "routes.ts"}, []byte(strings.Repeat("[", maxRouteDepth+1)))
	if len(states) != 0 || len(u) == 0 {
		t.Fatal("deep route not unknown")
	}
	root, _ := fixtureRepo(t)
	routers := make([]RouterFile, maxRouterFiles+1)
	for i := range routers {
		routers[i] = RouterFile{Path: "app/routes.js", Dialect: DialectUIRouter}
	}
	dialectManifest(t, root, routers, nil)
	rev := commitAll(t, root, "too many routers")
	if _, err := Build(context.Background(), root, "appmap.json", rev); codeOf(err) != "appmap-invalid-manifest" {
		t.Fatal(err)
	}
}

// AMAP-V0-041: shallow nesting resets collection ancestry at each shallow parent.
func TestAMAPV0041DeepShallow(t *testing.T) {
	p := &railsParser{}
	p.parse(blobEntry{path: "routes.rb"}, []byte("resources :posts, shallow: true do\nresources :comments do\nresources :notes\nend\nend"), railsScope{}, 0)
	screens := dialectScreens(t, p.states, p.unknowns)
	if s := screens["comment_notes"]; s.Template != "/comments/{comment_id}/notes" {
		t.Fatalf("deep shallow: %+v", s)
	}
}

// AMAP-V0-046: actual parsers, rather than fabricated states, share one collision index.
func TestAMAPV0046MixedDialectCollision(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "app/routes.js", `app.state('ui', {url:'/shared/:id'});`)
	writeFile(t, root, "app/routes.rb", "get 'shared/:other', to: 'shared#show', as: :rails\n")
	writeFile(t, root, "app/routes.tsx", `createBrowserRouter([{id:'react',path:'/shared/:slug'}])`)
	dialectManifest(t, root, []RouterFile{{Path: "app/routes.js", Dialect: DialectUIRouter}, {Path: "app/routes.rb", Dialect: DialectRails}, {Path: "app/routes.tsx", Dialect: DialectReact}}, nil)
	m := build(t, root, commitAll(t, root, "collisions"))
	if !hasUnknown(m, "template", "/shared/{}", "ambiguous-template") {
		t.Fatal(m.Unknowns)
	}
	ids, reason := newScreenIndex("", "", m.Screens).urlMatches("/shared/42")
	if len(ids) != 3 || reason != "" {
		t.Fatalf("matches: %v %s", ids, reason)
	}
}

// AMAP-V0-047: draws consume the existing file/byte bounds; no silent cap or bound swallowing.
func TestAMAPV0047DrawBounds(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "config/routes.rb", "draw :huge\n")
	writeFile(t, root, "config/routes/huge.rb", strings.Repeat("#", maxRouterBytes+1))
	dialectManifest(t, root, []RouterFile{{Path: "config/routes.rb", Dialect: DialectRails}}, nil)
	rev := commitAll(t, root, "large draw")
	if _, err := Build(context.Background(), root, "appmap.json", rev); codeOf(err) != "appmap-bound-exceeded" {
		t.Fatalf("large draw: %v", err)
	}
	var routes strings.Builder
	for i := 0; i < maxRouterFiles; i++ {
		fmt.Fprintf(&routes, "draw :part%d\n", i)
		writeFile(t, root, fmt.Sprintf("config/routes/part%d.rb", i), "# empty\n")
	}
	writeFile(t, root, "config/routes.rb", routes.String())
	rev = commitAll(t, root, "many draws")
	if _, err := Build(context.Background(), root, "appmap.json", rev); codeOf(err) != "appmap-bound-exceeded" {
		t.Fatalf("many draws: %v", err)
	}
}

// AMAP-V0-046: Rails required globs and React empty splats share a key, not empty-path semantics.
func TestAMAPV0046SplatEmptySemantics(t *testing.T) {
	rails := &railsParser{}
	rails.parse(blobEntry{path: "routes.rb"}, []byte("get 'files/*path', as: :rails\n"), railsScope{}, 0)
	react, u := parseReactRouter(blobEntry{path: "routes.ts"}, []byte(`createBrowserRouter([{id:'react',path:'/files/*'}])`))
	if len(u) > 0 || len(rails.unknowns) > 0 {
		t.Fatal(u, rails.unknowns)
	}
	screens, u := resolveScreens("test", append(rails.states, react...))
	if len(u) > 0 {
		t.Fatal(u)
	}
	idx := newScreenIndex("", "", screens)
	if id, why := idx.byURL("/files"); id != "screen:test:react" || why != "" {
		t.Fatal(id, why)
	}
	if _, why := idx.byURL("/files/a/b"); why != "ambiguous-template" {
		t.Fatal(why)
	}
}

// AMAP-V0-078: a route array bound once by a top-level const in the router file compiles like an
// inline one; an imported, reassigned, mutated or otherwise used binding stays UNKNOWN by name.
func TestAMAPV0078ReactSameFileRouteArray(t *testing.T) {
	for _, src := range []string{
		"const routes = [{id: 'home', path: '/home'}, {id: 'docs', path: '/docs', children: [{id: 'page', path: ':slug'}]}];\nexport const router = createBrowserRouter(routes);",
		"import { createBrowserRouter } from 'react-router-dom'\nexport const routes: RouteObject[] = [{id: 'home', path: '/home'}, {id: 'docs', path: '/docs', children: [{id: 'page', path: ':slug'}]}]\nconst router = createBrowserRouter(routes)",
	} {
		states, u := parseReactRouter(blobEntry{path: "routes.tsx"}, []byte(src))
		screens := dialectScreens(t, states, u)
		if screens["home"].Template != "/home" || screens["page"].Template != "/docs/{slug}" {
			t.Fatalf("%q: %+v", src, screens)
		}
	}
	for src, want := range map[string]string{
		"import { routes } from './routes';\ncreateBrowserRouter(routes);":                                                         "imported-react-routes",
		"import routes from './routes';\ncreateBrowserRouter(routes);":                                                             "imported-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nroutes.push({id: 'b', path: '/b'});\ncreateBrowserRouter(routes);":               "computed-react-routes",
		"let routes = [{id: 'a', path: '/a'}];\ncreateBrowserRouter(routes);":                                                      "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}].concat(more);\ncreateBrowserRouter(routes);":                                       "computed-react-routes",
		"function f() { const routes = [{id: 'a', path: '/a'}]; return createBrowserRouter(routes) }":                              "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nconst routes = [];\ncreateBrowserRouter(routes);":                                "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nextend(routes);\ncreateBrowserRouter(routes);":                                   "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}]\n  .concat(more)\ncreateBrowserRouter(routes)":                                     "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nconst x = `${routes.pop()}`;\ncreateBrowserRouter(routes);":                      "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nrout\\u0065s.length = 0;\ncreateBrowserRouter(routes);":                          "computed-react-routes",
		"const routes = [{id: 'a', path: '/a'}];\nconst alias = [...routes];\nalias[0].path = '/b';\ncreateBrowserRouter(routes);": "computed-react-routes",
		"createBrowserRouter(routes);": "non-literal-react-routes",
	} {
		states, u := parseReactRouter(blobEntry{path: "routes.tsx"}, []byte(src))
		screens, more := resolveScreens("test", states)
		for _, s := range screens {
			if s.Status == StatusResolved {
				t.Fatalf("%q guessed %+v", src, s)
			}
		}
		got := ""
		for _, x := range append(u, more...) {
			got += x.Reason + " "
		}
		if !strings.Contains(got, want) {
			t.Fatalf("%q: reasons %q want %s", src, got, want)
		}
	}
}

// AMAP-V0-079: an imported createBrowserRouter array is read through one literal named-import hop
// at the same revision; every other import form, a computed module array and a second hop stay
// UNKNOWN, and a change to the router or the module leaves derived screens not FRESH.
func TestAMAPV0079ReactImportedRouteArray(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "src/router.tsx", "import { createBrowserRouter } from 'react-router-dom';\nimport { appRoutes as routes } from './routes';\nexport const router = createBrowserRouter(routes);\n")
	writeFile(t, root, "src/routes.ts", "export const appRoutes: RouteObject[] = [{id: 'home', path: '/home'}, {id: 'docs', path: '/docs', children: [{id: 'page', path: ':slug'}]}];\n")
	dialectManifest(t, root, []RouterFile{{Path: "src/router.tsx", Dialect: DialectReact}}, nil)
	rev := commitAll(t, root, "imported routes")
	m := build(t, root, rev)
	page := screenByID(t, m, "page")
	if page.Status != StatusResolved || page.Template != "/docs/{slug}" || page.Anchor.Path != "src/routes.ts" || len(page.NameFrom) != 1 || page.NameFrom[0].Path != "src/router.tsx" {
		t.Fatalf("imported: %+v", page)
	}
	writeFile(t, root, "src/router.tsx", "import { createBrowserRouter } from 'react-router-dom';\nimport { appRoutes as routes } from './routes';\n// moved\nexport const router = createBrowserRouter(routes);\n")
	changed := commitAll(t, root, "router changed")
	if got := LineageFreshness(context.Background(), root, changed, m, []string{page.ID})[page.ID]; got == Fresh {
		t.Fatalf("router change left the imported screen %s", got)
	}
	writeFile(t, root, "src/routes.tsx", "export const appRoutes = [];\n")
	rev = commitAll(t, root, "ambiguous module")
	if m = build(t, root, rev); !hasUnknown(m, "router", "src/router.tsx:4", "ambiguous-react-routes-import") {
		t.Fatal(m.Unknowns)
	}
	writeFile(t, root, "src/router.tsx", "import { appRoutes } from './missing';\ncreateBrowserRouter(appRoutes);\n")
	rev = commitAll(t, root, "missing module")
	if m = build(t, root, rev); !hasUnknown(m, "router", "src/router.tsx:2", "unresolved-react-routes-import") {
		t.Fatal(m.Unknowns)
	}

	module := func(src string) reactImporter {
		return func(spec string) (blobEntry, []byte, string) {
			return blobEntry{path: "routes.ts"}, []byte(src), ""
		}
	}
	router := "import { appRoutes } from './routes';\ncreateBrowserRouter(appRoutes);"
	states, u := parseReactRouterWith(blobEntry{path: "router.tsx"}, []byte(router), module("export const appRoutes = [{id: 'home', path: '/home'}];"))
	if screens := dialectScreens(t, states, u); screens["home"].Template != "/home" {
		t.Fatalf("%+v", screens)
	}
	for name, c := range map[string]struct{ router, module, want string }{
		"default import":     {"import appRoutes from './routes';\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "imported-react-routes"},
		"namespace import":   {"import * as appRoutes from './routes';\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "imported-react-routes"},
		"type import":        {"import { type appRoutes } from './routes';\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "imported-react-routes"},
		"package import":     {"import { appRoutes } from 'routes-pkg';\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "imported-react-routes"},
		"import and const":   {"import { appRoutes } from './routes';\nconst appRoutes = [];\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "computed-react-routes"},
		"mutated in router":  {"import { appRoutes } from './routes';\nappRoutes.push({id: 'b', path: '/b'});\ncreateBrowserRouter(appRoutes);", "export const appRoutes = [{id: 'a', path: '/a'}];", "computed-react-routes"},
		"mutated in module":  {router, "export const appRoutes = [{id: 'a', path: '/a'}];\nappRoutes.push({id: 'b', path: '/b'});", "computed-react-routes"},
		"not exported":       {router, "const appRoutes = [{id: 'a', path: '/a'}];", "computed-react-routes"},
		"second hop":         {router, "import { appRoutes } from './more';\nexport { appRoutes };", "imported-react-routes"},
		"computed module":    {router, "export const appRoutes = base.concat([{id: 'a', path: '/a'}]);", "computed-react-routes"},
		"missing in module":  {router, "export const other = [{id: 'a', path: '/a'}];", "non-literal-react-routes"},
		"dynamic module":     {router, "export const appRoutes = [{id: 'a', path: flag ? '/a' : '/b'}];", "dynamic-route-context"},
		"conditional module": {router, "export const appRoutes = [{id: 'a', path: '/a'}];\nif (flag) { console.log(1) }", "dynamic-route-context"},
	} {
		states, u := parseReactRouterWith(blobEntry{path: "router.tsx"}, []byte(c.router), module(c.module))
		screens, more := resolveScreens("test", states)
		for _, s := range screens {
			if s.Status == StatusResolved {
				t.Fatalf("%s guessed %+v", name, s)
			}
		}
		got := " "
		for _, x := range append(u, more...) {
			got += x.Reason + " "
		}
		if !strings.Contains(got, " "+c.want+" ") {
			t.Fatalf("%s: reasons %q want %s", name, got, c.want)
		}
	}
}

// AMAP-V0-190: a statically placed JSX Routes tree inside a hooks component reads every literal
// Route; a non-literal child records its own unknown and conditional placement still abstains
// (guard statements before the return are AMAP-V0-193).
func TestAMAPV0190ReactRoutesInHooksComponent(t *testing.T) {
	states, unknowns := parseReactRouter(blobEntry{path: "finances-app.tsx"}, dialectData(t, "finances-app.tsx"))
	if len(unknowns) != 1 || unknowns[0].Reason != "non-literal-react-children" || unknowns[0].Line != 35 {
		t.Fatalf("unknowns = %+v, want one non-literal-react-children at line 35", unknowns)
	}
	screens, more := resolveScreens("test", states)
	if len(more) != 0 {
		t.Fatalf("unexpected unknowns: %+v", more)
	}
	got := map[string]bool{}
	for _, s := range screens {
		if s.Status != StatusResolved {
			t.Fatalf("not resolved: %+v", s)
		}
		got[s.Template] = true
	}
	for _, want := range []string{"/", "/reports/{*splat}", "/budget", "/accounts/{id}", "/{*splat}"} {
		if !got[want] {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	if len(screens) != 5 || got["/hidden"] {
		t.Fatalf("screens = %v", got)
	}
	for name, src := range map[string]string{
		"logical child":     `function A() { return (<div>{on && (<Routes><Route path="/a" /></Routes>)}</div>); }`,
		"ternary":           `function A() { return on ? <Routes><Route path="/a" /></Routes> : null; }`,
		"guarded return":    `function A() { if (on) return (<Routes><Route path="/a" /></Routes>); return null; }`,
		"else return":       `function A() { if (on) { return null; } else return (<Routes><Route path="/a" /></Routes>); }`,
		"mapped":            `function A() { return (<div>{xs.map(x => (<Routes><Route path="/a" /></Routes>))}</div>); }`,
		"call argument":     `const A = memo(function A() { return (<Routes><Route path="/a" /></Routes>); }); const b = on ? 1 : 2;`,
		"render prop":       `function A() { return (<X render={() => <Routes><Route path="/a" /></Routes>} />); } const b = on ? 1 : 2;`,
		"object value":      `function A() { const v = {wide: <Routes><Route path="/a" /></Routes>}; return v[on ? 'wide' : 'x']; }`,
		"array element":     `function A() { return [<Routes key="a"><Route path="/a" /></Routes>][on ? 0 : 1]; }`,
		"conditional block": `function A() { if (on) { return (<Routes><Route path="/a" /></Routes>); } return null; }`,
		"trailing operator": `function A() { const r = <Routes><Route path="/a" /></Routes> || null; return r; } const b = on ? 1 : 2;`,
		"early return asi":  "function A() {\n  if (!enabled) return null\n  return (<Routes><Route path=\"/a\" /></Routes>)\n}\nconst b = on ? 1 : 2",
		"switch exit":       `function A() { switch (mode) { case 1: return null; } return (<Routes><Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
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
	for name, src := range map[string]string{
		"function":           `export function A() { const b = on ? 1 : 2; return (<div><Routes><Route path="/a" /></Routes></div>); }`,
		"arrow":              `export const A = () => { if (on) { go(); } return (<Routes><Route path="/a" /></Routes>); };`,
		"arrow expression":   `const A: React.FC = () => (<Routes><Route path="/a" /></Routes>); const b = on ? 1 : 2;`,
		"typed function":     `function A(): JSX.Element { useEffect(() => { if (on) go(); }); return <Routes><Route path="/a" /></Routes>; }`,
		"expression sibling": `function A() { return (<Routes>{extra}<Route path="/a" /></Routes>); } const b = on ? 1 : 2;`,
	} {
		states, _ := parseReactRouter(blobEntry{path: "a.tsx"}, []byte(src))
		screens, _ := resolveScreens("test", states)
		if len(screens) != 1 || screens[0].Status != StatusResolved || screens[0].Template != "/a" {
			t.Errorf("%s: screens %+v", name, screens)
		}
	}
}
