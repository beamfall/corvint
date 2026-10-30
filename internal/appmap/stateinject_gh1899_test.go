package appmap

import "testing"

// AMAP-V0-209 (#1899): a reached service that calls a static member of an object's class,
// `x.constructor.$delete(...)`, loads no module, so the constant registered beside it stays
// literal; reaching the constructor itself (`.constructor.constructor`, `.call`, `.apply`,
// `.bind`, a call, a bare value, `?.` or a computed member) still refuses with the module's line.
func TestAMAPV0209ConstructorStaticMember(t *testing.T) {
	const svc = "app/core/services/b.ts"
	reg := "import { ServiceB } from './services/b';\n" + gh1851RegSrc + "angular.module('core.downgrade').service('ServiceB', ServiceB);\n"
	routed := func(body string) (*Map, map[string]string) {
		return gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg,
			svc: "export class ServiceB {\n  del(x) { " + body + " }\n}\n"})
	}
	for name, body := range map[string]string{
		"issue repro":   "return x.constructor.$delete(`/a/${x.id}`);",
		"static read":   "return x.constructor.name + x.constructor.prototype;",
		"chained call":  "return x.constructor.query({ id: x.id }).then(r => r);",
		"static member": "const n = x.constructor.$resource; return n;",
	} {
		m, details := routed(body)
		gh1869Resolved(t, name, m, details)
	}
	want := "import-chain-bindings:" + gh1851Reg + ":1:ServiceB:reach-module-not-literal:" + svc + ":2:constructor"
	for name, body := range map[string]string{
		"function":       "return x.constructor.constructor('return 1')();",
		"call":           "return x.constructor.call(null, 'a');",
		"apply":          "return x.constructor.apply(null, ['a']);",
		"bind":           "return x.constructor.bind(null)('a');",
		"called":         "return x.constructor('a');",
		"bare":           "const C = x.constructor; return C;",
		"optional":       "return x.constructor?.constructor('a');",
		"computed":       "return x.constructor['constructor']('a');",
		"prototype hops": "return x.constructor.prototype.constructor('a');",
	} {
		m, _ := routed(body)
		if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
			t.Errorf("%s: %+v %q, want %s", name, u, d, want)
		}
	}
}
