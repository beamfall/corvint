package testsource

import (
	"context"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// assertionParityOf runs one link through the validity command and returns its assertion parity.
func assertionParityOf(t *testing.T, files map[string]string, current, legacy string) AssertionParity {
	t.Helper()
	root := t.TempDir()
	commitFiles(t, root, files)
	report, err := SelectedSourceValidity(OpenRepo(context.Background(), root),
		SourceValidityOptions{Links: []SeedLink{{New: current, Legacy: legacy}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.AssertionParity) != 1 {
		t.Fatalf("assertion parity = %+v", report.AssertionParity)
	}
	return report.AssertionParity[0]
}

// assertionRules lists the rule of each finding, in order.
func assertionRules(p AssertionParity) string {
	rules := []string{}
	for _, f := range p.Findings {
		if f.Severity != testvalidity.SeverityAdvisory {
			return "non-advisory " + f.Rule
		}
		rules = append(rules, f.Rule)
	}
	return strings.Join(rules, ",")
}

// TestTSAV0671AssertionTuplesRSpecParity proves RSpec expectations normalise into tuples
// (subject, matcher, class, polarity, expected) and identical sides are reported as parity
// (TSA-V0-671, TSA-V0-673).
func TestTSAV0671AssertionTuplesRSpecParity(t *testing.T) {
	spec := `RSpec.describe Order do
  it "totals" do
    order = create(:order)
    expect(order.total).to eq(5)
    expect(order.lines).not_to be_empty
    expect { order.cancel! }.to change(order, :state).from("open").to("cancelled")
    expect(order.reload.state).to eq "cancelled"
    expect(order.notes).to be_nil
  end
end
`
	p := assertionParityOf(t, map[string]string{"spec/legacy/order_spec.rb": spec, "spec/order_spec.rb": spec},
		"spec/order_spec.rb:2", "spec/legacy/order_spec.rb:2")
	if p.Status != testvalidity.AssertionParityStatusParity || assertionRules(p) != "" || len(p.Matches) != 5 {
		t.Fatalf("parity = %+v", p)
	}
	tuples := p.Sides[0].Tuples
	if got := tuples[0]; got.Subject != "order.total" || got.Matcher != "eq" || got.Class != "exact" || got.Expected != "5" || got.Framework != "rspec" {
		t.Fatalf("eq tuple = %+v", got)
	}
	if got := tuples[1]; !got.Negated || got.Class != "other" || got.Expected != "subject:order.lines" {
		t.Fatalf("negated tuple = %+v", got)
	}
	if got := tuples[2]; got.Subject != "{block}" || got.Class != "exact" || got.Expected != `subject:state from:open to:cancelled` {
		t.Fatalf("change tuple = %+v", got)
	}
	if got := tuples[4]; got.Expected != "null" || got.Class != "exact" {
		t.Fatalf("be_nil tuple = %+v", got)
	}
}

// TestTSAV0674CapybaraToPlaywrightFindings proves a Capybara legacy test against a Playwright
// migration reports each finding kind: a matched URL, a text expectation weakened to visibility of
// a located text, an unmatched legacy expectation and a new expectation without origin
// (TSA-V0-672, TSA-V0-674, TSA-V0-675).
func TestTSAV0674CapybaraToPlaywrightFindings(t *testing.T) {
	legacy := `feature "checkout" do
  scenario "places an order" do
    visit "/cart"
    click_button "Place order"
    expect(page).to have_content("Order placed")
    expect(page).to have_current_path("/orders")
    expect(page).to have_css(".flash", text: "Saved")
    expect(page).not_to have_content("Error")
  end
end
`
	current := `import { test, expect } from '@playwright/test';

test('places an order', async ({ page }) => {
  await page.goto('/cart');
  await page.getByRole('button', { name: 'Place order' }).click();
  await expect(page.getByText('Order placed')).toBeVisible();
  await expect(page).toHaveURL('/orders');
  await expect(page).not.toContainText('Error');
  await expect(page.getByTestId('count')).toHaveText('1');
});
`
	p := assertionParityOf(t, map[string]string{"spec/features/checkout_spec.rb": legacy, "e2e/checkout.spec.ts": current},
		"e2e/checkout.spec.ts:3", "spec/features/checkout_spec.rb:2")
	if p.Status != testvalidity.AssertionParityStatusDiverges {
		t.Fatalf("status = %s", p.Status)
	}
	if got := assertionRules(p); got != "ASSERTION-PARITY-WEAKENED,ASSERTION-PARITY-UNMATCHED-LEGACY,ASSERTION-PARITY-NEW-WITHOUT-ORIGIN" {
		t.Fatalf("rules = %s (%+v)", got, p.Findings)
	}
	if p.Sides[0].Tuples[0].Framework != "capybara" || p.Sides[1].Tuples[0].Framework != "playwright" {
		t.Fatalf("frameworks = %+v", p.Sides)
	}
	if weakened := p.Findings[0]; weakened.Location != "e2e/checkout.spec.ts:6" || weakened.Cause != "spec/features/checkout_spec.rb:5" {
		t.Fatalf("weakened = %+v", weakened)
	}
	if unmatched := p.Findings[1]; unmatched.Cause != "spec/features/checkout_spec.rb:7" || !strings.Contains(unmatched.Message, "Saved") {
		t.Fatalf("unmatched = %+v", unmatched)
	}
	if origin := p.Findings[2]; origin.Location != "e2e/checkout.spec.ts:9" {
		t.Fatalf("new without origin = %+v", origin)
	}
}

// TestTSAV0672JasmineChaiCypressForms proves Jasmine, Mocha/Chai and Cypress expectations
// normalise to the same tuples, and that a Cypress exact text weakened to Playwright containment
// is reported (TSA-V0-671, TSA-V0-672, TSA-V0-675).
func TestTSAV0672JasmineChaiCypressForms(t *testing.T) {
	jasmine := `describe('cart', () => {
  it('totals', () => {
    expect(cart.total).toEqual(5);
    expect(cart.open).toBeTrue();
    expect(cart.items).toContain('apple');
  });
});
`
	chai := `describe('cart', function () {
  it('totals', function () {
    expect(cart.total).to.equal(5);
    expect(cart.open).to.be.true;
    expect(cart.items).to.include('apple');
  });
});
`
	p := assertionParityOf(t, map[string]string{"test/legacy/cart.spec.js": jasmine, "test/cart.test.js": chai},
		"test/cart.test.js:2", "test/legacy/cart.spec.js:2")
	if p.Status != testvalidity.AssertionParityStatusParity || assertionRules(p) != "" {
		t.Fatalf("jasmine/chai = %+v", p)
	}
	if p.Sides[0].Tuples[1].Framework != "jasmine" || p.Sides[1].Tuples[0].Framework != "chai" {
		t.Fatalf("frameworks = %+v", p.Sides)
	}
	cypress := `describe('flash', () => {
  it('shows it', () => {
    cy.visit('/');
    cy.get('.flash').should('have.text', 'Saved').and('be.visible');
  });
});
`
	playwright := `import { test, expect } from '@playwright/test';
test('shows it', async ({ page }) => {
  await expect(page.locator('.flash')).toContainText('Saved');
  await expect(page.locator('.flash')).toBeVisible();
});
`
	p = assertionParityOf(t, map[string]string{"cypress/e2e/flash.cy.js": cypress, "e2e/flash.spec.ts": playwright},
		"e2e/flash.spec.ts:2", "cypress/e2e/flash.cy.js:2")
	if got := assertionRules(p); got != "ASSERTION-PARITY-WEAKENED" || p.Sides[0].Tuples[0].Framework != "cypress" || len(p.Sides[0].Tuples) != 2 {
		t.Fatalf("cypress = %s %+v", got, p)
	}
}

// TestTSAV0676AssertionParityAbstains proves a side the reading cannot fully follow abstains with
// a named reason and parity is never reported for it (TSA-V0-676).
func TestTSAV0676AssertionParityAbstains(t *testing.T) {
	plain := `describe('x', () => {
  it('y', () => {
    expect(a).toBe(1);
  });
});
`
	cases := []struct {
		name, legacy, reason string
	}{
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    expect_saved_flash\n  end\nend\n", assertReasonHelperUnresolved},
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    if a\n      expect(a).to eq(1)\n    end\n  end\nend\n", assertReasonConditional},
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    expect(a).to eq(1).and eq(2)\n  end\nend\n", assertReasonFormUnsupported},
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    a.should eq(1)\n  end\nend\n", assertReasonFormUnsupported},
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    visit \"/\"\n  end\nend\n", assertReasonNoneFound},
		{"spec/x_spec.rb", "describe X do\n  it \"y\" do\n    expect(a).to eq(1)\n  end\nend\n", ""},
		{"test/x.spec.js", "describe('x', () => {\n  it('y', () => {\n    if (a) { expect(a).toBe(1); }\n  });\n});\n", assertReasonConditional},
		{"test/x.spec.js", "describe('x', () => {\n  it('y', () => {\n    expectSaved(page);\n  });\n});\n", assertReasonHelperUnresolved},
		{"test/x.spec.js", "describe('x', () => {\n  it('y', () => {\n    expect(a).to.include(1).and.include(2);\n  });\n});\n", assertReasonFormUnsupported},
		{"docs/x.md", "# x\n\ny\n", assertReasonFileUnsupported},
	}
	for _, c := range cases {
		p := assertionParityOf(t, map[string]string{c.name: c.legacy, "test/new.spec.js": plain}, "test/new.spec.js:2", c.name+":2")
		if c.reason == "" {
			if p.Status != testvalidity.AssertionParityStatusParity {
				t.Fatalf("%s control: %+v", c.name, p)
			}
			continue
		}
		if p.Status != testvalidity.AssertionParityStatusNotAnalysed || p.Sides[0].Analysed || p.Sides[0].Reason != c.reason ||
			assertionRules(p) != testvalidity.RuleAssertionParityNotAnalysed || len(p.Matches) != 0 {
			t.Fatalf("%s %q: want %s, got %+v", c.name, c.legacy, c.reason, p)
		}
	}
	// A legacy example line that declares no example abstains as not found.
	p := assertionParityOf(t, map[string]string{"test/new.spec.js": plain, "test/old.spec.js": plain}, "test/new.spec.js:2", "test/old.spec.js:3")
	if p.Status != testvalidity.AssertionParityStatusNotAnalysed || p.Sides[0].Reason != assertReasonExampleNotFound {
		t.Fatalf("not found = %+v", p)
	}
}

// TestTSAV0837ValueFallbackIsNotConditional proves a `??` or `?:` that only computes a value
// before unconditional expectations does not make the side abstain (TSA-V0-837).
func TestTSAV0837ValueFallbackIsNotConditional(t *testing.T) {
	spec := "import { test, expect } from '@playwright/test';\ntest('a', async ({ page }) => {\n" +
		"  const origin = process.env.A ?? process.env.B ?? 'http://localhost:4000';\n" +
		"  const n = origin ? 1 : 2;\n  expect(n).toBe(1);\n});\n"
	p := assertionParityOf(t, map[string]string{"e2e/a.spec.ts": spec, "e2e/legacy/a.spec.ts": spec}, "e2e/a.spec.ts:2", "e2e/legacy/a.spec.ts:2")
	if strings.Contains(p.Status, "not-analysed") {
		t.Errorf("parity = %+v", p)
	}
}

// TestTSAV0844ConditionalOperandAsserts proves a `?:`, `??`, `||` or `&&` makes a side abstain as
// assertion-conditional exactly when an expectation is inside its conditionally evaluated operand,
// across brackets and braces, and that a fallback computing a value (also inside `expect(...)` or
// ended by a newline without a semicolon) leaves later expectations unconditional (TSA-V0-844).
func TestTSAV0844ConditionalOperandAsserts(t *testing.T) {
	plain := "describe('x', () => {\n  it('y', () => {\n    expect(a).toBe(1);\n  });\n});\n"
	wrap := func(body string) string {
		return "describe('x', () => {\n  it('y', () => {\n" + body + "\n  });\n});\n"
	}
	cases := []struct {
		body        string
		conditional bool
	}{
		{"    enabled ? ({ result: expect(a).toBe(1) }) : null;", true},
		{"    enabled ? [expect(a).toBe(1)] : null;", true},
		{"    const r = enabled ? null : { r: expect(a).toBe(1) };", true},
		{"    a || expect(a).toBe(1);", true},
		{"    a && expect(a).toBe(1);", true},
		{"    a ?? cy.get('#a').should('be.visible');", true},
		{"    expect(a ?? b).toBe(1);", false},
		{"    expect(a).toBe(b || 1);", false},
		{"    const o = a ?? b\n    const p = o || 'x'\n    expect(a).toBe(1)", false},
		{"    const o = enabled\n      ? 1\n      : 2\n    expect(a).toBe(o)", false},
	}
	for _, c := range cases {
		p := assertionParityOf(t, map[string]string{"test/old.spec.js": wrap(c.body), "test/new.spec.js": plain}, "test/new.spec.js:2", "test/old.spec.js:2")
		got := p.Sides[0].Reason == assertReasonConditional
		if got != c.conditional {
			t.Errorf("%q: conditional=%v, want %v (%+v)", c.body, got, c.conditional, p.Sides[0])
		}
	}
}

// TestTSAV0846RegexStaticBodyPairs proves a Cypress `cy.url().should('include', ...)` pairs with
// a Playwright `toHaveURL(/.../)` whose regular expression has a static body containing the
// legacy value, as a containment, and that a regex with metacharacters or another body does not
// pair (TSA-V0-846).
func TestTSAV0846RegexStaticBodyPairs(t *testing.T) {
	cypress := func(should string) string {
		return "describe('p', () => {\n  it('creates', () => {\n    cy.visit('/');\n    cy.url()." + should + ";\n  });\n});\n"
	}
	playwright := func(matcher string) string {
		return "import { test, expect } from '@playwright/test';\ntest('creates', async ({ page }) => {\n  await expect(page)." + matcher + ";\n});\n"
	}
	cases := []struct {
		legacy, current, status string
		weakened                bool
	}{
		{"should('include', '/p/new')", `toHaveURL(/\/p\/new/)`, testvalidity.AssertionParityStatusParity, false},
		{"should('include', '/p/new')", `toHaveURL(/^https:\/\/x\.test\/p\/new\/edit$/)`, testvalidity.AssertionParityStatusParity, false},
		{"should('eq', 'https://x.test/p/new')", `toHaveURL(/\/p\/new/)`, testvalidity.AssertionParityStatusDiverges, true},
		{"should('match', /\\/p\\/new/)", `toHaveURL('https://x.test/p/new')`, testvalidity.AssertionParityStatusParity, false},
		{"should('include', '/p/new')", `toHaveURL(/\/p\/.*/)`, testvalidity.AssertionParityStatusDiverges, false},
		{"should('include', '/p/new')", `toHaveURL(/\/q\/new/)`, testvalidity.AssertionParityStatusDiverges, false},
	}
	for _, c := range cases {
		p := assertionParityOf(t, map[string]string{"cypress/e2e/p.cy.js": cypress(c.legacy), "e2e/p.spec.ts": playwright(c.current)},
			"e2e/p.spec.ts:2", "cypress/e2e/p.cy.js:2")
		matched := len(p.Matches) == 1
		wantMatch := c.status == testvalidity.AssertionParityStatusParity || c.weakened
		if p.Status != c.status || matched != wantMatch || matched && p.Matches[0].Weakened != c.weakened {
			t.Errorf("%s vs %s: status=%s matches=%+v rules=%s, want %s weakened=%v", c.legacy, c.current, p.Status, p.Matches, assertionRules(p), c.status, c.weakened)
		}
	}
}

// TestTSAV0852IdenticalRegexIsNotWeakened proves the unanchored-regex downgrade applies to both
// sides (TSA-V0-852): identical `toHaveURL(/foo/)` expectations, or a legacy `should('match',
// /foo/)` against the same new regex, are parity, while a new regex against a legacy exact value
// is still weakened.
func TestTSAV0852IdenticalRegexIsNotWeakened(t *testing.T) {
	playwright := func(matcher string) string {
		return "import { test, expect } from '@playwright/test';\ntest('creates', async ({ page }) => {\n  await expect(page)." + matcher + ";\n});\n"
	}
	cypress := func(should string) string {
		return "describe('p', () => {\n  it('creates', () => {\n    cy.visit('/');\n    cy.url()." + should + ";\n  });\n});\n"
	}
	cases := []struct {
		legacyPath, legacy, current string
		weakened                    bool
	}{
		{"e2e/old.spec.ts", playwright(`toHaveURL(/foo/)`), `toHaveURL(/foo/)`, false},
		{"cypress/e2e/p.cy.js", cypress("should('match', /foo/)"), `toHaveURL(/foo/)`, false},
		{"e2e/old.spec.ts", playwright(`toHaveURL('foo')`), `toHaveURL(/foo/)`, true},
	}
	for _, c := range cases {
		p := assertionParityOf(t, map[string]string{c.legacyPath: c.legacy, "e2e/p.spec.ts": playwright(c.current)},
			"e2e/p.spec.ts:2", c.legacyPath+":2")
		if len(p.Matches) != 1 || p.Matches[0].Weakened != c.weakened {
			t.Errorf("%s vs %s: status=%s matches=%+v rules=%s, want weakened=%v", c.legacyPath, c.current, p.Status, p.Matches, assertionRules(p), c.weakened)
		}
	}
}
