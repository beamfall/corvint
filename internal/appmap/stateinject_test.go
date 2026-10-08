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
	router := `const AddOnsRoutes = ($stateProvider, IconConstants, StateConstants) => {
  $stateProvider
    .state(StateConstants.ADD_ONS, { url: 'add-ons' })
    .state('child', { parent: StateConstants.ADD_ONS, url: '/child' });
};
AddOnsRoutes.$inject = ['$stateProvider', 'IconConstants', 'StateConstants'];
export default AddOnsRoutes;

angular.module('admin').config(['$stateProvider', 'Names', function ($stateProvider, Names) {
  $stateProvider.state({ name: Names.NAMES, url: 'names' });
}]);

angular.module('admin').config(($stateProvider: ng.ui.IStateProvider, Inline?: any): void => {
  $stateProvider.state(Inline.INLINE, { url: 'inline' });
});
`
	core := `import { RoutesConstants } from '../consts/routes';

angular.module('admin')
  .constant('StateConstants', RoutesConstants)
  .constant('Inline', { INLINE: 'app.inline' });
`
	files := map[string]string{
		"app/consts/routes.ts":       "export const RoutesConstants = {\n  ADD_ONS: 'app.addons',\n} as const;\n",
		"app/core/core.module.ts":    core,
		"app/core/names.js":          "const Local = { NAMES: 'app.names' };\nangular.module('admin').constant('Names', Local);\nconst always = _.constant(1);\n",
		"app/market/states.const.ts": "angular.module('market').constant('StateConstants', { ADD_ONS: 'market.addons' });\n",
	}
	root, rev, m, err := injectRepo(t, `["app/core"]`, router, files)
	if err != nil {
		t.Fatal(err)
	}
	for state, want := range map[string]string{"app.addons": "/add-ons", "child": "/add-ons/child", "app.names": "/names", "app.inline": "/inline"} {
		if s := screenByID(t, m, state); s.Status != StatusResolved || s.Template != want {
			t.Errorf("%s: %s %s %q", state, s.Status, s.Reason, s.Template)
		}
	}
	addons := screenByID(t, m, "app.addons")
	want := []struct {
		path       string
		start, end int
	}{{"app/consts/routes.ts", 2, 2}, {"app/core/core.module.ts", 1, 1}, {"app/core/core.module.ts", 4, 4}, {constRouter, 1, 1}}
	if a := addons.NameFrom; len(a) != len(want) {
		t.Fatalf("name anchors %+v", a)
	}
	for i, w := range want {
		if a := addons.NameFrom[i]; a.Path != w.path || a.Start != w.start || a.End != w.end || a.Blob != gitTest(t, root, "rev-parse", rev+":"+w.path) {
			t.Errorf("anchor %d: %+v, want %+v", i, a, w)
		}
	}
	if child := screenByID(t, m, "child"); child.Parent != "screen:admin:app.addons" || len(child.ParentFrom) != 4 || child.NameFrom != nil {
		t.Fatalf("parent %s %+v", child.Parent, child.ParentFrom)
	}
	if a := screenByID(t, m, "app.names").NameFrom; len(a) != 3 || a[0].Path != "app/core/names.js" || a[0].Start != 1 || a[1].Start != 2 || a[2].Path != constRouter || a[2].Start != 9 {
		t.Fatalf("declared-table anchors %+v", a)
	}
	if a := screenByID(t, m, "app.inline").NameFrom; len(a) != 2 || a[0].Path != "app/core/core.module.ts" || a[0].Start != 5 || a[1].Path != constRouter || a[1].Start != 13 {
		t.Fatalf("inline-table anchors %+v", a)
	}

	// Re-pointing the registration makes the screen's lineage STALE; the state call is unchanged.
	writeFile(t, root, "app/core/core.module.ts", strings.Replace(core, "'StateConstants', RoutesConstants", "'StateConstants', { ADD_ONS: 'app.other' }", 1))
	commitAll(t, root, "re-point the registration")
	sv := screenDoc(t, m, "app.addons", Options{Root: root, Full: true})["screen"].(map[string]any)
	if sv["anchor"].(map[string]any)["freshness"] != Fresh || sv["lineage_freshness"] != Stale {
		t.Fatalf("screen: own %v lineage %v", sv["anchor"], sv["lineage_freshness"])
	}
}

// AMAP-V0-022: each injectable function shape and annotation the router file proves resolves.
func TestAMAPV0022InjectionAnnotations(t *testing.T) {
	reg := map[string]string{"app/core/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	for name, router := range map[string]string{
		"implicit arrow":     "const R = ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n};\nexport default R;\n",
		"expression arrow":   "export default ($stateProvider, Names) => $stateProvider\n  .state(Names.X, { url: 'x' });\n",
		"function $inject":   "function R($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n}\nR.$inject = ['$stateProvider', 'Names'];\nangular.module('a').config(R);\n",
		"config function":    "angular.module('a').config(function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n});\n",
		"config array arrow": "angular.module('a').config(['$stateProvider', 'Names', ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n}]);\n",
		"typed parameter":    "const R = ($stateProvider: ng.ui.IStateProvider, Names?: app.Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n};\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, m, err := injectRepo(t, `["app/core"]`, router, reg)
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
	const reg = "app/core/names.ts"
	plain := "const R = ($stateProvider, Names) => {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n};\n"
	good := "angular.module('admin').constant('Names', { X: 'app.x' });\n"
	for name, c := range map[string]struct {
		scope, router string
		files         map[string]string
	}{
		"no scope":               {"", plain, map[string]string{reg: good}},
		"no registration":        {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Other', { X: 'app.x' });\n"}},
		"two registrations":      {`["app/core"]`, plain, map[string]string{reg: good, "app/core/again.ts": good}},
		"computed name":          {`["app/core"]`, plain, map[string]string{reg: good, "app/core/dyn.ts": "angular.module('admin').constant(name, {});\n"}},
		"object map":             {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant({ Names: { X: 'app.x' } });\n"}},
		"object map computed":    {`["app/core"]`, plain, map[string]string{reg: good, "app/core/map.ts": "angular.module('admin').constant({ [k]: {} });\n"}},
		"object map by name":     {`["app/core"]`, plain, map[string]string{reg: good, "app/core/map.ts": "angular.module('admin').constant(TABLES);\n"}},
		"call value":             {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', makeNames());\n"}},
		"member value":           {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', Tables.names);\n"}},
		"spread value":           {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', { ...Base, X: 'app.x' });\n"}},
		"partial value":          {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', { X: 'app.x' } || other);\n"}},
		"mutated table":          {`["app/core"]`, plain, map[string]string{reg: "const T = { X: 'app.x' };\nT.X = 'app.y';\nangular.module('admin').constant('Names', T);\n"}},
		"table registered twice": {`["app/core"]`, plain, map[string]string{reg: "const T = { X: 'app.x' };\nangular.module('admin').constant('Names', T).constant('Copy', T);\n"}},
		"unresolved table":       {`["app/core"]`, plain, map[string]string{reg: "angular.module('admin').constant('Names', T);\n"}},
		"annotation renames":     {`["app/core"]`, "angular.module('a').config(['$stateProvider', 'Other', function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n}]);\n", map[string]string{reg: good}},
		"annotation length":      {`["app/core"]`, plain + "R.$inject = ['Names'];\n", map[string]string{reg: good}},
		"annotation unreadable":  {`["app/core"]`, plain + "R.$inject = list;\n", map[string]string{reg: good}},
		"annotated twice":        {`["app/core"]`, plain + "R.$inject = ['$stateProvider', 'Names'];\nR.$inject = ['$stateProvider', 'Names'];\n", map[string]string{reg: good}},
		"unnamed annotated":      {`["app/core"]`, strings.Replace(plain, "const R = ", "export default ", 1) + "Q.$inject = ['a'];\n", map[string]string{reg: good}},
		"array annotation here":  {`["app/core"]`, plain + "angular.module('a').config(['$stateProvider', 'Other', R]);\n", map[string]string{reg: good}},
		"nested parameter":       {`["app/core"]`, "const R = ($stateProvider) => {\n  [1].forEach((Names) => $stateProvider.state(Names.X, { url: 'x' }));\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n};\n", map[string]string{reg: good}},
		"read outside":           {`["app/core"]`, "const R = ($stateProvider, Names) => {};\n$stateProvider.state(Names.X, { url: 'x' });\n$stateProvider.state('kid', { parent: Names.X, url: '/k' });\n", map[string]string{reg: good}},
		"global":                 {`["app/core"]`, "$stateProvider.state(Names.X, { url: 'x' });\n$stateProvider.state('kid', { parent: Names.X, url: '/k' });\n", map[string]string{reg: good}},
		"defaulted parameter":    {`["app/core"]`, strings.Replace(plain, "Names)", "Names = {})", 1), map[string]string{reg: good}},
		"passed along":           {`["app/core"]`, strings.Replace(plain, "=> {\n", "=> {\n  use(Names);\n", 1), map[string]string{reg: good}},
		"written":                {`["app/core"]`, strings.Replace(plain, "=> {\n", "=> {\n  Names.X = 'app.y';\n", 1), map[string]string{reg: good}},
		"local declaration":      {`["app/core"]`, strings.Replace(plain, "=> {\n", "=> {\n  { const Names = { X: 'app.y' }; }\n", 1), map[string]string{reg: good}},
		"not injected position":  {`["app/core"]`, "angular.module('a').run(function ($stateProvider, Names) {\n  $stateProvider.state(Names.X, { url: 'x' });\n  $stateProvider.state('kid', { parent: Names.X, url: '/k' });\n});\n", map[string]string{reg: good}},
		"control":                {`["app/core"]`, plain, map[string]string{reg: good}},
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
	files := map[string]string{"app/core/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	_, _, m, err := injectRepo(t, `["app/core"]`, router, files)
	if err != nil || m.screen(screenID(m.App, "app.x")) == nil {
		t.Fatalf("control: %v", err)
	}
	files["app/core/binary.js"] = "angular.module('admin')\x00\xff\xfe.constant('Names', {});\n"
	_, _, m, err = injectRepo(t, `["app/core"]`, router, files)
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
	files := map[string]string{"app/core/names.ts": "angular.module('admin').constant('Names', { X: 'app.x' });\n"}
	many := `["` + strings.Repeat(`app/core", "`, 16) + `app/core"]`
	for name, scope := range map[string]string{
		"empty":     `[]`,
		"escaping":  `["../app"]`,
		"absolute":  `["/app/core"]`,
		"repeated":  `["app/core", "app/core"]`,
		"no source": `["app/none"]`,
		"too many":  many,
		"not array": `"app/core"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, m, err := injectRepo(t, scope, router, files); codeOf(err) != "appmap-invalid-manifest" || m != nil {
				t.Fatalf("%s: %v", scope, err)
			}
		})
	}
	_, _, m, err := injectRepo(t, `["app/core/names.ts", "app/consts"]`, router, map[string]string{
		"app/core/names.ts": files["app/core/names.ts"], "app/consts/a.ts": "export const A = 1;\n"})
	if err != nil {
		t.Fatal(err)
	}
	if s := screenByID(t, m, "app.lit"); s.Status != StatusResolved || s.NameFrom != nil {
		t.Fatalf("literal screen: %+v", s)
	}
}
