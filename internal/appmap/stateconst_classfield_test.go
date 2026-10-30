package appmap

import "testing"

// AMAP-V0-365 (V1-2965, #1951): a class field's type annotation is erased, so a model reached by
// the constant's import chain that declares `f: Function` loads no module and the states stay
// literal. Function used as a value -- an object literal property, a field initializer or a
// method parameter type the reader does not follow -- still refuses, naming the construct.
func TestAMAPV0365ClassFieldTypes(t *testing.T) {
	const svc, model = "app/core/services/b.ts", "app/core/models/provider.model.ts"
	reg := "import { ServiceB } from './services/b';\n" + gh1851RegSrc + "angular.module('core.downgrade').service('ServiceB', ServiceB);\n"
	routed := func(src string) (*Map, map[string]string) {
		return gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg,
			svc:   "import { P } from '../models/provider.model';\nexport class ServiceB {\n  make(p) { return new P(p); }\n}\n",
			model: src})
	}
	for name, src := range map[string]string{
		"issue repro": "export class P {\n  a: string;\n  // eslint-disable-next-line @typescript-eslint/ban-types\n  f: Function;\n" +
			"  constructor(p?: Partial<P>) { Object.assign(this, p) }\n}\n",
		"optional and modifiers": "export class P extends Base<string> implements Q {\n  b?: Function\n  private readonly c!: Function;\n" +
			"  static d: (cb: Function) => void = null;\n  e: Array<Function> = []\n}\n",
	} {
		m, details := routed(src)
		gh1869Resolved(t, name, m, details)
	}
	for name, c := range map[string]struct{ src, line string }{
		"object literal": {"export const P = {\n  f: Function,\n};\n", "2"},
		"initializer":    {"export class P {\n  f: unknown = Function;\n}\n", "2"},
		"method param":   {"export class P {\n  m(f: Function) { return f; }\n}\n", "2"},
		"typeof break":   {"export class P {\n  f = flag ? typeof\n    a : Function;\n}\n", "3"},
		"new break":      {"export class P {\n  f = flag ? new\n    A : Function;\n}\n", "3"},
	} {
		m, _ := routed(c.src)
		want := "import-chain-bindings:" + gh1851Reg + ":1:ServiceB:reach-module-not-literal:" + model + ":" + c.line + ":Function"
		if u, d := diUnknown(m), diDetails(m); len(u) != 1 || u[0].Reason != "not-read-whole" || len(d) != 1 || d[0] != want {
			t.Errorf("%s: %+v %q, want %s", name, u, d, want)
		}
	}
}
