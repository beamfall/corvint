package testsource

import (
	"strings"
	"testing"
)

// TestUnstubbedOutboundInputReported: a test input naming a stubbed integration parameter with a
// value no stub matches is reported at the input line, citing the nearest stubbed value's stub
// line; a value a glob stub covers is not (TSA-V0-169, TSA-V0-170).
func TestUnstubbedOutboundInputReported(t *testing.T) {
	src := `import { test, expect } from '@playwright/test';
test.beforeEach(async ({ page }) => {
  await page.route('**/api/lookup?name=Acme*', (r) => r.fulfill({ json: {} }));
  await page.route('**/api/lookup?name=Globex', (r) => r.fulfill({ json: {} }));
});
test('creates a customer', async ({ page, request }) => {
  await createCustomer(page, { name: 'Globox' });
  await createCustomer(page, { name: 'Acme Corp', email: 'a@b.c' });
  await request.get('/api/lookup?name=Acmf');
  await page.goto('/customers?name=Initech');
});
`
	r := ValidityFindings("e2e/customer.spec.ts", src)
	wantFindings(t, "playwright", r,
		"UNSTUBBED-OUTBOUND-INPUT advisory e2e/customer.spec.ts:7 e2e/customer.spec.ts:4 creates a customer",
		"UNSTUBBED-OUTBOUND-INPUT advisory e2e/customer.spec.ts:9 e2e/customer.spec.ts:3 creates a customer",
	)
	if len(r.Findings) == 2 && (r.Findings[0].Test != "e2e/customer.spec.ts:6" || !strings.Contains(r.Findings[0].Message, `nearest stubbed value is "Globex"`) || !strings.Contains(r.Findings[1].Message, `nearest stubbed value is "Acme*"`)) {
		t.Errorf("findings %+v", r.Findings)
	}

}

// TestUnstubbedOutboundInputCoverage: an MSW handler covers every query value at its path (MSW
// ignores the query when matching), a catch-all route covers every request, and a brace
// alternative in a route glob covers each alternative (TSA-V0-169).
func TestUnstubbedOutboundInputCoverage(t *testing.T) {
	msw := `const handlers = [http.get('https://crm.example/lookup?name=Acme', () => HttpResponse.json({}))];
test('imports', async ({ page }) => {
  await page.route('https://crm.example/lookup?name=Acme', (r) => r.fulfill({}));
  await importCustomers([{ name: 'Initech' }]);
});
`
	wantFindings(t, "msw", ValidityFindings("src/import.test.ts", msw))
	catchAll := `test.beforeEach(async ({ page }) => {
  await page.route('**/lookup?name=Acme', (r) => r.fulfill({}));
  await page.route('**/*', (r) => r.fulfill({}));
});
test('x', async ({ page }) => { await page.goto('https://example.test/lookup?name=Globex'); });
`
	wantFindings(t, "catch-all", ValidityFindings("e2e/all.spec.ts", catchAll))
	braces := `test.beforeEach(async ({ page }) => {
  await page.route('**/lookup?name={Acme,Globex}', (r) => r.fulfill({}));
});
test('x', async ({ page }) => {
  await page.goto('https://example.test/lookup?name=Acme');
  await page.goto('https://example.test/lookup?name=Initech');
});
`
	wantFindings(t, "braces", ValidityFindings("e2e/braces.spec.ts", braces),
		"UNSTUBBED-OUTBOUND-INPUT advisory e2e/braces.spec.ts:6 e2e/braces.spec.ts:2 x",
	)
}

// TestUnstubbedOutboundInputAbstains: a matcher that is not a literal string, an unconstrained
// wildcard stub, and a file without stubs give no finding (TSA-V0-171).
func TestUnstubbedOutboundInputAbstains(t *testing.T) {
	dynamic := `test.beforeEach(async ({ page }) => {
  await page.route('**/api/lookup?name=Acme', (r) => r.fulfill({}));
  await page.route(lookupPattern, (r) => r.fulfill({}));
});
test('x', async ({ page }) => { await createCustomer(page, { name: 'Initech' }); });
`
	wantFindings(t, "dynamic matcher", ValidityFindings("e2e/dynamic.spec.ts", dynamic))
	wildcard := `test.beforeEach(async ({ page }) => {
  await page.route('**/api/lookup?name=Acme', (r) => r.fulfill({}));
  await page.route('**/api/lookup*', (r) => r.fulfill({}));
});
test('x', async ({ page }) => { await createCustomer(page, { name: 'Initech' }); });
`
	wantFindings(t, "wildcard stub", ValidityFindings("e2e/wildcard.spec.ts", wildcard))
	none := `test('x', async ({ page }) => { await createCustomer(page, { name: 'Initech' }); });
`
	wantFindings(t, "no stubs", ValidityFindings("e2e/none.spec.ts", none))
}
