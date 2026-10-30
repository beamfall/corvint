package appmap

import (
	"fmt"
	"strings"
	"testing"
)

// AMAP-V0-369 (V1-3056, #1963): a `.constructor` read that is only tested, or constructed with no
// arguments, loads no module, so a model reached by the registering module's service keeps the
// states literal. A constructor value called, passed on or constructed with arguments still
// refuses, naming the construct.
func TestAMAPV0369ConstructorTests(t *testing.T) {
	const svc, model = "app/core/services/b.ts", "app/core/models/node.model.ts"
	reg := "import { ServiceB } from './services/b';\n" + gh1851RegSrc + "angular.module('core.downgrade').service('ServiceB', ServiceB);\n"
	routed := func(src string) (*Map, map[string]string) {
		return gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg,
			svc:   "import { N } from '../models/node.model';\nexport class ServiceB {\n  make(p) { return new N(p); }\n}\n",
			model: src})
	}
	for name, src := range map[string]string{
		"issue truthiness": "export class N {\n  t(model) {\n    const ns = model.constructor && model.constructor.ns;\n    return ns;\n  }\n}\n",
		"issue construct":  "export class N {\n  copy() {\n    const copy = new (this.constructor as any)();\n    return copy;\n  }\n}\n",
		"bare construct":   "export class N {\n  copy() { return new this.constructor(); }\n}\n",
		"negated":          "export function f(x) {\n  if (!x.constructor) { return 1; }\n  return 0;\n}\n",
	} {
		m, details := routed(src)
		gh1869Resolved(t, name, m, details)
	}
	for name, src := range map[string]string{
		"called":            "export function f(x) {\n  return x.constructor('return 1');\n}\n",
		"construct args":    "export function f(x) {\n  return new (x.constructor)('return 1');\n}\n",
		"or operand":        "export function f(x, y) {\n  return x.constructor || y;\n}\n",
		"and right operand": "export function f(x) {\n  return x.ok && x.constructor;\n}\n",
		"computed chain":    "export function f(x) {\n  return a[0].constructor && 1;\n}\n",
	} {
		m, _ := routed(src)
		want := "import-chain-bindings:" + gh1851Reg + ":1:ServiceB:reach-module-not-literal:" + model + ":2:constructor"
		if d := diDetails(m); len(d) != 1 || d[0] != want {
			t.Errorf("%s: %q, want %s", name, d, want)
		}
	}
}

// AMAP-V0-369 (V1-3056, #1963): the closure limit counts only the files whose import graph may
// reach the table. A service graph of 300 modules that never loads the table resolves; 300
// modules that do still refuse at the limit, the remedy names di_closure_limit, and raising it
// in the manifest resolves them.
func TestAMAPV0369ClosureReach(t *testing.T) {
	chain := func(files map[string]string, dir string, n int, last string) {
		for i := 0; i < n; i++ {
			next := fmt.Sprintf("import './m%d';\n", i+1)
			if i == n-1 {
				next = last
			}
			files[fmt.Sprintf("app/core/%s/m%d.ts", dir, i)] = next + fmt.Sprintf("export const V%d = %d;\n", i, i)
		}
	}
	reg := "import './far/m0';\n" + gh1851RegSrc
	unrelated := map[string]string{gh1851Reg: reg}
	chain(unrelated, "far", 300, "")
	m, details := gh1851Routed(t, gh1851Router, unrelated)
	gh1869Resolved(t, "unrelated graph", m, details)

	reaching := map[string]string{gh1851Reg: reg}
	chain(reaching, "far", 300, "import '../routes';\n")
	m, _ = gh1851Routed(t, gh1851Router, reaching)
	want := "import-chain-examined:closure-limit:256"
	d := diDetails(m)
	if len(d) != 1 || d[0] != want {
		t.Fatalf("reaching graph: %q, want %s", d, want)
	}
	for _, u := range m.Unknowns {
		if u.Kind == "di-constant" || u.Kind == "state" && u.Path == constRouter {
			if r := UnknownRemedy(m, u); r.Key != RemedyKeyDIClosureLimit || !strings.Contains(r.Uncertainty, "above 256") {
				t.Errorf("remedy for %+v: %+v", u, r)
			}
		}
	}

	files := gh1851Files()
	for p, text := range reaching {
		files[p] = text
	}
	_, _, m, err := injectRepo(t, gh1851Scope+`,`+"\n  \"di_closure_limit\": 512", gh1851Router, files)
	if err != nil {
		t.Fatal(err)
	}
	gh1869Resolved(t, "raised limit", m, stateDetails(m, "state"))
}

// AMAP-V0-369: di_closure_limit is 256 to 4096 and only with di_constants.
func TestAMAPV0369ClosureLimitManifest(t *testing.T) {
	for _, v := range []string{"100", "5000"} {
		files := gh1851Files()
		if _, _, _, err := injectRepo(t, gh1851Scope+`,`+"\n  \"di_closure_limit\": "+v, gh1851Router, files); err == nil ||
			!strings.Contains(err.Error(), "di_closure_limit") {
			t.Errorf("%s: %v", v, err)
		}
	}
}
