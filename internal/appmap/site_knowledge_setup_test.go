package appmap

import (
	"reflect"
	"sort"
	"testing"
)

func skSetupBy(t *testing.T, records []SKSetup, kind, name, target string) SKSetup {
	t.Helper()
	found := []SKSetup{}
	for _, s := range records {
		if s.Kind == kind && s.Name == name && s.Target == target {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("setup %s %s %q: %d records in %+v", kind, name, target, len(found), records)
	}
	return found[0]
}

func skSetupIn(records []SKSetup, path string) []SKSetup {
	in := []SKSetup{}
	for _, s := range records {
		if s.Path == path {
			in = append(in, s)
		}
	}
	return in
}

// SKH-V0-030, SKH-V0-031, SKH-V0-033, SKH-V0-034: test.extend fixtures with their
// chains, options, cleanup and dependencies; hooks; storageState reads and writes pinned by path
// and committed blob, never contents; route mocks labelled mocked; API seeding and cy.task; and
// each test linked to exactly the records that arrange it, with unresolved fixtures named.
func TestSKHV0_030_031_SetupRecordsAndTestLinks(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/fixtures.ts": `import { test as base, expect } from '@playwright/test';
type F = { account: { id: string }; loggedIn: void; locale: string };
export const test = base.extend<F>({
  account: async ({ request }, use) => {
    const res = await request.post('/api/test/accounts', { data: { name: 'a' } });
    await use(await res.json());
    await request.delete('/api/test/accounts');
  },
  loggedIn: [async ({ page, account }, use) => {
    await page.route('**/api/session', (route) => route.fulfill({ status: 200 }));
    await use();
  }, { auto: true }],
  locale: ['en-GB', { option: true }],
});
export const adminTest = test.extend({
  admin: [async ({ account, ghost }, use) => { await use(account); }, { scope: 'worker' }],
  seeded: async ({ page }, use, testInfo) => {
    await page.context().storageState({ path: 'e2e/.auth/admin.json' });
    await use('x');
  },
});
export { expect };
`,
		"e2e/specs/a.spec.ts": `import { type Page } from '@playwright/test';
import { adminTest as test, expect } from '../fixtures';
test.use({ storageState: 'e2e/state/viewer.json' });
test.describe('group', () => {
  test.beforeEach(async ({ page }) => {
    await page.route(` + "`**/api/items/${id}`" + `, (r) => r.abort());
  });
  test('uses admin', async ({ page, admin, missing }) => {
    await page.goto('/x');
  });
});
test('outside', async ({ page }) => {
  await installMocks(page);
  await page.goto('/y');
});
async function installMocks(page: Page) {
  await page.route('**/api/feed', (r) => r.fulfill({ json: [] }));
}
test.afterAll(async () => {});
`,
		"e2e/lib/ctx.ts": `import { type Browser } from '@playwright/test';
export async function login(browser: Browser, storageState: string) {
  return browser.newContext({ storageState });
}
`,
		"cypress/e2e/seed.cy.ts": `describe('c', () => {
  beforeEach(() => {
    cy.task('db:seed');
    cy.request('POST', '/api/reset');
    cy.intercept('GET', '/api/users', { fixture: 'users.json' });
  });
  it('works', () => { cy.visit('/u'); });
});
`,
	}), nil, nil)
	records, uses := k.extractSetup(map[string]string{"e2e/state/viewer.json": "1111111111111111111111111111111111111111"})

	account := skSetupBy(t, records, SetupFixture, "account", "")
	if account.Cleanup != "after-use" || !reflect.DeepEqual(account.Params, []string{"request"}) || account.Requires != nil ||
		account.Unresolved != nil || account.Test != "test" || account.Base != "base" || account.Status != StatusResolved {
		t.Fatalf("account = %+v", account)
	}
	post := skSetupBy(t, records, SetupAPISeed, "request.post", "/api/test/accounts")
	del := skSetupBy(t, records, SetupAPISeed, "request.delete", "/api/test/accounts")
	if post.Method != "POST" || post.Within != account.ID || del.Method != "DELETE" || del.Within != account.ID {
		t.Fatalf("seeds = %+v %+v", post, del)
	}
	loggedIn := skSetupBy(t, records, SetupFixture, "loggedIn", "")
	if !loggedIn.Auto || loggedIn.Cleanup != "" || !reflect.DeepEqual(loggedIn.Requires, []string{account.ID}) {
		t.Fatalf("loggedIn = %+v", loggedIn)
	}
	session := skSetupBy(t, records, SetupRouteMock, "route", "**/api/session")
	if !session.Mocked || session.Within != loggedIn.ID {
		t.Fatalf("session mock = %+v (SKH-V0-034)", session)
	}
	if locale := skSetupBy(t, records, SetupFixture, "locale", ""); !locale.Option {
		t.Fatalf("locale = %+v", locale)
	}
	admin := skSetupBy(t, records, SetupFixture, "admin", "")
	if admin.Scope != "worker" || admin.Test != "adminTest" || admin.Base != "test" ||
		!reflect.DeepEqual(admin.Requires, []string{account.ID}) || !reflect.DeepEqual(admin.Unresolved, []string{"ghost"}) {
		t.Fatalf("admin = %+v (fixture chain)", admin)
	}
	seeded := skSetupBy(t, records, SetupFixture, "seeded", "")
	writes := skSetupBy(t, records, SetupStorageState, "storageState", "e2e/.auth/admin.json")
	if writes.Mode != "writes" || writes.Within != seeded.ID || writes.Blob != "" {
		t.Fatalf("storage writes = %+v", writes)
	}
	reads := skSetupBy(t, records, SetupStorageState, "storageState", "e2e/state/viewer.json")
	if reads.Mode != "reads" || reads.Blob != "1111111111111111111111111111111111111111" || reads.Within != "" {
		t.Fatalf("storage reads = %+v (SKH-V0-033)", reads)
	}
	dynamic := skSetupBy(t, records, SetupStorageState, "storageState", "")
	if dynamic.Status != StatusUnknown || dynamic.Reason != OpaqueDynamicValue || dynamic.Within != "function:e2e/lib/ctx.ts#login" {
		t.Fatalf("dynamic storage = %+v", dynamic)
	}
	before := skSetupBy(t, skSetupIn(records, "e2e/specs/a.spec.ts"), SetupHook, "beforeEach", "")
	items := skSetupBy(t, records, SetupRouteMock, "route", "")
	if items.Status != StatusUnknown || items.Within != before.ID || before.Test != "test" {
		t.Fatalf("hook mock = %+v %+v", items, before)
	}
	after := skSetupBy(t, records, SetupHook, "afterAll", "")
	feed := skSetupBy(t, records, SetupRouteMock, "route", "**/api/feed")
	if after.Cleanup != "after-all" || feed.Within != "function:e2e/specs/a.spec.ts#installMocks" {
		t.Fatalf("afterAll/feed = %+v %+v", after, feed)
	}
	task := skSetupBy(t, records, SetupTask, "cy.task", "db:seed")
	reset := skSetupBy(t, records, SetupAPISeed, "cy.request", "/api/reset")
	users := skSetupBy(t, records, SetupRouteMock, "cy.intercept", "/api/users")
	if reset.Method != "POST" || users.Method != "GET" || !users.Mocked {
		t.Fatalf("cypress = %+v %+v", reset, users)
	}
	if len(records) != 19 {
		t.Fatalf("records = %d: %+v", len(records), records)
	}

	want := map[string]struct {
		uses       []string
		unresolved []string
	}{
		"uses admin": {[]string{admin.ID, account.ID, post.ID, del.ID, loggedIn.ID, session.ID, before.ID, items.ID, after.ID, reads.ID}, []string{"missing"}},
		"outside":    {[]string{account.ID, post.ID, del.ID, loggedIn.ID, session.ID, after.ID, reads.ID, feed.ID}, nil},
		"works":      {[]string{skSetupBy(t, skSetupIn(records, "cypress/e2e/seed.cy.ts"), SetupHook, "beforeEach", "").ID, task.ID, reset.ID, users.ID}, nil},
	}
	if len(uses) != len(want) {
		t.Fatalf("uses = %+v", uses)
	}
	for _, u := range uses {
		w, ok := want[u.Title]
		sort.Strings(w.uses)
		if !ok || !reflect.DeepEqual(u.Uses, w.uses) || !reflect.DeepEqual(u.Unresolved, w.unresolved) {
			t.Fatalf("%s uses %v unresolved %v, want %v %v (SKH-V0-031)", u.Title, u.Uses, u.Unresolved, w.uses, w.unresolved)
		}
	}
}
