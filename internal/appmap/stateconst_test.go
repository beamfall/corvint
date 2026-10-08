package appmap

import (
	"strconv"
	"strings"
	"testing"
)

// constRouter is a second router file added to the fixture manifest by constRepo.
const constRouter = "app/named-routes.ts"

// constRepo commits the fixture plus a second router reading state names through constants and
// the given files, and builds the map.
func constRepo(t *testing.T, router string, files map[string]string) (string, string, *Map) {
	t.Helper()
	root, _ := fixtureRepo(t)
	manifest := readText(t, root, "appmap.json")
	manifest = strings.Replace(manifest, `"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}]`,
		`"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}, {"path": "`+constRouter+`", "dialect": "ui-router-states/0"}]`, 1)
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, constRouter, router)
	for p, text := range files {
		writeFile(t, root, p, text)
	}
	rev := commitAll(t, root, "constant state names")
	return root, rev, build(t, root, rev)
}

// AMAP-V0-016: names and parents written as `X.Y` resolve through an imported object literal
// (`as const` or frozen), a string enum, a default export, a tsconfig path alias and a same-file
// table, each anchored to the declaring line at the map revision.
func TestAMAPV0016ConstantStateNames(t *testing.T) {
	router := `import { Screens } from './consts/screens';
import { Areas as Sections } from './consts/areas';
import Extra from './consts/extra';
import { Frozen } from '@consts/frozen';

const Local = { LOCAL: 'app.local' };

angular.module('admin').config(function ($stateProvider) {
  $stateProvider
    .state(Screens.REPORTS, { url: 'reports' })
    .state('audit', { parent: Sections.REPORTS, url: '/audit' })
    .state({ name: Screens.SUMMARY, url: 'summary' })
    .state(Extra.EXTRA, { url: 'extra' })
    .state(Frozen.FROZEN, { url: 'frozen' })
    .state(Local.LOCAL, { url: 'local' });
});
`
	files := map[string]string{
		"app/consts/screens.ts": `export const Screens = {
  REPORTS: 'app.reports',
  SUMMARY: 'app.summary',
} as const;
export type ScreenName = typeof Screens[keyof typeof Screens];
`,
		"app/consts/areas.ts": `export enum Areas {
  HOME = 'app.home',
  REPORTS = 'app.reports',
  COUNT,
}
`,
		"app/consts/extra.js":  "const Names = { EXTRA: 'app.extra' };\nexport default Names;\n",
		"app/consts/frozen.ts": "export const Frozen = Object.freeze({ FROZEN: 'app.frozen' });\n",
		"tsconfig.json":        `{"compilerOptions": {"baseUrl": ".", "paths": {"@consts/*": ["app/consts/*"]}}}`,
	}
	root, rev, m := constRepo(t, router, files)
	for state, want := range map[string]string{
		"app.reports": "/reports", "app.summary": "/summary", "audit": "/reports/audit",
		"app.extra": "/extra", "app.frozen": "/frozen", "app.local": "/local",
	} {
		s := screenByID(t, m, state)
		if s.Status != StatusResolved || s.Template != want {
			t.Errorf("%s: %s %s %q", state, s.Status, s.Reason, s.Template)
		}
	}
	reports := screenByID(t, m, "app.reports")
	if a := reports.NameFrom; len(a) != 2 || a[0].Path != "app/consts/screens.ts" || a[0].Start != 2 || a[0].End != 2 ||
		a[0].Blob != gitTest(t, root, "rev-parse", rev+":app/consts/screens.ts") || a[1].Path != constRouter || a[1].Start != 1 {
		t.Fatalf("name anchors %+v", a)
	}
	if reports.Parent != "screen:admin:app" || reports.ParentFrom != nil {
		t.Fatalf("dotted parent %s %+v", reports.Parent, reports.ParentFrom)
	}
	audit := screenByID(t, m, "audit")
	if a := audit.ParentFrom; len(a) != 2 || a[0].Path != "app/consts/areas.ts" || a[0].Start != 3 || a[1].Start != 2 || audit.Parent != "screen:admin:app.reports" || audit.NameFrom != nil {
		t.Fatalf("parent anchor %s %+v", audit.Parent, a)
	}
	if a := screenByID(t, m, "app.local").NameFrom; len(a) != 1 || a[0].Path != constRouter || a[0].Start != 6 {
		t.Fatalf("same-file anchor %+v", a)
	}

	// A literal-name map is byte-identical: the new members are omitted when unused.
	plainRoot, plainRev := fixtureRepo(t)
	if raw, err := Encode(build(t, plainRoot, plainRev)); err != nil || strings.Contains(string(raw), "name_from") || strings.Contains(string(raw), "parent_from") {
		t.Fatalf("literal map carries constant anchors: %v", err)
	}

	// Editing the declaration after the map revision makes the screen's lineage STALE.
	writeFile(t, root, "app/consts/screens.ts", strings.Replace(files["app/consts/screens.ts"], "'app.reports'", "'app.report'", 1))
	commitAll(t, root, "rename a constant")
	sv := screenDoc(t, m, "app.reports", Options{Root: root, Full: true})["screen"].(map[string]any)
	if sv["anchor"].(map[string]any)["freshness"] != Fresh || sv["lineage_freshness"] != Stale {
		t.Fatalf("screen: own %v lineage %v", sv["anchor"], sv["lineage_freshness"])
	}
}

// AMAP-V0-016: re-pointing only the import binding makes the screen's lineage STALE, although
// the state call and the original declaration are unchanged.
func TestAMAPV0016ImportBindingIsLineage(t *testing.T) {
	router := "import { Names } from './consts/a';\n\n$stateProvider.state(Names.X, { url: 'x' });\n"
	table := "export const Names = { X: 'app.x' };\n"
	root, _, m := constRepo(t, router, map[string]string{"app/consts/a.ts": table, "app/consts/b.ts": "export const Names = { X: 'app.y' };\n"})
	writeFile(t, root, constRouter, strings.Replace(router, "./consts/a", "./consts/b", 1))
	commitAll(t, root, "re-point the import")
	sv := screenDoc(t, m, "app.x", Options{Root: root, Full: true})["screen"].(map[string]any)
	if sv["anchor"].(map[string]any)["freshness"] != Fresh || sv["lineage_freshness"] != Stale {
		t.Fatalf("screen: own %v lineage %v", sv["anchor"], sv["lineage_freshness"])
	}
}

// AMAP-V0-016: every reference the map cannot prove stays UNKNOWN non-literal-name (a name) or
// non-literal-value (a parent); nothing is guessed.
func TestAMAPV0016UnprovableConstantsStayUnknown(t *testing.T) {
	const table = "app/consts/names.ts"
	for name, c := range map[string]struct{ imp, decl, extra string }{
		"computed key":      {"import { Names } from './consts/names';", "const k = 'X';\nexport const Names = { [k]: 'a', X: 'app.x' };\n", ""},
		"spread":            {"import { Names } from './consts/names';", "const Base = {};\nexport const Names = { ...Base, X: 'app.x' };\n", ""},
		"reassigned there":  {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\nNames.X = 'app.y';\n", ""},
		"reassigned here":   {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "Names.X += '.y';\n"},
		"passed along":      {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\nObject.assign(Names, {});\n", ""},
		"non-string value":  {"import { Names } from './consts/names';", "export const Names = { X: 42 };\n", ""},
		"numeric enum":      {"import { Names } from './consts/names';", "export enum Names { X }\n", ""},
		"duplicate enum":    {"import { Names } from './consts/names';", "export enum Names { X = 'app.x' }\nexport enum Names { Y = 'app.y' }\n", ""},
		"let binding":       {"import { Names } from './consts/names';", "export let Names = { X: 'app.x' };\n", ""},
		"not exported":      {"import { Names } from './consts/names';", "const Names = { X: 'app.x' };\n", ""},
		"absent member":     {"import { Names } from './consts/names';", "export const Names = { Y: 'app.x' };\n", ""},
		"unresolved import": {"import { Names } from './consts/missing';", "export const Names = { X: 'app.x' };\n", ""},
		"package import":    {"import { Names } from 'some-package';", "export const Names = { X: 'app.x' };\n", ""},
		"type-only import":  {"import type { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", ""},
		"not imported":      {"", "export const Names = { X: 'app.x' };\n", ""},
		"shadowed":          {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "function f(Names) { return Names; }\n"},
		"local and import":  {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "const Names = { X: 'app.x' };\n"},
		"no default":        {"import Names from './consts/names';", "export const Names = { X: 'app.x' };\n", ""},
		"default value":     {"import Names from './consts/names';", "const Names = { X: 'app.x' };\nexport default Names.X;\n", ""},
		"alias there":       {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\nconst holder = { names: Names };\nholder.names.X = 'app.y';\n", ""},
		"alias here":        {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "const holder = { names: Names };\n"},
		"prefix increment":  {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n++Names.X;\n", ""},
		"parenthesized":     {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "(Names.X) = 'app.y';\n"},
		"destructured":      {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", "[Names.X] = ['app.y'];\n"},
		"control":           {"import { Names } from './consts/names';", "export const Names = { X: 'app.x' };\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			router := c.imp + "\n" + c.extra + "$stateProvider.state(Names.X, { url: 'x' });\n$stateProvider.state('kid', { parent: Names.X, url: '/k' });\n"
			_, _, m := constRepo(t, router, map[string]string{table: c.decl})
			if name == "control" {
				if s := screenByID(t, m, "kid"); s.Status != StatusResolved || s.Template != "/x/k" || s.ParentFrom == nil {
					t.Errorf("control: %s %s %q", s.Status, s.Reason, s.Template)
				}
				return
			}
			lines := strings.Count(c.imp+"\n"+c.extra, "\n")
			if !hasUnknown(m, "state", constRouter+":"+strconv.Itoa(lines+1), "non-literal-name") {
				t.Errorf("name: %+v", m.Unknowns)
			}
			if s := screenByID(t, m, "kid"); s.Status != StatusUnknown || s.Reason != "non-literal-value" || s.ParentFrom != nil {
				t.Errorf("parent: %s %s %+v", s.Status, s.Reason, s.ParentFrom)
			}
		})
	}
}
