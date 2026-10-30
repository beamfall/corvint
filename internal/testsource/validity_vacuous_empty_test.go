package testsource

import (
	"strings"
	"testing"
)

// TestVacuousEmptyEqualityReported: a count or equality between two collection-derived operands
// with no non-empty precondition is reported, suggesting one (TSA-V0-172, TSA-V0-173).
func TestVacuousEmptyEqualityReported(t *testing.T) {
	src := `test('lists rows', async ({ page, request }) => {
  const served = await (await request.get('/api/rows')).json();
  const rows = page.locator('tr');
  await expect(rows).toHaveCount(served.length);
});
test('equal lengths', async () => {
  expect(shown.length).toBe(served.length);
  expect(await rows.count()).toEqual(served.size);
});
`
	r := ValidityFindings("e2e/rows.spec.ts", src)
	wantFindings(t, "reported", r,
		"VACUOUS-EMPTY-EQUALITY advisory e2e/rows.spec.ts:4 e2e/rows.spec.ts:4 lists rows",
		"VACUOUS-EMPTY-EQUALITY advisory e2e/rows.spec.ts:7 e2e/rows.spec.ts:7 equal lengths",
		"VACUOUS-EMPTY-EQUALITY advisory e2e/rows.spec.ts:8 e2e/rows.spec.ts:8 equal lengths",
	)
	if len(r.Findings) > 0 && (r.Findings[0].Test != "e2e/rows.spec.ts:1" || !strings.Contains(r.Findings[0].Message, "expect(served.length).toBeGreaterThan(0)")) {
		t.Errorf("finding %+v", r.Findings[0])
	}
}

// TestVacuousEmptyEqualityNegatives: a non-empty precondition on either collection (a lower bound,
// a positive count, a negated empty assertion, a visible member), a literal count, and a
// non-collection operand are not reported (TSA-V0-173).
func TestVacuousEmptyEqualityNegatives(t *testing.T) {
	src := `test('bound', async ({ page }) => {
  expect(served.length).toBeGreaterThan(0);
  await expect(page.locator('tr')).toHaveCount(served.length);
});
test('literal count', async ({ page }) => {
  await expect(page.locator('tr')).toHaveCount(3);
  expect(shown.length).toBe(total);
});
test('visible member', async ({ page }) => {
  const rows = page.locator('tr');
  await expect(rows.first()).toBeVisible();
  await expect(rows).toHaveCount(served.length);
});
test('negated empty', async () => {
  expect(served).not.toHaveLength(0);
  await expect(rows).toHaveCount(served.length);
});
test('seeded count', async () => {
  expect(shown).toHaveLength(2);
  expect(shown.length).toBe(served.length);
});
test('negated equality', async () => {
  expect(shown.length).not.toBe(served.length);
});
`
	wantFindings(t, "negatives", ValidityFindings("e2e/ok.spec.ts", src))
}

// TestVacuousEmptyEqualityOutsideTest: an assertion outside any test declaration, for example in a
// helper, is not reported, since no test owns it.
func TestVacuousEmptyEqualityOutsideTest(t *testing.T) {
	src := `async function same(rows, served) { await expect(rows).toHaveCount(served.length); }
`
	wantFindings(t, "helper", ValidityFindings("e2e/helper.spec.ts", src))
}

// TestVacuousEmptyEqualityEmptyText: `toHaveText([])` or an empty text passes on an empty
// collection, so it is not a non-empty precondition (TSA-V0-173).
func TestVacuousEmptyEqualityEmptyText(t *testing.T) {
	src := `test('empty text', async ({ page }) => {
  const rows = page.locator('tr');
  await expect(rows).toHaveText([]);
  await expect(rows).toHaveCount(served.length);
});
`
	wantFindings(t, "empty text", ValidityFindings("e2e/empty.spec.ts", src),
		"VACUOUS-EMPTY-EQUALITY advisory e2e/empty.spec.ts:4 e2e/empty.spec.ts:4 empty text",
	)
}
