package appmap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// AMAP-V0-350: issue #1940. The refs check over --root sources of a map whose tests live in an
// aliased root used to pass with every test file unread and no references when no --repo bound
// that root; it now refuses naming the alias and the flag, and answers once the root is bound.
func TestAMAPV0350RefsUnboundTestRoot(t *testing.T) {
	ctx := context.Background()
	app, appRev, e2e, _ := twoRoots(t, false)
	m, err := BuildRoots(ctx, app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil || len(m.Files) == 0 {
		t.Fatalf("two-root map: %v", err)
	}
	_, err = CheckRefs(ctx, m, []string{"app"}, Options{Root: app, Full: true})
	if codeOf(err) != "appmap-root-unavailable" || !strings.Contains(err.Error(), "root e2e holds") || !strings.Contains(err.Error(), "--repo e2e=DIR") {
		t.Fatalf("unbound test root: %v", err)
	}
	r, err := CheckRefs(ctx, m, []string{"app"}, Options{Root: app, Full: true, Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	var doc refsDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil || doc.TestFilesRead != len(m.Files) || doc.TestFilesUnread != 0 || doc.References == 0 {
		t.Fatalf("bound test root: %v %+v", err, doc)
	}
}

// AMAP-V0-815: a bound aliased root's own playwright.config sets the test-id attribute its tests
// use, so a custom-attribute locator there is checked rather than left without a name.
func TestAMAPV0815AliasedRootTestIDAttribute(t *testing.T) {
	ctx := context.Background()
	app, _, e2e, _ := twoRoots(t, false)
	writeFile(t, e2e, "playwright.config.ts", "import { defineConfig } from '@playwright/test';\nexport default defineConfig({ use: { testIdAttribute: 'qa-id' } });\n")
	writeFile(t, e2e, "e2e/specs/qa.spec.ts", "test('qa', async ({ page }) => {\n  await page.locator('[qa-id=ghost-button]').click();\n});\n")
	commitAll(t, e2e, "qa attribute")
	appRev := gitTest(t, app, "rev-parse", "HEAD")
	m, err := BuildRoots(ctx, app, "appmap.json", appRev, Roots{Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := CheckRefs(ctx, m, []string{"app"}, Options{Root: app, Full: true, Repos: map[string]string{"e2e": e2e}})
	if err != nil {
		t.Fatal(err)
	}
	var doc refsDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil {
		t.Fatal(err)
	}
	if keys := strings.Join(missKeys(doc), "\n"); !strings.Contains(keys, "selector|ghost-button|exact|") {
		t.Fatalf("aliased root's configured attribute not checked:\n%s", keys)
	}
}
