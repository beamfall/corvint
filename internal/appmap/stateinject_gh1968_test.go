package appmap

import "testing"

// AMAP-V0-373 (V1-3065, #1968): a module the registering module loads may declare a framework
// module through class-decorator metadata whose property values are arrays of names and calls,
// and a provider object whose template substitution calls a function; neither is an inline DI
// annotation nor reaches the injector or the constant, so the states read the constant. An array
// outside decorator metadata, and a substitution naming the constant, still refuse, now naming the
// line and the construct.
func TestAMAPV0373QuietDecoratorsAndSubstitutions(t *testing.T) {
	const feature = "app/core/feature.module.ts"
	reg := "import { FeatureModule } from './feature.module';\n" + gh1851RegSrc + "angular.module('core.downgrade').value('F', FeatureModule);\n"
	routed := func(src string) (*Map, map[string]string) {
		return gh1851Routed(t, gh1851Router, map[string]string{gh1851Reg: reg, feature: src, "app/core/svc.ts": "export class Svc {}\n",
			"package.json": `{"private":true,"dependencies":{"@angular/core":"17.0.0","@ngrx/store":"17.0.0"},"devDependencies":{"@playwright/test":"1.0.0"}}`})
	}
	issue := "import { NgModule } from '@angular/core';\nimport { StoreModule } from '@ngrx/store';\nimport { Svc } from './svc';\n" +
		"const KEY = 'feature';\nfunction reducer(s) { return s; }\n" +
		"@NgModule({ imports: [StoreModule.forFeature(KEY, reducer)], providers: [Svc] })\nexport class FeatureModule {}\n" +
		"export const loader = { prefix: '/i18n/', suffix: `.json?${new Date().getTime()}` };\n"
	m, details := routed(issue)
	gh1869Resolved(t, "issue shape", m, details)

	for name, c := range map[string]struct{ src, want string }{
		"plain call array": {"import { Svc } from './svc';\nfunction reducer(s) { return s; }\nregister([Svc, reducer]);\nexport class FeatureModule {}\n",
			"import-chain-examined:closure-not-quiet:" + feature + ":3:computed-annotation"},
		"array outside the decorator": {"import { Svc } from './svc';\n@NgModule({ providers: [Svc] })\nexport class FeatureModule {}\nboot([Svc, run()]);\n",
			"import-chain-examined:closure-not-quiet:" + feature + ":4:computed-annotation"},
		"decorator not on a class": {"import { Svc } from './svc';\nexport class FeatureModule {\n  @Input({ deps: [Svc, run()] }) x;\n}\n",
			"import-chain-examined:closure-not-quiet:" + feature + ":3:computed-annotation"},
		"substitution naming the constant": {"export class FeatureModule {}\nexport const s = `${keys(StateConstants)}`;\n",
			"import-chain-examined:closure-not-quiet:" + feature + ":2:template-substitution"},
		"substitution naming the injector": {"export class FeatureModule {}\nexport const s = `${$injector.get('A')}`;\n",
			"import-chain-examined:closure-not-quiet:" + feature + ":2:$injector;closure-not-quiet:" + feature + ":2:template-substitution"},
	} {
		m, _ := routed(c.src)
		if d := diDetails(m); len(d) != 1 || d[0] != c.want {
			t.Errorf("%s: %q, want %s", name, d, c.want)
		}
	}
}
