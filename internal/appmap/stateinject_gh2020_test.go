package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-425, AMAP-V0-426 (#2020): an array binding pattern that is a whole parameter is no array
// value, and a typed const passed whole to a proved local method parameter (AMAP-V0-424 (e)) reaches
// only element-wise uses. Each helper is called so the AMAP-V0-420 dormant-function proof cannot hide it.
func TestAMAPV0425ParameterPatternsAndMethodConsts(t *testing.T) {
	for name, c := range map[string][2]string{
		"entries pattern":  {`export function each(o) { Object.entries(o).forEach(([, v]) => v); } each(data);`, ""},
		"nested pattern":   {`export const pick = (k, [a, [b, ...rest]]) => a === b; pick(1, data);`, ""},
		"function pattern": {`export function first([head, , third]) { return head === third; } first(data);`, ""},
		"async pattern":    {`export const g = async ([a, b]) => a === b; g(data);`, ""},
		"typed const method": {"export class T {\n  update(t, kinds) { for (const k of kinds) { if (k === t) return true } return kinds.includes(t); }\n" +
			"  go(t, rules) { const ns: Kind[] = [Kind.A, ...rules.map(r => r.name)]; return this.update(t, ns); }\n}\nnew T().go(1, data);\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			m, details := gh1994Routed(t, c[0], c[1])
			gh1869Resolved(t, name, m, details)
		})
	}
}

// AMAP-V0-427 (#2020): the remaining reported forms, and every injector-facing form, keep the named
// blocker. Each refusal has a counterexample in which the array, or a sequence rebuilt from its
// elements, can reach an injector outside the proved uses.
func TestAMAPV0427ReportedFormsStayRefused(t *testing.T) {
	for name, c := range map[string][2]string{
		// 1. combine([[dep], [fn]]) returns [[dep, fn]]: the product builds an annotation from its input.
		"parameter copy product": {"export function combine(xs) {\n  xs = [...xs]\n  let out = [...xs.shift()].map(v => [v])\n" +
			"  for (const ys of xs) { const next = []; out.forEach(r => ys.forEach(y => next.push([...r, y]))); out = next }\n  return out\n}\ncombine(data);\n", ""},
		// 2. A returned copy reaches unexamined uses at its callers: run(svc.sorted(deps)).
		"sorted copy returned": {`export class Svc { sorted(list) { return [...list].sort((a, b) => a - b) } } new Svc().sorted(data);`, ""},
		// 3. [...a, ...b] is the annotation builder itself.
		"spread rebuild returned":   {`export function join(a, b) { return [...a, ...b] } join(data, data);`, ""},
		"spread rebuild reassigned": {`export class R { walk(n, acc) { acc = [...acc, n]; return n.next ? this.walk(n.next, acc) : acc } } new R().walk(data, []);`, ""},
		// 4. The parameter escapes into a constructed object, whose consumer may hand it to an injector.
		"method parameter into new": {"export class Svc {\n  add(t, n) { return this.build([...t.children, n]) }\n" +
			"  build(children = []) { return new Node({ children }) }\n}\nnew Svc().add(data, 1);\n", ""},
		"method parameter into object": {"export class Svc {\n  add(t, n) { return this.build([...t.children, n]) }\n" +
			"  build(children) { return { children } }\n}\nnew Svc().add(data, 1);\n", ""},
		// 5. A typed const passed to a local method whose parameter escapes, or to an injector-facing name.
		"typed const method escapes": {"export class T {\n  update(t, ns) { return { ...t, rules: ns } }\n" +
			"  go(t) { const ns: Kind[] = [Kind.A, ...t.rules.map(r => r.name)]; return this.update(t, ns); }\n}\nnew T().go(data);\n", ""},
		"typed const injector method": {"export class T {\n  invoke(ns) { return ns.length }\n" +
			"  go(t) { const ns: Kind[] = [Kind.A, ...t.rules]; return this.invoke(ns); }\n}\nnew T().go(data);\n", ""},
		// The parameter's spread copy ends in a plain element, so the caller can pop it and register the rebuilt list.
		"typed const spread rebuild": {"export class T {\n  update(nodes) { return [...nodes, 0]; }\n" +
			"  go(input) { const held = [...input]; return this.update(held); }\n}\nconst annotation = new T().go(data);\nannotation.pop();\nregister(annotation);\n", ""},
		"literal spread rebuild": {"export class T {\n  update(nodes) { return [...nodes, 0]; }\n" +
			"  go(input) { return this.update([...input]); }\n}\nconst annotation = new T().go(data);\nannotation.pop();\nregister(annotation);\n", ""},
		"typed const other receiver": {"export class T {\n  update(ns) { return ns.length }\n" +
			"  go(o, t) { const ns: Kind[] = [Kind.A, ...t.rules]; return o.update(ns); }\n}\nnew T().go(data, data);\n", ""},
		// 6. A held dependency's method is outside the closure; it may be a service calling $injector.invoke.
		"held dependency call": {"export class Svc {\n  constructor(dep) { this.dep = dep }\n  a() { return 1 }\n  b() { return 2 }\n" +
			"  go() { return this.dep.all([this.a(), this.b()]) }\n}\nnew Svc(data).go();\n", ""},
		// 7. fmt may push each element in order, rebuilding the literal's sequence.
		"destructured map": {`export function f(m) { const [from, to] = [m.from, m.to].map(v => fmt(v)); return from === to } f(data);`, ""},
		// Injector-facing forms.
		"provider deps":           {`export const P = { provide: X, useFactory: f, deps: [A, B] };`, ""},
		"module provider deps":    {"@NgModule({ providers: [{ provide: X, useFactory: f, deps: [A, B] }] })\nexport class M {}\n", ""},
		"computed $inject":        {`export function F(a) {} F.$inject = [...deps];`, ""},
		"method decorator array":  {`export class C { @Hook([...deps, fn]) m() {} }`, ""},
		"pattern default":         {`export const g = ([a] = [...deps, fn]) => a; g();`, ""},
		"parenthesized argument":  {`register(([dep, fn]));`, ""},
		"typed parameter pattern": {`export const g = ([a, b]: [A, B]) => a; g(data);`, ""},
		"rest parameter pattern":  {`export const g = (...[a, fn]) => a; g(data);`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, c[0], c[1])
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), ":computed-annotation") {
				t.Fatalf("unproved reach resolved: %+v", m.Unknowns)
			}
		})
	}
}
