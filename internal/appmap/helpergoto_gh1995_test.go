package appmap

import (
	"strings"
	"testing"
)

// AMAP-V0-450 (V1-3098, #1995): an unplaceable direct goto does not erase followed
// helper candidates or leave the unresolved-target detail without the blocking sites.
func TestAMAPV0450UnresolvedTargetKeepsHelperGotos(t *testing.T) {
	t.Run("AMAP-V0-450 inherited candidates and direct sites", func(t *testing.T) {
		root, _ := fixtureRepo(t)
		manifest := strings.Replace(readText(t, root, "appmap.json"), `"hash_prefix": "#!",`, `"hash_prefix": "#",`, 1)
		writeFile(t, root, "appmap.json", manifest)
		writeFile(t, root, "app/routes.js", "angular.module('admin').config(function ($stateProvider) {\n  $stateProvider.state('a', { url: '/a/:id' });\n});\n")
		writeFile(t, root, "e2e/pages/Base.ts", "export class Base {\n  async openRoute(p) { await this.page.goto('/app#' + p); }\n}\n")
		writeFile(t, root, "e2e/pages/Mid.ts", "import { Base } from './Base';\nexport default class Mid extends Base {}\n")
		const leaf = "e2e/pages/Leaf.ts"
		writeFile(t, root, leaf, "import Mid from './Mid';\nexport class Leaf extends Mid {\n"+
			"  async a(id) { await this.openRoute(`/a/${id}`); }\n"+
			"  async b(u) { await this.page.goto(`${u}/x`); }\n"+
			"  async c() { await this.page.goto('/missing'); }\n"+
			"  async d() { await this.page.goto('/a/known'); }\n}\n")
		m := build(t, root, commitAll(t, root, "unresolved direct and inherited helper navigation"))
		f := m.file(leaf)
		if f == nil {
			t.Fatal("leaf page object missing")
		}
		if f.Screen != "" || f.ScreenBasis != "" {
			t.Errorf("unresolved page object was bound: %+v", f)
		}
		if len(f.HelperGotos) != 1 {
			t.Errorf("helper gotos %+v, want one retained candidate", f.HelperGotos)
		} else if g := f.HelperGotos[0]; g.Line != 3 || g.URL != "/a/${}" || g.Screen != screenID(m.App, "a") || g.Status != StatusUnknown || g.Reason != "helper-candidate:e2e/pages/Base.ts:this.openRoute:0" {
			t.Errorf("helper candidate %+v", g)
		}
		want := leaf + ":4:non-literal-url," + leaf + ":5:no-matching-screen"
		found := false
		for _, u := range m.Unknowns {
			if u.Reason == "page-object-unresolved-target" {
				if u.Detail == "" {
					t.Errorf("empty unresolved-target detail: %+v", u)
				}
				if u.Path == leaf {
					found = true
					if u.Detail != want {
						t.Errorf("detail %q, want %q", u.Detail, want)
					}
				}
			}
		}
		if !found {
			t.Fatal("leaf unresolved-target unknown missing")
		}
	})
}
