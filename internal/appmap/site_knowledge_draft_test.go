package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

func skDraft(t *testing.T, root string, m *Map, goal string, conditions []string) (SKDraft, []byte) {
	t.Helper()
	o := Options{Root: root, Full: true}
	raw, err := HowDraft(context.Background(), m, []string{"e2e", "app/views"}, goal, conditions, nil, nil, o)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := HowDraft(context.Background(), m, []string{"e2e", "app/views"}, goal, conditions, nil, nil, o); !bytes.Equal(raw, again) {
		t.Fatal("how draft is not byte-stable")
	}
	var doc struct {
		Schema string  `json:"schema"`
		Draft  SKDraft `json:"draft"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Schema != HowSchema {
		t.Fatalf("%v\n%s", err, raw)
	}
	return doc.Draft, raw
}

// SKH-V0-153 SKH-V0-154 SKH-V0-155 SKH-V0-157 SKH-V0-160: a COMPLETE answer drafts the primary
// test's file copied line for line without the file's other tests, guarded by test.fixme after the
// declaration (the flow's representative test here is the second of three), beside the primary test; every line maps to its source line and blob, the digest
// covers the lines, and the read leaves the repository unchanged.
func TestSKHV0_153_HowDraftCopiesPrimaryTest(t *testing.T) {
	root, m := skHowFixture(t)
	before := skRepoState(t, root)
	d, raw := skDraft(t, root, m, "ship an order", nil)
	want := []string{
		"import { test, expect } from '@playwright/test';",
		"test('ships another order', async ({ page }) => {",
		DraftGuardLine,
		"  await page.goto('/orders');",
		"  await page.getByRole('button', { name: 'Ship' }).click();",
		"  await page.goto('/orders/1');",
		"  await expect(page.getByText('Shipped')).toBeVisible();",
		"});",
	}
	if d.Status != HowComplete || len(d.Gaps) != 0 || strings.Join(d.Lines, "\n") != strings.Join(want, "\n") || d.GuardLine != 3 {
		t.Fatalf("draft = %s", raw)
	}
	if !strings.HasPrefix(d.ProposedPath, "e2e/specs/how-") || !strings.HasSuffix(d.ProposedPath, ".spec.ts") || !strings.Contains(d.Label, "not accepted intent") {
		t.Fatalf("path/label = %+v", d)
	}
	if d.Digest != digest([]byte(strings.Join(want, "\n")+"\n")) || len(d.Evidence) != len(want) {
		t.Fatalf("digest/evidence = %s", raw)
	}
	for i, ev := range d.Evidence {
		if ev.Line != i+1 {
			t.Fatalf("evidence order %+v", ev)
		}
		switch {
		case i == 2:
			if ev.Basis != DraftBasisGuard || ev.Path != "" {
				t.Fatalf("guard evidence %+v", ev)
			}
		case ev.Basis != DraftBasisCopied || ev.Path != "e2e/specs/flows.spec.ts" || ev.Source == 0 || len(ev.Blob) != 40:
			t.Fatalf("copied evidence %+v", ev)
		}
	}
	if after := skRepoState(t, root); after != before {
		t.Fatal("how draft changed the repository")
	}
}

// SKH-V0-156 SKH-V0-158: an opaque step, an unresolved condition and an arranged condition each
// become a marked TODO line; the draft is INCOMPLETE, keeps the guard, and copies the setup's
// test.use line verbatim rather than synthesising one.
func TestSKHV0_156_HowDraftMarksTodos(t *testing.T) {
	root, m := skHowFixture(t)
	d, raw := skDraft(t, root, m, "archive an invoice", []string{"e2e/.auth/user.json", "a refund exists"})
	if d.Status != HowIncomplete || d.GuardLine == 0 || d.Lines[d.GuardLine-1] != DraftGuardLine {
		t.Fatalf("draft = %s", raw)
	}
	todos := map[string]int{}
	for _, ev := range d.Evidence {
		if ev.Basis == DraftBasisTodo {
			todos[ev.Gap]++
			if !strings.Contains(d.Lines[ev.Line-1], "// TODO "+ev.Gap+": ") {
				t.Fatalf("unmarked todo line %q", d.Lines[ev.Line-1])
			}
		}
	}
	if todos[GapNoPageObject] != 1 || todos[GapNoSetup] != 2 {
		t.Fatalf("todos = %v\n%s", todos, raw)
	}
	text := strings.Join(d.Lines, "\n")
	if !strings.Contains(text, "test.use({ storageState: 'e2e/.auth/user.json' });") || !strings.Contains(text, "  await archiveIt(page);") {
		t.Fatalf("lines = %s", text)
	}
}

// SKH-V0-159: a goal the answer abstains on drafts nothing, and a primary test mined from another
// framework abstains as source-framework-unsupported naming the test.
func TestSKHV0_159_HowDraftAbstains(t *testing.T) {
	root, m := skHowFixture(t)
	d, raw := skDraft(t, root, m, "launch the rocket", nil)
	if d.Status != HowAbstained || len(d.Lines) != 0 || len(d.Gaps) != 1 || d.Gaps[0].Code != GapNoRoute {
		t.Fatalf("no-route draft = %s", raw)
	}
	a := &howAnswer{test: &SKTest{ID: "test:spec/system/orders_spec.rb#3", Framework: "capybara", Path: "spec/system/orders_spec.rb",
		Anchor: Anchor{Path: "spec/system/orders_spec.rb", Start: 3, End: 9}}, k: &SKModel{}}
	d, err := a.draft(context.Background(), root, "ship")
	if err != nil || d.Status != HowAbstained || len(d.Gaps) != 1 || d.Gaps[0].Code != GapSourceFramework ||
		!strings.Contains(d.Gaps[0].Detail, "orders_spec.rb") || d.Gaps[0].Anchor == nil || len(d.Lines) != 0 {
		t.Fatalf("framework draft = %+v %v", d, err)
	}
	a.test = &SKTest{ID: "test:e2e/a.spec.ts#2", Framework: "playwright", Path: "e2e/a.spec.ts"}
	a.setup = []HowSetup{{Setup: "setup:spec/support/login.rb#1", Kind: SetupStorageState, Anchor: Anchor{Path: "spec/support/login.rb", Start: 1}}}
	if d, _ := a.draft(context.Background(), root, "ship"); d.Status != HowAbstained || d.Gaps[0].Code != GapSourceFramework || !strings.Contains(d.Gaps[0].Detail, "login.rb") {
		t.Fatalf("setup framework draft = %+v", d)
	}
}

// SKH-V0-157: a secret-shaped source line is never copied; it becomes a secret-screened TODO.
func TestSKHV0_157_HowDraftScreensSecrets(t *testing.T) {
	secret := "password=" + strings.Repeat("Zx9", 8)
	if !secretscreen.MatchString(secret) {
		t.Fatal("fixture line is not secret-shaped")
	}
	root, m := railsFixture(t, func(files map[string]string) {
		files["config/routes.rb"] = skEdgeRoutes
		delete(files, "e2e/specs/orders.spec.ts")
		files["e2e/specs/flows.spec.ts"] = strings.ReplaceAll(skFlowSpecs, "  await page.goto('/orders/1');\n",
			"  await page.goto('/orders/1');\n  const login = '"+secret+"';\n")
	})
	d, raw := skDraft(t, root, m, "ship an order", nil)
	if d.Status != HowIncomplete || strings.Contains(string(raw), secret) {
		t.Fatalf("secret copied: %s", raw)
	}
	found := false
	for _, ev := range d.Evidence {
		found = found || ev.Gap == GapSecretScreened && ev.Basis == DraftBasisTodo
	}
	if !found {
		t.Fatalf("no secret-screened todo: %s", raw)
	}
}
