package appmap

import (
	"slices"
	"strings"
	"testing"
)

const inferManifest = `{"schema": "application-map-manifest/0", "app": "shop",
	"routers": [{"path": "src/App.tsx", "dialect": "react-router/0"},
		{"path": "src/areas/manage/index.tsx", "dialect": "react-router/0"},
		{"path": "src/areas/admin.tsx", "dialect": "react-router/0"}],
	"tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}}`

// The host renders the manage area through a React.lazy wrapper with a relative dynamic import and
// the admin area through a static import of a tsconfig paths alias.
const inferApp = `import { lazy } from "react";
import { Shell } from "./Shell";
import { Admin } from "@/areas/admin";
import type { Props } from "./types";

const ManageArea = lazy(() =>
  import("./areas/manage").then((m) => ({ default: m.ManageArea })),
);

export function App() {
  return (
    <Routes>
      <Route path="/setup" element={<Setup />} />
      <Route path="/manage/*" element={<Shell><ManageArea /></Shell>} />
      <Route path="/admin/*" element={<Admin />} />
      <Route path="*" element={<Navigate to="/setup" replace />} />
    </Routes>
  );
}
`

const inferManage = `export function ManageArea() {
  return (
    <Routes>
      <Route index element={<Overview />} />
      <Route path="libraries" element={<Libraries />} />
      <Route path="libraries/:id" element={<Library />} />
    </Routes>
  );
}
`

const inferAdmin = `export function Admin() {
  return (
    <Routes>
      <Route path="users" element={<Users />} />
    </Routes>
  );
}
`

func inferRepo(t *testing.T, manifest, app string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, "tsconfig.json", `{"compilerOptions": {"baseUrl": ".", "paths": {"@/*": ["src/*"]}}}`)
	writeFile(t, root, "src/App.tsx", app)
	writeFile(t, root, "src/Shell.tsx", "export function Shell({ children }) { return children; }\n")
	writeFile(t, root, "src/types.ts", "export type Props = {};\n")
	writeFile(t, root, "src/areas/manage/index.tsx", inferManage)
	writeFile(t, root, "src/areas/admin.tsx", inferAdmin)
	writeFile(t, root, "e2e/pages/libraries.page.ts", "export class LibrariesPage {\n  constructor(page) { this.page = page; }\n  async open() { await this.page.goto('/manage/libraries'); }\n}\n")
	writeFile(t, root, "e2e/pages/users.page.ts", "export class UsersPage {\n  constructor(page) { this.page = page; }\n  async open() { await this.page.goto('/admin/users'); }\n}\n")
	writeFile(t, root, "e2e/specs/area.spec.ts", "import { LibrariesPage } from '../pages/libraries.page';\nimport { UsersPage } from '../pages/users.page';\n\n"+
		"test('areas', async ({ page }) => {\n  const libraries = new LibrariesPage(page);\n  await libraries.open();\n  const users = new UsersPage(page);\n  await users.open();\n});\n")
	gitTest(t, root, "init", "-q")
	commitAll(t, root, "fixture")
	return root
}

func templates(m *Map) map[string]Screen {
	got := map[string]Screen{}
	for _, s := range m.Screens {
		got[s.Template] = s
	}
	return got
}

// AMAP-V0-979: an unmounted React router that a lazily loaded (dynamic import) or statically
// imported area component renders from another router's terminal splat route is mounted there
// as a declared mount would be, so its nested routes compose under the parent path, the host
// splat is abstract, and page objects navigating to composed URLs bind and yield a test-sequence
// edge. A tsconfig paths alias resolves through the index's resolver.
func TestAMAPV0979InferredAreaMount(t *testing.T) {
	root := inferRepo(t, inferManifest, inferApp)
	m := build(t, root, "HEAD")
	got := templates(m)
	for _, want := range []string{"/setup", "/manage", "/manage/libraries", "/manage/libraries/{id}", "/admin/users"} {
		if s, ok := got[want]; !ok || s.Status != StatusResolved || s.Abstract {
			t.Errorf("%s: %+v (present %v) in %v", want, s, ok, slices.Collect(mapsKeys(got)))
		}
	}
	for _, host := range []string{"/manage/{*splat}", "/admin/{*splat}"} {
		if s := got[host]; !s.Abstract {
			t.Errorf("%s must be abstract: %+v", host, s)
		}
	}
	for _, stale := range []string{"/libraries", "/users"} {
		if _, ok := got[stale]; ok {
			t.Errorf("%s stayed at the root after the inferred mount", stale)
		}
	}
	if s := got["/manage/libraries"]; !slices.ContainsFunc(s.ParentFrom, func(a Anchor) bool { return a.Path == "src/App.tsx" }) {
		t.Errorf("composed screen lacks the host anchor: %+v", s.ParentFrom)
	}
	edge := false
	for _, e := range m.Edges {
		if e.Basis == "test-sequence" && e.From == got["/manage/libraries"].ID && e.To == got["/admin/users"].ID {
			edge = true
		}
	}
	if !edge {
		t.Errorf("no test-sequence edge between composed screens: %+v", m.Edges)
	}
	for _, u := range m.Unknowns {
		if strings.Contains(u.Reason, "react-mount") {
			t.Errorf("unexpected mount unknown %+v", u)
		}
	}
}

// AMAP-V0-979: a declared mount wins over inference; an unresolved specifier on a splat route that
// renders no router reads unresolved-react-mount-import; a router two splat routes render stays
// unmounted and reads ambiguous-react-mount; an unimported or locally declared component infers
// nothing.
func TestAMAPV0979InferenceBounds(t *testing.T) {
	t.Run("declared", func(t *testing.T) {
		manifest := strings.Replace(inferManifest, `{"path": "src/areas/admin.tsx", "dialect": "react-router/0"}`,
			`{"path": "src/areas/admin.tsx", "dialect": "react-router/0", "mount": {"router": "src/App.tsx", "path": "/manage/*"}}`, 1)
		got := templates(build(t, inferRepo(t, manifest, inferApp), "HEAD"))
		if _, ok := got["/manage/users"]; !ok {
			t.Errorf("declared mount not applied: %v", slices.Collect(mapsKeys(got)))
		}
		if _, ok := got["/admin/users"]; ok {
			t.Errorf("inference overrode a declared mount")
		}
	})
	t.Run("unresolved", func(t *testing.T) {
		app := strings.Replace(inferApp, `from "@/areas/admin"`, `from "~areas/admin"`, 1)
		m := build(t, inferRepo(t, inferManifest, app), "HEAD")
		if _, ok := templates(m)["/users"]; !ok {
			t.Errorf("unresolved area should stay unmounted")
		}
		if !slices.ContainsFunc(m.Unknowns, func(u Unknown) bool {
			return u.Reason == "unresolved-react-mount-import" && u.Path == "src/App.tsx" && u.Line == 3 && strings.Contains(u.Detail, "/admin/*")
		}) {
			t.Errorf("no unresolved-react-mount-import: %+v", m.Unknowns)
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		app := strings.Replace(inferApp, `<Route path="/setup" element={<Setup />} />`, `<Route path="/staff/*" element={<Admin />} />`, 1)
		m := build(t, inferRepo(t, inferManifest, app), "HEAD")
		if _, ok := templates(m)["/users"]; !ok {
			t.Errorf("an area two routes render should stay unmounted")
		}
		if !slices.ContainsFunc(m.Unknowns, func(u Unknown) bool { return u.Reason == "ambiguous-react-mount" && u.Path == "src/areas/admin.tsx" }) {
			t.Errorf("no ambiguous-react-mount: %+v", m.Unknowns)
		}
	})
	t.Run("local", func(t *testing.T) {
		app := strings.Replace(inferApp, `import { Admin } from "@/areas/admin";`, `function Admin() { return null; }`, 1)
		got := templates(build(t, inferRepo(t, inferManifest, app), "HEAD"))
		if _, ok := got["/users"]; !ok {
			t.Errorf("a locally declared component must not infer a mount")
		}
	})
}

func mapsKeys(m map[string]Screen) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// AMAP-V0-979: the binding reader keeps static default, named and renamed imports and a const
// holding exactly one dynamic import, and drops type-only, namespace and doubly bound names.
func TestAMAPV0979ModuleBindings(t *testing.T) {
	src := `import Def, { A, B as C, type T } from "./x";
import type { U } from "./u";
import * as NS from "./ns";
const Lazy = lazy(() => import("./lazy"));
const Two = pick(import("./a"), import("./b"));
import { D } from "./d";
function D() {}
export const E = React.lazy(() => import("@/e"))
const F = 1
`
	b := topLevelModuleBindings([]byte(src))
	want := map[string]string{"Def": "./x", "A": "./x", "C": "./x", "Lazy": "./lazy", "E": "@/e"}
	for name, spec := range want {
		if bs := b[name]; len(bs) != 1 || bs[0].spec != spec {
			t.Errorf("%s: %+v, want %s", name, bs, spec)
		}
	}
	for _, name := range []string{"T", "U", "NS", "B", "Two", "D", "F"} {
		if bs := b[name]; len(bs) == 1 {
			t.Errorf("%s must not bind exactly one module: %+v", name, bs)
		}
	}
	if got := elementTags(`< Area > < ManageArea / > < / Area >`); !slices.Equal(got, []string{"Area", "ManageArea"}) {
		t.Errorf("elementTags = %v", got)
	}
}
