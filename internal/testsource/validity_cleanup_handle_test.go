package testsource

import (
	"strings"
	"testing"
)

// TestCleanupHandleOutlivedInTest: a test registering a closure on an in-file worker-scoped
// registry that captures a test-scoped fixture is reported at the registration line, citing the
// registry declaration and the handle's declaration; an afterAll reading a test-scoped handle a
// beforeEach stored is reported too. A closure taking its own context or creating one is not
// (TSA-V0-165, TSA-V0-166).
func TestCleanupHandleOutlivedInTest(t *testing.T) {
	src := `import { test as base, expect } from '@playwright/test';
const test = base.extend({
  registry: [async ({}, use) => { await use(new Registry()); }, { scope: 'worker' }],
  account: async ({ page }, use) => { await use(new Account(page)); },
});
test('creates a row', async ({ account, registry: reg, request }) => {
  reg.add(async () => { await account.remove(); });
  reg.add(async ({ request }) => { await request.delete('/x'); });
  reg.add(async () => { const own = await playwright.request.newContext(); await own.delete('/x'); });
  expect(1).toBe(1);
});
let ctx;
test.beforeEach(async ({ request }) => { ctx = request; });
test.afterAll(async () => { await ctx.delete('/rows'); });
`
	r := ValidityFindings("e2e/rows.spec.ts", src)
	wantFindings(t, "in test", r,
		"CLEANUP-HANDLE-OUTLIVED advisory e2e/rows.spec.ts:7 e2e/rows.spec.ts:3 creates a row",
		"CLEANUP-HANDLE-OUTLIVED advisory e2e/rows.spec.ts:14 e2e/rows.spec.ts:14 afterAll",
	)
	if len(r.Findings) == 2 {
		if r.Findings[0].Test != "e2e/rows.spec.ts:6" || !strings.Contains(r.Findings[0].Message, "`account` (declared at e2e/rows.spec.ts:4)") || !strings.Contains(r.Findings[0].Message, "playwright.request.newContext()") {
			t.Errorf("registration finding %+v", r.Findings[0])
		}
		if !strings.Contains(r.Findings[1].Message, "`request` (a Playwright built-in)") {
			t.Errorf("afterAll finding %+v", r.Findings[1])
		}
	}
}

// TestCleanupHandleOutlivedFixtureFile: a fixture file (not a test file) whose test-scoped fixture
// registers a closure capturing its own request context on a worker-scoped fixture is reported,
// with the registration line as the test (TSA-V0-166).
func TestCleanupHandleOutlivedFixtureFile(t *testing.T) {
	src := `import { test as base } from '@playwright/test';
export const test = base.extend<{ seeded: Seed }, { cleanup: Cleanup }>({
  cleanup: [async ({}, use) => {
    const tasks = [];
    await use({ add: (fn) => tasks.push(fn) });
    for (const t of tasks) await t();
  }, { scope: 'worker' }],
  seeded: async ({ request, cleanup }, use) => {
    const api = request;
    const row = await api.post('/rows');
    cleanup.add(() => api.delete('/rows/1'));
    await use(row);
  },
});
`
	r := ValidityFindings("e2e/fixtures.ts", src)
	wantFindings(t, "fixture file", r, "CLEANUP-HANDLE-OUTLIVED advisory e2e/fixtures.ts:11 e2e/fixtures.ts:3 seeded")
	if len(r.Findings) == 1 && (r.Findings[0].Test != "e2e/fixtures.ts:11" || !strings.Contains(r.Findings[0].Message, "`request` (a Playwright built-in)")) {
		t.Errorf("finding %+v", r.Findings[0])
	}
	if r.Uncertain {
		t.Errorf("uncertain %+v", r)
	}
}

// TestCleanupHandleOutlivedAbstains: a registry whose scope the file does not state (imported from
// elsewhere, or a non-literal scope), a test-scoped registry, and a closure capturing no handle are
// not reported (TSA-V0-167).
func TestCleanupHandleOutlivedAbstains(t *testing.T) {
	imported := `import { test } from './fixtures';
test('x', async ({ registry, request }) => {
  registry.add(() => request.delete('/x'));
});
`
	wantFindings(t, "imported registry", ValidityFindings("e2e/imported.spec.ts", imported))
	dynamic := `const test = base.extend({
  registry: [async ({}, use) => { await use(new Registry()); }, { scope: SCOPE }],
});
test('x', async ({ registry, request }) => {
  registry.add(() => request.delete('/x'));
});
`
	wantFindings(t, "non-literal scope", ValidityFindings("e2e/dynamic.spec.ts", dynamic))
	testScoped := `const test = base.extend({
  registry: async ({}, use) => { await use(new Registry()); },
});
test('x', async ({ registry, request }) => {
  registry.add(() => request.delete('/x'));
  registry.add(() => console.log('done'));
});
`
	wantFindings(t, "test-scoped registry", ValidityFindings("e2e/scoped.spec.ts", testScoped))
}

// TestCleanupHandleOutlivedShadowed: a closure that declares its own context under the handle's
// name does not capture the outer handle, so it is not reported (TSA-V0-165).
func TestCleanupHandleOutlivedShadowed(t *testing.T) {
	src := `const test = base.extend({
  registry: [async ({}, use) => { await use(new Registry()); }, { scope: 'worker' }],
});
test('x', async ({ registry, request, playwright }) => {
  registry.add(async () => {
    const request = await playwright.request.newContext();
    await request.delete('/x');
  });
  registry.add(async () => { await request.delete('/y'); });
});
`
	wantFindings(t, "shadowed", ValidityFindings("e2e/shadow.spec.ts", src),
		"CLEANUP-HANDLE-OUTLIVED advisory e2e/shadow.spec.ts:9 e2e/shadow.spec.ts:2 x",
	)
}
