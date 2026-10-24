package appmap

import (
	"slices"
	"strings"
	"testing"
)

// SKH-V0-177: how answers with the variation whose own titles and steps carry the goal's terms, so
// "ship with a note" drafts from the note test and keeps its fill step; a goal every variation
// matches equally keeps the representative variation (TestSKHV0_143).
func TestSKHV0_177_HowPicksGoalVariation(t *testing.T) {
	root, m := skHowFixture(t)
	doc, raw := skHow(t, root, m, "ship with a note", nil, nil)
	if doc.Candidate == nil {
		t.Fatalf("no candidate: %s", raw)
	}
	fill := false
	for _, s := range doc.Steps {
		fill = fill || s.Kind == StepFill
	}
	if !fill {
		t.Fatalf("note variation not chosen; steps = %+v", doc.Steps)
	}
	d, draftRaw := skDraft(t, root, m, "ship with a note", nil)
	if !slices.Contains(d.Lines, "test('ships with a note', async ({ page }) => {") {
		t.Fatalf("draft copies another variation's test: %s", draftRaw)
	}
}

// SKH-V0-178: the draft guard goes after the line that opens the callback body of a multi-line
// test declaration, never inside its argument list; a test whose body opener is not located
// abstains.
func TestSKHV0_178_DraftGuardAfterBodyOpener(t *testing.T) {
	multi := strings.Replace(skFlowSpecs, "test('ships another order', async ({ page }) => {",
		"test(\n  'ships another order',\n  async ({ page }) => {", 1)
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = multi
		files["app/views/orders/index.html.erb"] = "<%= link_to \"Open\", order_path(@order) %>\n"
	})
	d, raw := skDraft(t, root, m, "ship an order", nil)
	want := []string{"test(", "  'ships another order',", "  async ({ page }) => {", DraftGuardLine, "  await page.goto('/orders');"}
	if d.Status != HowComplete || d.GuardLine != 5 || len(d.Lines) < 6 || !slices.Equal(d.Lines[1:6], want) {
		t.Fatalf("multi-line declaration guard = %s", raw)
	}
	same := "test('ships another order', async ({ page }) => { await page.goto('/orders');"
	inline := strings.Replace(skFlowSpecs, "test('ships another order', async ({ page }) => {\n  await page.goto('/orders');", same, 1)
	root, m = railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = inline
		files["app/views/orders/index.html.erb"] = "<%= link_to \"Open\", order_path(@order) %>\n"
	})
	if d, raw = skDraft(t, root, m, "ship an order", nil); slices.Contains(d.Lines, same) {
		t.Fatalf("same-line body drafted instead of abstaining: %s", raw)
	}
	if d.Status != HowAbstained || !slices.Contains(skGapCodes(d.Gaps), GapSourceFramework) {
		t.Fatalf("same-line body = %s", raw)
	}
}

// SKH-V0-179: a setup project whose file selection is UNKNOWN (an unsupported testMatch glob)
// still arranges the dependent project's tests, so how reports the setup gap rather than an
// answer with the dependency silently dropped.
func TestSKHV0_179_UnknownSetupProjectStaysAGap(t *testing.T) {
	config := "import { defineConfig } from '@playwright/test';\nexport default defineConfig({\n  projects: [\n" +
		"    { name: 'setup', testMatch: '**/*.setup.ts' },\n    { name: 'chromium', dependencies: ['setup'] },\n  ],\n});\n"
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = skFlowSpecs
		files["e2e/playwright.config.ts"] = config
		files["e2e/auth/login.setup.ts"] = "import { test as setup } from '@playwright/test';\nsetup('login', async ({ page }) => {\n" +
			"  await page.context().storageState({ path: 'e2e/.auth/user.json' });\n});\n"
		files["app/views/orders/index.html.erb"] = "<%= link_to \"Open\", order_path(@order) %>\n"
	})
	doc := skSetupQuery(t, root, m, []string{"e2e"}, nil)
	var project SKSetup
	for _, s := range doc.Setup {
		if s.Kind == SetupProject && s.Name == "setup" {
			project = s
		}
	}
	if project.Status != StatusUnknown {
		t.Fatalf("setup project = %+v", project)
	}
	used := false
	for _, u := range doc.Tests {
		used = used || strings.HasPrefix(u.Test, "test:e2e/specs/flows.spec.ts#") && slices.Contains(u.Uses, project.ID)
	}
	if !used {
		t.Fatalf("UNKNOWN setup project dropped from the spec tests: %+v", doc.Tests)
	}
	how, raw := skHow(t, root, m, "ship an order", nil, nil)
	if how.Status == HowComplete || !slices.Contains(skGapCodes(how.Gaps), GapNoSetup) {
		t.Fatalf("how hides the UNKNOWN setup dependency: %s", raw)
	}
}

// SKH-V0-111: static edges are read from markup and code, never from HTML or ERB comments, and a
// navigation call inside a string literal or after an inline comment is not a link.
func TestSKHV0_111_CommentedLinksAreNotEdges(t *testing.T) {
	if s := templateLinks(maskComments("<!-- <a href=\"/orders/1\">Open</a> -->\n<a href=\"/orders\">All</a>\n", "<!--", "-->")); len(s) != 1 || s[0].target != "/orders" || s[0].line != 2 {
		t.Fatalf("commented anchor read: %+v", s)
	}
	for line, want := range map[string]int{
		"router.push('/orders');":                     1,
		"const s = \"router.push('/orders')\";":       0,
		"x(); // router.push('/orders')":              0,
		"x(); /* router.push('/a') */":                0,
		"/* note */ router.push('/orders');":          1,
		"const s = 'it\\'s'; router.push('/orders');": 1,
		"const s = `navigate('/orders')`;":            0,
		"const s = 'a'; navigate('/orders');":         1,
	} {
		if got := len(scriptLinks(line)); got != want {
			t.Errorf("scriptLinks(%q) = %d sites, want %d", line, got, want)
		}
	}
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		files["app/views/orders/index.html.erb"] = "<!--\n  <a href=\"/orders/1\">Open</a>\n-->\n<%# link_to \"Open\", order_path(@order) %>\n<a href=\"/orders\">All</a>\n"
	})
	doc, raw := skLinksOf(t, root, m, "app/views")
	if len(doc.Edges) != 1 || doc.Edges[0].Target != "/orders" || doc.Edges[0].Anchor.Start != 5 {
		t.Fatalf("commented links became edges: %s", raw)
	}
}
