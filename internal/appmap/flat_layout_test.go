package appmap

import (
	"context"
	"testing"
)

// flatManifest is the fixture manifest with tests replaced.
func flatManifest(tests string) string {
	return `{"schema":"application-map-manifest/0","app":"admin","hash_prefix":"#!","routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"}],"flows":"flows","tests":` + tests + `}`
}

// AMAP-V0-780: an empty or omitted tests.page_objects builds with no page objects, and a directory
// declared as both specs and page objects reads spec-named files as specs and every other file as
// a page object; every other repeated directory still refuses.
func TestAMAPV0780FlatLayout(t *testing.T) {
	root, _ := fixtureRepo(t)
	ctx := context.Background()
	for name, tests := range map[string]string{
		"omitted": `{"root":"e2e","specs":["e2e/specs"],"workflows":["e2e/workflows"]}`,
		"empty":   `{"root":"e2e","specs":["e2e/specs"],"page_objects":[],"workflows":["e2e/workflows"]}`,
	} {
		writeFile(t, root, "appmap.json", flatManifest(tests))
		m, err := Build(ctx, root, "appmap.json", commitAll(t, root, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, f := range m.Files {
			if f.Role == rolePageObject {
				t.Errorf("%s: %s reads as a page object", name, f.Path)
			}
		}
		if f := m.file("e2e/pages/home.page.ts"); f == nil || f.Role != roleSupport {
			t.Errorf("%s: undeclared page directory: %+v", name, f)
		}
	}

	writeFile(t, root, "e2e/specs/helpers.ts", "import { Page } from '@playwright/test';\nexport async function openHome(page: Page) { await page.goto('/#!/home'); }\n")
	writeFile(t, root, "appmap.json", flatManifest(`{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/specs"],"workflows":["e2e/workflows"]}`))
	m, err := Build(ctx, root, "appmap.json", commitAll(t, root, "flat"))
	if err != nil {
		t.Fatalf("flat: %v", err)
	}
	for p, want := range map[string]string{"e2e/specs/helpers.ts": rolePageObject, "e2e/specs/login.spec.ts": roleSpec, "e2e/workflows/booking.ts": roleWorkflow} {
		if f := m.file(p); f == nil || f.Role != want {
			t.Errorf("flat %s: %+v, want role %s", p, f, want)
		}
	}

	for name, tests := range map[string]string{
		"specs twice":           `{"root":"e2e","specs":["e2e/specs","e2e/specs"]}`,
		"page objects twice":    `{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages","e2e/pages"]}`,
		"specs and workflows":   `{"root":"e2e","specs":["e2e/specs"],"workflows":["e2e/specs"]}`,
		"page objects and scen": `{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"],"scenarios":["e2e/pages"]}`,
		"no specs":              `{"root":"e2e","specs":[],"page_objects":["e2e/pages"]}`,
	} {
		writeFile(t, root, "bad.json", flatManifest(tests))
		if _, err := Build(ctx, root, "bad.json", commitAll(t, root, name)); codeOf(err) != "appmap-invalid-manifest" {
			t.Errorf("%s: %v", name, err)
		}
	}
}
