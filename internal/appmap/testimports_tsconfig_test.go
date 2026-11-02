package appmap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AMAP-V0-195: the test import index applies the tests root's tsconfig `baseUrl` and `paths`
// through the index's web import resolver, so `lib/*` aliases and baseUrl-relative bare
// specifiers resolve and join; a bare specifier nothing resolves stays `unresolved-import` and
// the unknown names it (V1-2700, issue #1806).
func TestAMAPV0195TsconfigPathsResolveTestImports(t *testing.T) {
	root, _ := fixtureRepo(t)
	files := map[string]string{
		"e2e/tsconfig.json":            `{"compilerOptions": {"baseUrl": ".", "paths": {"lib/*": ["./lib/*"]}}}` + "\n",
		"e2e/lib/configs.ts":           "export default { base: '/' };\n",
		"e2e/lib/pages/sheet.ts":       "export { TeeSheetPage } from '../../pages/teesheet.page';\n",
		"e2e/specs/configured.spec.ts": "import { test } from '@playwright/test';\nimport envConfig from 'lib/configs';\nimport { TeeSheetPage } from 'lib/pages/sheet';\nimport { HomePage } from 'pages/home.page';\ntest('uses aliases', async ({ page }) => {\n  await new HomePage(page).goToTeeSheets();\n  await new TeeSheetPage(page).book();\n  void envConfig;\n});\n",
		"e2e/specs/missing.spec.ts":    "import { test } from '@playwright/test';\nimport { Gone } from 'lib/gone';\ntest('misses', async () => { void Gone; });\n",
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := build(t, root, commitAll(t, root, "tsconfig paths"))
	f := m.file("e2e/specs/configured.spec.ts")
	if f == nil || f.Join != StatusResolved {
		t.Fatalf("aliased spec join: %+v", f)
	}
	want := map[string]string{"lib/configs": "e2e/lib/configs.ts", "lib/pages/sheet": "e2e/lib/pages/sheet.ts", "pages/home.page": "e2e/pages/home.page.ts"}
	for _, imp := range f.Imports {
		if w, ok := want[imp.Specifier]; ok {
			if imp.Status != importResolved || imp.Resolved != w {
				t.Errorf("%s: %+v", imp.Specifier, imp)
			}
			delete(want, imp.Specifier)
		}
	}
	if len(want) != 0 {
		t.Fatalf("imports missing from the index: %+v", want)
	}
	if hasUnknown(m, "import", f.ID, "unresolved-import") {
		t.Fatal("aliased spec still reads unresolved-import")
	}
	missing := false
	for _, u := range m.Unknowns {
		if u.Kind == "import" && u.Ref == fileID("e2e/specs/missing.spec.ts") && u.Reason == "unresolved-import" {
			missing = u.Detail == "specifier lib/gone"
		}
	}
	if !missing {
		t.Fatalf("unresolved alias does not name its specifier: %+v", m.Unknowns)
	}
	if !strings.HasPrefix(m.file("e2e/specs/missing.spec.ts").Join, StatusUnknown) {
		t.Fatal("unresolved alias join is not UNKNOWN")
	}
}

// AMAP-V0-195 AMAP-V0-075: tests.root "." in an aliased repository uses that repository's
// root tsconfig, even when the application has conflicting paths for the same specifier.
func TestAMAPV0195AliasedWholeRootTsconfig(t *testing.T) {
	app, _, e2e, _ := twoRoots(t, false)
	manifest := readText(t, app, "appmap.json")
	writeFile(t, app, "appmap.json", strings.Replace(manifest, `"root": "e2e"`, `"root": "."`, 1))
	writeFile(t, app, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["wrong/*"]}}}`)
	writeFile(t, app, "wrong/configs.ts", "export default {};\n")
	appRev := commitAll(t, app, "conflicting application tsconfig")
	writeFile(t, e2e, "tsconfig.json", `{"compilerOptions":{"baseUrl":".","paths":{"lib/*":["./lib/*"]}}}`)
	writeFile(t, e2e, "lib/configs.ts", "export default {};\n")
	writeFile(t, e2e, "lib/sheet.ts", "export { TeeSheetPage } from '../e2e/pages/teesheet.page';\n")
	writeFile(t, e2e, "e2e/specs/root-alias.spec.ts", "import envConfig from 'lib/configs';\nimport { TeeSheetPage } from 'lib/sheet';\nimport { HomePage } from 'e2e/pages/home.page';\nvoid envConfig;\nvoid TeeSheetPage;\nvoid HomePage;\n")
	e2eRev := commitAll(t, e2e, "root tsconfig aliases")
	m, err := BuildRoots(context.Background(), app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Roots) != 1 || m.Roots[0].Repo != "e2e" || m.Roots[0].Revision != e2eRev {
		t.Fatalf("wrong test repository pin: %+v", m.Roots)
	}
	f := m.file("e2e/specs/root-alias.spec.ts")
	if f == nil || f.Join != StatusResolved || f.Anchor.Repo != "e2e" {
		t.Fatalf("aliased root join: %+v", f)
	}
	want := map[string]string{"lib/configs": "lib/configs.ts", "lib/sheet": "lib/sheet.ts", "e2e/pages/home.page": "e2e/pages/home.page.ts"}
	for _, imp := range f.Imports {
		if w, ok := want[imp.Specifier]; ok {
			if imp.Status != importResolved || imp.Resolved != w {
				t.Errorf("%s: %+v", imp.Specifier, imp)
			}
			delete(want, imp.Specifier)
		}
	}
	if len(want) != 0 || hasUnknown(m, "import", f.ID, "unresolved-import") {
		t.Fatalf("missing imports %v; unknowns %+v", want, m.Unknowns)
	}
	for _, a := range screenByID(t, m, "app.clubs.teesheets").Specs {
		if a.File == f.Path && a.Basis == "import" {
			return
		}
	}
	t.Fatal("aliased import chain did not attribute the spec to its screen")
}

// AMAP-V0-195: a bare specifier the resolver cannot read (a package-named `extends` the index
// does not hold) stays unresolved even though package.json declares its package, and the unknown
// says so; a computed `import(name)` is an unresolved-import naming no specifier.
func TestAMAPV0195ResolverRefusalAndComputedLoadStayUnknown(t *testing.T) {
	root, _ := fixtureRepo(t)
	files := map[string]string{
		"e2e/sub/tsconfig.json":      `{"extends": "@tsconfig/node20/tsconfig.json"}` + "\n",
		"e2e/sub/specs/ext.spec.ts":  "import { test } from '@playwright/test';\ntest('extends', async () => {});\n",
		"e2e/specs/computed.spec.ts": "import { test } from '@playwright/test';\nconst name = './dyn';\ntest('loads', async () => {\n  await import(name);\n});\n",
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := build(t, root, commitAll(t, root, "refusal and computed load"))
	details := map[string]string{}
	for _, u := range m.Unknowns {
		if u.Kind == "import" && u.Reason == "unresolved-import" {
			details[u.Ref] = u.Detail
		}
	}
	if got := details[fileID("e2e/sub/specs/ext.spec.ts")]; got != "specifier @playwright/test (config resolution unknown)" {
		t.Fatalf("resolver refusal detail %q: %+v", got, m.Unknowns)
	}
	if got := details[fileID("e2e/specs/computed.spec.ts")]; got != "computed specifier" {
		t.Fatalf("computed load detail %q: %+v", got, m.Unknowns)
	}
	for _, p := range []string{"e2e/sub/specs/ext.spec.ts", "e2e/specs/computed.spec.ts"} {
		if !strings.HasPrefix(m.file(p).Join, StatusUnknown) {
			t.Fatalf("%s join is not UNKNOWN: %+v", p, m.file(p))
		}
	}
	if login := m.file("e2e/specs/login.spec.ts"); login == nil || hasUnknown(m, "import", login.ID, "unresolved-import") {
		t.Fatalf("a spec outside the unreadable config lost its package import: %+v", login)
	}
}

// AMAP-V0-195, V1-2714 decision: a package-named `extends` leaves only the fields no config before
// it declares unknown, so a leaf tsconfig that itself declares every resolution field (baseUrl,
// paths, moduleSuffixes, typeRoots, moduleResolution, module, target) reads @playwright/test as
// external while an alias its paths do not declare stays unresolved.
func TestAMAPV0195LeafDeclaredFieldsOutrankPackageExtends(t *testing.T) {
	root, _ := fixtureRepo(t)
	files := map[string]string{
		"e2e/sub/tsconfig.json": `{"extends": "@tsconfig/node20/tsconfig.json", "compilerOptions": {"baseUrl": ".", "paths": {"@pages/*": ["pages/*"]},` +
			` "moduleSuffixes": [""], "typeRoots": [], "moduleResolution": "bundler", "module": "esnext", "target": "es2022"}}` + "\n",
		"e2e/sub/specs/ext.spec.ts":   "import { test } from '@playwright/test';\ntest('extends', async () => {});\n",
		"e2e/sub/specs/alias.spec.ts": "import { test } from '@playwright/test';\nimport { x } from '@undeclared/x';\ntest('alias', async () => {});\n",
	}
	for p, body := range files {
		dst := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := build(t, root, commitAll(t, root, "leaf-declared resolution fields"))
	ext := m.file("e2e/sub/specs/ext.spec.ts")
	if ext == nil || hasUnknown(m, "import", ext.ID, "unresolved-import") {
		t.Fatalf("a fully declared leaf config left @playwright/test unresolved: %+v %+v", ext, m.Unknowns)
	}
	for _, imp := range ext.Imports {
		if imp.Specifier == "@playwright/test" && imp.Status != importExternal {
			t.Fatalf("@playwright/test = %+v, want external", imp)
		}
	}
	alias := m.file("e2e/sub/specs/alias.spec.ts")
	if alias == nil || !hasUnknown(m, "import", alias.ID, "unresolved-import") || !strings.HasPrefix(alias.Join, StatusUnknown) {
		t.Fatalf("an undeclared alias did not stay unresolved: %+v %+v", alias, m.Unknowns)
	}
}
