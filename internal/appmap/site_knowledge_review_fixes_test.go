package appmap

import (
	"slices"
	"strings"
	"testing"
)

// SKH-V0-140: an outcome keeps the full selector identity, so assertions on two buttons that
// differ only by accessible name are distinct outcomes and never a false shared one.
func TestSKHV0_140_OutcomeKeepsSelectorName(t *testing.T) {
	_, m := skFlowsFixture(t)
	assertOn := func(id, name string) SKTest {
		at := Anchor{Path: "e2e/" + id + ".spec.ts", Start: 1, End: 9, Blob: strings.Repeat("1", 40)}
		return SKTest{ID: "test:" + id, Title: id, Path: at.Path, Complete: true, Anchor: at, Steps: []SKStep{
			{Kind: StepVisit, Target: "/orders"},
			{Kind: StepClick, Locator: &Selector{Kind: "role", Value: "button", Name: "Ship"}},
			{Kind: StepVisit, Target: "/orders/1"},
			{Kind: StepAssert, Matcher: "toBeVisible", Locator: &Selector{Kind: "role", Value: "button", Name: name}},
		}}
	}
	k := &SKModel{Tests: []SKTest{assertOn("a", "Success"), assertOn("b", "Failure")}}
	flows, _ := k.inferFlows(m, nil)
	if len(flows) != 1 || len(flows[0].Outcomes) != 0 || flows[0].OutcomeStatus != StatusUnknown {
		t.Fatalf("named assertions merged into a shared outcome: %+v", flows)
	}
	k = &SKModel{Tests: []SKTest{assertOn("a", "Success"), assertOn("b", "Success")}}
	flows, _ = k.inferFlows(m, nil)
	if len(flows) != 1 || len(flows[0].Outcomes) != 1 || flows[0].Outcomes[0].Locator != "role=button name=Success" {
		t.Fatalf("shared named outcome = %+v", flows)
	}
}

// SKH-V0-141: a Rails route helper resolves a link target only when the helper call is the whole
// argument; any trailing expression makes the target dynamic.
func TestSKHV0_141_RubyHelperConsumesWholeArgument(t *testing.T) {
	for arg, want := range map[string]string{
		"order_path":                          "order",
		"order_path(order)":                   "order",
		"order_path(order, format: :json)":    "order",
		"order_path(order) + '/edit'":         "",
		"order_path(order).sub('a', 'b')":     "",
		"order_path(order)(x)":                "",
		"order_path(order":                    "",
		"order_path(foo(1), bar(\")\"))":      "order",
		"orders_url(page: params[:page]) + x": "",
	} {
		target, _, dynamic := rubyTarget(arg)
		if want == "" && (!dynamic || target != "") || want != "" && (dynamic || target != want) {
			t.Errorf("rubyTarget(%q) = %q dynamic=%v, want %q", arg, target, dynamic, want)
		}
	}
	rb := parseRubyStructure("app/controllers/a_controller.rb", []byte("class AController\n  def update\n    redirect_to(order_path(order) + '/edit')\n  end\nend\n"))
	if sites := rubyLinks(rb.code, rb.raw); len(sites) != 1 || !sites[0].dynamic || sites[0].target != "" {
		t.Fatalf("concatenated helper target resolved: %+v", sites)
	}
}

// SKH-V0-142: a setup project selects files under its literal testDir (relative to the config's
// directory, or the config's top-level testDir), and a testIgnore or non-literal testDir leaves
// the selection UNKNOWN rather than over-selecting.
func TestSKHV0_142_SetupProjectHonorsTestDir(t *testing.T) {
	login := "import { test as setup } from '@playwright/test';\nsetup('login', async ({ page }) => {\n" +
		"  await page.context().storageState({ path: 'e2e/.auth/user.json' });\n});\n"
	spec := "import { test } from '@playwright/test';\ntest('orders', async ({ page }) => {\n  await page.goto('/orders');\n});\n"
	run := func(top, setup string) (SKSetup, skSetupDoc) {
		t.Helper()
		config := "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n" + top +
			"  projects: [\n    { name: 'setup', " + setup + "testMatch: /.*\\.ts$/ },\n" +
			"    { name: 'chromium', dependencies: ['setup'] },\n  ],\n});\n"
		root, m := railsFixture(t, func(files map[string]string) {
			files["e2e/playwright.config.ts"] = config
			files["e2e/auth/login.ts"] = login
			files["e2e/specs/orders.ts"] = spec
		})
		doc := skSetupQuery(t, root, m, []string{"e2e"}, nil)
		for _, s := range doc.Setup {
			if s.Kind == SetupProject && s.Name == "setup" {
				return s, doc
			}
		}
		t.Fatalf("no setup project in %+v", doc.Setup)
		return SKSetup{}, doc
	}
	uses := func(doc skSetupDoc, file, id string) bool {
		for _, u := range doc.Tests {
			if strings.HasPrefix(u.Test, "test:"+file+"#") {
				return slices.Contains(u.Uses, id)
			}
		}
		return false
	}
	for _, c := range []struct{ name, top, setup string }{
		{"project testDir", "", "testDir: './auth', "},
		{"top-level testDir", "  testDir: 'auth',\n", ""},
	} {
		s, doc := run(c.top, c.setup)
		writes := skSetupBy(t, skSetupIn(doc.Setup, "e2e/auth/login.ts"), SetupStorageState, "storageState", "e2e/.auth/user.json")
		if s.Status != StatusResolved || !slices.Equal(s.Requires, []string{writes.ID}) {
			t.Fatalf("%s: setup project = %+v", c.name, s)
		}
		// The spec outside testDir is a test the setup project arranges, not one of its files.
		if !uses(doc, "e2e/specs/orders.ts", s.ID) || uses(doc, "e2e/auth/login.ts", s.ID) {
			t.Fatalf("%s: spec outside testDir classified as setup: %+v", c.name, doc.Tests)
		}
	}
	for _, c := range []struct{ name, top, setup, reason string }{
		{"top-level testDir with no match", "  testDir: './nothing',\n", "", "no-matching-setup-file"},
		{"testIgnore", "", "testDir: './auth', testIgnore: /x/, ", "unsupported-test-filter"},
		{"top-level testIgnore", "  testIgnore: '**/skip/**',\n", "", "unsupported-test-filter"},
		{"dynamic testDir", "", "testDir: dir, ", "unsupported-test-filter"},
		{"escaping testDir", "", "testDir: '../../..', ", "unsupported-test-filter"},
	} {
		if s, _ := run(c.top, c.setup); s.Status != StatusUnknown || s.Reason != c.reason {
			t.Errorf("%s = %+v, want UNKNOWN %s", c.name, s, c.reason)
		}
	}
}
