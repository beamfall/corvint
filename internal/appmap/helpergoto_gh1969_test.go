package appmap

import (
	"fmt"
	"strings"
	"testing"
)

// AMAP-V0-374 (V1-3066, #1969): `this.open(...)` defined two classes up (Leaf extends Mid extends
// Base) is followed through the test-root extends chain; a chain that cycles, runs past the bound
// or names a class the reader cannot resolve is the gap `inherited-depth` at the call, never
// `no-navigation`.
func TestAMAPV0374InheritedChain(t *testing.T) {
	root, _ := fixtureRepo(t)
	gitTest(t, root, "rm", "-q", "e2e/specs/alias.spec.ts", "e2e/specs/unresolved.spec.ts")
	writeFile(t, root, "e2e/support/nav.ts", "export async function helper(page, route: string, timeout = 60_000) {\n  await page.goto(`/app#!${route}`, { timeout });\n}\n")
	writeFile(t, root, "e2e/pages/base.page.ts", "import { helper } from '../support/nav';\n\nexport class Base {\n"+
		"  async open(route: string, timeout = 60_000) { await helper(this.page, route, timeout) }\n}\n")
	writeFile(t, root, "e2e/pages/mid.page.ts", "import { Base } from './base.page';\n\nexport class Mid extends Base {\n  async tap() { await this.page.click('#x'); }\n}\n")
	writeFile(t, root, "e2e/pages/leaf.page.ts", "import { Mid } from './mid.page';\n\nexport class Leaf extends Mid {\n"+
		"  async go(id: string) {\n    await this.open(`/clubs/${id}/settings`);\n  }\n}\n")
	writeFile(t, root, "e2e/pages/cyc-a.page.ts", "import { CycB } from './cyc-b.page';\n\nexport class CycA extends CycB {\n  async go() {\n    await this.open('/home');\n  }\n}\n")
	writeFile(t, root, "e2e/pages/cyc-b.page.ts", "import { CycA } from './cyc-a.page';\n\nexport class CycB extends CycA {\n  async tap() { await this.page.click('#y'); }\n}\n")
	writeFile(t, root, "e2e/pages/ext.page.ts", "import { External } from '@playwright/test';\n\nexport class Ext extends External {\n  async go() {\n    await this.open('/home');\n    await this.idle();\n  }\n}\n")
	// A chain one class longer than the bound.
	prev := "Base"
	from := "./base.page"
	for i := 0; i < maxInheritDepth; i++ {
		name := fmt.Sprintf("Deep%d", i)
		writeFile(t, root, fmt.Sprintf("e2e/pages/deep%d.page.ts", i), fmt.Sprintf("import { %s } from '%s';\n\nexport class %s extends %s {}\n", prev, from, name, prev))
		prev, from = name, fmt.Sprintf("./deep%d.page", i)
	}
	writeFile(t, root, "e2e/pages/deepest.page.ts", fmt.Sprintf("import { %s } from '%s';\n\nexport class Deepest extends %s {\n  async go() {\n    await this.open('/home');\n  }\n}\n", prev, from, prev))
	m := build(t, root, commitAll(t, root, "inherited chain"))
	details := map[string]string{}
	for _, u := range m.Unknowns {
		if u.Reason == "page-object-unbound" {
			details[u.Path] = u.Detail
		}
	}
	for _, f := range m.Files {
		if f.Path != "e2e/pages/leaf.page.ts" {
			continue
		}
		settings := screenID(m.App, "app.clubs.settings")
		if len(f.HelperGotos) != 1 || f.HelperGotos[0].Screen != settings || f.HelperGotos[0].Reason != "helper-candidate:e2e/support/nav.ts:this.open:0" {
			t.Fatalf("leaf helper gotos %+v", f.HelperGotos)
		}
	}
	for p, want := range map[string]string{
		"e2e/pages/cyc-a.page.ts":   "helper-unfollowed:e2e/pages/cyc-a.page.ts:5:this.open:inherited-depth",
		"e2e/pages/ext.page.ts":     "helper-unfollowed:e2e/pages/ext.page.ts:5:this.open:inherited-depth",
		"e2e/pages/deepest.page.ts": "helper-unfollowed:e2e/pages/deepest.page.ts:5:this.open:inherited-depth",
	} {
		if details[p] != want {
			t.Errorf("%s: detail %q, want %q", p, details[p], want)
		}
	}
	if d := details["e2e/pages/leaf.page.ts"]; !strings.HasPrefix(d, "helper-candidates:") {
		t.Errorf("leaf detail %q", d)
	}
}
