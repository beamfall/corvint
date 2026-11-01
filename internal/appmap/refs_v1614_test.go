package appmap

import (
	"reflect"
	"strings"
	"testing"
)

func refStrings(refs []testRef) []string {
	got := []string{}
	for _, r := range refs {
		got = append(got, r.kind+" "+r.v.value+r.v.reason+" "+r.v.mode)
	}
	return got
}

// AMAP-V0-814: Cypress `cy` calls give route, endpoint and selector references; a `cy.get` alias
// is unchecked and a `get` on any other receiver is not a reference.
func TestAMAPV0814CypressRefs(t *testing.T) {
	src := "cy.visit('/orders')\n" +
		"cy.intercept('GET', '/api/v2/orders')\n" +
		"cy.intercept('/api/ghost')\n" +
		"cy.get('[data-testid=save-order]')\n" +
		"cy.get('@orders')\n" +
		"cy.get('.btn.ghost-btn')\n" +
		"list.get('#x'); store.visit('/y'); win.cy.get('#z')\n"
	want := []string{"route /orders exact",
		"endpoint api exact", "endpoint v2 exact", "endpoint orders exact",
		"endpoint api exact", "endpoint ghost exact",
		"selector save-order exact",
		"selector cypress-alias ",
		"selector btn exact", "selector ghost-btn exact"}
	if got := refStrings(scanRefs(src)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}

	root, _ := refsFixture(t)
	writeFile(t, root, "e2e/specs/orders.cy.ts", "it('ships', () => {\n  cy.visit('/reports');\n  cy.intercept('POST', '/api/v2/ghost-endpoint');\n  cy.get('#ghost-panel');\n  cy.get('@alias');\n});\n")
	m := build(t, root, commitAll(t, root, "cypress"))
	doc, _ := checkRefs(t, root, m, Options{Full: true}, "src")
	keys := strings.Join(missKeys(doc), "\n")
	for _, k := range []string{"route|/reports|exact|catch-all-route", "endpoint|ghost-endpoint|exact|", "selector|ghost-panel|exact|"} {
		if !strings.Contains(keys, k) {
			t.Errorf("missing %s in\n%s", k, keys)
		}
	}
	found := false
	for _, u := range doc.Unchecked {
		found = found || u.Kind == refSelector && u.Reason == "cypress-alias" && u.Count == 1
	}
	if !found {
		t.Errorf("no cypress-alias unchecked entry: %+v", doc.Unchecked)
	}
}

// AMAP-V0-815: a configured test-id attribute makes an exact `[<name>=...]` locator a test id;
// data-testid stays one, and every configured name is honoured.
func TestAMAPV0815TestIDAttribute(t *testing.T) {
	cfg := "export default defineConfig({ use: { testIdAttribute: 'qa-id' } });\n" +
		"selectors.setTestIdAttribute(\"data-qa\");\nselectors.setTestIdAttribute(name);\n"
	attrs := testIDAttrs([]byte(cfg))
	if !reflect.DeepEqual(attrs, []string{"data-qa", "qa-id"}) {
		t.Fatalf("attrs %q", attrs)
	}
	src := "page.locator('[qa-id=ghost-button]')\npage.locator(\"[data-qa='save-order']\")\npage.locator('[data-testid=save-order]')\npage.locator('[other=x]')\n"
	want := []string{"selector ghost-button exact", "selector save-order exact", "selector save-order exact", "selector no-checkable-name "}
	if got := refStrings(scanRefs(src, attrs...)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
	if got := refStrings(scanRefs("page.locator('[qa-id=ghost-button]')")); !reflect.DeepEqual(got, []string{"selector no-checkable-name "}) {
		t.Fatalf("unconfigured: %q", got)
	}

	root, _ := refsFixture(t)
	writeFile(t, root, "playwright.config.ts", "import { defineConfig } from '@playwright/test';\nexport default defineConfig({ use: { testIdAttribute: 'qa-id' } });\n")
	writeFile(t, root, "e2e/specs/qa.spec.ts", "test('qa', async ({ page }) => {\n  await page.locator('[qa-id=ghost-button]').click();\n});\n")
	m := build(t, root, commitAll(t, root, "qa"))
	doc, _ := checkRefs(t, root, m, Options{Full: true}, "src")
	if keys := strings.Join(missKeys(doc), "\n"); !strings.Contains(keys, "selector|ghost-button|exact|") {
		t.Fatalf("configured attribute not checked:\n%s", keys)
	}
}

// AMAP-V0-816: a route expression anchored at both ends is evaluated against the screens; any
// other keeps the regular-expression label.
func TestAMAPV0816AnchoredRegexRoutes(t *testing.T) {
	_, m := refsFixture(t)
	screens := []Screen{}
	for _, s := range m.Screens {
		if !strings.HasSuffix(s.ID, ":fallback") {
			screens = append(screens, s)
		}
	}
	check := func(x *screenIndex, src string) (bool, string) {
		refs := scanRefs("expect(page).toHaveURL(" + src + ")")
		if len(refs) != 1 || refs[0].v.reason != "" {
			t.Fatalf("%s: %+v", src, refs)
		}
		miss, class, reason := routeCheck(x, refs[0].v)
		if reason != "" {
			t.Fatalf("%s: %s", src, reason)
		}
		if miss && class == "" {
			class = refs[0].v.class
		}
		return miss, class
	}
	x := newScreenIndex(m.HashPrefix, m.BasePath, screens)
	for _, tc := range []struct {
		src   string
		miss  bool
		class string
	}{
		{`/^\/home$/`, false, ""},
		{`/^\/HOME$/i`, false, ""},
		{`/^\/(home|orders)$/`, false, ""},
		{`/^\/orders\/\d+$/`, false, ""},
		{`/^\/files\/.+$/`, false, ""},
		{`/^\/home\/extra$/`, true, ""},
		{`/^\/reports$/`, true, ""},
		{`/^\/reports\/(?=x)$/`, true, classRegex}, // not RE2
		{`/^\/home\/extra$/g`, true, classRegex},
		{`/^\/home\/extra/`, true, classRegex},
		{`/^\/home\/extra\$/`, true, classRegex},
		{`/\/home\/extra$/`, true, classRegex},
	} {
		if miss, class := check(x, tc.src); miss != tc.miss || class != tc.class {
			t.Errorf("%s: miss=%v class=%q", tc.src, miss, class)
		}
	}
	if miss, class := check(newScreenIndex(m.HashPrefix, m.BasePath, m.Screens), `/^\/reports$/`); !miss || class != classCatchAll {
		t.Errorf("root splat: miss=%v class=%q", miss, class)
	}
	based := newScreenIndex("#!", "/app", screens)
	for _, src := range []string{`/^\/app\/home$/`, `/^\/home$/`, `/^\/app\/#!\/home$/`} {
		if miss, _ := check(based, src); miss {
			t.Errorf("%s misses under base path and hash prefix", src)
		}
	}
}
