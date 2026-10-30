package appmap

import "testing"

// AMAP-V0-384, AMAP-V0-385 (V1-3082, #1979): a module the registering module loads may hold a
// module-scope `const LIST = [ModA, ModB, ModC]` read only by class-decorator metadata or a spread,
// and a route data object `{ only: [Enum.MEMBER, 'literal'] }`; neither array can reach the
// injector as an annotation, so the states read the constant. An array ending in a function, a
// const passed to a call, a plain-ended array passed to a call, and one a binding holds and may
// mutate still refuse. AMAP-V0-420 now proves unconsumed exported lists quiet too.
func TestAMAPV0384QuietConstListsAndDataArrays(t *testing.T) {
	const feature = "app/core/feature.module.ts"
	reg := "import { FeatureModule } from './feature.module';\n" + gh1851RegSrc + "angular.module('core.downgrade').value('F', FeatureModule);\n"
	routed := func(src string) (*Map, map[string]string) {
		return gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg, feature: src, "app/core/mods.ts": "export class ModA {}\nexport class ModB {}\nexport class ModC {}\n",
			"package.json": `{"private":true,"dependencies":{"@angular/core":"17.0.0"},"devDependencies":{"@playwright/test":"1.0.0"}}`})
	}
	for name, src := range map[string]string{
		"exported const": "import { ModA, ModB } from './mods';\nexport class FeatureModule {}\nexport const LIST = [ModA, ModB];\n",
		"const list": "import { NgModule } from '@angular/core';\nimport { ModA, ModB, ModC } from './mods';\n" +
			"const LIST = [ModA, ModB, ModC];\nconst MORE = [ModC, ModB]\n" +
			"@NgModule({ imports: LIST, exports: [...LIST, ...MORE] })\nexport class FeatureModule {}\n",
		"data array": "import { Role } from './mods';\nexport class FeatureModule {}\n" +
			"export const route = { name: 'admin', data: { permissions: { only: [Role.ADMIN, 'owner'] } } };\n",
	} {
		m, details := routed(src)
		gh1869Resolved(t, name, m, details)
	}

	const mods = "import { ModA, ModB } from './mods';\nexport class FeatureModule {}\n"
	for name, c := range map[string]struct{ src, want string }{
		"const ending in a function": {mods + "const LIST = [ModA, function (a) {}];\n@NgModule({ imports: LIST })\nexport class X {}\n", "3"},
		"const ending in an arrow":   {mods + "const LIST = [ModA, (a) => a];\n", "3"},
		"const passed to a call":     {mods + "const LIST = [ModA, ModB];\nboot(LIST);\n", "3"},
		"const in a substitution":    {mods + "const LIST = [ModA, ModB];\nexport const s = `${boot(LIST)}`;\n", "3"},
		"const spread into a call":   {mods + "const LIST = [ModA, ModB];\nboot(...LIST);\n", "3"},
		"const extended by a call":   {mods + "const LIST = [ModA, ModB].concat(run);\n", "3"},
		"plain-ended call argument":  {mods + "boot([ModA.x, 'owner']);\n", "3"},
		"let list":                   {mods + "let LIST = [ModA, ModB];\n", "3"},
		// V1-3089: a plain-ended array a binding holds may be completed later.
		"plain-ended array mutated": {mods + "const dep = 'Na' + 'mes'; const a = [dep, 0]; a.pop(); a.push(function(n) { n.HOME = '/changed'; }); angular.module('app').run(a);\n", "3"},
	} {
		m, _ := routed(c.src)
		want := "import-chain-examined:closure-not-quiet:" + feature + ":" + c.want + ":computed-annotation"
		if d := diDetails(m); len(d) != 1 || d[0] != want {
			t.Errorf("%s: %q, want %s", name, d, want)
		}
	}
}
