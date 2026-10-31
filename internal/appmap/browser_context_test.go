package appmap

import (
	"context"
	"testing"
)

// AMAP-V0-781, AMAP-V0-782: imports inside page.evaluate-style callbacks are browser-context
// references resolved against tests.web_root, never unresolved spec imports; a computed one is a
// browser-reference unknown; the same specifier imported in Node context keeps AMAP-V0-005.
func TestAMAPV0781BrowserContextImports(t *testing.T) {
	root, _ := fixtureRepo(t)
	ctx := context.Background()
	writeFile(t, root, "app/src/core/vault.ts", "export function open() { return true; }\n")
	writeFile(t, root, "e2e/specs/vault.spec.ts", `import { test } from '@playwright/test';

test('vault', async ({ page }) => {
  await page.goto('/#!/login');
  await page.evaluate(async () => {
    const { open } = await import("/src/core/vault.ts");
    return open();
  });
  const name = 'vault';
  await page.evaluate(async (n) => { await import(`+"`/src/core/${n}.ts`"+`); }, name);
  await page.addInitScript(async () => { await import("/src/missing.ts"); });
});
`)
	writeFile(t, root, "e2e/specs/node.spec.ts", `import { test } from '@playwright/test';
import { open } from "/src/core/vault.ts";

test('node', async ({ page }) => {
  await page.evaluate(async () => { await import("/src/core/vault.ts"); });
});
`)
	manifest := func(webRoot string) string {
		return flatManifest(`{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"],"workflows":["e2e/workflows"],"scenarios":["e2e/scenarios"]` + webRoot + `}`)
	}
	unknowns := func(m *Map, p string) map[string]Unknown {
		out := map[string]Unknown{}
		for _, u := range m.Unknowns {
			if u.Path == p {
				out[u.Kind+" "+u.Reason+" "+u.Detail] = u
			}
		}
		return out
	}

	writeFile(t, root, "appmap.json", manifest(`,"web_root":"app"`))
	m, err := Build(ctx, root, "appmap.json", commitAll(t, root, "browser imports"))
	if err != nil {
		t.Fatal(err)
	}
	spec := m.file("e2e/specs/vault.spec.ts")
	if spec == nil || spec.Join != StatusResolved {
		t.Fatalf("browser-context imports change the spec's join: %+v", spec)
	}
	got := map[string]Import{}
	for _, imp := range spec.Imports {
		got[imp.Specifier] = imp
	}
	if imp := got["/src/core/vault.ts"]; imp.Status != importBrowser || imp.Resolved != "app/src/core/vault.ts" || imp.Line != 6 {
		t.Errorf("literal browser import: %+v", imp)
	}
	if imp := got["/src/missing.ts"]; imp.Status != importBrowser || imp.Resolved != "" {
		t.Errorf("missing browser import: %+v", imp)
	}
	us := unknowns(m, spec.Path)
	for _, want := range []string{"browser-reference browser-computed-specifier computed specifier", "browser-reference browser-target-missing specifier /src/missing.ts"} {
		if _, ok := us[want]; !ok {
			t.Errorf("missing unknown %q in %v", want, us)
		}
	}
	for k, u := range us {
		if u.Kind == "import" {
			t.Errorf("spec keeps import unknown %q", k)
		}
	}
	node := m.file("e2e/specs/node.spec.ts")
	if node == nil || node.Join != StatusUnknown {
		t.Fatalf("a Node-context import of the same specifier is no longer unresolved: %+v", node)
	}
	if _, ok := unknowns(m, node.Path)["import unresolved-import specifier /src/core/vault.ts"]; !ok {
		t.Errorf("node spec unknowns: %v", unknowns(m, node.Path))
	}

	writeFile(t, root, "e2e/specs/node.spec.ts", "import { test } from '@playwright/test';\n")
	writeFile(t, root, "appmap.json", manifest(""))
	m, err = Build(ctx, root, "appmap.json", commitAll(t, root, "no web root"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unknowns(m, "e2e/specs/vault.spec.ts")["browser-reference browser-root-undeclared specifier /src/core/vault.ts"]; !ok {
		t.Errorf("undeclared web root: %v", unknowns(m, "e2e/specs/vault.spec.ts"))
	}
	if m.file("e2e/specs/vault.spec.ts").Join != StatusResolved {
		t.Error("an undeclared web root makes the join unknown")
	}

	writeFile(t, root, "appmap.json", manifest(`,"web_root":"app/nope"`))
	if _, err := Build(ctx, root, "appmap.json", commitAll(t, root, "missing web root")); codeOf(err) != "appmap-invalid-manifest" {
		t.Errorf("missing web_root directory: %v", err)
	}
}
