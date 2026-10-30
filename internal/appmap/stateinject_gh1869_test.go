package appmap

import (
	"fmt"
	"strings"
	"testing"
)

// gh1869Resolved fails unless the gh1851 state app.club.add_ons resolved with no router or
// di-constant unknown.
func gh1869Resolved(t *testing.T, name string, m *Map, details map[string]string) {
	t.Helper()
	for _, u := range m.Unknowns {
		if u.Kind == "di-constant" {
			t.Errorf("%s: unknown %+v", name, u)
		}
	}
	if s := m.screen(screenID(m.App, "app.club.add_ons")); len(details) != 0 || s == nil {
		t.Errorf("%s: not resolved: %v %+v", name, details, m.Unknowns)
	}
}

// AMAP-V0-206 (#1869): a timer called with a function literal loads no module, so a service
// registered beside the table whose graph only schedules callbacks no longer blocks the read; a
// loader the reader cannot name still refuses, and the detail names its line, in the binding's
// reach reason or in the examined closure hop.
func TestAMAPV0206ChainLoaderDetail(t *testing.T) {
	const actions = "app/core/actions/integration.actions.ts"
	reg := "import { ServiceB } from './services/b';\n" + gh1851RegSrc + "angular.module('core.downgrade').service('ServiceB', ServiceB);\n"
	service := map[string]string{gh1851Reg: reg,
		"app/core/services/b.ts": "import { go } from '../actions/integration.actions';\nexport class ServiceB {\n  run(h) { go(h); }\n}\n"}
	with := func(files map[string]string, p, text string) map[string]string {
		out := map[string]string{p: text}
		for k, v := range files {
			out[k] = v
		}
		return out
	}
	callbacks := "export function go(h) {\n  setTimeout(() => h(), 0);\n  setInterval(function () { return 1; }, 5);\n" +
		"  window.setTimeout(async () => h(), 1);\n  setTimeout(x => x, 2);\n}\n"
	m, details := gh1851Routed(t, gh1851Router, with(service, actions, callbacks))
	gh1869Resolved(t, "timer callbacks", m, details)

	// A timer given a value the reader cannot see may evaluate a string.
	m, _ = gh1851Routed(t, gh1851Router, with(service, actions, "export function go(h) {\n  setTimeout(h, 0);\n}\n"))
	want := "import-chain-bindings:" + gh1851Reg + ":1:ServiceB:reach-module-not-literal:" + actions + ":2:setTimeout"
	if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
		t.Fatalf("binding detail %+v %q, want %s", u, d, want)
	}

	// A module the registering file loads only for its effects still bounds the examined closure.
	sideEffect := "import './polyfills';\n" + gh1851RegSrc
	m, details = gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: sideEffect, "app/core/polyfills.ts": "setInterval(() => tick(), 10);\n"})
	gh1869Resolved(t, "side-effect callback", m, details)
	m, _ = gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: sideEffect, "app/core/polyfills.ts": "\nsetInterval(tick, 10);\n"})
	want = "import-chain-examined:closure-module-not-literal:app/core/polyfills.ts:2:setInterval"
	if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
		t.Fatalf("closure detail %+v %q, want %s", u, d, want)
	}
}

// AMAP-V0-207 (#1869): an inline `config([...names, fn])` annotation binds each name to its
// parameter position, a trailing comma after the function included; a list whose arity or
// position does not match the parameter stays unknown with a detail that names it.
func TestAMAPV0207ConfigArrayAnnotation(t *testing.T) {
	const names = "angular.module('app').config([\n  '$stateProvider',\n  'IconConstants',\n  'StateConstants',\n"
	const params = "  ($stateProvider, IconConstants, StateConstants) =>"
	resolved := map[string]string{
		"expression":       names + params + "\n    $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' })\n]);\n",
		"expression comma": names + params + "\n    $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' }),\n]);\n",
		"block comma":      names + params + " {\n    $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n  },\n]);\n",
		"function comma":   names + "  function ($stateProvider, IconConstants, StateConstants) {\n    $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' });\n  },\n]);\n",
	}
	for name, router := range resolved {
		m, details := gh1851Routed(t, router, nil)
		gh1869Resolved(t, name, m, details)
	}
	const read = "\n  $stateProvider.state(StateConstants.ADD_ONS, { url: '/a' }),\n]);\n"
	refused := map[string]struct{ router, detail string }{
		"arity": {"angular.module('app').config(['$stateProvider', 'StateConstants', ($stateProvider, IconConstants, StateConstants) =>" + read,
			"annotation-arity:" + constRouter + ":1:StateConstants"},
		"position": {"angular.module('app').config(['$stateProvider', 'IconConstants', 'StateConstants', ($stateProvider, StateConstants, IconConstants) =>" + read,
			"annotation-position:" + constRouter + ":1:StateConstants"},
	}
	for name, c := range refused {
		m, details := gh1851Routed(t, c.router, nil)
		if got := details[constRouter+":2"]; got != c.detail {
			t.Errorf("%s: detail %q, want %q; unknowns %+v", name, got, c.detail, m.Unknowns)
		}
	}
}

// AMAP-V0-208 (#1869): a router's own table import follows `export * from` barrels to a bounded
// depth: through several levels, across a cycle and a diamond that reach one declaration. Two
// declarations, a chain past maxBarrelDepth, and a barrel that exports no such name stay unknown.
func TestAMAPV0208RouterBarrelStar(t *testing.T) {
	const router = "import { RoutesConstants } from './core/routes';\n$stateProvider.state(RoutesConstants.ADD_ONS, { url: '/a' });\n"
	const dir = "app/core/routes/"
	m, details := gh1851Routed(t, router, nil)
	gh1869Resolved(t, "one level", m, details)
	if a := m.screen(screenID(m.App, "app.club.add_ons")).NameFrom; len(a) != 3 || a[0].Path != gh1851Decl || a[1].Path != gh1851Index || a[2].Path != constRouter {
		t.Fatalf("name_from %+v", a)
	}
	resolved := map[string]map[string]string{
		"two levels": {gh1851Index: "export * from './inner';\n", dir + "inner.ts": gh1851Star},
		"cycle":      {gh1851Index: "export * from './a';\n" + gh1851Star, dir + "a.ts": "export * from './index';\n"},
		"diamond": {gh1851Index: "export * from './a';\nexport * from './b';\n",
			dir + "a.ts": gh1851Star, dir + "b.ts": gh1851Star},
	}
	for name, files := range resolved {
		m, details := gh1851Routed(t, router, files)
		gh1869Resolved(t, name, m, details)
	}

	deep := map[string]string{gh1851Index: "export * from './b1';\n", dir + "b8.ts": gh1851Star}
	for i := 1; i < maxBarrelDepth; i++ {
		deep[fmt.Sprintf("%sb%d.ts", dir, i)] = fmt.Sprintf("export * from './b%d';\n", i+1)
	}
	refused := map[string]struct {
		files  map[string]string
		detail string
	}{
		"ambiguous": {map[string]string{gh1851Index: gh1851Star + "export * from './other';\n", dir + "other.ts": "export const RoutesConstants = { ADD_ONS: 'x' };\n"},
			"ambiguous-barrel:ambiguous-reexport:" + dir + "other.ts," + gh1851Decl},
		"too deep": {deep, "barrel-depth-exceeded:barrel-depth:" + dir + "b7.ts:1:" + dir + "b8.ts"},
		"not exported": {map[string]string{gh1851Index: "export * from './other';\n", dir + "other.ts": "export const Other = { A: 'a' };\n"},
			"identifier-not-found:not-exported:" + gh1851Index + ":RoutesConstants"},
	}
	for name, c := range refused {
		m, details := gh1851Routed(t, router, c.files)
		if got, want := details[constRouter+":2"], "const:RoutesConstants:"+c.detail; got != want {
			t.Errorf("%s: detail %q, want %q; unknowns %+v", name, got, want, m.Unknowns)
		}
	}
}

// AMAP-V0-206 (#1869 review): a function expression called in the argument hands the timer its
// result, which may be a string, so only a whole function followed by `,` or `)` is a callback;
// an immediately invoked one, or one with a return type, stays a loader.
func TestAMAPV0206TimerCallbackIIFE(t *testing.T) {
	sideEffect := "import './polyfills';\n" + gh1851RegSrc
	for _, poly := range []string{
		"\nsetTimeout(function tick() { run(); }, 10);\n",
		"\nsetInterval(async function* () { yield 1; }, 10);\n",
	} {
		m, details := gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: sideEffect, "app/core/polyfills.ts": poly})
		gh1869Resolved(t, poly, m, details)
	}
	for _, poly := range []string{
		"\nsetTimeout(function () { return \"loadExtraCode()\"; }(), 0);\n",
		"\nsetTimeout(async function named() { return 1; }(), 0);\n",
		"\nsetTimeout(function (): string { return \"x\"; }, 0);\n",
	} {
		m, _ := gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: sideEffect, "app/core/polyfills.ts": poly})
		want := "import-chain-examined:closure-module-not-literal:app/core/polyfills.ts:2:setTimeout"
		if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
			t.Errorf("%q: detail %+v %q, want %s", poly, u, d, want)
		}
	}
}

// AMAP-V0-208 (#1869 review): a barrel on the current path is a cycle only for the same export
// name, so a re-export chain that returns to a barrel under another name is still followed.
func TestAMAPV0208BarrelCycleIdentity(t *testing.T) {
	const router = "import { RoutesConstants } from './core/routes';\n$stateProvider.state(RoutesConstants.ADD_ONS, { url: '/a' });\n"
	const dir = "app/core/routes/"
	files := map[string]string{
		gh1851Index:    "export * from './a';\nexport * from './one';\n",
		dir + "a.ts":   "export { U as RoutesConstants } from './b';\nexport { RoutesConstants as V } from './two';\n",
		dir + "b.ts":   "export { V as U } from './a';\n",
		dir + "one.ts": "export const RoutesConstants = { ADD_ONS: 'one' };\n",
		dir + "two.ts": "export const RoutesConstants = { ADD_ONS: 'two' };\n",
	}
	m, details := gh1851Routed(t, router, files)
	want := "const:RoutesConstants:ambiguous-barrel:ambiguous-reexport:" + dir + "one.ts," + dir + "two.ts"
	if got := details[constRouter+":2"]; got != want {
		t.Fatalf("detail %q, want %q; unknowns %+v", got, want, m.Unknowns)
	}
}

// AMAP-V0-208 (#1869 review): a duplicate `export *` edge is followed once and each (barrel,
// name, depth) once, so a lattice of repeated stars resolves in linear work; a walk past
// maxBarrelVisits barrels refuses with its named hop instead of running on.
func TestAMAPV0208BarrelVisitBudget(t *testing.T) {
	const router = "import { RoutesConstants } from './core/routes';\n$stateProvider.state(RoutesConstants.ADD_ONS, { url: '/a' });\n"
	const dir = "app/core/routes/"
	lattice := map[string]string{gh1851Index: strings.Repeat("export * from './b1';\n", 16), dir + "b7.ts": gh1851Star}
	for i := 1; i < 7; i++ {
		lattice[fmt.Sprintf("%sb%d.ts", dir, i)] = strings.Repeat(fmt.Sprintf("export * from './b%d';\n", i+1), 16)
	}
	m, details := gh1851Routed(t, router, lattice)
	gh1869Resolved(t, "duplicate stars", m, details)

	wide := map[string]string{}
	stars := make([]string, 0, maxBarrelVisits+1)
	for i := 0; i <= maxBarrelVisits; i++ {
		stars = append(stars, fmt.Sprintf("export * from './m%d';\n", i))
		wide[fmt.Sprintf("%sm%d.ts", dir, i)] = gh1851Star
	}
	wide[gh1851Index] = strings.Join(stars, "")
	m, details = gh1851Routed(t, router, wide)
	want := fmt.Sprintf("const:RoutesConstants:barrel-depth-exceeded:barrel-visit-limit:%s:%d", gh1851Index, maxBarrelVisits)
	if got := details[constRouter+":2"]; got != want {
		t.Fatalf("detail %q, want %q; unknowns %+v", got, want, m.Unknowns)
	}
}
