package appmap

import "testing"

// AMAP-V0-340 (#1932): a reached service that imports a template, stylesheet or data module
// (default, named or side-effect) loads no code, so the constant registered beside it stays
// literal; a reached script the reader cannot read still refuses.
func TestAMAPV0340AssetImportReach(t *testing.T) {
	const svc = "app/core/services/b.ts"
	reg := "import { ServiceB } from './services/b';\n" + gh1851RegSrc + "angular.module('core.downgrade').service('ServiceB', ServiceB);\n"
	for name, c := range map[string]struct{ imp, asset string }{
		"html default": {"import tpl from './t.html';\n", "app/core/services/t.html"},
		"css side":     {"import './b.css';\n", "app/core/services/b.css"},
		"svg named":    {"import { ReactComponent as Icon } from './i.svg';\n", "app/core/services/i.svg"},
		"json default": {"import data from './d.json';\n", "app/core/services/d.json"},
	} {
		m, details := gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg, c.asset: "<div>{{ x }}</div>\n",
			svc: c.imp + "export class ServiceB {\n  del(x) { return tpl; }\n}\n"})
		gh1869Resolved(t, name, m, details)
	}
	m, _ := gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg, "app/core/services/u.ts": "export { a } from './gone';\nexport * as \"q\" from './x';\n",
		svc: "import { a } from './u';\nexport class ServiceB {\n  del(x) { return a; }\n}\n"})
	if u := diUnknown(m); len(u) != 1 {
		t.Fatalf("unread script: %+v", m.Unknowns)
	}
}

// AMAP-V0-341 (#1932): a named router function registered by an inline `.config([...names, Fn])`
// annotation binds each name to its parameter position, like `Fn.$inject`; an annotation of
// another arity or order stays unknown with a detail that names it.
func TestAMAPV0341ConfigArrayIdentifier(t *testing.T) {
	const fn = "const Routes = ($stateProvider, $urlServiceProvider, IconConstants, StateConstants) => {\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n};\n"
	for name, reg := range map[string]string{
		"array":       "angular.module('app').config(['$stateProvider', '$urlServiceProvider', 'IconConstants', 'StateConstants', Routes]);\n",
		"array comma": "angular.module('app').config([\n  '$stateProvider',\n  '$urlServiceProvider',\n  'IconConstants',\n  'StateConstants',\n  Routes,\n]);\n",
		"function":    "",
	} {
		router := fn + reg
		if reg == "" {
			router = "function Routes($stateProvider, $urlServiceProvider, IconConstants, StateConstants) {\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n}\n" +
				"angular.module('app').config(['$stateProvider', '$urlServiceProvider', 'IconConstants', 'StateConstants', Routes]);\n"
		}
		m, details := gh1851Routed(t, router, nil)
		gh1869Resolved(t, name, m, details)
	}
	for name, c := range map[string]struct{ reg, detail string }{
		"arity":    {"angular.module('app').config(['$stateProvider', 'StateConstants', Routes]);\n", "annotation-arity:" + constRouter + ":1:StateConstants"},
		"position": {"angular.module('app').config(['$stateProvider', '$urlServiceProvider', 'StateConstants', 'IconConstants', Routes]);\n", "annotation-position:" + constRouter + ":1:StateConstants"},
		"two annotations": {"Routes.$inject = ['$stateProvider', '$urlServiceProvider', 'IconConstants', 'StateConstants'];\n" +
			"angular.module('app').config(['$stateProvider', '$urlServiceProvider', 'StateConstants', 'IconConstants', Routes]);\n", "annotation-position:" + constRouter + ":1:StateConstants"},
		"other use": {"angular.module('app').config(['$stateProvider', '$urlServiceProvider', 'IconConstants', 'StateConstants', Routes]);\nwindow.r = Routes;\n", "not-injected:" + constRouter + ":StateConstants"},
	} {
		m, details := gh1851Routed(t, fn+c.reg, nil)
		if got := details[constRouter+":2"]; got != c.detail {
			t.Errorf("%s: detail %q, want %q; unknowns %+v", name, got, c.detail, m.Unknowns)
		}
	}
}
