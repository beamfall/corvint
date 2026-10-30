package appmap

import (
	"strings"
	"testing"
)

// gh1851Routed builds the gh1851 fixture with router and extra files and reports the state
// details at constRouter keyed by line, failing the test on a build error.
func gh1851Routed(t *testing.T, router string, extra map[string]string) (*Map, map[string]string) {
	t.Helper()
	files := gh1851Files()
	for p, text := range extra {
		files[p] = text
	}
	_, _, m, err := injectRepo(t, gh1851Scope, router, files)
	if err != nil {
		t.Fatal(err)
	}
	return m, stateDetails(m, "state")
}

// AMAP-V0-203 (#1851): the registering module's other imports block the table read only when the
// imported module can reach the table's declaration. The reported shape -- a service imported
// through a tsconfig paths alias to another package's `export *` barrel -- resolves, and a binding
// that does reach the table names why in its detail.
func TestAMAPV0203RegisteringImportReach(t *testing.T) {
	alias := map[string]string{
		"tsconfig.json":                               `{"compilerOptions": {"baseUrl": ".", "paths": {"@scope/common/*": ["libs/common/src/*"]}}}`,
		"libs/common/src/cookies/index.ts":            "export * from './cookie.service';\n",
		"libs/common/src/cookies/cookie.service.ts":   "export class ServiceA {\n  constructor(private http: any) {}\n}\n",
		"libs/common/src/cookies/cookie.constants.ts": "export const CookieNames = { A: 'a' };\n",
		"package.json":                                `{"name": "fixture-admin", "private": true, "dependencies": {"lodash": "4.0.0"}, "devDependencies": {"@playwright/test": "1.0.0"}}`,
	}
	cases := map[string]string{
		"service registered": "import { ServiceA } from '@scope/common/cookies';\n" + gh1851RegSrc +
			"angular.module('core.downgrade').service('ServiceA', ServiceA);\n",
		"unused import": "import { ServiceA } from '@scope/common/cookies';\n" + gh1851RegSrc,
		"declared package": "import { debounce } from 'lodash';\n" + gh1851RegSrc +
			"angular.module('core.downgrade').value('debounce', debounce);\n",
	}
	for name, reg := range cases {
		files := map[string]string{gh1851Reg: reg}
		for p, text := range alias {
			files[p] = text
		}
		m, _ := gh1851Routed(t, gh1851Router, files)
		for _, u := range m.Unknowns {
			if u.Kind == "di-constant" || u.Path == constRouter {
				t.Errorf("%s: unknown %+v", name, u)
			}
		}
		if s := m.screen(screenID(m.App, "app.club.add_ons")); s == nil || s.Status != StatusResolved {
			t.Fatalf("%s: state not resolved: %+v", name, m.Unknowns)
		}
	}

	// A passed binding whose module re-exports a file that imports the table may relay it.
	reg := "import { Widgets } from './widgets';\n" + gh1851RegSrc + "angular.module('w').component('w', Widgets);\n"
	m, _ := gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg,
		"app/core/widgets/index.ts":  "export * from './widget';\n",
		"app/core/widgets/widget.ts": "import { RoutesConstants } from '../routes';\nexport const Widgets = { r: RoutesConstants };\n"})
	want := "import-chain-bindings:" + gh1851Reg + ":1:Widgets:reaches-table:app/core/routes/index.ts:./routes.constants"
	if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
		t.Fatalf("relay detail %+v %q, want %s", u, d, want)
	}
	// A side-effect import loads the declaring file too.
	m, _ = gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg,
		"app/core/widgets/index.ts":  "export * from './widget';\nimport '../routes/routes.constants';\n",
		"app/core/widgets/widget.ts": "export const Widgets = {};\n"})
	want = "import-chain-bindings:" + gh1851Reg + ":1:Widgets:reaches-table:app/core/widgets/index.ts:../routes/routes.constants"
	if d := diDetails(m); len(d) != 1 || d[0] != want {
		t.Fatalf("side-effect import detail %q, want %s", d, want)
	}
}

// AMAP-V0-204 (#1851): router-side forms that name an injected member read through -- typed and
// generic parameters, a nested function's own injection, plain template reads and type
// assertions -- resolve; the forms that may still change or hide the binding refuse with a detail.
func TestAMAPV0204RouterInjectionForms(t *testing.T) {
	const head = "const R = ($stateProvider, StateConstants) => {\n"
	resolved := map[string]string{
		"typeof param": "const R = ($stateProvider: StateProvider, StateConstants: typeof RoutesConstants) => {\n" +
			"  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
		"generic param": "const R = ($stateProvider: Provider<Map<string, number>>, StateConstants: Record<string, string> | null) => {\n" +
			"  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
		"nested injection": head + "  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a',\n" +
			"    controller: ['StateConstants', function (StateConstants) { this.c = StateConstants.CLUB; }] });\n};\n",
		// A nested function whose own parameter shadows the name reads another binding.
		"annotated shadow": head + "  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a',\n" +
			"    controller: ['Other', function (StateConstants) { this.c = StateConstants.CLUB; }] });\n};\n",
		"template read": head + "  const t = `${StateConstants.CLUB}/x`;\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
		"as assertion":  head + "  $stateProvider.state(StateConstants.ADD_ONS as string, { url: '/a' });\n};\n",
		"non-null":      head + "  $stateProvider.state(StateConstants.ADD_ONS!, { url: '/a' });\n};\n",
	}
	for name, router := range resolved {
		m, details := gh1851Routed(t, router, nil)
		if len(details) != 0 || m.screen(screenID(m.App, "app.club.add_ons")) == nil {
			t.Errorf("%s: not resolved: %+v", name, m.Unknowns)
		}
	}

	refused := map[string]struct{ router, ref, detail string }{
		"opaque template": {head + "  const t = `${f(StateConstants)}`;\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
			constRouter + ":3", "router-template-use:" + constRouter + ":StateConstants"},
		"asserted write": {head + "  (StateConstants.ADD_ONS as any) = 'x';\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
			constRouter + ":3", "router-binding:" + constRouter + ":2:StateConstants"},
		// A tag receives its substitutions' values: it may write the table.
		"tagged template": {head + "  mutate`${StateConstants}`;\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n",
			constRouter + ":3", "router-template-use:" + constRouter + ":StateConstants"},
		"shadowed read": {head + "  [1].forEach(function (StateConstants) {\n    $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n  });\n};\n",
			constRouter + ":3", "shadowed:" + constRouter + ":2:StateConstants"},
	}
	// An annotation naming the constant twice binds a second, unchecked parameter to it.
	twice := "angular.module('p').provider('p', ['StateConstants', 'StateConstants', function (StateConstants, alias) {\n" +
		"  alias.ADD_ONS = 'changed';\n}]);\n" + head + "  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n"
	if m, details := gh1851Routed(t, twice, nil); details[constRouter+":5"] == "" {
		t.Errorf("duplicate annotation resolved: %+v", m.Unknowns)
	}
	for name, c := range refused {
		m, details := gh1851Routed(t, c.router, nil)
		if got, ok := details[c.ref]; !ok || got != c.detail {
			t.Errorf("%s: detail %q (%v), want %q; unknowns %+v", name, got, ok, c.detail, m.Unknowns)
		}
	}
}

// AMAP-V0-205 (#1851): every non-literal-name state unknown carries a detail. A name the reader
// does not follow names its form; a router-own constant names why it was not read.
func TestAMAPV0205NameFormDetail(t *testing.T) {
	const head = "const R = ($stateProvider, StateConstants) => {\n"
	cases := map[string]struct{ line, detail string }{
		"optional chain": {"$stateProvider.state(StateConstants?.ADD_ONS, { url: '/a' });", "name-expression:optional-chain"},
		"non-null chain": {"$stateProvider.state(StateConstants!.ADD_ONS, { url: '/a' });", "name-expression:non-null"},
		"concatenation":  {"$stateProvider.state(StateConstants.ADD_ONS + '.x', { url: '/a' });", "name-expression:member-+"},
		"identifier":     {"$stateProvider.state(name, { url: '/a' });", "name-expression:identifier"},
		"call":           {"$stateProvider.state(name(), { url: '/a' });", "name-expression:call"},
		"template":       {"$stateProvider.state(`a.${x}`, { url: '/a' });", "name-expression:template"},
		"no name":        {"$stateProvider.state({ url: '/a' });", "name-missing"},
	}
	for name, c := range cases {
		m, details := gh1851Routed(t, head+"  "+c.line+"\n};\n", nil)
		if got := details[constRouter+":2"]; got != c.detail {
			t.Errorf("%s: detail %q, want %q; unknowns %+v", name, got, c.detail, m.Unknowns)
		}
	}
	// Every non-literal-name unknown, in any of the cases above, carries some detail.
	m, _ := gh1851Routed(t, "const Local = load();\n$stateProvider.state(Local.A, { url: '/a' });\n", nil)
	for _, u := range m.Unknowns {
		if u.Reason == "non-literal-name" && (u.Detail == "" || !strings.HasPrefix(u.Detail, "const:Local:")) {
			t.Errorf("router-own constant detail %+v", u)
		}
	}
}
