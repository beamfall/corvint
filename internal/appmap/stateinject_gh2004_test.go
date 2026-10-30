package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-423 (#2004): a class body's `static $inject = [exact strings]` is that class's
// annotation, read with the same checks as `Name.$inject = [...]`.
func TestAMAPV0423StaticInjectField(t *testing.T) {
	for name, src := range map[string]string{
		"reported field":  "export class ThingActions {\n  static $inject = ['Action', 'Store', 'Model']\n  constructor(private Action, private Store, private Model) {}\n}\n",
		"semicolon":       `export class A { static $inject = ['a', 'b']; constructor(a, b) {} }`,
		"closing brace":   `export class A { static $inject = ['a'] }`,
		"readonly typed":  "export class A extends B.Base<T> {\n  static readonly $inject: string[] = ['a']\n  constructor(a) { super() }\n}\n",
		"default export":  "export default class A {\n  static $inject = ['a']\n}\n",
		"two classes":     "export class A { static $inject = ['a'] }\nexport class B { static $inject = ['b'] }\n",
		"with assignment": "function F(a) { return a }\nF.$inject = ['a'];\nexport class A { static $inject = ['b'] }\n",
	} {
		t.Run(name, func(t *testing.T) {
			m, details := gh1994Routed(t, src, "")
			gh1869Resolved(t, name, m, details)
		})
	}
}

// AMAP-V0-423: a static list naming the constant, a computed or duplicated list, a later write,
// an instance field, or a class the reader cannot name keeps the file blocking.
func TestAMAPV0423StaticInjectRefusals(t *testing.T) {
	for name, src := range map[string]string{
		"names constant":   `export class A { static $inject = ['StateConstants']; constructor(s) { s.CLUB = 'x' } }`,
		"computed":         `export class A { static $inject = [dep] }`,
		"spread":           `export class A { static $inject = ['a', ...deps] }`,
		"substitution":     "export class A { static $inject = [`${dep}`] }",
		"duplicate":        `export class A { static $inject = ['a']; static $inject = ['b'] }`,
		"written later":    "export class A { static $inject = ['a'] }\nA.$inject = ['b'];\n",
		"pushed later":     "export class A { static $inject = ['a'] }\nA.$inject.push(dep);\n",
		"constructor push": `export class A { static $inject = ['a']; f() { this.constructor.$inject.push(dep) } }`,
		"instance field":   `export class A { $inject = ['a'] }`,
		"class expression": `export const A = class { static $inject = ['a'] }`,
		"nested class":     `export function f() { class A { static $inject = ['a'] } return A }`,
		"trailing call":    `export class A { static $inject = ['a'].concat(deps) }`,
		"next-line call":   "export class A {\n  static $inject = ['a']\n  .concat(deps)\n}\n",
		"function form":    "export function F(a) { return a }\nF.$inject = ['a']\n(deps);\n",
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, src, "")
			d := strings.Join(diDetails(m), ";")
			if !strings.Contains(d, "closure-not-quiet:app/core/util.ts:") || !strings.Contains(d, ":$inject") && !strings.Contains(d, ":string") {
				t.Fatalf("unreadable static annotation resolved: %+v", m.Unknowns)
			}
		})
	}
}

// AMAP-V0-424 (#2005): the reported real forms of non-injector array uses leave the file quiet.
// Each helper is called (`f(data)`) so the AMAP-V0-420 dormant-function proof cannot hide it.
func TestAMAPV0424ElementWiseArrayUses(t *testing.T) {
	for name, c := range map[string][2]string{
		"minimal repro": {`export function f(xs) { xs = [...xs]; const h = xs.shift(); for (const v of xs) {} return [1, 2].includes(h) } f(data);`, ""},
		"decorator spread": {`export const PIPES = [PipeA, PipeB];`,
			"import { PIPES } from './util';\n@NgModule({ declarations: [...PIPES], exports: [...PIPES] })\nexport class M {}\n"},
		"local spread":       {"const PIPES = [PipeA, PipeB];\nexport const ALL = { list: [...PIPES, 'x'] };\n", ""},
		"copy element calls": {`export function f(xs, k) { xs = [...xs]; let n = 0; xs.forEach(v => { if (typeof v === 'string') n++; }); return xs.some((v, i) => v === k) || xs.indexOf(k) > n; } f(data);`, ""},
		"copy scalar loop":   {`export function f(xs, k) { xs = [...xs]; for (const v of xs) { if (v !== k) continue; return true; } return xs.find(w => k == w) === undefined; } f(data);`, ""},
		"local push":         {`export function f(r, v) { const out = []; out.push([...r, v]); return out.length; } f(data);`, ""},
		"literal includes":   {`export class K { is() { return [A.X, A.Y].includes(this.type); } }`, ""},
		"typed const":        {`export function f(list) { const names: Kind[] = [Kind.A, ...list.map(x => x.n)]; return names.includes(list[0]); } f(data);`, ""},
		"generic const":      {`export function f(list) { const names: Array<Kind> = [Kind.A, ...list]; return names.join(','); } f(data);`, ""},
		"method argument": {"export class T {\n  build(nodes: Node[]) { for (const n of nodes) { if (n === null) continue } return nodes.length; }\n" +
			"  add(a, node) { return this.build([...a.children, node]); }\n}\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			m, details := gh1994Routed(t, c[0], c[1])
			gh1869Resolved(t, name, m, details)
		})
	}
}

// AMAP-V0-424: an array (or its copy, element or derived array) that may reach an injector-facing
// or unknown call keeps blocking.
func TestAMAPV0424ElementWiseRefusals(t *testing.T) {
	for name, c := range map[string][2]string{
		"factory":          {`angular.module('app').factory('x', [...deps, fn]);`, ""},
		"invoke":           {`$injector.invoke([...deps, fn]);`, ""},
		"copy escapes":     {`export function f(xs) { xs = [...xs]; xs.shift(); register(xs); } f(data);`, ""},
		"copy callback":    {`export function f(xs) { xs = [...xs]; return xs.forEach(g); } f(data);`, ""},
		"copy third param": {`export function f(xs) { xs = [...xs]; xs.forEach((v, i, a) => register(a)); } f(data);`, ""},
		"copy spread call": {`export function f(xs) { xs = [...xs]; register(...xs); } f(data);`, ""},
		"identity map":     {`angular.module('app').run([...deps].map(x => x));`, ""},
		"concat":           {`register([...deps].concat(fn));`, ""},
		"decorator call":   {`export const PIPES = [dep, fn];`, "import { PIPES } from './util';\nregister([...PIPES]);\n"},
		"push returned":    {`export function f(r, v) { const out = []; out.push([...r, v]); return out; } f(data);`, ""},
		"push iterated":    {`export function f(r, v) { const out = []; out.push([...r, v]); for (const row of out) register(row); } f(data);`, ""},
		"push indexed":     {`export function f(r, v) { const out = []; out.push([...r, v]); register(out[0]); } f(data);`, ""},
		"typed passed":     {`export function f() { const names: Kind[] = [...deps, fn]; register(names); } f(data);`, ""},
		"typed exported":   {`export const names: Kind[] = [...deps, fn];`, "import { names } from './util';\nangular.module('app').run(names);\n"},
		"method escapes":   {`export class T { build(nodes) { register(nodes); } add(a, n) { this.build([...a, n]); } }`, ""},
		"method arguments": {`export class T { build(nodes) { register(arguments[0]); } add(a, n) { this.build([...a, n]); } }`, ""},
		"method override": {"export class T { build(nodes) { return nodes.length; } add(a, n) { this.build([...a, n]); } }\n" +
			"export class U extends T { build(nodes) { register(nodes); } }\n", ""},
		"method elsewhere": {`export class T { build(nodes) { return nodes.length; } add(a, n) { this.build([...a, n]); } }`,
			"export const o = { build: register };\n"},
		"method sink name": {`export class T { run(nodes) { return nodes.length; } add(a, n) { this.run([...a, n]); } }`, ""},
		"method this":      {`export class T { build(nodes) { return nodes.length; } add(a, n) { return function () { return this.build([...a, n]); }; } }`, ""},
		"method missing":   {`export class T { add(a, n) { this.build([...a, n]); } }`, ""},
		"method on other":  {`export class T { build(nodes) { return nodes.length; } add(o, a, n) { o.build([...a, n]); } }`, ""},
		// Codex review of 1df6e2743: an element that is stored, passed or returned can rebuild the list.
		"for-of push":      {`export function f(xs) { xs = [...xs]; const out = []; for (const v of xs) out.push(v); register(out); } f(data);`, ""},
		"for-of pattern":   {`export function f(xs) { xs = [...xs]; for (const [v] of xs) register(v); } f(data);`, ""},
		"for-of returns":   {`export function f(xs) { xs = [...xs]; for (const v of xs) { return v; } } register(f(data));`, ""},
		"forEach push":     {`export function f(xs) { xs = [...xs]; const out = []; xs.forEach(v => out.push(v)); register(out); } f(data);`, ""},
		"forEach stores":   {`export function f(xs) { xs = [...xs]; let s; xs.forEach(v => { s = v; }); register(s); } f(data);`, ""},
		"find escapes":     {`export function f(xs) { xs = [...xs]; register(xs.find(v => v)); } f(data);`, ""},
		"find bound":       {`export function f(xs) { xs = [...xs]; const h = xs.find(v => v === 1); register(h); } f(data);`, ""},
		"shift escapes":    {`export function f(xs) { xs = [...xs]; const h = xs.shift(); register([h]); } f(data);`, ""},
		"shift spread map": {`export function f(xs) { xs = [...xs]; let r = [...xs.shift()].map(v => [v]); return r; } register(f(data));`, ""},
		"map rebuild":      {`export function f(xs) { xs = [...xs]; register(xs.map(v => [v]).map(r => r[0])); } f(data);`, ""},
		"literal rebuild":  {`export function f(a) { register([...a].map(v => ({ v })).map(o => o.v)); } f(data);`, ""},
		"spread prefix":    {`export class T { build(first, nodes) { register(first); return nodes.length; } add(prefix, a) { this.build(...prefix, [...a]); } }`, ""},
		"this alias":       {`export class T { build(nodes) { return nodes.length; } add(a) { const receiver = this; receiver['bu' + 'ild'] = register; this.build([...a]); } }`, ""},
		"this returned":    {`export class T { build(nodes) { return nodes.length; } self() { return this; } add(a) { this.build([...a]); } }`, ""},
		"computed member": {"export class T { build(nodes) { return nodes.length; } add(a) { this.build([...a]); } }\n" +
			"export class U extends T { ['bu' + 'ild'](nodes) { register(nodes); } }\n", ""},
		"computed write": {`export class T { build(nodes) { return nodes.length; } add(a) { this.build([...a]); } }`,
			"import { T } from './util';\nT.prototype['bu' + 'ild'] ??= register;\n"},
		"defineProperty": {`export class T { build(nodes) { return nodes.length; } add(a) { this.build([...a]); } }`,
			"import { T } from './util';\nObject.defineProperty(T.prototype, 'bu' + 'ild', { value: register });\n"},
		"push initializer": {`export function f(r, v) { const out = [] && { push: register }; out.push([...r, v]); return out.length; } f(data);`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, c[0], c[1])
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), ":computed-annotation") {
				t.Fatalf("unproved reach resolved: %+v", m.Unknowns)
			}
		})
	}
}
