package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
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

type skSetupDoc struct {
	Schema        string           `json:"schema"`
	NotRead       []SetupNotRead   `json:"not_read"`
	Kinds         map[string]int   `json:"record_kinds"`
	PreLinked     int              `json:"preconditions_linked"`
	Conditions    []SetupCondition `json:"conditions"`
	Preconditions []SetupCondition `json:"preconditions"`
	Setup         []SKSetup        `json:"setup"`
	Tests         []SKSetupUse     `json:"tests"`
}

func skSetupQuery(t *testing.T, root string, m *Map, sources, conditions []string) skSetupDoc {
	t.Helper()
	raw, err := SetupQuery(context.Background(), m, sources, conditions, Options{Root: root, Full: true})
	if err != nil {
		t.Fatal(err)
	}
	again, _ := SetupQuery(context.Background(), m, sources, conditions, Options{Root: root, Full: true})
	if !bytes.Equal(raw, again) {
		t.Fatal("setup rerun differs (SKH-V0-007)")
	}
	var doc skSetupDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%v\n%s", err, raw)
	}
	return doc
}

// SKH-V0-030 SKH-V0-031 SKH-V0-032 SKH-V0-100 SKH-V0-101 SKH-V0-102 SKH-V0-103 SKH-V0-104
// SKH-V0-105: Rails factories, traits, YAML fixtures, seeds and test-setup endpoints arrive
// through the AMAP-V0-114 writer index as typed records with what they produce; Ruby tests link
// to the factories, traits and fixture files they use and name the ones with no record; Playwright
// setup projects bind to the setup files their testMatch selects and arrange the tests of
// unrestricted dependents; the condition query and the map's flow preconditions return candidates
// with their basis, or unresolved; and a map with no Rails router names each Rails source not read.
func TestSKHV0_100_105_RailsSetupProjectsAndConditions(t *testing.T) {
	root, m := railsFixture(t, func(files map[string]string) {
		files["db/seeds.rb"] = "# demo data\nOrder.create!(paid: true, locked: false)\nFactoryBot.create(:order, :shipped)\n"
		files["e2e/features/orders_spec.rb"] = "feature \"orders\" do\n  fixtures :orders, :ghosts\n  scenario \"ships\" do\n" +
			"    create(:order, :shipped, :rush)\n    create(:missing)\n    visit \"/orders\"\n  end\nend\n"
		files["e2e/playwright.config.ts"] = `import { defineConfig } from '@playwright/test';
export default defineConfig({
  projects: [
    { name: 'setup', testMatch: /.*\.setup\.ts/, teardown: 'cleanup' },
    { name: 'cleanup', testMatch: 'global.teardown.ts' },
    { name: 'chromium', dependencies: ['setup'], use: { storageState: 'e2e/.auth/user.json' } },
    { name: 'smoke', dependencies: ['dynamic'], testMatch: /smoke/ },
    { name: 'dynamic', testMatch: pattern },
  ],
});
`
		files["e2e/auth.setup.ts"] = `import { test as setup } from '@playwright/test';
setup('authenticate', async ({ page }) => {
  await page.goto('/login');
  await page.context().storageState({ path: 'e2e/.auth/user.json' });
});
`
	})
	m.Flows = []Flow{{FlowID: "ship", Preconditions: []string{"an Order that is shipped", "a signed-in admin"}}}
	doc := skSetupQuery(t, root, m, []string{"e2e"}, []string{"Order.shipped", "e2e/.auth/user.json", "Invoice.total"})
	if doc.Schema != "site-knowledge-setup/1" || len(doc.NotRead) != 0 {
		t.Fatalf("schema/not_read = %s %+v (SKH-V0-101, SKH-V0-105)", doc.Schema, doc.NotRead)
	}
	factory := skSetupBy(t, doc.Setup, SetupFactory, "factory :order", "order")
	trait := skSetupBy(t, doc.Setup, SetupTrait, "trait :shipped (factory :order)", "order:shipped")
	fixture := skSetupBy(t, doc.Setup, SetupFixtureFile, "fixtures orders", "orders")
	endpoint := skSetupBy(t, doc.Setup, SetupEndpoint, "POST /test_setup/orders", "/test_setup/orders")
	seed := skSetupBy(t, doc.Setup, SetupSeed, "seeds seeds.rb", "db/seeds.rb")
	if !reflect.DeepEqual(factory.Produces, []string{"order", "order.paid"}) || factory.Anchor.Path != "spec/factories/orders.rb" || factory.Anchor.Start != 2 ||
		!reflect.DeepEqual(trait.Produces, []string{"order", "order.shipped"}) ||
		!reflect.DeepEqual(fixture.Produces, []string{"order", "order.locked", "order.paid"}) || fixture.Basis != "fixture-keys" {
		t.Fatalf("factory/trait/fixture = %+v %+v %+v (SKH-V0-100)", factory, trait, fixture)
	}
	if endpoint.Method != "POST" || endpoint.Anchor.Path != "config/routes.rb" || endpoint.Params != nil || endpoint.Basis != "literal-hash" ||
		!reflect.DeepEqual(endpoint.Produces, []string{"order", "order.paid", "order.shipped"}) {
		t.Fatalf("endpoint = %+v (handler model writes, SKH-V0-100)", endpoint)
	}
	if !reflect.DeepEqual(seed.Produces, []string{"order", "order.locked", "order.paid", "order.shipped"}) || seed.Anchor.Path != "db/seeds.rb" {
		t.Fatalf("seed = %+v (SKH-V0-100)", seed)
	}
	setup := skSetupBy(t, doc.Setup, SetupProject, "setup", "/.*\\.setup\\.ts/")
	writes := skSetupBy(t, skSetupIn(doc.Setup, "e2e/auth.setup.ts"), SetupStorageState, "storageState", "e2e/.auth/user.json")
	if setup.Status != StatusResolved || setup.Cleanup != "teardown:cleanup" || !reflect.DeepEqual(setup.Arranges, []string{"chromium"}) ||
		!reflect.DeepEqual(setup.Requires, []string{writes.ID}) {
		t.Fatalf("setup project = %+v (SKH-V0-104)", setup)
	}
	cleanup := skSetupBy(t, doc.Setup, SetupProject, "cleanup", "global.teardown.ts")
	dynamic := skSetupBy(t, doc.Setup, SetupProject, "dynamic", "")
	if cleanup.Status != StatusUnknown || cleanup.Reason != "no-matching-setup-file" || dynamic.Status != StatusUnknown || dynamic.Reason != OpaqueDynamicValue {
		t.Fatalf("unresolved projects = %+v %+v (SKH-V0-104)", cleanup, dynamic)
	}
	byTitle := map[string]SKSetupUse{}
	for _, u := range doc.Tests {
		byTitle[u.Title] = u
	}
	ships := byTitle["ships"]
	if !reflect.DeepEqual(ships.Uses, uniqueSorted([]string{factory.ID, trait.ID, fixture.ID})) ||
		!reflect.DeepEqual(ships.Unresolved, []string{"factory:missing", "fixtures:ghosts", "trait:order:rush"}) {
		t.Fatalf("ruby links = %+v (SKH-V0-102)", ships)
	}
	if auth := byTitle["authenticate"]; slices.Contains(auth.Uses, setup.ID) {
		t.Fatalf("the setup file is arranged by its own project: %+v", auth)
	}
	if orders := byTitle["orders"]; !slices.Contains(orders.Uses, setup.ID) || !slices.Contains(orders.Uses, writes.ID) {
		t.Fatalf("spec of the unrestricted dependent not arranged: %+v (SKH-V0-104)", orders)
	}

	want := map[string][]string{
		"Order.shipped":       {trait.ID, endpoint.ID, seed.ID},
		"e2e/.auth/user.json": {writes.ID},
		"Invoice.total":       nil,
	}
	for _, c := range doc.Conditions {
		got := []string{}
		for _, cand := range c.Candidates {
			got = append(got, cand.Setup)
		}
		w := want[c.Condition]
		if w == nil {
			if c.Status != ConditionUnresolved || c.Reason != "no-producing-record" || len(got) != 0 {
				t.Fatalf("%s = %+v (SKH-V0-103)", c.Condition, c)
			}
			continue
		}
		if c.Status != ConditionCandidates || !reflect.DeepEqual(got, uniqueSorted(w)) {
			t.Fatalf("%s candidates %v, want %v (SKH-V0-031, SKH-V0-103)", c.Condition, got, uniqueSorted(w))
		}
	}
	if len(doc.Preconditions) != 2 || doc.PreLinked != 1 || doc.Preconditions[0].Flow != "ship" || doc.Preconditions[0].Status != ConditionCandidates ||
		doc.Preconditions[1].Status != ConditionUnresolved {
		t.Fatalf("preconditions = %+v (SKH-V0-031)", doc.Preconditions)
	}

	// A map with no Rails router reads none of the Rails sources and says so.
	jsRoot, rev := fixtureRepo(t)
	jsMap := build(t, jsRoot, rev)
	plain := skSetupQuery(t, jsRoot, jsMap, []string{"e2e"}, nil)
	if len(plain.NotRead) != 4 || plain.NotRead[0] != (SetupNotRead{Source: "factorybot", Reason: "no-rails-router"}) {
		t.Fatalf("not_read = %+v (SKH-V0-101)", plain.NotRead)
	}
}
