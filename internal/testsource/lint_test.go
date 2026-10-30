package testsource

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// fixtureRevision is the revision the cross-variant fixtures lint and their runs declare.
const fixtureRevision = "0123456789abcdef0123456789abcdef01234567"

// variantRuns is Playwright evidence for e2e/a.spec.ts :: "shows the name" in projects a and b.
func variantRuns(revision, statusA, statusB string) []RunEvidence {
	return []RunEvidence{{Revision: revision, Tests: []RunTest{
		{Project: "a", File: "a.spec.ts", TitlePath: []string{"shows the name"}, Status: statusA, Executed: true},
		{Project: "b", File: "a.spec.ts", TitlePath: []string{"shows the name"}, Status: statusB, Executed: true}}}}
}

const exactTextSpec = `test('shows the name', async ({ page }) => {
  await expect(page.getByText('Ada Lovelace', { exact: true })).toHaveCount(1);
});`

// lintFixtures holds, for every rule, one source that triggers it and one that does not, with the
// run evidence the cross-variant rule joins (TSA-V0-013, TSA-V0-014, TSA-V0-029, TSA-V0-031).
var lintFixtures = map[string]struct {
	positive, negative string
	positiveRuns       []RunEvidence
	negativeRuns       []RunEvidence
}{
	RuleNoAssertion: {
		positive: `test('opens the page', async ({ page }) => {
  await page.goto('/');
});`,
		negative: `test('shows the total', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('total')).toHaveText('42');
});`,
	},
	RuleWeakAssertionOnly: {
		positive: `test('logs in', async ({ page }) => {
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page).toHaveURL('/home');
  await expect(page.getByText('Welcome')).toBeVisible();
});`,
		negative: `test('logs in', async ({ page }) => {
  await expect(page).toHaveURL('/home');
  await expect(page.getByTestId('user')).toHaveText('Ada');
});`,
	},
	RuleConditional: {
		positive: `test('banner', async ({ page }) => {
  if (await page.getByText('Promo').isVisible()) {
    await expect(page.getByText('Promo')).toHaveText('Sale');
  }
});`,
		negative: `test('banner', async ({ page }) => {
  for (const name of ['a', 'b']) {
    await expect(page.getByText(name)).toHaveText(name);
  }
});`,
	},
	RuleSwallowedWait: {
		positive: `test('saves', async ({ page }) => {
  await page.waitForResponse('**/save').catch(() => {});
  await expect(page.getByTestId('status')).toHaveText('Saved');
});`,
		negative: `test('saves', async ({ page }) => {
  await page.waitForResponse('**/save').catch((error) => { throw new Error('save never answered', { cause: error }); });
  await expect(page.getByTestId('status')).toHaveText('Saved');
});`,
	},
	RuleFixedWait: {
		positive: `test('loads', async ({ page }) => {
  await page.goto('/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(500);
  await expect(page.getByTestId('list')).toHaveCount(3);
});`,
		negative: `test('loads', async ({ page }) => {
  test.setTimeout(60000);
  await page.goto('/');
  await expect(page.getByTestId('list')).toHaveCount(3);
});`,
	},
	RuleTautological: {
		positive: `test('math', async () => {
  const total = 4;
  expect(true).toBe(true);
  expect(total).toBe(total);
});`,
		negative: `test('math', async () => {
  const total = add(2, 2);
  expect(total).toBe(4);
});`,
	},
	RuleBrittleSelector: {
		positive: `test('menu', async ({ page }) => {
  await page.locator('//div[@id="menu"]/a').click();
  await page.locator('ul > li:nth-child(2)').click();
  await page.click('button.btn-primary');
  await page.getByRole('row').nth(3).click();
  await expect(page.getByTestId('menu')).toHaveText('Open');
});`,
		negative: `test('menu', async ({ page }) => {
  await page.getByRole('button', { name: 'Menu' }).click();
  await page.locator('[data-testid="menu.item"]').click();
  await page.getByLabel('Email').fill('ada.lovelace@example.com');
  await expect(page.getByTestId('menu')).toHaveText('Open');
});`,
	},
	RuleSkipOnly: {
		positive: `test.only('focused', async ({ page }) => {
  await expect(page).toHaveTitle('x');
});
test.skip('later', async ({ page }) => {
  await expect(page).toHaveTitle('x');
});
test('flaky on webkit', async ({ page, browserName }) => {
  test.skip(browserName === 'webkit');
  await expect(page).toHaveTitle('x');
});`,
		negative: `test('flaky on webkit', async ({ page, browserName }) => {
  test.skip(browserName === 'webkit', 'webkit drops the drag event, tracked in #12');
  await expect(page).toHaveTitle('x');
});`,
	},
	RuleOneShotUIState: {
		positive: `class Panel {
  constructor(page) { this.badge = page.getByTestId('badge'); }
  async isSomethingPresent() {
    return await this.badge.isVisible();
  }
}
test('panel', async ({ page }) => {
  const panel = new Panel(page);
  expect(await page.getByTestId('rows').count()).toBe(3);
  expect(await panel.isSomethingPresent()).toBe(true);
});`,
		negative: `test('panel', async ({ page }) => {
  await expect(page.getByTestId('badge')).toBeVisible();
  await expect(page.getByTestId('rows')).toHaveCount(3);
  await expect.poll(async () => await page.getByTestId('rows').count()).toBe(3);
  await expect(async () => {
    expect(await page.getByTestId('badge').isVisible()).toBe(true);
  }).toPass();
});`,
	},
	RuleExactTextVariant: {
		positive:     exactTextSpec,
		positiveRuns: variantRuns(fixtureRevision, "expected", "unexpected"),
		negative:     exactTextSpec,
		negativeRuns: variantRuns(fixtureRevision, "expected", "expected"),
	},
}

func rulesOf(findings []Finding) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		out[f.Rule]++
	}
	return out
}

// TestLintRuleFixtures proves every rule fires on its positive fixture and stays silent on its
// negative fixture (TSA-V0-013, TSA-V0-014).
func TestLintRuleFixtures(t *testing.T) {
	if len(lintFixtures) != len(Rules) {
		t.Fatalf("fixtures cover %d of %d rules", len(lintFixtures), len(Rules))
	}
	for _, rule := range Rules {
		fixture := lintFixtures[rule]
		positive := lintText("e2e/a.spec.ts", fixture.positive, fixture.positiveRuns, fixtureRevision)
		if rulesOf(positive.Findings)[rule] == 0 {
			t.Errorf("%s: positive fixture did not fire: %+v", rule, positive.Findings)
		}
		negative := lintText("e2e/a.spec.ts", fixture.negative, fixture.negativeRuns, fixtureRevision)
		if len(negative.Findings) != 0 {
			t.Errorf("%s: negative fixture fired: %+v", rule, negative.Findings)
		}
		if len(positive.Uncertain)+len(negative.Uncertain) != 0 {
			t.Errorf("%s: fixture lexed uncertainly", rule)
		}
	}
}

// TestLintRuleDetails pins the finding counts and lines of the multi-pattern fixtures (TSA-V0-013).
func TestLintRuleDetails(t *testing.T) {
	brittle := LintText("e2e/a.spec.ts", lintFixtures[RuleBrittleSelector].positive)
	if got := rulesOf(brittle.Findings)[RuleBrittleSelector]; got != 4 {
		t.Fatalf("brittle-selector findings = %d, want 4: %+v", got, brittle.Findings)
	}
	skip := LintText("e2e/a.spec.ts", lintFixtures[RuleSkipOnly].positive)
	lines := []int{}
	for _, f := range skip.Findings {
		lines = append(lines, f.Line)
	}
	if !reflect.DeepEqual(lines, []int{1, 4, 8}) {
		t.Fatalf("skip-only lines = %v, want [1 4 8]: %+v", lines, skip.Findings)
	}
	fixed := LintText("e2e/a.spec.ts", lintFixtures[RuleFixedWait].positive)
	if got := rulesOf(fixed.Findings)[RuleFixedWait]; got != 2 {
		t.Fatalf("fixed-wait findings = %d, want 2", got)
	}
	tryCatch := LintText("e2e/a.spec.ts", `test('t', async ({ page }) => {
  try {
    await page.waitForSelector('[data-testid=done]');
  } catch (e) {
    console.log(e);
  }
  try {
    await expect(page.getByTestId('x')).toHaveText('y');
  } catch {
    throw new Error('x');
  }
});`)
	got := rulesOf(tryCatch.Findings)
	// The second catch always rethrows, so its try's assertion is not conditional (TSA-V0-652).
	if got[RuleSwallowedWait] != 1 || got[RuleConditional] != 0 {
		t.Fatalf("try/catch findings = %+v", tryCatch.Findings)
	}
}

// TestLintSuppression proves a reasoned annotation suppresses exactly the named rule on its target
// line and is echoed; one without a reason or with an unknown rule suppresses nothing
// (TSA-V0-015).
func TestLintSuppression(t *testing.T) {
	report := LintText("e2e/a.spec.ts", `test('t', async ({ page }) => {
  await page.waitForTimeout(100); // corvint-lint-allow fixed-wait: the animation has no observable end state
  // corvint-lint-allow brittle-selector: third-party widget exposes no role or test id
  await page.locator('.vendor-widget').click();
  await page.waitForTimeout(200); // corvint-lint-allow fixed-wait
  await page.locator('.other').click(); // corvint-lint-allow fixed-wait: wrong rule named
  await page.locator('.third').click(); // corvint-lint-allow made-up-rule: anything
  await expect(page.getByTestId('x')).toHaveText('y');
});`)
	findings := []string{}
	for _, f := range report.Findings {
		findings = append(findings, f.Rule+"@"+strconv.Itoa(f.Line))
	}
	if want := []string{"fixed-wait@5", "brittle-selector@6", "brittle-selector@7"}; !reflect.DeepEqual(findings, want) {
		t.Fatalf("findings = %v, want %v", findings, want)
	}
	suppressed := []string{}
	for _, f := range report.Suppressed {
		suppressed = append(suppressed, f.Rule+"@"+strconv.Itoa(f.Line))
	}
	if want := []string{"fixed-wait@2", "brittle-selector@4"}; !reflect.DeepEqual(suppressed, want) {
		t.Fatalf("suppressed = %v, want %v", suppressed, want)
	}
	if len(report.Suppressions) != 5 {
		t.Fatalf("suppressions echoed = %d, want 5: %+v", len(report.Suppressions), report.Suppressions)
	}
	byLine := map[int]Suppression{}
	for _, s := range report.Suppressions {
		byLine[s.Line] = s
	}
	if s := byLine[3]; !s.Valid || !s.Used || s.Target != 4 || s.Reason == "" {
		t.Fatalf("own-line suppression = %+v", s)
	}
	if s := byLine[5]; s.Valid || s.Used || s.Problem != "missing-reason" {
		t.Fatalf("reasonless suppression = %+v", s)
	}
	if s := byLine[6]; !s.Valid || s.Used {
		t.Fatalf("wrong-rule suppression = %+v", s)
	}
	if s := byLine[7]; s.Valid || s.Problem != "unknown-rule" {
		t.Fatalf("unknown-rule suppression = %+v", s)
	}
}

// TestLintTextDeterministic proves the report encodes byte-identically across runs (TSA-V0-016).
func TestLintTextDeterministic(t *testing.T) {
	var first []byte
	for range 5 {
		text := strings.Join([]string{lintFixtures[RuleBrittleSelector].positive, lintFixtures[RuleSkipOnly].positive}, "\n")
		encoded, err := json.Marshal(LintText("e2e/a.spec.ts", text))
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = encoded
		} else if string(first) != string(encoded) {
			t.Fatal("lint report is not deterministic")
		}
	}
}
