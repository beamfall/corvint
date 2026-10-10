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
		"asserted write":          {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  (Names.X as string) = 'app.y';\n", 1), map[string]string{reg: good}},
		"non-null write":          {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  Names.X! = 'app.y';\n", 1), map[string]string{reg: good}},
		"optional call":           {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  Names.reset?.();\n", 1), map[string]string{reg: good}},
		"delete cast":             {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  delete <any>Names.X;\n", 1), map[string]string{reg: good}},
		"delete object cast":      {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  delete <{}>Names.X;\n", 1), map[string]string{reg: good}},
		"typeof increment":        {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  typeof Names.X++;\n", 1), map[string]string{reg: good}},
		"typeof method call":      {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  typeof Names.reset();\n", 1), map[string]string{reg: good}},
		"typeof computed write":   {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  typeof Names['X']++;\n", 1), map[string]string{reg: good}},
		"spread rest write":       {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  [...Names.X] = ['app.y'];\n", 1), map[string]string{reg: good}},
		"template write":          {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  `${(Names.X = 'app.y')}`;\n", 1), map[string]string{reg: good}},
		"escaped write":           {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  N\\u0061mes.X = 'app.y';\n", 1), map[string]string{reg: good}},
		"escaped template write":  {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  `${N\\u0061mes.X = 'app.y'}`;\n", 1), map[string]string{reg: good}},
		"increment division":      {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  let n = 1;\n  n++ / (Names.X = 'app.y', 1) / 2;\n", 1), map[string]string{reg: good}},
		"decrement division":      {`["app/setup"]`, strings.Replace(plain, "=> {\n", "=> {\n  let n = 1;\n  n-- / (Names.X = 'app.y', 1) / 2;\n", 1), map[string]string{reg: good}},
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

// AMAP-V0-025: every file on the chain -- the registering file, the barrel and the declaring file --
// must pass the write checks for any other binding of the table: an alias import of its name, or a
// namespace import, dynamic import or require of a module that is the declaring file or may
// re-export it. One that cannot be proved only read leaves the name UNKNOWN (not-read-whole).
func TestAMAPV0025ChainBindingsFailClosed(t *testing.T) {
	const decl, index = "app/tables/routes/routes.constants.ts", "app/tables/routes/index.ts"
	table := "export const SectionTable = { REPORTS: 'ledger' };\n"
	star := "export * from './routes.constants';\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	// A nested package.json makes app/tables/routes a package directory, so the workspace cases
	// import the declaring file directly.
	direct := strings.Replace(diRegText, "'./tables'", "'../tables/routes/routes.constants'", 1)
	for name, c := range map[string]struct {
		reg   string
		files map[string]string
	}{
		"barrel alias":         {reg, map[string]string{index: star + "import { SectionTable as T } from './routes.constants';\nT.REPORTS = 'other';\n", decl: table}},
		"barrel same name":     {reg, map[string]string{index: star + "import { SectionTable } from './routes.constants';\nSectionTable.REPORTS = 'other';\n", decl: table}},
		"barrel namespace":     {reg, map[string]string{index: star + "import * as NS from './routes.constants';\nNS.SectionTable.REPORTS = 'other';\n", decl: table}},
		"barrel require":       {reg, map[string]string{index: star + "require('./routes.constants').SectionTable.REPORTS = 'other';\n", decl: table}},
		"barrel dynamic":       {reg, map[string]string{index: star + "import('./routes.constants').then((m) => { m.SectionTable.REPORTS = 'other'; });\n", decl: table}},
		"barrel computed":      {reg, map[string]string{index: star + "require('./routes' + '.constants').SectionTable.REPORTS = 'other';\n", decl: table}},
		"barrel string item":   {reg, map[string]string{index: star + "import { \"SectionTable\" as T } from './routes.constants';\nT.REPORTS = 'other';\n", decl: table}},
		"barrel unresolved":    {reg, map[string]string{index: star + "import * as NS from '@app/routes';\nNS.SectionTable.REPORTS = 'other';\n", decl: table}},
		"registering alias":    {reg + "import { SectionTable as T } from '../tables/routes';\nT.REPORTS = 'other';\n", map[string]string{index: star, decl: table}},
		"registering ns":       {reg + "import * as NS from '../tables/routes';\nNS.SectionTable.REPORTS = 'other';\n", map[string]string{index: star, decl: table}},
		"declaring self ns":    {reg, map[string]string{index: star, decl: table + "import * as me from './routes.constants';\nme.SectionTable.REPORTS = 'other';\n"}},
		"declaring via barrel": {reg, map[string]string{index: star, decl: table + "import { SectionTable as S } from './index';\nS.REPORTS = 'other';\n"}},
		"direct alias":         {direct + "import { SectionTable as T } from '../tables/routes/routes.constants';\nT.REPORTS = 'other';\n", map[string]string{decl: table}},
		"declared beside":      {"const T = { REPORTS: 'ledger' };\nangular.module('admin').constant('SectionNames', T);\nimport * as me from './setup.module';\nme.T.REPORTS = 'other';\n", nil},
		// Another name the declaring file or a barrel exports the table under (round 9).
		"registering default": {reg + "import T from '../tables/routes/routes.constants';\nT.REPORTS = 'other';\n", map[string]string{index: star, decl: table + "export default SectionTable;\n"}},
		"registering renamed": {strings.Replace(diRegText, "{ SectionTable } from './tables'", "{ Renamed as SectionTable } from '../tables/routes'", 1) +
			"import { SectionTable as S } from '../tables/routes/routes.constants';\nS.REPORTS = 'other';\n",
			map[string]string{index: "export { SectionTable as Renamed } from './routes.constants';\n", decl: table}},
		"registering relay": {reg + "import { R } from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": "import { SectionTable } from './routes.constants';\nexport { SectionTable as R };\n"}},
		// The review's round-10 input (the registration moved to line 2, where checkBarrel expects
		// it): a parenthesized default export of the imported table.
		"registering paren default": {"import { SectionTable } from '../tables/routes.constants';\nangular.module('admin').constant('SectionNames', SectionTable);\n" +
			"import R from '../tables/relay';\nR.REPORTS = 'other';\n",
			map[string]string{"app/tables/routes.constants.ts": table, "app/tables/relay.ts": "import { SectionTable } from './routes.constants';\nexport default (SectionTable);\n"}},
		// The review's round-11 input (registration on line 2): a member read of a namespace that a
		// named import binds, exported in parentheses.
		"registering ns member": {"import { SectionTable } from '../tables/routes.constants';\nangular.module('admin').constant('SectionNames', SectionTable);\n" +
			"import R from '../tables/relay';\nR.REPORTS = 'other';\n",
			map[string]string{"app/tables/routes.constants.ts": table, "app/tables/namespace.ts": "export * as NS from './routes.constants';\n",
				"app/tables/relay.ts": "import { NS } from './namespace';\nexport default (NS.SectionTable);\n"}},
		// The review's round-12 inputs: a module loaded inside a template substitution, and a
		// workspace package, which the resolver reads as a package, relaying the table.
		"relay template require": {reg + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": "let R;\n`${R = require('./routes.constants').SectionTable}`;\nexport default R;\n"}},
		"relay require alias": {reg + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": "const load = require;\nexport default load('./routes.constants').SectionTable;\n"}},
		"relay workspace package": {direct + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "package.json": `{"private":true,"workspaces":["app/tables/routes"]}`,
				"app/tables/routes/package.json": `{"name":"@local/tables","exports":"./routes.constants.ts"}`,
				"app/tables/routes/relay.ts":     "import { SectionTable as T } from '@local/tables';\nexport default (T);\n"}},
		"registering workspace package": {direct + "import R from '@local/tables';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "package.json": `{"private":true,"workspaces":["app/tables/routes"]}`,
				"app/tables/routes/package.json": `{"name":"@local/tables","exports":"./relay.ts"}`,
				"app/tables/routes/relay.ts":     "import { SectionTable } from './routes.constants';\nexport default (SectionTable);\n"}},
		"relay file dependency": {reg + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "package.json": `{"private":true,"dependencies":{"tables":"file:app/tables/routes"}}`,
				"app/tables/routes/relay.ts": "import { SectionTable as T } from 'tables';\nexport default (T);\n"}},
	} {
		t.Run(name, func(t *testing.T) { checkBarrel(t, c.reg, c.files, "not-read-whole", "") })
	}
	// Any other expression a relay exports, or any use of an import it does not prove a read,
	// makes it a possible holder of the table (round 10).
	for name, relay := range map[string]string{
		"paren":     "export default (SectionTable);\n",
		"as":        "export default SectionTable as typeof SectionTable;\n",
		"satisfies": "export default SectionTable satisfies object;\n",
		"non-null":  "export default SectionTable!;\n",
		"sequence":  "export default (0, SectionTable);\n",
		"or":        "export default SectionTable || {};\n",
		"call":      "export default wrap(SectionTable);\n",
		"alias":     "const S = SectionTable;\nexport default S;\n",
		"named":     "export const R = SectionTable;\n",
		// A member read of a named import that holds the table (round 11).
		"wrapped member":  "export const W = { T: SectionTable };\n",
		"member default":  "import { W } from './wrap';\nexport default W.T;\n",
		"member paren":    "import { W } from './wrap';\nexport default (W.T);\n",
		"member const":    "import { W } from './wrap';\nexport const R = W.T;\n",
		"member local":    "import { W } from './wrap';\nconst S = W.T;\nexport { S as R };\n",
		"member function": "import { W } from './wrap';\nexport function R() { return W.T; }\n",
	} {
		t.Run("relay "+name, func(t *testing.T) {
			checkBarrel(t, reg+"import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
				map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": "import { SectionTable } from './routes.constants';\n" + relay,
					"app/tables/routes/wrap.ts": "import { SectionTable } from './routes.constants';\nexport const W = { T: SectionTable };\n"}, "not-read-whole", "")
		})
	}
	// The review's round-13 inputs, a module loaded through a property name in a relay and in the
	// registering file, and every other loader form the reader fails closed on.
	t.Run("registering module.require", func(t *testing.T) {
		checkBarrel(t, reg+"module.require('../tables/routes/routes.constants').SectionTable.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table}, "not-read-whole", "")
	})
	for name, relay := range map[string]string{
		"module.require":   "export default module.require('./routes.constants').SectionTable;\n",
		"mainModule":       "export default process.mainModule.require('./routes.constants').SectionTable;\n",
		"createRequire":    "import { createRequire } from 'module';\nexport default createRequire(import.meta.url)('./routes.constants').SectionTable;\n",
		"webpack":          "export default __webpack_require__('./routes.constants').SectionTable;\n",
		"eval key":         "export default globalThis['eval'](\"require('./routes.constants')\").SectionTable;\n",
		"constructor":      "export default ({}).constructor.constructor(\"return require('./routes.constants')\")().SectionTable;\n",
		"System.import":    "export default await System.import('./routes.constants');\n",
		"module.children":  "export default module.children[0].exports.SectionTable;\n",
		"template require": "let R;\n`${R = module.require('./routes.constants').SectionTable}`;\nexport default R;\n",
	} {
		t.Run("loader "+name, func(t *testing.T) {
			checkBarrel(t, reg+"import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
				map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": relay}, "not-read-whole", "")
		})
	}
	// The round-11 input without parentheses, and through a named export.
	for name, relay := range map[string]string{
		"ns member bare":  "import { NS } from './namespace';\nexport default NS.SectionTable;\n",
		"ns member const": "import { NS } from './namespace';\nexport const R = NS.SectionTable;\n",
	} {
		t.Run(name, func(t *testing.T) {
			r := reg + "import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n"
			if strings.Contains(relay, "const R") {
				r = reg + "import { R } from '../tables/routes/relay';\nR.REPORTS = 'other';\n"
			}
			checkBarrel(t, r, map[string]string{index: star, decl: table, "app/tables/routes/namespace.ts": "export * as NS from './routes.constants';\n",
				"app/tables/routes/relay.ts": relay}, "not-read-whole", "")
		})
	}
	t.Run("relay namespace", func(t *testing.T) {
		checkBarrel(t, reg+"import R from '../tables/routes/relay';\nR.REPORTS = 'other';\n",
			map[string]string{index: star, decl: table, "app/tables/routes/relay.ts": "import * as NS from './routes.constants';\nexport default NS.SectionTable;\n"}, "not-read-whole", "")
	})
	// The thirteenth review's DI writes: another injection or lookup of the registered name in a
	// file the reader examines, the registering file, the chain's files and the files they import.
	run := "angular.module('admin').run(['SectionNames', function (s) { s.REPORTS = 'other'; }]);\n"
	for name, c := range map[string]struct {
		reg   string
		files map[string]string
	}{
		"run injected":        {reg + run, nil},
		"injector get":        {reg + "angular.module('admin').run(['$injector', function ($injector) { $injector.get('SectionNames').REPORTS = 'other'; }]);\n", nil},
		"injector computed":   {reg + "angular.module('admin').run(['$injector', function (i) { i.get('Section' + 'Names').REPORTS = 'other'; }]);\n", nil},
		"parameter name":      {reg + "angular.module('admin').run(function (SectionNames) { SectionNames.REPORTS = 'other'; });\n", nil},
		"$inject list":        {reg + "function boot(s) { s.REPORTS = 'other'; }\nboot.$inject = ['SectionNames'];\nangular.module('admin').run(boot);\n", nil},
		"decorator computed":  {reg + "const n = 'Section' + 'Names';\nangular.module('admin').decorator(n, (d) => { d.REPORTS = 'other'; return d; });\n", nil},
		"template":            {reg + "angular.module('admin').run([`SectionNames`, function (s) { s.REPORTS = 'other'; }]);\n", nil},
		"substitution":        {reg + "`${angular.module('admin').run(['SectionNames', function (s) { s.REPORTS = 'other'; }])}`;\n", nil},
		"barrel run":          {reg, map[string]string{index: star + run}},
		"declaring run":       {reg, map[string]string{decl: table + run}},
		"side-effect import":  {reg + "import './setup.run';\n", map[string]string{"app/setup/setup.run.ts": run}},
		"computed annotation": {reg + "angular.module('admin').run(['Section' + 'Names', function (s) { s.REPORTS = 'other'; }]);\n", nil},
		"regex annotation":    {reg + "angular.module('admin').run([/SectionNames/.source, (s) => { s.REPORTS = 'other'; }]);\n", nil},
		"computed $inject":    {reg + "function boot(s) { s.REPORTS = 'other'; }\nboot.$inject = [['Section', 'Names'].join('')];\nangular.module('admin').run(boot);\n", nil},
		"imported service":    {reg + "import { boot } from './setup.run';\nangular.module('admin').run(boot);\n", map[string]string{"app/setup/setup.run.ts": "export function boot($injector) { $injector.get('SectionNames').REPORTS = 'other'; }\n"}},
		// The fourteenth review's inputs: a computed annotation whose function is passed by name, and
		// a write through an ordinary import in a file a side-effect import loads.
		"named callback":     {reg + "function mutate(s) { s.REPORTS = 'other'; }\nangular.module('admin').config(['Section' + 'Names', mutate]);\n", nil},
		"member callback":    {reg + "const h = { mutate(s) { s.REPORTS = 'other'; } };\nangular.module('admin').run(['Section' + 'Names', h.mutate]);\n", nil},
		"member name":        {reg + "const N = { T: 'Section' + 'Names' };\nfunction mutate(s) { s.REPORTS = 'other'; }\nangular.module('admin').run([N.T, mutate]);\n", nil},
		"stored annotation":  {reg + "function mutate(s) { s.REPORTS = 'other'; }\nconst a = ['Section' + 'Names', mutate];\nangular.module('admin').run(a);\n", nil},
		"side-effect write":  {reg + "import './mutate';\n", map[string]string{decl: "export enum SectionTable { REPORTS = 'ledger' }\n", "app/setup/mutate.ts": "import { SectionTable as T } from '../tables/routes/routes.constants';\n(T as any).REPORTS = 'other';\n"}},
		"two-hop write":      {reg + "import './boot';\n", map[string]string{"app/setup/boot.ts": "import './mutate';\n", "app/setup/mutate.ts": "import { SectionTable as T } from '../tables/routes/routes.constants';\n(T as any).REPORTS = 'other';\n"}},
		"named-import write": {reg + "import { x } from './helpers';\n", map[string]string{"app/setup/helpers.ts": "import { SectionTable as T } from '../tables/routes';\nT.REPORTS = 'other';\nexport const x = 1;\n"}},
		"declaring import":   {reg, map[string]string{decl: table + "import './mutate';\n", "app/tables/routes/mutate.ts": "import { SectionTable as T } from './index';\nT.REPORTS = 'other';\n"}},
		"unresolved import":  {reg + "import '@app/mutate';\n", nil},
	} {
		t.Run("di "+name, func(t *testing.T) {
			files := map[string]string{index: star, decl: table}
			for p, text := range c.files {
				files[p] = text
			}
			checkBarrel(t, c.reg, files, "not-read-whole", "")
		})
	}
	// An injection of other names, a lodash constant and an object key of the name stay readable.
	t.Run("di other names", func(t *testing.T) {
		checkBarrel(t, reg+"angular.module('admin').run(['$rootScope', function ($rootScope) { $rootScope.x = /SectionNames/.test('a'); }]);\n"+
			"const k = _.constant('SectionNames');\nconst o = { SectionNames: 1 };\n", map[string]string{index: star, decl: table}, "", index)
	})
	// Files the chain loads through any import form, transitively, that only read the table, and
	// modules that cannot run code, stay readable.
	t.Run("di closure reads", func(t *testing.T) {
		checkBarrel(t, reg+"import './read';\n", map[string]string{index: star, decl: table, "app/setup/style.css": "p {}\n",
			"app/setup/read.ts": "import { SectionTable as T } from '../tables/routes';\nconst r = T.REPORTS;\nimport './style.css';\nimport { x } from './more';\n",
			"app/setup/more.ts": "export const x = [1, 2];\nconst [a, b] = x;\nangular.module('m', [uiRouter, ngAnimate]);\n"}, "", index)
	})
	t.Run("di router side-effect write", func(t *testing.T) {
		_, _, m, err := injectRepo(t, `["app/setup"]`, "import './setup/mutate';\n"+diRouter, map[string]string{diRegAt: reg, index: star, decl: table,
			"app/setup/mutate.ts": "import { SectionTable as T } from '../tables/routes/routes.constants';\n(T as any).REPORTS = 'other';\n"})
		if err != nil {
			t.Fatal(err)
		}
		if diResolved(t, m) {
			t.Fatalf("router side-effect write resolved: %+v", m.Unknowns)
		}
	})
	t.Run("di router run", func(t *testing.T) {
		_, _, m, err := injectRepo(t, `["app/setup"]`, diRouter+run, map[string]string{diRegAt: reg, index: star, decl: table})
		if err != nil {
			t.Fatal(err)
		}
		got := diUnknown(m)
		if diResolved(t, m) || len(got) != 1 || got[0] != (Unknown{Kind: "di-constant", Ref: "SectionNames", Reason: "not-read-whole", Path: diRegAt, Line: 2}) {
			t.Fatalf("router run resolved: %+v", m.Unknowns)
		}
	})
	// Other bindings the rule proves only read, and namespace imports that cannot reach the table.
	t.Run("reads", func(t *testing.T) {
		checkBarrel(t, reg+"import { SectionTable as T } from '../tables/routes';\nconst r = T.REPORTS;\nimport * as U from '../util';\n"+
			"import { Other } from '../tables/routes/routes.constants';\nconst o = Other.A;\nimport { helper } from '../util';\nhelper.call(null);\n",
			map[string]string{index: star + "import { SectionTable as T } from './routes.constants';\nconst s = T.REPORTS;\nimport * as P from '@playwright/test';\n",
				decl: table + "export const Other = { A: 'a' };\nimport * as V from '../../util';\nrequire('../../util');\n", "app/util.ts": "import { test } from '@playwright/test';\nexport const helper = test.info;\n"}, "", index)
	})
}

// AMAP-V0-025: the lexer does not read JSX, whose text can open a string that hides a write, so a
// .tsx or .jsx file holding any `<` is unread.
func TestAMAPV0025JSXFileUnread(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.tsx"
	files := map[string]string{decl: "export const SectionTable = { REPORTS: 'ledger' };\n" +
		"export const el = <p>it's {SectionTable.REPORTS = 'other'}</p>;\n"}
	t.Run("direct", func(t *testing.T) {
		checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes/routes.constants'", 1), files, "not-read-whole", "")
	})
	t.Run("star", func(t *testing.T) {
		files["app/tables/routes/index.ts"] = "export * from './routes.constants';\n"
		checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), files, "identifier-not-found", "")
	})
}

// AMAP-V0-025: a hashbang line ends at any line terminator (LF, CR, U+2028, U+2029); one ended by a
// lone CR or a separator, after which code shares the counted line, makes the file unread.
func TestAMAPV0025HashbangLineEnds(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	for name, end := range map[string]string{"carriage": "\r", "separator": "\u2028", "paragraph": "\u2029"} {
		files := map[string]string{decl: "#!/usr/bin/env node" + end + "setTimeout(() => { SectionTable.REPORTS = 'other'; });\n" +
			"export const SectionTable = { REPORTS: 'ledger' };\n"}
		t.Run("direct "+name, func(t *testing.T) {
			checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes/routes.constants'", 1), files, "not-read-whole", "")
		})
		t.Run("star "+name, func(t *testing.T) {
			all := map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n", decl: files[decl]}
			checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), all, "identifier-not-found", "")
		})
	}
	t.Run("reads", func(t *testing.T) {
		checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), map[string]string{
			"app/tables/routes/index.ts": "export * from './routes.constants';\n",
			decl:                         "#!/usr/bin/env node\r\nexport const SectionTable = { REPORTS: 'ledger' };\n// crlf\r\n"}, "", "app/tables/routes/index.ts")
	})
}

// AMAP-V0-025: a name the imported file exports itself (a local export list, or any declaration
// form, read loosely when the reader does not read it name by name) shadows every `export *`, as
// in ECMAScript; the reader does not follow it, so the name stays UNKNOWN instead of resolving the
// star source's table.
func TestAMAPV0025LocalExportShadowsStar(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, index := range map[string]string{
		"local alias":      "const Local = { REPORTS: 'other' };\nexport { Local as SectionTable };\nexport * from './routes.constants';\n",
		"local name":       "let SectionTable = { REPORTS: 'other' };\nexport * from './routes.constants';\nexport { SectionTable };\n",
		"imported alias":   "import { Other } from './other';\nexport { Other as SectionTable };\nexport * from './routes.constants';\n",
		"declare":          "export declare const SectionTable: { REPORTS: string };\nexport * from './routes.constants';\n",
		"abstract":         "export abstract class SectionTable {}\nexport * from './routes.constants';\n",
		"let declarator":   "export let a = 1, SectionTable = { REPORTS: 'other' };\nexport * from './routes.constants';\n",
		"declarator":       "export const a = 1, SectionTable = { REPORTS: 'other' };\nexport * from './routes.constants';\n",
		"table declarator": "export const a = { A: 'a' }, SectionTable = { REPORTS: 'other' };\nexport * from './routes.constants';\n",
		"namespace":        "export namespace SectionTable {\n  export const REPORTS = 'other';\n}\nexport * from './routes.constants';\n",
		"destructure":      "export const { SectionTable } = tables;\nexport * from './routes.constants';\n",
	} {
		t.Run(name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": index, decl: enum,
				"app/tables/routes/other.ts": "export const Other = { REPORTS: 'other' };\n"}, "identifier-not-found", "")
		})
	}
	// A barrel whose brackets do not balance may hide an export statement.
	checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nwrap(\n",
		decl: enum}, "ambiguous-barrel", "")
	// A loose export that does not mention the name shadows nothing.
	checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export declare const Other: string;\nexport let a = 1, b = 2;\nexport * from './routes.constants';\n",
		decl: enum}, "", "app/tables/routes/index.ts")
	// A type-only local export list exports no value and shadows nothing.
	checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "type Local = string;\nexport type { Local as SectionTable };\nexport * from './routes.constants';\n",
		decl: enum}, "", "app/tables/routes/index.ts")
}

// AMAP-V0-025: a star source that exports the name in a form the reader does not read as a table
// (a let, var, function, class, typed, declared or later-declarator const, abstract class,
// namespace, destructuring or local export list), or whose brackets do not balance, is still a
// candidate, so beside a readable table the name is ambiguous, and alone it is not found.
func TestAMAPV0025UnreadStarExportIsAmbiguous(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, copy := range map[string]string{
		"let":              "export let SectionTable = { REPORTS: 'other' };\n",
		"var":              "export var SectionTable = { REPORTS: 'other' };\n",
		"function":         "export function SectionTable() {}\n",
		"async":            "export async function SectionTable() {}\n",
		"generator":        "export function* SectionTable() {}\n",
		"class":            "export class SectionTable {}\n",
		"typed const":      "export const SectionTable: Tables = { REPORTS: 'other' };\n",
		"export list":      "const SectionTable = { REPORTS: 'other' };\nexport { SectionTable };\n",
		"destructure":      "export const { SectionTable } = tables;\n",
		"declare":          "export declare const SectionTable: { REPORTS: string };\n",
		"abstract":         "export abstract class SectionTable {}\n",
		"let declarator":   "export let a = 1, SectionTable = { REPORTS: 'other' };\n",
		"declarator":       "export const a = 1, SectionTable = { REPORTS: 'other' };\n",
		"table declarator": "export const a = { A: 'a' }, SectionTable = { REPORTS: 'other' };\n",
		"namespace":        "export namespace SectionTable {\n  export const REPORTS = 'other';\n}\n",
		"unbalanced":       "wrap(\nexport const SectionTable = { REPORTS: 'other' };\n",
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

// AMAP-V0-025: a file whose exports the reader cannot list -- an export statement a reader
// rejects or cannot decode (a string export name, an undecodable module name), a body that does
// not close, or an escaped identifier the lexer splits -- may export any name, so the table never
// resolves through it by elimination, as the barrel or as a competing star source.
func TestAMAPV0025UnlistableExportsFailClosed(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	enum := "export enum SectionTable {\n  REPORTS = 'ledger',\n}\n"
	reg := strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1)
	for name, form := range map[string]string{
		"quoted namespace":   "export * as \"SectionTable\" from './other';\n",
		"undecodable module": "export * from './\\uD800';\n",
		"unclosed enum":      "export enum Other {\n",
		"escaped class":      "export class \\u0053ectionTable {}\n",
		"quoted list name":   "const Local = { REPORTS: 'other' };\nexport { Local as \"SectionTable\" };\n",
		"dangling export":    "export\n",
		"mismatched bracket": "export enum Other {]\n",
	} {
		other := "export const Unrelated = { A: 'a' };\n"
		t.Run("barrel "+name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n" + form,
				decl: enum, "app/tables/routes/other.ts": other}, "ambiguous-barrel", "")
		})
		t.Run("named "+name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export { SectionTable } from './routes.constants';\n" + form,
				decl: enum, "app/tables/routes/other.ts": other}, "ambiguous-barrel", "")
		})
		t.Run("star source "+name, func(t *testing.T) {
			checkBarrel(t, reg, map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\nexport * from './copy';\n",
				decl: enum, "app/tables/routes/copy.ts": form, "app/tables/routes/other.ts": other}, "ambiguous-barrel", "")
		})
	}
}

// AMAP-V0-025: an unread declaring file never yields its declaration, whether it is reached
// through a star or named barrel or imported directly: an escaped identifier can write the table,
// and a skipped or undecodable statement, or source the lexer cannot place, can hide such a write.
func TestAMAPV0025UnreadDeclaringFileFailsClosed(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	table := "export const SectionTable = { REPORTS: 'ledger' };\n"
	other := "export const Unrelated = { A: 'a' };\n"
	for name, tail := range map[string]string{
		"escaped write":      "\\u0053ectionTable.REPORTS = 'other';\n",
		"unclosed enum":      "export enum Other {\n",
		"undecodable module": "export * from './\\uD800';\n",
		"mismatched bracket": "export enum Other {]\n",
		// Source the lexer cannot place as code, comment or literal (AMAP-V0-025).
		"escaped template":   "`${\\u0053ectionTable.REPORTS = 'other'}`;\n",
		"contextual of":      "let of = 4;\nof / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"non-null division":  "const n = 4;\nn! / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"type args division": "const v = n as Box<number> / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"regex after block":  "{}\n/'/.test('a'); SectionTable.REPORTS = 'other';\n",
		"separator comment":  "// note\u2028SectionTable.REPORTS = 'other';\n",
		"paragraph comment":  "// note\u2029SectionTable.REPORTS = 'other';\n",
		"carriage comment":   "// note\rSectionTable.REPORTS = 'other';\n",
		"double non-null":    "const n = 4;\nn!! / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"label after break":  "l: {\nbreak l\n/`/.test('a'); SectionTable.REPORTS = 'other';\n}\n// ` }\n",
		"debugger regex":     "debugger\n/`/.test('a'); SectionTable.REPORTS = 'other';\n// `\n",
		"unicode space":      "SectionTable\u00a0.REPORTS = 'other';\n",
		"unclosed comment":   "/* SectionTable.REPORTS = 'other';\n",
		"html comment":       "<!-- `\nSectionTable.REPORTS = 'other';\n// `\n",
	} {
		files := map[string]string{decl: table + tail, "app/tables/routes/other.ts": other}
		for via, barrel := range map[string]string{
			"star":  "export * from './routes.constants';\n",
			"named": "export { SectionTable } from './routes.constants';\n",
		} {
			t.Run(via+" "+name, func(t *testing.T) {
				all := map[string]string{"app/tables/routes/index.ts": barrel}
				for p, text := range files {
					all[p] = text
				}
				checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), all, "identifier-not-found", "")
			})
		}
		t.Run("direct "+name, func(t *testing.T) {
			checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes/routes.constants'", 1), files, "not-read-whole", "")
		})
		t.Run("direct default "+name, func(t *testing.T) {
			reg := strings.Replace(diRegText, "{ SectionTable } from './tables'", "SectionTable from '../tables/routes/routes.constants'", 1)
			checkBarrel(t, reg, map[string]string{decl: "export default { REPORTS: 'ledger' };\n" + tail}, "not-read-whole", "")
		})
	}
}

// AMAP-V0-025, AMAP-V0-026: any use of the table in its declaring file that the structural
// pure-read rule cannot prove is only a read -- an assertion on an assignment target, a computed
// or compound write, an increment, a delete, a call through the table, a for-in/of head, a
// destructuring element, an alias -- makes it not read whole, behind a star barrel or imported
// directly.
func TestAMAPV0026PossibleWritesFailClosed(t *testing.T) {
	const decl = "app/tables/routes/routes.constants.ts"
	table := "export const SectionTable = { REPORTS: 'ledger' };\n"
	for name, write := range map[string]string{
		"non-null assertion": "SectionTable.REPORTS! = 'other';\n",
		"as assertion":       "(SectionTable.REPORTS as string) = 'other';\n",
		"angle assertion":    "(<any>SectionTable).REPORTS = 'other';\n",
		"computed member":    "SectionTable['REPORTS'] = 'other';\n",
		"compound":           "SectionTable.REPORTS += '.other';\n",
		"postfix increment":  "SectionTable.REPORTS++;\n",
		"delete":             "delete SectionTable.REPORTS;\n",
		"object assign":      "Object.assign(SectionTable, { REPORTS: 'other' });\n",
		"define property":    "Object.defineProperty(SectionTable, 'REPORTS', { value: 'other' });\n",
		"alias":              "const Alias = SectionTable;\nAlias.REPORTS = 'other';\n",
		"first element":      "[SectionTable.REPORTS, rest] = ['other', 1];\n",
		"object pattern":     "({ a: SectionTable.REPORTS, b } = { a: 'other', b: 1 });\n",
		"for of head":        "for (SectionTable.REPORTS of ['other']) {}\n",
		"method call":        "SectionTable.reset();\n",
		"export default":     "export default SectionTable.REPORTS = 'other';\n",
		"optional call":      "SectionTable.reset?.();\n",
		"delete cast":        "delete <any>SectionTable.REPORTS;\n",
		"delete object cast": "delete <{}>SectionTable.REPORTS;\n",
		"typeof increment":   "typeof SectionTable.REPORTS++;\n",
		"typeof method call": "typeof SectionTable.reset();\n",
		"typeof computed":    "typeof SectionTable['REPORTS']++;\n",
		"default computed":   "export default SectionTable['REPORTS'] = 'other';\n",
		"spread rest":        "[...SectionTable.REPORTS] = ['other'];\n",
		"template write":     "`${(SectionTable.REPORTS = 'other')}`;\n",
		"brace in template":  "const s = `${'{'}`;\nSectionTable.REPORTS = 'other';\n",
		// Writes a lexer that misplaced a regular expression or division would hide.
		"increment division": "let n = 1;\nn++ / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"decrement division": "let n = 1;\nn-- / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"keyword property":   "const o = { return: 4 };\no.return / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"method named if":    "const o = { if: (n) => n };\no.if(1) / (SectionTable.REPORTS = 'other', 1) / 2;\n",
		"regex after break":  "for (;;) { break\n/'/.test('a'); SectionTable.REPORTS = 'other';\n}\n",
		"for await regex":    "async function f(y) { for await (const x of y) /'/.test('a'); SectionTable.REPORTS = 'other';\n}\n",
	} {
		files := map[string]string{decl: table + write}
		if strings.Contains(write, "reset") {
			files[decl] = "export const SectionTable = { REPORTS: 'ledger', reset: function () { this.REPORTS = 'other'; } };\n" + write
		}
		t.Run("star "+name, func(t *testing.T) {
			all := map[string]string{"app/tables/routes/index.ts": "export * from './routes.constants';\n"}
			for p, text := range files {
				all[p] = text
			}
			checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), all, "not-read-whole", "")
		})
		t.Run("direct "+name, func(t *testing.T) {
			checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes/routes.constants'", 1), files, "not-read-whole", "")
		})
	}
	// Reads the rule proves stay resolved: comparisons, a ternary, a member of the value, a
	// parenthesized argument, a nested object value, a statement without a semicolon, `typeof`
	// reads, a template that does not name the table, and an earlier block holding `++`.
	t.Run("reads", func(t *testing.T) {
		reads := "const same = SectionTable.REPORTS === 'x' ? SectionTable.REPORTS : (SectionTable.REPORTS);\n" +
			"const len = SectionTable.REPORTS.length;\nconst nested = { a: [SectionTable.REPORTS] }\nuse(SectionTable.REPORTS)\n" +
			"if (typeof SectionTable === 'object') { void typeof SectionTable['REPORTS']; }\nconst u = `${'a'}/b`;\n" +
			"function f() { let n = 0; n++; }\nconst d = SectionTable.REPORTS;\n" +
			"const q = 6 / 2 / 1, r = /a'b/.test('c') ? (q) / 2 : [q][0] / 2;\nif (q) /x/.test('y');\nconst g = (a) => /z/.test(a);\n" +
			"function h(n) { return /r/.test(n) || typeof /t/ === 'object' || void /v/; }\nlet m = 3; m++; m--;\nconst w = m++ / 2 + -/k/.source.length;\n" +
			"const bb = !!/y/.test('z') && (q ? /b/ : /c/).test('d') ? [1].length / 2 : 'x'.length / 2;\n"
		checkBarrel(t, strings.Replace(diRegText, "'./tables'", "'../tables/routes'", 1), map[string]string{
			"app/tables/routes/index.ts": "export * from './routes.constants';\n", decl: table + reads}, "", "app/tables/routes/index.ts")
	})
}
