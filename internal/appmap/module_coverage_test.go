package appmap

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// AMAP-V0-810 AMAP-V0-811: a two-router fixture with partial coverage reads the expected
// per-module counts; unknown and abstract screens are never reached or unreached, and an
// undeclared router_globs file is an UNKNOWN module with null counts.
func TestAMAPV0810ModuleCoverage(t *testing.T) {
	root, _ := fixtureRepo(t)
	writeFile(t, root, "app/admin.routes.js", "angular.module('admin').config(function ($stateProvider) {\n  $stateProvider\n    .state('adminUsers', { url: '/admin/users' })\n    .state('adminAudit', { url: '/admin/audit' });\n});\n")
	writeFile(t, root, "app/extra.routes.js", "// not declared\n")
	writeFile(t, root, "e2e/specs/admin.spec.ts", "import { test } from '@playwright/test';\ntest('users', async ({ page }) => { await page.goto('/#!/admin/users'); });\n")
	writeFile(t, root, "appmap.json", `{"schema":"application-map-manifest/0","app":"admin","hash_prefix":"#!",`+
		`"routers":[{"path":"app/routes.js","dialect":"ui-router-states/0"},{"path":"app/admin.routes.js","dialect":"ui-router-states/0"}],`+
		`"router_globs":["app/*.routes.js"],"flows":"flows",`+
		`"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"],"workflows":["e2e/workflows"],"scenarios":["e2e/scenarios"]},`+
		`"page_object_screens":[{"path":"e2e/pages/teesheet.page.ts","state":"app.clubs.teesheets"}]}`)
	m := build(t, root, commitAll(t, root, "modules"))

	mods := moduleCoverage(m)
	byName := map[string]moduleRow{}
	for _, r := range mods {
		byName[r.Module] = r
	}
	admin := byName["app/admin.routes.js"]
	if admin.Status != StatusResolved || *admin.Total != 2 || *admin.Reached != 1 || *admin.Unreached != 1 ||
		!slices.Equal(admin.UnreachedID, []string{screenID("admin", "adminAudit")}) ||
		!slices.Equal(admin.ReachedBy, []moduleCount{{"goto", 1}}) {
		t.Fatalf("admin module: %+v", admin)
	}
	base := byName["app/routes.js"]
	if base.Status != StatusUnknown || *base.Abstract != 1 || *base.UnknownN == 0 {
		t.Fatalf("base module: %+v", base)
	}
	declared := 0
	for _, s := range m.Screens {
		if s.Anchor.Path == "app/routes.js" {
			declared++
		}
	}
	if *base.Total+*base.UnknownN+*base.Abstract != declared || *base.Reached+*base.Unreached != *base.Total {
		t.Fatalf("base does not reconcile with %d screens: %+v", declared, base)
	}
	for _, u := range base.Unknown {
		if slices.Contains(base.UnreachedID, u.Screen) || u.Reason == "" {
			t.Fatalf("unknown screen counted or unexplained: %+v", u)
		}
		if s := m.screen(u.Screen); s == nil || s.Status == StatusResolved {
			t.Fatalf("listed unknown %s is not an unknown screen", u.Screen)
		}
	}
	reached := 0
	for _, s := range m.Screens {
		if s.Anchor.Path == "app/routes.js" && s.Status == StatusResolved && !s.Abstract && len(s.Specs) > 0 {
			reached++
		}
	}
	if *base.Reached != reached || reached == 0 {
		t.Fatalf("base reached %d, want %d", *base.Reached, reached)
	}
	extra := byName["app/extra.routes.js"]
	if extra.Status != StatusUnknown || extra.Total != nil || extra.Reached != nil || extra.TotalSource != "router-not-declared" {
		t.Fatalf("undeclared module: %+v", extra)
	}

	raw, err := ProjectModuleCoverage(context.Background(), m, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := ProjectModuleCoverage(context.Background(), m, Options{Full: true}); string(again) != string(raw) {
		t.Fatal("not deterministic")
	}
	var doc struct {
		Schema    string           `json:"schema"`
		Status    string           `json:"status"`
		Total     int              `json:"screens_total"`
		Reached   int              `json:"screens_reached"`
		Unreached int              `json:"screens_unreached"`
		Modules   []map[string]any `json:"modules"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Schema != ModuleCoverageSchema || doc.Status != StatusUnknown || len(doc.Modules) != 3 {
		t.Fatalf("report: %s", raw)
	}
	if doc.Total != *base.Total+2 || doc.Reached != *base.Reached+1 || doc.Reached+doc.Unreached != doc.Total {
		t.Fatalf("head totals: %s", raw)
	}
	if doc.Modules[2]["module"] != "app/routes.js" || doc.Modules[1]["screens_total"] != nil {
		t.Fatalf("module order or null counts: %s", raw)
	}
}
