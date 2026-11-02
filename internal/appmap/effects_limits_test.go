package appmap

import (
	"strings"
	"testing"
)

// ANL-V0-811 (V1-1236): test-id, raw-selector and property-bound clicks and spec-local helpers
// stay unknown at their own line; only a link-role click is navigation, and no edge is added.
func TestANLV0811StepEffectLimitsStayNamed(t *testing.T) {
	root, _ := fixtureRepo(t)
	home := strings.Replace(effectHomePage, "constructor(private readonly page: Page) {}",
		"constructor(private readonly page: Page) {}\n\n  readonly save = this.page.getByTestId('save');\n\n  async submit() {\n    await this.save.click();\n  }", 1)
	writeFile(t, root, "e2e/pages/home.page.ts", home)
	writeFile(t, root, "e2e/pages/teesheet.page.ts", effectSheetPage)
	cases := map[string]struct{ step, effect, ref, call string }{
		"property": {"await home.submit();", EffectUnknown, "e2e/pages/home.page.ts:9", "click"},
		"testid":   {"await page.getByTestId('save').click();", EffectUnknown, "e2e/specs/testid.spec.ts:7", "click"},
		"locator":  {"await page.locator('#save').click();", EffectUnknown, "e2e/specs/locator.spec.ts:7", "click"},
		"raw":      {"await page.click('#save');", EffectUnknown, "e2e/specs/raw.spec.ts:7", "click"},
		"link":     {"await page.getByRole('link', { name: 'Sheets' }).click();", EffectNavigation, "", ""},
	}
	for name, c := range cases {
		writeFile(t, root, "e2e/specs/"+name+".spec.ts", effectSpec(c.step))
	}
	// A helper the spec itself declares is still a bare call this reader does not follow.
	writeFile(t, root, "e2e/specs/local.spec.ts", strings.Replace(effectSpec("await openHome(page);"),
		"\n\ntest(", "\nasync function openHome(page) { await page.goto('/'); }\n\ntest(", 1))
	cases["local"] = struct{ step, effect, ref, call string }{"", EffectUnknown, "e2e/specs/local.spec.ts:8", "openHome"}
	m := build(t, root, commitAll(t, root, "limits"))
	for name, c := range cases {
		edges := stepEdges(m, "e2e/specs/"+name+".spec.ts")
		if len(edges) != 2 {
			t.Fatalf("%s: want the two construction edges only, got %+v", name, edges)
		}
		if e := edges[0]; e.Effect != c.effect || e.EffectRef != c.ref || e.EffectCall != c.call {
			t.Errorf("%s: got %s %s %s, want %s %s %s", name, e.Effect, e.EffectRef, e.EffectCall, c.effect, c.ref, c.call)
		}
	}
}
