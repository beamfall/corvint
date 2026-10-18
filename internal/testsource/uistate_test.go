package testsource

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// oneShot returns the one-shot-ui-state-assertion findings as "line:resolution" strings.
func oneShot(findings []Finding) []string {
	out := []string{}
	for _, f := range findings {
		if f.Rule == RuleOneShotUIState {
			out = append(out, strconv.Itoa(f.Line)+":"+f.Resolution)
		}
	}
	return out
}

// TestOneShotHelperForms proves direct reads and reads returned through a same-file method, an
// arrow-function property, a function and a local variable are flagged with the helper chain and
// the web-first replacement, while web-first matchers, expect.poll, toPass callbacks, unawaited
// subjects and non-UI awaits stay silent (TSA-V0-029).
func TestOneShotHelperForms(t *testing.T) {
	report := LintText("e2e/a.spec.ts", `class Panel {
  constructor(page) { this.page = page; }
  async hasBadge() {
    const shown = await this.page.getByTestId('badge').isVisible();
    return shown;
  }
  rowCount = async () => this.page.getByTestId('rows').count();
  async ready() { return this.hasBadge(); }
}
async function title(page) { return page.getByRole('heading').textContent(); }
test('panel', async ({ page, request }) => {
  const panel = new Panel(page);
  expect(await panel.hasBadge()).toBe(true);
  expect(await panel.rowCount()).toBe(2);
  expect(await panel.ready()).toBe(true);
  expect(await title(page)).toBe('Panel');
  expect(await page.getByLabel('Name').inputValue()).toBe('Ada');
  await expect(page.getByTestId('badge')).toBeVisible();
  await expect.poll(() => panel.rowCount()).toBe(2);
  await expect(async () => { expect(await panel.hasBadge()).toBe(true); }).toPass();
  expect((await request.get('/api')).ok()).toBe(true);
  expect(panel.rowCount).toBeDefined();
});`)
	if got, want := oneShot(report.Findings), []string{"13:", "14:", "15:", "16:", "17:"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("one-shot findings = %v, want %v: %+v", got, want, report.Findings)
	}
	messages := map[int]string{}
	for _, f := range report.Findings {
		if !f.Advisory {
			t.Fatalf("one-shot finding is not advisory: %+v", f)
		}
		messages[f.Line] = f.Message
	}
	for line, want := range map[int][]string{
		13: {"isVisible() read (e2e/a.spec.ts:4)", "hasBadge (e2e/a.spec.ts:3)", "toBeVisible"},
		14: {"count() read", "rowCount (e2e/a.spec.ts:7)", "toHaveCount"},
		15: {"ready (e2e/a.spec.ts:8) -> hasBadge (e2e/a.spec.ts:3)"},
		16: {"textContent() read", "title (e2e/a.spec.ts:10)", "toHaveText"},
		17: {"inputValue() read (e2e/a.spec.ts:17)", "toHaveValue"},
	} {
		for _, part := range want {
			if !strings.Contains(messages[line], part) {
				t.Errorf("line %d message %q lacks %q", line, messages[line], part)
			}
		}
	}
	if report.Blocking() != 0 {
		t.Fatalf("advisory findings counted as blocking: %d", report.Blocking())
	}
}

// TestOneShotUnknown proves an unresolved helper and a chain deeper than MaxHelperDepth yield
// UNKNOWN findings that state why, never silence (TSA-V0-030).
func TestOneShotUnknown(t *testing.T) {
	report := LintText("e2e/a.spec.ts", `async function one(p) { return two(p); }
async function two(p) { return three(p); }
async function three(p) { return four(p); }
async function four(p) { return p.getByTestId('x').isVisible(); }
test('t', async ({ page }) => {
  expect(await missingHelper(page)).toBe(true);
  expect(await one(page)).toBe(true);
  expect(await three(page)).toBe(true);
});`)
	if got, want := oneShot(report.Findings), []string{"6:UNKNOWN", "7:UNKNOWN", "8:"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("one-shot findings = %v, want %v: %+v", got, want, report.Findings)
	}
	if !strings.Contains(report.Findings[0].Message, "no definition of missingHelper in e2e/a.spec.ts") {
		t.Fatalf("unresolved message = %q", report.Findings[0].Message)
	}
	if !strings.Contains(report.Findings[1].Message, "stops after 3 hops at four (e2e/a.spec.ts:3)") {
		t.Fatalf("depth message = %q", report.Findings[1].Message)
	}
}

// TestNewRuleSuppression proves the two advisory rules honour reasoned annotations (TSA-V0-032).
func TestNewRuleSuppression(t *testing.T) {
	report := lintText("e2e/a.spec.ts", `test('shows the name', async ({ page }) => {
  // corvint-lint-allow one-shot-ui-state-assertion: the count is read after an explicit settle
  expect(await page.getByTestId('rows').count()).toBe(3);
  // corvint-lint-allow exact-text-cross-variant: both variants render the same label
  await expect(page.getByText('Ada', { exact: true })).toBeVisible();
});`, variantRuns(fixtureRevision, "expected", "unexpected"), fixtureRevision)
	if len(report.Findings) != 0 || len(report.Suppressed) != 2 {
		t.Fatalf("findings = %+v suppressed = %+v", report.Findings, report.Suppressed)
	}
}

// TestCrossVariantEvidence proves the exact-text rule names the locator when the runs split by
// project at the revision, follows a helper to an exact locator, and reports each typed
// NOT_PRODUCED reason without findings otherwise (TSA-V0-031, TSA-V0-032).
func TestCrossVariantEvidence(t *testing.T) {
	helper := `async function nameCell(page) { return page.getByRole('cell', { name: 'Ada', exact: true }); }
test.describe('people', () => {
  test('shows the name', async ({ page }) => {
    await expect(await nameCell(page)).toBeVisible();
  });
});`
	runs := []RunEvidence{{Revision: fixtureRevision[:12], Root: "e2e", Tests: []RunTest{
		{Project: "chromium", File: "a.spec.ts", TitlePath: []string{"people", "shows the name"}, Status: "expected", Executed: true},
		{Project: "mobile", File: "a.spec.ts", TitlePath: []string{"people", "shows the name"}, Status: "unexpected", Executed: true}}}}
	report := lintText("e2e/a.spec.ts", helper, runs, fixtureRevision)
	cv := report.CrossVariant
	if cv.Status != "PRODUCED" || cv.Runs != 1 || !reflect.DeepEqual(cv.Projects, []string{"chromium", "mobile"}) || len(cv.Splits) != 1 || !cv.Splits[0].ExactText {
		t.Fatalf("cross-variant = %+v", cv)
	}
	found := false
	for _, f := range report.Findings {
		if f.Rule == RuleExactTextVariant {
			found = f.Line == 4 && f.Advisory && strings.Contains(f.Message, "chromium passes and mobile fails at the same revision") &&
				strings.Contains(f.Message, "nameCell() uses getByRole")
		}
	}
	if !found {
		t.Fatalf("exact-text finding missing: %+v", report.Findings)
	}

	other := variantRuns(fixtureRevision, "expected", "unexpected")
	other[0].Revision = "fedcba9876543210"
	undeclared := variantRuns("", "expected", "unexpected")
	single := variantRuns(fixtureRevision, "expected", "unexpected")
	single[0].Tests = single[0].Tests[:1]
	for _, tc := range []struct {
		runs   []RunEvidence
		reason string
	}{{nil, ReasonNoRunEvidence}, {undeclared, ReasonRevisionNotDeclared}, {other, ReasonNoRunAtRevision}, {single, ReasonSingleProject}} {
		report := lintText("e2e/a.spec.ts", exactTextSpec, tc.runs, fixtureRevision)
		if report.CrossVariant.Status != "NOT_PRODUCED" || report.CrossVariant.Reason != tc.reason || len(report.Findings) != 0 {
			t.Errorf("%s: cross-variant = %+v findings = %+v", tc.reason, report.CrossVariant, report.Findings)
		}
	}
}

// TestOneShotLocalsAndNesting proves a typed returned local and an awaited, parenthesised returned
// local are followed, a standalone helper named like a UI-state read is resolved or UNKNOWN, and a
// return inside a nested function is not attributed to the enclosing helper (TSA-V0-029,
// TSA-V0-030).
func TestOneShotLocalsAndNesting(t *testing.T) {
	report := LintText("e2e/a.spec.ts", `class Panel {
  constructor(page) { this.badge = page.getByTestId('badge'); }
  async typed(): Promise<boolean> {
    const shown: boolean = await this.badge.isVisible();
    return shown;
  }
  async deferred() {
    const shown = this.badge.isVisible();
    return await (shown);
  }
  async nested() {
    const check = async () => { return this.badge.isVisible(); };
    [1].forEach(function () { return this.badge.count(); });
    await check();
    return true;
  }
}
async function isVisible(page) { return page.getByTestId('x').isVisible(); }
test('panel', async ({ page }) => {
  const panel = new Panel(page);
  expect(await panel.typed()).toBe(true);
  expect(await panel.deferred()).toBe(true);
  expect(await panel.nested()).toBe(true);
  expect(await isVisible(page)).toBe(true);
  expect(await isHidden(page)).toBe(true);
});`)
	if got, want := oneShot(report.Findings), []string{"21:", "22:", "24:", "25:UNKNOWN"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("one-shot findings = %v, want %v: %+v", got, want, report.Findings)
	}
	for _, f := range report.Findings {
		switch f.Line {
		case 21:
			if !strings.Contains(f.Message, "isVisible() read (e2e/a.spec.ts:4) returned through typed (e2e/a.spec.ts:3)") {
				t.Errorf("typed local message = %q", f.Message)
			}
		case 24:
			if !strings.Contains(f.Message, "returned through isVisible (e2e/a.spec.ts:18)") {
				t.Errorf("standalone helper message = %q", f.Message)
			}
		case 25:
			if !strings.Contains(f.Message, "no definition of isHidden") {
				t.Errorf("unresolved standalone helper message = %q", f.Message)
			}
		}
	}
}

// TestCrossVariantFollowing proves the exact-text rule collects and deduplicates every reachable
// exact locator, reports a missing helper, a chain past MaxHelperDepth and a non-literal exact
// option as UNKNOWN, reads quoted keys and stays silent on a literal false (TSA-V0-030,
// TSA-V0-031).
func TestCrossVariantFollowing(t *testing.T) {
	report := lintText("e2e/a.spec.ts", `async function inner(page) { return page.getByLabel('Name', { exact: true }); }
async function cells(page) {
  await page.getByRole('cell', { name: 'Ada', exact: true }).click();
  await inner(page); await inner(page);
  return page.getByText('Lovelace', { "exact": true });
}
async function one(p) { return two(p); }
async function two(p) { return three(p); }
async function three(p) { return four(p); }
async function four(p) { return p.getByText('x', { exact: true }); }
test('shows the name', async ({ page }) => {
  await expect(await cells(page)).toBeVisible();
  await missing(page);
  await one(page);
  await expect(page.getByText('a', { exact: true && false })).toBeVisible();
  await expect(page.getByText('b', { exact: flag })).toBeVisible();
  await expect(page.getByText('c', opts)).toBeVisible();
  await expect(page.getByText('d', { exact: false })).toBeVisible();
  await expect(page.getByText('e', { ...opts })).toBeVisible();
  await expect(page.getByText('f', { ...opts, exact: true })).toBeVisible();
});`, variantRuns(fixtureRevision, "expected", "unexpected"), fixtureRevision)
	got, messages := []string{}, map[int]string{}
	for _, f := range report.Findings {
		if f.Rule == RuleExactTextVariant {
			got = append(got, strconv.Itoa(f.Line)+":"+f.Resolution)
			messages[f.Line] = f.Message
		}
	}
	want := []string{"12:", "13:UNKNOWN", "14:UNKNOWN", "15:UNKNOWN", "16:UNKNOWN", "17:UNKNOWN", "19:UNKNOWN", "20:"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("exact-text findings = %v, want %v: %+v", got, want, report.Findings)
	}
	for line, part := range map[int]string{
		12: "cells() uses getByLabel(..., { exact: true }) at e2e/a.spec.ts:1, cells() uses getByRole(..., { exact: true }) at e2e/a.spec.ts:3, cells() uses getByText(..., { exact: true }) at e2e/a.spec.ts:5 is a likely cause",
		13: "missing(): no definition of missing in e2e/a.spec.ts",
		14: "one(): helper-following stops after 3 hops at four (e2e/a.spec.ts:9)",
		15: "is not a literal (exact:true&&false)",
		16: "is not a literal (exact:flag)",
		17: "is not a literal (options opts)",
		19: "is not a literal (...opts)",
	} {
		if !strings.Contains(messages[line], part) {
			t.Errorf("line %d message %q lacks %q", line, messages[line], part)
		}
	}
	if n := strings.Count(messages[12], "getByLabel"); n != 1 {
		t.Errorf("the twice-reached locator is listed %d times: %q", n, messages[12])
	}
	if !report.CrossVariant.Splits[0].ExactText {
		t.Errorf("split does not record the exact-text cause: %+v", report.CrossVariant)
	}
}
