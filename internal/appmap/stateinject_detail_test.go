package appmap

import (
	"context"
	"strings"
	"testing"
)

// The #1805 / #1851 shape: a registering module imports the enum through a directory specifier
// (`./routes` -> routes/index.ts), whose only statement is `export * from './routes.constants'`;
// the di_constants scope names the registering file and the `routes` directory.
const (
	gh1851Reg    = "app/core/core-downgrade.module.ts"
	gh1851Index  = "app/core/routes/index.ts"
	gh1851Decl   = "app/core/routes/routes.constants.ts"
	gh1851Scope  = `["app/core/core-downgrade.module.ts", "app/core/routes"]`
	gh1851RegSrc = "import { RoutesConstants } from './routes'\n" +
		"angular.module('core.downgrade', [])\n  .constant('StateConstants', RoutesConstants);\n"
	gh1851Star   = "export * from './routes.constants'\n"
	gh1851Enum   = "export enum RoutesConstants {\n  CLUB = 'app.club',\n  ADD_ONS = 'app.club.add_ons',\n}\n"
	gh1851Router = "const AddOnsRoutes = ($stateProvider, IconConstants, StateConstants) => {\n" +
		"  $stateProvider.state(StateConstants.CLUB, { url: '/club' });\n" +
		"  $stateProvider.state(StateConstants.ADD_ONS, { url: '/add-ons' });\n};\n" +
		"AddOnsRoutes.$inject = ['$stateProvider', 'IconConstants', 'StateConstants'];\nexport default AddOnsRoutes;\n"
)

func gh1851Files() map[string]string {
	return map[string]string{gh1851Reg: gh1851RegSrc, gh1851Index: gh1851Star, gh1851Decl: gh1851Enum}
}

// stateDetails maps each unknown of kind (state or screen) read from path to its detail.
func stateDetails(m *Map, kind string) map[string]string {
	out := map[string]string{}
	for _, u := range m.Unknowns {
		if u.Kind == kind && (u.Path == constRouter || kind == "screen") {
			out[u.Ref] = u.Detail
		}
	}
	return out
}

// AMAP-V0-201 (V1-2699, #1805, #1851): the reported two-repository build -- manifest in the
// aliased e2e root, a directory specifier, a one-level `export *` barrel, a directory
// di_constants entry and a router injecting a second constant -- resolves every state, anchored
// through the barrel's re-export.
func TestAMAPV0201DirectoryBarrelTwoRepo(t *testing.T) {
	app, _, e2e, _ := twoRoots(t, true)
	manifest := readText(t, e2e, "appmap.json")
	manifest = strings.Replace(manifest, `"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}]`,
		`"routers": [{"path": "app/routes.js", "dialect": "ui-router-states/0"}, {"path": "`+constRouter+`", "dialect": "ui-router-states/0"}]`, 1)
	manifest = strings.Replace(manifest, `"flows": "flows",`, `"flows": "flows",`+"\n  \"di_constants\": "+gh1851Scope+",", 1)
	if !strings.Contains(manifest, "di_constants") || !strings.Contains(manifest, constRouter) {
		t.Fatal("manifest not rewritten")
	}
	writeFile(t, e2e, "appmap.json", manifest)
	commitAll(t, e2e, "inject")
	writeFile(t, app, constRouter, gh1851Router)
	for p, text := range gh1851Files() {
		writeFile(t, app, p, text)
	}
	rev := commitAll(t, app, "downgrade module")
	m, err := BuildRoots(context.Background(), app, "appmap.json", rev, Roots{Repos: map[string]string{"e2e": e2e}, ManifestRepo: "e2e"})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range m.Unknowns {
		if u.Kind == "di-constant" || u.Path == constRouter {
			t.Errorf("unknown %+v", u)
		}
	}
	s := m.screen(screenID(m.App, "app.club.add_ons"))
	if s == nil || s.Status != StatusResolved || m.screen(screenID(m.App, "app.club")) == nil {
		t.Fatalf("states not resolved: %+v", m.Unknowns)
	}
	if a := s.NameFrom; len(a) != 5 || a[0].Path != gh1851Decl || a[1].Path != gh1851Index || a[2].Path != gh1851Reg || a[4].Path != constRouter {
		t.Fatalf("name_from %+v", a)
	}
}

// AMAP-V0-201 AMAP-V0-202 (V1-2699, #1851): every refusal on the injection path names its hop. The
// di-constant unknown carries the import, barrel or binding that failed, and the router's state
// (or screen, for a parent) unknown carries why its injected read refused.
func TestAMAPV0202InjectionRefusalDetail(t *testing.T) {
	const routerOne = "const R = ($stateProvider, StateConstants) => {\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n"
	stateRef := constRouter + ":2"
	cases := map[string]struct {
		router string
		files  map[string]string
		reason string // the di-constant unknown's reason; "" when none is carried
		di     string // its detail
		state  string // the state unknown's detail at stateRef
	}{
		"directory unresolved": {routerOne, map[string]string{gh1851Reg: strings.Replace(gh1851RegSrc, "'./routes'", "'./route'", 1)},
			"out-of-scope", "import-unresolved:" + gh1851Reg + ":1:./route", ""},
		"barrel unlisted": {routerOne, map[string]string{gh1851Index: "export const \\u0061b = 1;\n" + gh1851Star},
			"ambiguous-barrel", "barrel-unlisted:" + gh1851Index, ""},
		"star unresolved": {routerOne, map[string]string{gh1851Index: gh1851Star + "export * from './gone'\n"},
			"out-of-scope", "import-unresolved:" + gh1851Index + ":2:./gone", ""},
		"barrel depth": {routerOne, map[string]string{gh1851Decl: "export * from './deeper'\n", "app/core/routes/deeper.ts": gh1851Enum},
			"barrel-depth-exceeded", "barrel-depth:" + gh1851Index + ":1:" + gh1851Decl, ""},
		"function export shadows": {routerOne, map[string]string{gh1851Index: "export function RoutesConstants() {}\n" + gh1851Star},
			"identifier-not-found", "not-a-table:" + gh1851Index + ":RoutesConstants", ""},
		"local export shadows": {routerOne, map[string]string{gh1851Index: "const X = { A: 'a' };\nexport { X as RoutesConstants };\n" + gh1851Star},
			"identifier-not-found", "barrel-local-export:" + gh1851Index + ":RoutesConstants", ""},
		"no candidate": {routerOne, map[string]string{gh1851Index: "export * from './other'\n", "app/core/routes/other.ts": "export const Other = { A: 'a' };\n"},
			"identifier-not-found", "barrel-no-candidate:" + gh1851Index + ":RoutesConstants", ""},
		"chain binding": {routerOne, map[string]string{gh1851Reg: "import { Widgets } from './widgets';\n" + gh1851RegSrc + "angular.module('w').component('w', Widgets);\n",
			"app/core/widgets/index.ts": "export * from './widget';\n", "app/core/widgets/widget.ts": "import { RoutesConstants } from '../routes';\nexport const Widgets = { r: RoutesConstants };\n"},
			"not-read-whole", "import-chain-bindings:" + gh1851Reg + ":1:Widgets:reaches-table:app/core/routes/index.ts:./routes.constants", ""},
		"router binding": {"const R = ($stateProvider, StateConstants) => {\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n  register(StateConstants);\n};\n", nil,
			"", "", "router-binding:" + constRouter + ":3:StateConstants"},
		"scope unreadable": {routerOne, map[string]string{"app/core/routes/broken.ts": "angular.module('x').constant('A', {\n"},
			"", "", "scope-unreadable:app/core/routes/broken.ts:1"},
		"two registrations": {routerOne, map[string]string{"app/core/routes/again.ts": "angular.module('y').constant('StateConstants', { ADD_ONS: 'x' });\n"},
			"", "", "ambiguous-registration:StateConstants:" + gh1851Reg + ":3," + "app/core/routes/again.ts:1"},
		"global not injected": {"const R = ($stateProvider, StateConstants) => {\n  $stateProvider.state(window.ADD_ONS, { url: '/a' });\n};\n", nil,
			"", "", "not-injected:" + constRouter + ":window"},
		"no registration": {routerOne, map[string]string{gh1851Reg: strings.Replace(gh1851RegSrc, "'StateConstants'", "'OtherNames'", 1)},
			"", "", "no-registration:StateConstants"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			files := gh1851Files()
			for p, text := range c.files {
				files[p] = text
			}
			_, _, m, err := injectRepo(t, gh1851Scope, c.router, files)
			if err != nil {
				t.Fatal(err)
			}
			var di []Unknown
			for _, u := range m.Unknowns {
				if u.Kind == "di-constant" {
					di = append(di, u)
				}
			}
			want := c.state
			if c.reason != "" {
				if len(di) != 1 || di[0].Reason != c.reason || di[0].Detail != c.di {
					t.Fatalf("di-constant %+v, want %s %s", di, c.reason, c.di)
				}
				want = "di-constant:StateConstants:" + c.reason + ":" + c.di
			} else if len(di) != 0 {
				t.Fatalf("di-constant %+v", di)
			}
			if got, ok := stateDetails(m, "state")[stateRef]; !ok || got != want {
				t.Fatalf("state detail %q (%v), want %q; unknowns %+v", got, ok, want, m.Unknowns)
			}
		})
	}

	// A member read outside the injectable function, and a parent read through injection, name
	// their refusal too.
	router := "const R = ($stateProvider, StateConstants) => {\n  $stateProvider.state('kid', { parent: StateConstants.MISSING, url: '/k' });\n};\n" +
		"$stateProvider.state(StateConstants.CLUB, { url: '/c' });\n"
	_, _, m, err := injectRepo(t, gh1851Scope, router, gh1851Files())
	if err != nil {
		t.Fatal(err)
	}
	if got := stateDetails(m, "state")[constRouter+":4"]; got != "outside-injectable:"+constRouter+":4:StateConstants" {
		t.Fatalf("outside detail %q: %+v", got, m.Unknowns)
	}
	if got := stateDetails(m, "screen")[screenID(m.App, "kid")]; got != "di-constant:StateConstants:member-not-found" {
		t.Fatalf("parent detail %q: %+v", got, m.Unknowns)
	}
	// The stats projection's example anchors keep the detail the comment could not see.
	out, err := ProjectStats(context.Background(), m, Options{Full: true})
	if err != nil || !strings.Contains(string(out), `"ref":"screen:admin:kid","detail":"di-constant:StateConstants:member-not-found"`) {
		t.Fatalf("stats detail missing (%v): %s", err, out)
	}
}
