package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-420, AMAP-V0-421: exercise the reported shapes through the real registration/import
// closure. Exported data and dormant helpers need consumer evidence, not a file-name exemption.
func TestAMAPV0420ArrayInjectorReach(t *testing.T) {
	for name, src := range map[string]string{
		"rxjs arrow body":             `import { mergeMap } from 'rxjs/operators'; export const effect = mergeMap(xs => [new ActionA({ xs }), new ActionB()]);`,
		"rxjs parenthesized params":   `import { mergeMap as project } from 'rxjs'; export const effect = project((xs: any) => [new ActionA({ xs }), new ActionB()]);`,
		"called scalar helper":        `export function f(xs) { xs = [...xs]; return xs.length; } f(data);`,
		"called minimal repro":        `export function f(xs) { xs = [...xs]; return [new A(), new B()].length } f(data);`,
		"arrow body in call argument": `export function actions(xs) { return mergeMap(xs => [new ActionA({ xs }), new ActionB()]); }`,
		"different get":               `export function aFactory($injector: any) { return $injector.get('SomeOtherService') }`,
		"unwritten get binding":       `const other = $injector.get('Other'); const label = other.label;`,
		"member receiver":             `export class Person { name() { return [this.first, this.last].join(' '); } }`,
		"exported list":               `export const PIPES = [PipeA, PipeB, PipeC];`,
		"spread helper":               `export function copy(arrays) { arrays = [...arrays]; return arrays; }`,
		"minimal repro":               `export function f(xs) { xs = [...xs]; return [new A(), new B()].length }`,
	} {
		t.Run(name, func(t *testing.T) {
			m, details := gh1994Routed(t, src, "")
			gh1869Resolved(t, name, m, details)
		})
	}
}

// AMAP-V0-420: references escaping to calls, aliases, namespace imports or re-exports cannot
// establish quietness. A different service lookup is exempt only with an exact, unwritten result.
func TestAMAPV0420ArrayReachRefusals(t *testing.T) {
	for name, src := range map[string]string{
		"async projection":    `import { mergeMap } from 'rxjs'; mergeMap(async xs => [dep, fn]);`,
		"unknown projection":  `import { map } from 'rxjs'; map(xs => [dep, fn]);`,
		"shadowed projection": `import { mergeMap } from 'rxjs'; function f(mergeMap) { mergeMap(xs => [dep, fn]); } f(register);`,
		"named shadow":        `import { mergeMap } from 'rxjs'; function f() { function mergeMap(fn) { register(fn()); } mergeMap(xs => [dep, fn]); } f();`,
		"inline registration": `angular.module('app').run([dep, function(n) { n.HOME = 'changed'; }]);`,
		"invoke":              `$injector.invoke([dep, fn]);`,
		"instantiate":         `$injector.instantiate([dep, fn]);`,
		"inject assignment":   `F.$inject = [dep];`,
		"unknown call":        `register([dep, fn]);`,
		"unknown callback":    `register(x => [dep, fn]);`,
		"spread into call":    `register([...deps]);`,
		"returned helper":     `function copy(xs) { xs = [...xs]; return xs; } angular.module('app').run(copy(deps));`,
		"computed get":        `$injector.get(service);`,
		"same get":            `$injector.get('StateConstants');`,
		"written get":         `$injector.get('Other').HOME = 'changed';`,
		"written get binding": `const other = $injector.get('Other'); other.HOME = 'changed';`,
		"escaped injector":    `function f($injector) { save($injector); return $injector.get('Other'); }`,
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, src, "")
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), "closure-not-quiet:") {
				t.Fatalf("unproved reach resolved: %+v", m.Unknowns)
			}
		})
	}
	for name, consumer := range map[string]string{
		"named":     `import { PIPES } from './util'; angular.module('app').run(PIPES);`,
		"alias":     `import { PIPES as list } from './util'; register(list);`,
		"namespace": `import * as util from './util'; register(util.PIPES);`,
		"barrel":    `export { PIPES } from './util';`,
		"helper":    `import { copy } from './util'; angular.module('app').run(copy(deps));`,
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, `export const PIPES = [dep, fn]; export function copy(xs) { xs = [...xs]; return xs; }`, consumer)
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), "computed-annotation") {
				t.Fatalf("escaped export resolved: %+v", m.Unknowns)
			}
		})
	}
	t.Run("local operator", func(t *testing.T) {
		m, _ := gh1851Routed(t, gh1851Router, map[string]string{
			gh1851Reg:              "import './util';\n" + gh1851RegSrc,
			"app/core/util.ts":     `import { mergeMap } from './operator'; mergeMap(xs => [dep, fn]);`,
			"app/core/operator.ts": `export function mergeMap(fn) { register(fn()); }`,
		})
		if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), "computed-annotation") {
			t.Fatalf("local operator was trusted as RxJS: %+v", m.Unknowns)
		}
	})
}

// AMAP-V0-420: only a function declaration can be dormant. A named function expression is a
// value that may run, so the array it returns must keep blocking.
func TestAMAPV0420DormantNeedsDeclaration(t *testing.T) {
	for name, src := range map[string]string{
		"named expression": `export const k = 1; const make = function hidden(){ return [dep, fn]; }; angular.module('app').run(make());`,
		"export default":   `export default function hidden(){ return [dep, fn]; }`,
		"parenthesised":    `export const k = 1; (function hidden(){ return [dep, fn]; })();`,
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := gh1994Routed(t, src, "")
			if d := diDetails(m); len(d) == 0 || !strings.Contains(strings.Join(d, ";"), "closure-not-quiet:") {
				t.Fatalf("function expression treated as dormant: %+v", m.Unknowns)
			}
		})
	}
	m, details := gh1994Routed(t, `export const k = 1; function hidden(){ return [dep, fn]; }`, "")
	gh1869Resolved(t, "declaration", m, details)
}

// AMAP-V0-422: preserve every file/line/construct, including more than one bad annotation.
func TestAMAPV0422AllClosureBlockers(t *testing.T) {
	m, _ := gh1994Routed(t, "register([dep, fn]);\n$injector.get(service);\nF.$inject = [dep];\nG.$inject = [other];\n",
		"register([other, fn]);\n$provide.decorator(service, fn);\n`${$injector}`;\n`${$injector}`;\n")
	detail := strings.Join(diDetails(m), ";")
	for _, want := range []string{
		"app/core/util.ts:1:computed-annotation", "app/core/util.ts:2:$injector",
		"app/core/util.ts:3:$inject", "app/core/util.ts:4:$inject",
		"app/core/consumer.ts:1:computed-annotation", "app/core/consumer.ts:2:$provide", "app/core/consumer.ts:2:decorator",
		"app/core/consumer.ts:3:$injector", "app/core/consumer.ts:4:$injector",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("missing %s in %s", want, detail)
		}
	}
}

// AMAP-V0-420: the first router's proof of an unused export cannot be reused after another
// router adds a consumer that hands that export to an unknown call.
func TestAMAPV0420ReachMemoTracksClosure(t *testing.T) {
	m, details := gh1851Routed(t, gh1851Router, map[string]string{
		gh1851Reg:          "import './util';\n" + gh1851RegSrc,
		"app/core/util.ts": `export const PIPES = [dep, fn];`,
		"app/routes.js":    "import { PIPES } from './core/util'; register(PIPES);\n" + gh1851Router,
	})
	if len(details) != 0 {
		t.Fatalf("quiet router refused: %v", details)
	}
	blocked := false
	for _, u := range m.Unknowns {
		if u.Kind == "state" && u.Path == "app/routes.js" && strings.Contains(u.Detail, "computed-annotation") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("consumer closure reused quiet proof: %+v", m.Unknowns)
	}
}

func gh1994Routed(t *testing.T, src, consumer string) (*Map, map[string]string) {
	t.Helper()
	return gh1851Routed(t, gh1851Router, map[string]string{
		gh1851Reg:              "import './util';\nimport './consumer';\n" + gh1851RegSrc,
		"app/core/util.ts":     src,
		"app/core/consumer.ts": consumer,
		"package.json":         `{"private":true,"dependencies":{"rxjs":"7.8.2"},"devDependencies":{"@playwright/test":"1.0.0"}}`,
	})
}
