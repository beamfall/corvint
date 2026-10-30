package appmap

import (
	"maps"
	"testing"
)

// AMAP-V0-394 (V1-3083, #1980): a middle class that is the default export of its file and the
// default import of the leaf is followed in the inherited helper walk, through a relative and a
// bare (tsconfig baseUrl) specifier; a default import of a file whose default export is no class
// stays the gap `inherited-depth`, and a lost leaf that also has an unplaceable direct goto names
// that gap in its `page-object-unresolved-target` detail.
func TestAMAPV0394DefaultImportedMiddleClass(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	writeFile(t, root, "e2e/tsconfig.json", `{"compilerOptions": {"baseUrl": "."}}`+"\n")
	writeFile(t, root, "e2e/support/nav.ts", "export async function helper(page, route: string, timeout = 60_000) {\n  await page.goto(`/app#!${route}`, { timeout });\n}\n")
	writeFile(t, root, "e2e/pages/base.page.ts", "import { helper } from '../support/nav';\n\nexport class Base {\n"+
		"  async open(route: string, timeout = 60_000) { await helper(this.page, route, timeout) }\n}\n")
	writeFile(t, root, "e2e/pages/Mid.ts", "import { Base } from './base.page';\n\nexport default class Mid extends Base {\n  async tap() { await this.page.click('#x'); }\n}\n")
	writeFile(t, root, "e2e/pages/named.page.ts", "import { Base } from './base.page';\n\nexport class Named extends Base {}\n")
	leaf := func(imp string) string {
		return imp + "\n\nexport class Leaf extends Mid {\n  async go(id: string) {\n    await this.open(`/clubs/${id}/settings`);\n  }\n}\n"
	}
	writeFile(t, root, "e2e/pages/sub/relative.page.ts", leaf("import Mid from '../Mid';"))
	writeFile(t, root, "e2e/pages/sub/bare.page.ts", leaf("import Mid from 'pages/Mid';"))
	writeFile(t, root, "e2e/pages/sub/mixed.page.ts", leaf("import Mid, { type Base } from '../Mid';"))
	// Named exports its class by name only, so its default clause names no class.
	writeFile(t, root, "e2e/pages/sub/lost.page.ts", leaf("import Mid from '../named.page';"))
	writeFile(t, root, "e2e/pages/sub/lost-goto.page.ts", "import Mid from '../named.page';\n\nexport class Leaf extends Mid {\n"+
		"  async go(id: string) {\n    await this.page.goto('/nowhere-at-all');\n    await this.open(`/clubs/${id}/settings`);\n  }\n}\n")
	m := build(t, root, commitAll(t, root, "default-imported middle class"))
	settings := screenID(m.App, "app.clubs.settings")
	for _, p := range []string{"e2e/pages/sub/relative.page.ts", "e2e/pages/sub/bare.page.ts", "e2e/pages/sub/mixed.page.ts"} {
		f := m.file(p)
		if f == nil || len(f.HelperGotos) != 1 || f.HelperGotos[0].Screen != settings || f.HelperGotos[0].Reason != "helper-candidate:e2e/support/nav.ts:this.open:0" {
			t.Errorf("%s helper gotos %+v", p, f)
		}
	}
	details := map[string]string{}
	for _, u := range m.Unknowns {
		if u.Kind == "page-object" {
			details[u.Path] = u.Reason + "|" + u.Detail
		}
	}
	for p, want := range map[string]string{
		"e2e/pages/sub/lost.page.ts":      "page-object-unbound|helper-unfollowed:e2e/pages/sub/lost.page.ts:5:this.open:inherited-depth",
		"e2e/pages/sub/lost-goto.page.ts": "page-object-unresolved-target|e2e/pages/sub/lost-goto.page.ts:5:no-matching-screen,helper-unfollowed:e2e/pages/sub/lost-goto.page.ts:6:this.open:inherited-depth",
	} {
		if details[p] != want {
			t.Errorf("%s: %q, want %q", p, details[p], want)
		}
	}
}

// AMAP-V0-394: the default-clause reading binds the default name of each static import and nothing
// else: no named, namespace, type-only or dynamic import, and no name two statements bind.
func TestAMAPV0394DefaultImportBindings(t *testing.T) {
	src := "import Mid from '../Mid';\nimport Both, { x } from './both';\nimport Ns, * as all from './ns2';\n" +
		"import { y } from './y';\nimport * as ns from './ns';\nimport type T from './t';\nfoo(); import Late from './late';\n" +
		"const d = import('./dyn');\nimport Twice from './a';\nimport Twice from './b';\n"
	toks, _ := lexJS(src)
	got := defaultImports(toks)
	want := map[string]string{"Mid": "../Mid", "Both": "./both", "Ns": "./ns2", "Late": "./late"}
	if !maps.Equal(got, want) {
		t.Fatalf("default imports %v, want %v", got, want)
	}
}
