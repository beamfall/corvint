package appmap

import (
	"context"
	"strings"
	"testing"
)

// injectRepo commits the fixture plus the constRouter router, a di_constants scope (none when
// scope is empty) and files, and builds the map.
func injectRepo(t *testing.T, scope, router string, files map[string]string) (string, string, *Map, error) {
	t.Helper()
	root, _ := fixtureRepo(t)
	manifest := readText(t, root, "appmap.json")
	manifest = strings.Replace(manifest, `"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}]`,
		`"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}, {"path": "`+constRouter+`", "dialect": "ui-router-states/0"}]`, 1)
	if scope != "" {
		manifest = strings.Replace(manifest, `"flows": "flows",`, `"flows": "flows",`+"\n  \"di_constants\": "+scope+",", 1)
	}
	writeFile(t, root, "appmap.json", manifest)
	writeFile(t, root, constRouter, router)
	for p, text := range files {
		writeFile(t, root, p, text)
	}
	rev := commitAll(t, root, "injected state names")
	m, err := Build(context.Background(), root, "appmap.json", rev)
	return root, rev, m, err
}

// nameUnknownIn reports whether the map read a state name in path as non-literal-name.
func nameUnknownIn(m *Map, path string) bool {
	for _, u := range m.Unknowns {
		if u.Kind == "state" && u.Path == path && u.Reason == "non-literal-name" {
			return true
		}
	}
	return false
}

// AMAP-V0-022: a router function's injected parameter resolves through the one `.constant(...)`
// registration in the di_constants scope -- an imported table, a table declared beside the
// registration, or an object literal argument -- anchored to the declaration, the import, the
// registration and the parameter list; a registration outside the scope and lodash's `_.constant`
// are not read; editing the registration makes the screen's lineage STALE.
func TestAMAPV0022InjectedStateNames(t *testing.T) {
	router := `const WidgetRoutes = ($stateProvider, WidgetConstants, AppStateNames) => {
  $stateProvider
    .state(AppStateNames.WIDGETS, { url: 'widgets' })
    .state('child', { parent: AppStateNames.WIDGETS, url: '/child' });
};
WidgetRoutes.$inject = ['$stateProvider', 'WidgetConstants', 'AppStateNames'];
export default WidgetRoutes;

angular.module('admin').config(['$stateProvider', 'Names', function ($stateProvider, Names) {
  $stateProvider.state({ name: Names.NAMES, url: 'names' });
}]);

angular.module('admin').config(($stateProvider: ng.ui.IStateProvider, Inline?: any): void => {
  $stateProvider.state(Inline.INLINE, { url: 'inline' });
});
`
	core := `import { RouteNames } from '../tables/routes';

angular.module('admin')
  .constant('AppStateNames', RouteNames)
  .constant('Inline', { INLINE: 'app.inline' });
`
	files := map[string]string{
		"app/tables/routes.ts":      "export const RouteNames = {\n  WIDGETS: 'app.widgets',\n} as const;\n",
		"app/setup/setup.module.ts": core,
		"app/setup/names.js":        "const Local = { NAMES: 'app.names' };\nangular.module('admin').constant('Names', Local);\nconst always = _.constant(1);\nconst fake = _.constant('Names', { NAMES: 'app.fake' });\nconst map = lodash.constant({ Names: {} });\n",
		"app/second/names.ts":       "angular.module('second').constant('AppStateNames', { WIDGETS: 'second.widgets' });\n",
	}
	root, rev, m, err := injectRepo(t, `["app/setup"]`, router, files)
	if err != nil {
		t.Fatal(err)
	}
	for state, want := range map[string]string{"app.widgets": "/widgets", "child": "/widgets/child", "app.names": "/names", "app.inline": "/inline"} {
		if s := screenByID(t, m, state); s.Status != StatusResolved || s.Template != want {
			t.Errorf("%s: %s %s %q", state, s.Status, s.Reason, s.Template)
		}
	}
	widgets := screenByID(t, m, "app.widgets")
	want := []struct {
		path       string
		start, end int
	}{{"app/tables/routes.ts", 2, 2}, {"app/setup/setup.module.ts", 1, 1}, {"app/setup/setup.module.ts", 4, 4}, {constRouter, 1, 1}}
	if a := widgets.NameFrom; len(a) != len(want) {
		t.Fatalf("name anchors %+v", a)
	}
	for i, w := range want {
		if a := widgets.NameFrom[i]; a.Path != w.path || a.Start != w.start || a.End != w.end || a.Blob != gitTest(t, root, "rev-parse", rev+":"+w.path) {
			t.Errorf("anchor %d: %+v, want %+v", i, a, w)
		}
	}
	if child := screenByID(t, m, "child"); child.Parent != "screen:admin:app.widgets" || len(child.ParentFrom) != 4 || child.NameFrom != nil {
		t.Fatalf("parent %s %+v", child.Parent, child.ParentFrom)
	}
	if a := screenByID(t, m, "app.names").NameFrom; len(a) != 3 || a[0].Path != "app/setup/names.js" || a[0].Start != 1 || a[1].Start != 2 || a[2].Path != constRouter || a[2].Start != 9 {
		t.Fatalf("declared-table anchors %+v", a)
	}
	if a := screenByID(t, m, "app.inline").NameFrom; len(a) != 2 || a[0].Path != "app/setup/setup.module.ts" || a[0].Start != 5 || a[1].Path != constRouter || a[1].Start != 13 {
		t.Fatalf("inline-table anchors %+v", a)
	}

	// Re-pointing the registration makes the screen's lineage STALE; the state call is unchanged.
	writeFile(t, root, "app/setup/setup.module.ts", strings.Replace(core, "'AppStateNames', RouteNames", "'AppStateNames', { WIDGETS: 'app.other' }", 1))
	commitAll(t, root, "re-point the registration")
	sv := screenDoc(t, m, "app.widgets", Options{Root: root, Full: true})["screen"].(map[string]any)
	if sv["anchor"].(map[string]any)["freshness"] != Fresh || sv["lineage_freshness"] != Stale {
		t.Fatalf("screen: own %v lineage %v", sv["anchor"], sv["lineage_freshness"])
	}
}

// AMAP-V0-022: each injectable function shape and annotation the router file proves resolves.
func TestAMAPV0022InjectionAnnotations(t *testing.T) {
	reg := map[string]string{"app/setup/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	for name, router := range map[string]string{
		"implicit arrow":     "const R = ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n};\nexport default R;\n",
		"expression arrow":   "export default ($stateProvider, Names) => $stateProvider\n  .state(Names.X, { url: 'x' });\n",
		"function $inject":   "function R($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n}\nR.$inject = ['$stateProvider', 'Names'];\nangular.module('a').config(R);\n",
		"config function":    "angular.module('a').config(function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n});\n",
		"config array arrow": "angular.module('a').config(['$stateProvider', 'Names', ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n}]);\n",
		"typed parameter":    "const R = ($stateProvider: ng.ui.IStateProvider, Names?: app.Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n};\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, `["app/setup"]`, router, reg)
			if err != nil {
				t.Fatal(err)
			}
			if s := screenByID(t, m, "app.x"); s.Status != StatusResolved || s.Template != "/x" || len(s.NameFrom) != 2 || s.NameFrom[1].Path != constRouter {
				t.Errorf("%s %s %q %+v", s.Status, s.Reason, s.Template, s.NameFrom)
			}
		})
	}
}

// AMAP-V0-023: an injected name the map cannot prove stays UNKNOWN non-literal-name (a parent
// non-literal-value); nothing is guessed.
func TestAMAPV0023UnprovableInjectionStaysUnknown(t *testing.T) {
	const reg = "app/setup/names.ts"
	plain := "const R = ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n};\n"
	good := "angular.module('admin').constant('Names', { X: 'app.x' });\n"
	for name, c := range map[string]struct {
		scope, router string
		files         map[string]string
	}{
		"no scope":                {"", plain, map[string]string{reg: good}},
		"no registration":         {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Other', { X: 'app.x' });\n"}},
		"two registrations":       {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/again.ts": good}},
		"computed name":           {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/dyn.ts": "angular.module('admin').constant(name, {});\n"}},
		"object map":              {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant({ Names: { X: 'app.x' } });\n"}},
		"object map computed":     {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/map.ts": "angular.module('admin').constant({ [k]: {} });\n"}},
		"object map by name":      {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/map.ts": "angular.module('admin').constant(TABLES);\n"}},
		"call value":              {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', makeNames());\n"}},
		"member value":            {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', Tables.names);\n"}},
		"spread value":            {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', { ...Base, X: 'app.x' });\n"}},
		"partial value":           {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', { X: 'app.x' } || other);\n"}},
		"mutated table":           {`["app/setup"]`, plain, map[string]string{reg: "const T = { X: 'app.x' };\nT.X = 'app.y';\nangular.module('admin').constant('Names', T);\n"}},
		"table registered twice":  {`["app/setup"]`, plain, map[string]string{reg: "const T = { X: 'app.x' };\nangular.module('admin').constant('Names', T).constant('Copy', T);\n"}},
		"unresolved table":        {`["app/setup"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', T);\n"}},
		"annotation renames":      {`["app/setup"]`, "angular.module('a').config(['$stateProvider', 'Other', function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n}]);\n", map[string]string{reg: good}},
		"annotation length":       {`["app/setup"]`, plain + "R.$inject = ['Names'];\n", map[string]string{reg: good}},
		"annotation unreadable":   {`["app/setup"]`, plain + "R.$inject = list;\n", map[string]string{reg: good}},
		"annotated twice":         {`["app/setup"]`, plain + "R.$inject = ['$stateProvider', 'Names'];\nR.$inject = ['$stateProvider', 'Names'];\n", map[string]string{reg: good}},
		"unnamed annotated":       {`["app/setup"]`, strings.Replace(plain, "const R = ", "export default ", 1) + "Q.$inject = ['a'];\n", map[string]string{reg: good}},
		"array annotation here":   {`["app/setup"]`, plain + "angular.module('a').config(['$stateProvider', 'Other', R]);\n", map[string]string{reg: good}},
		"nested parameter":        {`["app/setup"]`, "const R = ($stateProvider) => {\n  [1].forEach((Names) => $stateProvider.state(Names.X, { url: 'x' }));\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n};\n", map[string]string{reg: good}},
		"read outside":            {`["app/setup"]`, "const R = ($stateProvider, Names) => {};\n$stateProvider.state(Names.X, { url: 'x' });\n$stateProvider.state('kid', { parent: Names.X, url: '/k' });\n", map[string]string{reg: good}},
		"global":                  {`["app/setup"]`, "$stateProvider.state(Names.X, { url: 'x' });\n$stateProvider.state('kid', { parent: Names.X, url: '/k' });\n", map[string]string{reg: good}},
		"defaulted parameter":     {`["app/setup"]`, strings.Replace(plain, "Names)", "Names = {})", 1), map[string]string{reg: good}},
		"passed along":            {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  use(Names);\n", 1), map[string]string{reg: good}},
		"written":                 {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  Names.X = 'app.y';\n", 1), map[string]string{reg: good}},
		"local declaration":       {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  { const Names = { X: 'app.y' }; }\n", 1), map[string]string{reg: good}},
		"not injected position":   {`["app/setup"]`, "angular.module('a').run(function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n});\n", map[string]string{reg: good}},
		"object map expression":   {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/map.ts": "angular.module('admin').constant({ Other: {} } && dynamicTables);\n"}},
		"literal name expression": {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/dyn.ts": "angular.module('admin').constant('Na' + suffix, {});\n"}},
		"unclosed registration":   {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/dyn.ts": "angular.module('admin').constant(\n"}},
		"object return type":      {`["app/setup"]`, strings.Replace(plain, ") => {", "): { ok: boolean } => {", 1), map[string]string{reg: good}},
		"unclosed after a call":   {`["app/setup"]`, plain, map[string]string{reg: good, "app/setup/dyn.ts": "angular.module('admin').constant('Other', makeValue()\n"}},
		"control":                 {`["app/setup"]`, plain, map[string]string{reg: good}},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, c.scope, c.router, c.files)
			if err != nil {
				t.Fatal(err)
			}
			if name == "control" {
				if s := screenByID(t, m, "kid"); s.Status != StatusResolved || s.Template != "/x/k" || s.ParentFrom == nil {
					t.Errorf("control: %s %s %q", s.Status, s.Reason, s.Template)
				}
				return
			}
			if !nameUnknownIn(m, constRouter) || m.screen(screenID(m.App, "app.x")) != nil {
				t.Errorf("name resolved: %+v", m.Unknowns)
			}
			if s := screenByID(t, m, "kid"); s.Status != StatusUnknown || s.Reason != "non-literal-value" || s.ParentFrom != nil {
				t.Errorf("parent: %s %s %+v", s.Status, s.Reason, s.ParentFrom)
			}
		})
	}
}

// AMAP-V0-023: a scope source the reader cannot read as text could register any name, so every
// injected name in that build stays UNKNOWN.
func TestAMAPV0023ScopePoisoned(t *testing.T) {
	router := "const R = ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n};\n"
	files := map[string]string{"app/setup/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	_, _, m, err := injectRepo(t, `["app/setup"]`, router, files)
	if err != nil || m.screen(screenID(m.App, "app.x")) == nil {
		t.Fatalf("control: %v", err)
	}
	files["app/setup/binary.js"] = "angular.module('admin')\x00\xff\xfe.constant('Names', {});\n"
	_, _, m, err = injectRepo(t, `["app/setup"]`, router, files)
	if err != nil {
		t.Fatal(err)
	}
	if !nameUnknownIn(m, constRouter) || m.screen(screenID(m.App, "app.x")) != nil {
		t.Fatalf("poisoned scope resolved: %+v", m.Unknowns)
	}
}

// AMAP-V0-021: a malformed di_constants scope refuses; a valid one that no router reads through
// leaves every literal screen as it was.
func TestAMAPV0021DIConstantsManifest(t *testing.T) {
	router := "$stateProvider.state('app.lit', { url: 'lit' });\n"
	files := map[string]string{"app/setup/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	many := `["` + strings.Repeat(`app/setup", "`, 16) + `app/setup"]`
	for name, scope := range map[string]string{
		"empty":     `[]`,
		"escaping":  `["../app"]`,
		"absolute":  `["/app/setup"]`,
		"repeated":  `["app/setup", "app/setup"]`,
		"no source": `["app/none"]`,
		"too many":  many,
		"not array": `"app/setup"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, m, err := injectRepo(t, scope, router, files); codeOf(err) != "appmap-invalid-manifest" || m != nil {
				t.Fatalf("%s: %v", scope, err)
			}
		})
	}
	_, _, m, err := injectRepo(t, `["app/setup/names.ts", "app/tables"]`, router, map[string]string{
		"app/setup/names.ts": files["app/setup/names.ts"], "app/tables/a.ts": "export const A = 1;\n"})
	if err != nil {
		t.Fatal(err)
	}
	if s := screenByID(t, m, "app.lit"); s.Status != StatusResolved || s.NameFrom != nil {
		t.Fatalf("literal screen: %+v", s)
	}
}

// diRouter reads two injected names; diRegText registers SectionNames from the module ./tables.
const (
	diRouter  = "const R = ($stateProvider, SectionNames) => {\n  $stateProvider.state(SectionNames.REPORTS, { url: 'ledger' });\n  $stateProvider.state('kid', { parent: SectionNames.REPORTS, url: '/k' });\n};\n"
	diRegText = "import { SectionTable } from './tables';\nangular.module('admin').constant('SectionNames', SectionTable);\n"
	diRegAt   = "app/setup/setup.module.ts"
)

// diUnknown returns the di-constant unknowns of a map.
func diUnknown(m *Map) []Unknown {
	out := []Unknown{}
	for _, u := range m.Unknowns {
		if u.Kind == "di-constant" {
			out = append(out, u)
		}
	}
	return out
}

// diResolved reports whether both injected reads of diRouter resolved.
func diResolved(t *testing.T, m *Map) bool {
	t.Helper()
	s := m.screen(screenID(m.App, "kid"))
	return m.screen(screenID(m.App, "ledger")) != nil && s != nil && s.Status == StatusResolved && s.Template == "/ledger/k"
}

// AMAP-V0-024: an injected string enum is a table only when every member has a literal string
// initializer; one numeric, computed or auto-numbered member makes it no table, and the build says
// so with a di-constant non-literal-member unknown.
func TestAMAPV0024InjectedStringEnum(t *testing.T) {
	for name, c := range map[string]struct {
		table string
		ok    bool
	}{
		"string enum":      {"export enum SectionTable {\n  REPORTS = 'ledger',\n  HOME = 'home',\n}\n", true},
		"const enum":       {"export const enum SectionTable {\n  REPORTS = `ledger`,\n  'QUOTED' = 'quoted'\n}\n", true},
		"auto member":      {"export enum SectionTable {\n  REPORTS = 'ledger',\n  COUNT,\n}\n", false},
		"numeric member":   {"export enum SectionTable {\n  REPORTS = 'ledger',\n  LIMIT = 4,\n}\n", false},
		"computed member":  {"export enum SectionTable {\n  REPORTS = 'ledger',\n  OTHER = prefix + 'other',\n}\n", false},
		"template subst":   {"export enum SectionTable {\n  REPORTS = 'ledger',\n  OTHER = `${prefix}other`,\n}\n", false},
		"computed key":     {"export enum SectionTable {\n  REPORTS = 'ledger',\n  [key] = 'other',\n}\n", false},
		"member reference": {"export enum SectionTable {\n  REPORTS = 'ledger',\n  ALIAS = REPORTS,\n}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, map[string]string{diRegAt: diRegText, "app/setup/tables.ts": c.table})
			if err != nil {
				t.Fatal(err)
			}
			got := diUnknown(m)
			if c.ok {
				if !diResolved(t, m) || len(got) != 0 {
					t.Fatalf("enum did not resolve: %+v", m.Unknowns)
				}
				if a := screenByID(t, m, "ledger").NameFrom; len(a) != 4 || a[0].Path != "app/setup/tables.ts" || a[0].Start != 2 || a[1].Path != diRegAt || a[2].Start != 2 || a[3].Path != constRouter {
					t.Fatalf("enum anchors %+v", a)
				}
				return
			}
			if diResolved(t, m) || !nameUnknownIn(m, constRouter) {
				t.Fatalf("partial enum resolved: %+v", m.Unknowns)
			}
			if len(got) != 1 || got[0] != (Unknown{Kind: "di-constant", Ref: "SectionNames", Reason: "non-literal-member", Path: diRegAt, Line: 2}) {
				t.Fatalf("diagnostic %+v", got)
			}
		})
	}
	// A router's own imported enum keeps the AMAP-V0-016 rule: a member without a string
	// initializer is absent, the others still resolve.
	router := "import { Areas } from './consts/areas';\n$stateProvider.state(Areas.REPORTS, { url: 'ledger' });\n"
	_, _, m, err := injectRepo(t, "", router, map[string]string{"app/consts/areas.ts": "export enum Areas {\n  REPORTS = 'ledger',\n  COUNT,\n}\n"})
	if err != nil || m.screen(screenID(m.App, "ledger")) == nil {
		t.Fatalf("AMAP-V0-016 enum: %v %+v", err, m.Unknowns)
	}
}

// AMAP-V0-025: the registering file's import of T may reach the table through one re-export in
// the imported file (a directory index or the file itself): `export *`, `export { T }` or
// `export { S as T }` from a module that declares it. The re-export statement joins the anchors
// between the declaration and the import; a second candidate, a second level, or a module the
// reader cannot see stays UNKNOWN with its reason.
func TestAMAPV0025InjectedTableThroughBarrel(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	object := "export const SectionTable = {\n  REPORTS: 'ledger',\n} as const;\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, c := range map[string]struct {
		reg    string
		files  map[string]string
		reason string // "" resolves
		via    string // the re-export's file when it resolves
	}{
		"star index enum": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n", decl: enum}, "", "app/tables/routes/index.ts"},
		"named index object": {reg, map[string]string{"app/tables/routes/index.ts": "export { SectionTable } from './routes.constants';\nexport * from './other';\n",
			decl: object, "app/tables/routes/other.ts": "export const SectionTable = { REPORTS: 'other' };\n"}, "", "app/tables/routes/index.ts"},
		"renamed": {reg, map[string]string{"app/tables/routes/index.ts": "export {\n  Inner as SectionTable,\n} from './routes.constants';\n",
			decl: strings.Replace(enum, "SectionTable", "Inner", 1)}, "", "app/tables/routes/index.ts"},
		"default as name": {reg, map[string]string{"app/tables/routes/index.ts": "export { default as SectionTable } from './routes.constants';\n",
			decl: "export default {\n  REPORTS: 'ledger',\n};\n"}, "", "app/tables/routes/index.ts"},
		"imported file": {strings.Replace(diRegText, "'./tables'", "'../tables/all'", 1), map[string]string{"app/tables/all.ts": "export * from './routes/routes.constants';\nexport * from './empty';\n",
			decl: enum, "app/tables/empty.ts": "export const Unrelated = { A: 'a' };\n"}, "", "app/tables/all.ts"},
		"two stars": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport * from './copy';\n",
			decl: enum, "app/tables/routes/copy.ts": enum}, "ambiguous-barrel", ""},
		"two named": {reg, map[string]string{"app/tables/routes/index.ts": "export { SectionTable } from './routes.constants';\nexport { SectionTable } from './copy';\n",
			decl: enum, "app/tables/routes/copy.ts": enum}, "ambiguous-barrel", ""},
		"unread item": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport { a.b } from './copy';\n", decl: enum}, "ambiguous-barrel", ""},
		"second level": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './inner';\n",
			"app/tables/routes/inner.ts": "export * from './routes.constants';\n", decl: enum}, "barrel-depth-exceeded", ""},
		"second level beside": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport * from './inner';\n",
			"app/tables/routes/inner.ts": "export * from './more';\n", decl: enum}, "barrel-depth-exceeded", ""},
		"package star":   {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport * from 'some-package';\n", decl: enum}, "out-of-scope", ""},
		"missing module": {reg, map[string]string{"app/tables/routes/index.ts": "export { SectionTable } from './missing';\n", decl: enum}, "out-of-scope", ""},
		"package import": {strings.Replace(diRegText, "'./tables'", "'some-package'", 1), map[string]string{decl: enum}, "out-of-scope", ""},
		"not exported":   {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n", decl: "enum SectionTable {\n  REPORTS = 'ledger',\n}\n"}, "identifier-not-found", ""},
		"type only":      {reg, map[string]string{"app/tables/routes/index.ts": "export type { SectionTable } from './routes.constants';\n", decl: enum}, "identifier-not-found", ""},
		"namespace":      {reg, map[string]string{"app/tables/routes/index.ts": "export * as SectionTable from './routes.constants';\n", decl: enum}, "identifier-not-found", ""},
		"partial behind barrel": {reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n",
			decl: "export enum SectionTable {\n  REPORTS = 'ledger',\n  COUNT,\n}\n"}, "non-literal-member", ""},
	} {
		t.Run(name, func(t *testing.T) { checkBarrel(t, c.reg, c.files, c.reason, c.via) })
	}
}

// checkBarrel builds diRouter with the registration reg and files, and checks that the injected
// table resolves through the re-export in via (reason "") or stays UNKNOWN with the di-constant reason.
func checkBarrel(t *testing.T, reg string, files map[string]string, reason, via string) {
	t.Helper()
	all := map[string]string{diRegAt: reg}
	for p, text := range files {
		all[p] = text
	}
	_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, all)
	if err != nil {
		t.Fatal(err)
	}
	got := diUnknown(m)
	if reason == "" {
		if !diResolved(t, m) || len(got) != 0 {
			t.Fatalf("barrel did not resolve: %+v", m.Unknowns)
		}
		a := screenByID(t, m, "ledger").NameFrom
		if len(a) != 5 || a[1].Path != via || a[2].Path != diRegAt || a[2].Start != 1 || a[3].Start != 2 || a[4].Path != constRouter {
			t.Fatalf("barrel anchors %+v", a)
		}
		return
	}
	if diResolved(t, m) || !nameUnknownIn(m, constRouter) {
		t.Fatalf("barrel resolved: %+v", m.Unknowns)
	}
	if len(got) != 1 || got[0] != (Unknown{Kind: "di-constant", Ref: "SectionNames", Reason: reason, Path: diRegAt, Line: 2}) {
		t.Fatalf("diagnostic %+v", got)
	}
}

// AMAP-V0-025: a name the imported file exports itself through a local export list shadows every
// `export *`, as in ECMAScript; the reader does not follow the local binding, so the name stays
// UNKNOWN instead of resolving the star source's table.
func TestAMAPV0025LocalExportShadowsStar(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, index := range map[string]string{
		"local alias":    "const Local = { REPORTS: 'other' };\nexport { Local as SectionTable };\nexport * from './routes.constants';\n",
		"local name":     "let SectionTable = { REPORTS: 'other' };\nexport * from './routes.constants';\nexport { SectionTable };\n",
		"imported alias": "import { Other } from './other';\nexport { Other as SectionTable };\nexport * from './routes.constants';\n",
	} {
		t.Run(name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": index, decl: enum,
				"app/tables/routes/other.ts": "export const Other = { REPORTS: 'other' };\n"}, "identifier-not-found", "")
		})
	}
	// An exported destructuring may export the name; the reader cannot tell.
	checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export const { SectionTable } = tables;\nexport * from './routes.constants';\n",
		decl: enum}, "ambiguous-barrel", "")
	// A type-only local export list exports no value and shadows nothing.
	checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "type Local = string;\nexport type { Local as SectionTable };\nexport * from './routes.constants';\n",
		decl: enum}, "", "app/tables/routes/index.ts")
}

// AMAP-V0-025: a star source that exports the name in a form the reader does not read as a table
// (a let, var, function, class, typed const or local export list) is still a candidate, so beside a
// readable table the name is ambiguous, and alone it is not found.
func TestAMAPV0025UnreadStarExportIsAmbiguous(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, copy := range map[string]string{
		"let":         "export let SectionTable = { REPORTS: 'other' };\n",
		"var":         "export var SectionTable = { REPORTS: 'other' };\n",
		"function":    "export function SectionTable() {}\n",
		"async":       "export async function SectionTable() {}\n",
		"generator":   "export function* SectionTable() {}\n",
		"class":       "export class SectionTable {}\n",
		"typed const": "export const SectionTable: Tables = { REPORTS: 'other' };\n",
		"export list": "const SectionTable = { REPORTS: 'other' };\nexport { SectionTable };\n",
		"destructure": "export const { SectionTable } = tables;\n",
	} {
		t.Run(name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport * from './copy';\n",
				decl: enum, "app/tables/routes/copy.ts": copy}, "ambiguous-barrel", "")
		})
	}
	t.Run("alone", func(t *testing.T) {
		checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export * from './copy';\n",
			"app/tables/routes/copy.ts": "export let SectionTable = { REPORTS: 'ledger' };\n"}, "identifier-not-found", "")
	})
}

// AMAP-V0-026: when the one in-scope registration of an injected name does not resolve, the map
// carries a di-constant unknown naming the registration and why; a resolving build, a build
// without the scope and an unregistered name carry none.
func TestAMAPV0026DIConstantDiagnostics(t *testing.T) {
	table := "export const SectionTable = { REPORTS: 'ledger' };\n"
	for name, c := range map[string]struct {
		reg, table, reason string
	}{
		"identifier not found":  {"angular.module('admin')\n  .constant('SectionNames', Missing);\n", table, "identifier-not-found"},
		"member not found":      {diRegText, "export const SectionTable = { HOME: 'home' };\n", "member-not-found"},
		"call value":            {"angular.module('admin')\n  .constant('SectionNames', makeNames());\n", table, "unreadable-registration"},
		"object map":            {"angular.module('admin')\n  .constant({ SectionNames: {} });\n", table, "unreadable-registration"},
		"mutated":               {diRegText, table + "SectionTable.REPORTS = 'other';\n", "not-read-whole"},
		"passed along":          {diRegText + "use(SectionTable);\n", table, "not-read-whole"},
		"spread":                {diRegText, "export const SectionTable = { ...Base, REPORTS: 'ledger' };\n", "non-literal-member"},
		"inline spread":         {"angular.module('admin')\n  .constant('SectionNames', { ...Base, REPORTS: 'ledger' });\n", table, "non-literal-member"},
		"declared and imported": {diRegText + "const SectionTable = { REPORTS: 'ledger' };\n", table, "ambiguous-binding"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter, map[string]string{diRegAt: c.reg, "app/setup/tables.ts": c.table})
			if err != nil {
				t.Fatal(err)
			}
			if diResolved(t, m) {
				t.Fatalf("resolved: %+v", m.Unknowns)
			}
			if got := diUnknown(m); len(got) != 1 || got[0] != (Unknown{Kind: "di-constant", Ref: "SectionNames", Reason: c.reason, Path: diRegAt, Line: 2}) {
				t.Fatalf("diagnostic %+v", got)
			}
		})
	}
	for name, c := range map[string]struct{ scope, reg string }{
		"resolves":     {`["app/setup"]`, diRegText},
		"no scope":     {"", diRegText},
		"unregistered": {`["app/setup"]`, strings.Replace(diRegText, "'SectionNames'", "'Other'", 1)},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, c.scope, diRouter, map[string]string{diRegAt: c.reg, "app/setup/tables.ts": table})
			if err != nil {
				t.Fatal(err)
			}
			if got := diUnknown(m); len(got) != 0 || diResolved(t, m) != (name == "resolves") {
				t.Fatalf("%+v", m.Unknowns)
			}
		})
	}
}
