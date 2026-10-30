package appmap

import (
	"encoding/json"
	"strings"
	"testing"
)

// Synthetic fixtures for site knowledge stages 1 and 2 (SKH-V0-010..014, SKH-V0-020..023).

func skSrc(files map[string]string) []skSource {
	out := []skSource{}
	for p, body := range files {
		out = append(out, skSource{entry: blobEntry{path: p, oid: digest([]byte(body)), size: len(body)}, data: []byte(body)})
	}
	return out
}

func skPO(t *testing.T, k *SKModel, id string) SKPageObject {
	t.Helper()
	for _, po := range k.PageObjects {
		if po.ID == id {
			return po
		}
	}
	t.Fatalf("page object %s not found in %v", id, skIDs(k))
	return SKPageObject{}
}

func skIDs(k *SKModel) []string {
	out := []string{}
	for _, po := range k.PageObjects {
		out = append(out, po.ID)
	}
	return out
}

func skMethod(t *testing.T, po SKPageObject, name string) SKMethod {
	t.Helper()
	for _, m := range po.Methods {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("method %s not found on %s", name, po.ID)
	return SKMethod{}
}

func skKinds(steps []SKStep) string {
	out := []string{}
	for _, s := range steps {
		k := s.Kind
		if s.Reason != "" {
			k += ":" + s.Reason
		}
		out = append(out, k)
	}
	return strings.Join(out, ",")
}

func skTest(t *testing.T, k *SKModel, title string) SKTest {
	t.Helper()
	for _, tt := range k.Tests {
		if tt.Title == title {
			return tt
		}
	}
	t.Fatalf("test %q not found", title)
	return SKTest{}
}

const skBasePage = `import { Page } from '@playwright/test';
export class BasePage {
  constructor(readonly page: Page) {}
  async open(path: string) {
    await this.page.goto(path);
  }
}
`

const skLoginPage = `import { Page, Locator } from '@playwright/test';
import { BasePage } from './base-page';

export class LoginPage extends BasePage {
  readonly email: Locator;
  readonly submit = this.page.getByRole('button', { name: 'Sign in' });

  constructor(page: Page) {
    super(page);
    this.email = page.getByLabel('Email');
  }

  get password() {
    return this.page.getByTestId('password');
  }

  async login(user: string, secret: string) {
    await this.email.fill(user);
    await this.password.fill(secret);
    await this.submit.click();
  }

  async visit() {
    await this.open('/login');
  }

  async signIn(user: string) {
    await this.visit();
    await this.login(user, 'not-a-real-pass');
  }
}
`

const skLoginSpec = `import { test, expect } from '@playwright/test';
import { LoginPage } from './pages/login-page';

test('signs in', async ({ page }) => {
  const login = new LoginPage(page);
  await login.visit();
  await login.login('alice', 'hunter2');
  await expect(page).toHaveURL('/home');
  await expect(page.getByRole('heading', { name: 'Welcome' })).toBeVisible();
});

test('signs in again', async ({ page }) => {
  const login = new LoginPage(page);
  await login.visit();
  await login.login('alice', 'hunter2');
  await expect(page).toHaveURL('/home');
  await expect(page.getByRole('heading', { name: 'Welcome' })).toBeVisible();
});

test('signs in through the helper method', async ({ page }) => {
  const login = new LoginPage(page);
  await login.signIn('bob');
});
`

func TestSKHV0_010_PlaywrightClassPageObject(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/pages/base-page.ts":  skBasePage,
		"e2e/pages/login-page.ts": skLoginPage,
		"e2e/login.spec.ts":       skLoginSpec,
	}), map[string]TestFile{"e2e/pages/login-page.ts": {Screen: "screen:login", ScreenBasis: "path"}}, nil)
	po := skPO(t, k, "page-object:e2e/pages/login-page.ts#LoginPage")
	if po.Form != FormPlaywrightClass || po.Extends != "BasePage" || po.Authority != "learned" {
		t.Fatalf("form/extends/authority = %s/%s/%s", po.Form, po.Extends, po.Authority)
	}
	if po.Screen != "screen:login" || po.Join != StatusResolved {
		t.Fatalf("screen join = %s/%s, want the map's screen (SKH-V0-013)", po.Screen, po.Join)
	}
	if po.Anchor.Blob == "" || po.Anchor.SpanSHA256 == "" || po.Anchor.Start == 0 {
		t.Fatalf("page object anchor is not pinned: %+v", po.Anchor)
	}
	got := map[string]string{}
	for _, b := range po.Bindings {
		got[b.Name] = b.Locator.Kind + "=" + b.Locator.Value + "/" + b.Locator.Strength
	}
	want := map[string]string{"email": "label=Email/medium", "submit": "role=button/medium", "password": "test-id=password/strong"}
	for n, w := range want {
		if got[n] != w {
			t.Fatalf("binding %s = %q, want %q (all %v)", n, got[n], w, got)
		}
	}
	login := skMethod(t, po, "login")
	if strings.Join(login.Params, ",") != "user,secret" || skKinds(login.Actions) != "fill,fill,click" {
		t.Fatalf("login params %v actions %s", login.Params, skKinds(login.Actions))
	}
	if login.Actions[0].Slots[0] != "user" || login.Effect != EffectState || login.EffectReason != "" {
		// The fills are state effects; under ANL-V0-035 precedence the button click does not
		// make the method unknown (SKH-V0-016).
		t.Fatalf("login slot %v effect %s", login.Actions[0].Slots, login.Effect)
	}
	visit := skMethod(t, po, "visit")
	if len(visit.Hops) != 1 || visit.Hops[0].Kind != "inherited" || visit.Hops[0].Status != StatusResolved ||
		visit.Hops[0].Resolved != "page-object:e2e/pages/base-page.ts#BasePage.open" {
		t.Fatalf("visit hops = %+v, want one resolved inherited hop (SKH-V0-012)", visit.Hops)
	}
	if visit.Effect != EffectNavigation {
		t.Fatalf("visit effect = %s", visit.Effect)
	}
	signIn := skMethod(t, po, "signIn")
	if len(signIn.Hops) != 2 || signIn.Hops[0].Kind != "method" || signIn.Hops[1].Status != StatusResolved {
		t.Fatalf("signIn hops = %+v", signIn.Hops)
	}
}

func TestSKHV0_021_InlinesCallsWithBoundArguments(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/pages/base-page.ts":  skBasePage,
		"e2e/pages/login-page.ts": skLoginPage,
		"e2e/login.spec.ts":       skLoginSpec,
	}), nil, nil)
	tt := skTest(t, k, "signs in")
	if skKinds(tt.Steps) != "visit,fill,fill,click,assert,assert" || !tt.Complete {
		t.Fatalf("steps = %s complete=%v", skKinds(tt.Steps), tt.Complete)
	}
	if tt.Steps[0].Target != "/login" || len(tt.Steps[0].Via) != 2 {
		t.Fatalf("visit = %+v, want /login bound through visit -> open", tt.Steps[0])
	}
	if tt.Steps[1].Value != "alice" || tt.Steps[1].Slots != nil {
		t.Fatalf("user fill = %+v, want the bound literal", tt.Steps[1])
	}
	// The password field is sensitive: its literal is screened, never kept (SKH-V0-006).
	if tt.Steps[2].Value != "" || !tt.Steps[2].Screened {
		t.Fatalf("password fill = %+v, want screened", tt.Steps[2])
	}
	if a := tt.Steps[4]; a.Matcher != "toHaveURL" || a.Expected != "/home" || a.Locator != nil {
		t.Fatalf("url assert = %+v", a)
	}
	if a := tt.Steps[5]; a.Matcher != "toBeVisible" || a.Locator == nil || a.Locator.Name != "Welcome" {
		t.Fatalf("heading assert = %+v", a)
	}
	deep := skTest(t, k, "signs in through the helper method")
	if skKinds(deep.Steps) != "visit,fill,fill,click" || deep.Steps[1].Value != "bob" || !deep.Steps[2].Screened {
		t.Fatalf("nested inline = %s %+v", skKinds(deep.Steps), deep.Steps)
	}
	for _, s := range append(tt.Steps, deep.Steps...) {
		if !ValidateSKStep(s) || s.Kind == skCall {
			t.Fatalf("step outside the closed IR: %+v", s)
		}
	}
}

func TestSKHV0_022_DeduplicatesSequencesWithUsage(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/pages/base-page.ts":  skBasePage,
		"e2e/pages/login-page.ts": skLoginPage,
		"e2e/login.spec.ts":       skLoginSpec,
	}), nil, nil)
	a, b := skTest(t, k, "signs in"), skTest(t, k, "signs in again")
	if a.Record != b.Record {
		t.Fatalf("identical bodies got records %s and %s", a.Record, b.Record)
	}
	var body, method *SKRecord
	for i, r := range k.Records {
		if r.ID == a.Record {
			body = &k.Records[i]
		}
		if r.Source == "page-object:e2e/pages/login-page.ts#LoginPage.login" && r.Steps == 3 && r.Usage == 2 {
			method = &k.Records[i]
		}
	}
	if body == nil || body.Usage != 2 || len(body.Tests) != 2 {
		t.Fatalf("body record = %+v, want usage 2 citing both tests", body)
	}
	if method == nil {
		t.Fatalf("no login method record with usage 2 in %+v", k.Records)
	}
}

func TestSKHV0_020_OpaqueStepsAreKept(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/odd.spec.ts": `import { test } from '@playwright/test';
test('odd', async ({ page }) => {
  await page.goto(baseURL + '/x');
  await page.getByLabel('Name').fill(randomName());
  if (flag) {
    await page.getByRole('button', { name: 'Go' }).click();
  }
  await page.mouse.wheel(0, 10);
  await somethingElse(page);
  await page.getByTestId('save').click();
});
`,
	}), nil, nil)
	tt := skTest(t, k, "odd")
	want := "opaque:dynamic-value,opaque:dynamic-value,opaque:parse-unsupported,opaque:parse-unsupported,opaque:unresolved-call,click"
	if skKinds(tt.Steps) != want || tt.Complete {
		t.Fatalf("steps = %s complete=%v, want %s", skKinds(tt.Steps), tt.Complete, want)
	}
	for _, s := range tt.Steps {
		if !ValidateSKStep(s) {
			t.Fatalf("invalid step %+v", s)
		}
	}
	if ValidateSKStep(SKStep{Kind: "drag"}) || ValidateSKStep(SKStep{Kind: StepOpaque, Reason: "guess"}) ||
		ValidateSKStep(SKStep{Kind: StepClick, Reason: OpaqueDynamicValue}) {
		t.Fatal("a kind or reason outside the closed IR validated (SKH-V0-023)")
	}
}

func TestSKHV0_024_PressFocusAndHoverAreTyped(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/keys.spec.ts": `import { test } from '@playwright/test';
test('keys', async ({ page }) => {
  await page.getByRole('textbox', { name: 'Search' }).focus();
  await page.keyboard.type('shoes');
  await page.keyboard.press('Enter');
  await page.getByTestId('menu').hover();
  await page.getByLabel('Name').press('Tab');
  await page.press('#q', 'ArrowDown');
  await page.keyboard.down('Shift');
  await page.keyboard.press(key);
});
`,
		"spec/support/pages/menu_page.rb": `class MenuPage < SitePrism::Page
  element :menu, '[data-testid=menu]'

  def open_menu
    menu.hover
    menu.send_keys :enter
  end
end
`,
	}), nil, nil)
	tt := skTest(t, k, "keys")
	want := "focus,fill,press,hover,press,press,opaque:parse-unsupported,opaque:dynamic-value"
	if skKinds(tt.Steps) != want {
		t.Fatalf("steps = %s, want %s (SKH-V0-024)", skKinds(tt.Steps), want)
	}
	st := tt.Steps
	if st[0].Locator == nil || st[0].Locator.Kind != "role" || st[0].Target != "" {
		t.Fatalf("focus = %+v", st[0])
	}
	if st[1].Target != skKeyboard || st[1].Value != "shoes" || st[1].Locator != nil {
		t.Fatalf("keyboard.type = %+v", st[1])
	}
	if st[2].Target != skKeyboard || st[2].Value != "Enter" || st[2].Locator != nil {
		t.Fatalf("keyboard.press = %+v", st[2])
	}
	if st[3].Locator == nil || st[3].Locator.Kind != "test-id" || st[3].Locator.Value != "menu" {
		t.Fatalf("hover = %+v", st[3])
	}
	if st[4].Locator == nil || st[4].Locator.Kind != "label" || st[4].Value != "Tab" || st[4].Target != "" {
		t.Fatalf("locator press = %+v", st[4])
	}
	if st[5].Locator == nil || st[5].Locator.Value != "#q" || st[5].Value != "ArrowDown" {
		t.Fatalf("legacy page.press = %+v", st[5])
	}
	if st[6].Call != "keyboard.down" {
		t.Fatalf("keyboard.down = %+v", st[6])
	}
	for _, s := range st {
		if !ValidateSKStep(s) {
			t.Fatalf("invalid step %+v", s)
		}
	}
	m := skMethod(t, skPO(t, k, "page-object:spec/support/pages/menu_page.rb#MenuPage"), "open_menu")
	if skKinds(m.Actions) != "hover,press" || m.Actions[1].Value != "enter" || m.Actions[0].Locator.Value != "menu" {
		t.Fatalf("ruby hover/send_keys = %s %+v", skKinds(m.Actions), m.Actions)
	}
	if _, err := DecodeSKStep([]byte(`{"kind":"press","target":"keyboard","value":"Enter","anchor":{}}`)); err != nil {
		t.Fatalf("press refused by the strict decoder: %v", err)
	}
}

func TestSKHV0_015_016_URLJoinAndNamedEffects(t *testing.T) {
	screens := newScreenIndex("", "", []Screen{
		{ID: "screen:settings", Status: StatusResolved, Key: "/users/{}/settings"},
		{ID: "screen:a", Status: StatusResolved, Key: "/a"},
		{ID: "screen:b", Status: StatusResolved, Key: "/b"},
	})
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/flows.spec.ts": `import { test, expect, type Page } from '@playwright/test';
async function openSettings(page: Page, id: string) {
  await page.goto(` + "`/users/${id}/settings`" + `);
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
}
async function mockApi(page: Page) {
  await page.route('**/api/**', (r) => r.fulfill({ body: '{}' }));
}
async function openMenu(page: Page) {
  await page.getByTestId('menu').hover();
}
async function save(page: Page, name: string) {
  await openSettings(page, 'u1');
  await page.getByRole('button', { name: 'Save' }).click();
  await page.getByLabel('Name').fill(name);
}
async function audit(page: Page) {
  await openSettings(page, 'u1');
  await page.evaluate(() => 1);
}
async function seed(page: Page) {
  await mockApi(page);
  await page.request.post('/api/users');
}
async function rename(page: Page) {
  await page.getByLabel('Name').fill(randomName());
}
async function nothing(page: Page) {
  console.log('x');
}
async function openDialog(page: Page): Promise<{ dialog: Locator; ok: boolean }> {
  await page.goto('/users/u2/settings');
  return { dialog: page.getByRole('dialog'), ok: true };
}
test('t', async ({ page }) => {
  await seed(page);
  await save(page, 'x');
});
`,
		"e2e/pages/nav.ts": `import { type Page } from '@playwright/test';
export async function toA(page: Page) { await page.goto('/a'); }
export async function toB(page: Page) { await page.goto('/b'); }
`,
		"e2e/pages/dyn.ts": `import { type Page } from '@playwright/test';
export async function go(page: Page) { await page.goto(baseURL + '/x'); }
`,
		"e2e/pages/none.ts": `import { type Page } from '@playwright/test';
export async function press(page: Page) { await page.getByRole('link', { name: 'Home' }).click(); }
`,
		"e2e/pages/lost.ts": `import { type Page } from '@playwright/test';
export async function go(page: Page) { await page.goto('/nowhere'); }
`,
	}), nil, screens)
	po := skPO(t, k, "page-object:e2e/flows.spec.ts")
	if po.Join != StatusResolved || po.Screen != "screen:settings" || po.ScreenBasis != "page-object-url" || po.JoinReason != "" {
		t.Fatalf("flows join = %s %s %s %s (SKH-V0-015)", po.Join, po.Screen, po.ScreenBasis, po.JoinReason)
	}
	want := map[string][2]string{
		"openSettings": {EffectNavigation, ""},
		"mockApi":      {EffectUnknown, EffectReasonArranges},
		"openMenu":     {EffectUnknown, EffectReasonPointer},
		"save":         {EffectState, ""},
		"audit":        {EffectUnknown, OpaqueParseUnsupported},
		"seed":         {EffectState, ""},
		"rename":       {EffectState, ""},
		"nothing":      {EffectUnknown, EffectReasonNoActions},
		"openDialog":   {EffectNavigation, ""},
	}
	for name, w := range want {
		m := skMethod(t, po, name)
		if m.Effect != w[0] || m.EffectReason != w[1] {
			t.Fatalf("%s effect = %s/%s, want %s/%s (SKH-V0-016)", name, m.Effect, m.EffectReason, w[0], w[1])
		}
	}
	for id, reason := range map[string][2]string{"page-object:e2e/pages/nav.ts": {JoinReasonAmbiguous, ""},
		"page-object:e2e/pages/dyn.ts":  {JoinReasonUnresolvedTarget, JoinDetailDynamic},
		"page-object:e2e/pages/lost.ts": {JoinReasonUnresolvedTarget, "no-matching-screen"},
		"page-object:e2e/pages/none.ts": {JoinReasonUnbound, ""}} {
		if p := skPO(t, k, id); p.Join != StatusUnknown || p.JoinReason != reason[0] || p.JoinDetail != reason[1] || p.Screen != "" {
			t.Fatalf("%s join = %s/%s/%s, want unknown/%v (SKH-V0-015)", id, p.Join, p.JoinReason, p.JoinDetail, reason)
		}
	}
	if m := skMethod(t, skPO(t, k, "page-object:e2e/pages/none.ts"), "press"); m.Effect != EffectNavigation {
		t.Fatalf("link click effect = %s/%s", m.Effect, m.EffectReason)
	}
}

func TestSKHV0_010_ModulePageObjectAndSetupHelpers(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/lib/nav.ts": `import type { Page } from '@playwright/test';
export async function openSettings(page: Page, section: string) {
  await page.getByRole('link', { name: 'Settings' }).click();
  await page.getByTestId(section).click();
}
export const seedUser = async (request: APIRequestContext) => {
  await request.post('/api/users');
};
`,
		"e2e/settings.spec.ts": `import { test, expect } from '@playwright/test';
import { openSettings, seedUser } from './lib/nav';
test('opens settings', async ({ page, request }) => {
  await seedUser(request);
  await openSettings(page, 'privacy');
  await expect(page.getByTestId('privacy-panel')).toHaveText('Privacy');
});
`,
	}), nil, nil)
	po := skPO(t, k, "page-object:e2e/lib/nav.ts")
	if po.Form != FormPlaywrightModule || len(po.Methods) != 1 || po.Methods[0].Name != "openSettings" {
		t.Fatalf("module page object = %+v", po)
	}
	m := po.Methods[0]
	if m.Actions[1].LocatorSlot != "section" || m.Actions[1].Locator.Strength != strengthUnknown {
		t.Fatalf("parameter locator = %+v", m.Actions[1])
	}
	tt := skTest(t, k, "opens settings")
	if skKinds(tt.Steps) != "click,click,assert" || strings.Join(tt.Setup, ",") != "seedUser" {
		t.Fatalf("steps %s setup %v", skKinds(tt.Steps), tt.Setup)
	}
	if l := tt.Steps[1].Locator; l == nil || l.Kind != "test-id" || l.Value != "privacy" || l.Strength != strengthStrong {
		t.Fatalf("bound locator = %+v, want test-id privacy rebuilt from the argument", l)
	}
	if tt.Steps[2].Expected != "Privacy" {
		t.Fatalf("assert = %+v", tt.Steps[2])
	}
}

func TestSKHV0_010_CypressClass(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"cypress/pages/cart.ts": `export class CartPage {
  open() {
    cy.visit('/cart');
  }
  checkout(code) {
    cy.get('[data-testid=coupon]').type(code);
    cy.contains('Checkout').click();
    cy.get('.total').should('have.text', '$10');
  }
}
`,
		"cypress/e2e/cart.cy.ts": `import { CartPage } from '../pages/cart';
it('checks out', () => {
  const cart = new CartPage();
  cart.open();
  cart.checkout('SAVE10');
  cy.login();
});
`,
	}), nil, nil)
	po := skPO(t, k, "page-object:cypress/pages/cart.ts#CartPage")
	if po.Form != FormCypressClass {
		t.Fatalf("form = %s", po.Form)
	}
	tt := skTest(t, k, "checks out")
	if tt.Framework != "cypress" || skKinds(tt.Steps) != "visit,fill,click,assert,opaque:unresolved-call" {
		t.Fatalf("framework %s steps %s", tt.Framework, skKinds(tt.Steps))
	}
	if s := tt.Steps[1]; s.Value != "SAVE10" || s.Locator.Kind != "test-id" {
		t.Fatalf("type step = %+v", s)
	}
	if s := tt.Steps[3]; s.Matcher != "have.text" || s.Expected != "$10" {
		t.Fatalf("should step = %+v", s)
	}
}

func TestSKHV0_010_CapybaraAndSitePrism(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"spec/support/pages/base_page.rb": `class BasePage
  def go_home
    visit '/'
  end
end
`,
		"spec/support/pages/sign_in_page.rb": `class SignInPage < BasePage
  def sign_in(email, password)
    go_home
    fill_in 'Email', with: email
    fill_in 'Password', with: password
    click_button 'Sign in'
  end
end
`,
		"spec/support/pages/search_page.rb": `class SearchPage < SitePrism::Page
  set_url '/search'
  element :query, '[data-testid=query]'
  element :go, 'button.go'

  def search(term)
    query.set term
    go.click
  end
end
`,
		"spec/features/sign_in_spec.rb": `require 'rails_helper'
RSpec.feature 'Sign in' do
  scenario 'user signs in' do
    user = create(:user)
    page_object = SignInPage.new
    page_object.sign_in('a@example.com', 'pw-123')
    expect(page).to have_content 'Signed in'
  end

  scenario 'user searches' do
    search = SearchPage.new
    search.load
    search.search('shoes')
    sign_out_everywhere
  end
end
`,
		"spec/Login.java": "class Login {}",
	}), nil, nil)
	cap := skPO(t, k, "page-object:spec/support/pages/sign_in_page.rb#SignInPage")
	if cap.Form != FormCapybaraClass || cap.Extends != "BasePage" {
		t.Fatalf("capybara page = %+v", cap)
	}
	m := skMethod(t, cap, "sign_in")
	if skKinds(m.Actions) != "call,fill,fill,click" || m.Hops[0].Kind != "inherited" || m.Hops[0].Status != StatusResolved {
		t.Fatalf("sign_in actions %s hops %+v", skKinds(m.Actions), m.Hops)
	}
	sp := skPO(t, k, "page-object:spec/support/pages/search_page.rb#SearchPage")
	if sp.Form != FormSitePrismPage || sp.URL != "/search" || len(sp.Bindings) != 2 {
		t.Fatalf("siteprism page = %+v", sp)
	}
	tt := skTest(t, k, "user signs in")
	if tt.Framework != "capybara" || skKinds(tt.Steps) != "visit,fill,fill,click,assert" || !tt.Complete {
		t.Fatalf("framework %s steps %s", tt.Framework, skKinds(tt.Steps))
	}
	if tt.Steps[1].Value != "a@example.com" || !tt.Steps[2].Screened || tt.Steps[2].Value != "" {
		t.Fatalf("ruby fills = %+v / %+v", tt.Steps[1], tt.Steps[2])
	}
	if strings.Join(tt.Setup, ",") != "create" {
		t.Fatalf("setup = %v", tt.Setup)
	}
	s2 := skTest(t, k, "user searches")
	if skKinds(s2.Steps) != "visit,fill,click,opaque:unresolved-call" || s2.Steps[0].Target != "/search" || s2.Steps[1].Value != "shoes" {
		t.Fatalf("siteprism steps %s %+v", skKinds(s2.Steps), s2.Steps)
	}
	found := false
	for _, u := range k.Unknowns {
		if u.Path == "spec/Login.java" && u.Reason == "form-unsupported" && u.Form == "selenium-java" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unsupported form not named: %+v (SKH-V0-010)", k.Unknowns)
	}
}

func TestSKHV0_002_UnsupportedImportAndDeterminism(t *testing.T) {
	files := map[string]string{
		"e2e/legacy.spec.js":      "const { Builder } = require('x');\nimport { Builder } from 'selenium-webdriver';\n",
		"e2e/pages/base-page.ts":  skBasePage,
		"e2e/pages/login-page.ts": skLoginPage,
		"e2e/login.spec.ts":       skLoginSpec,
	}
	a := extractSiteKnowledge(skSrc(files), nil, nil)
	b := extractSiteKnowledge(skSrc(files), nil, nil)
	ja, _ := json.Marshal([]any{a.PageObjects, a.Tests, a.Records, a.Unknowns})
	jb, _ := json.Marshal([]any{b.PageObjects, b.Tests, b.Records, b.Unknowns})
	if string(ja) != string(jb) {
		t.Fatal("extraction is not deterministic (SKH-V0-003)")
	}
	if len(a.Unknowns) != 1 || a.Unknowns[0].Form != "selenium-js" {
		t.Fatalf("unknowns = %+v", a.Unknowns)
	}
}

func TestSKHV0_012_SpecLocalHelpersTemplatesAndValueAsserts(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/lib/reporter.mjs": `export default class CountingReporter {
  onTestEnd(test, result) { this.seen++; }
}
`,
		"e2e/area.spec.ts": `import { test, expect, type Page } from '@playwright/test';
const BASE = '/web';
async function seed(page: Page, rows: Record<string, Row>, label: string) {
  await page.getByLabel(label).fill('x');
}
test.describe('area', () => {
  async function gotoArea(page: Page, route: string): Promise<void> {
    await page.goto(` + "`${BASE}/area/${route}`" + `);
    await expect(page.getByRole('navigation', { name: 'Area' })).toBeVisible();
  }
  test('opens settings', async ({ page }) => {
    const errors: string[] = [];
    await gotoArea(page, 'settings');
    expect(errors, errors.join('\n')).toEqual([]);
  });
});
test.describe('other', () => {
  function gotoArea(page: Page) {}
});
`,
	}), nil, nil)
	for _, po := range k.PageObjects {
		if strings.Contains(po.ID, "reporter") {
			t.Fatalf("a class with no page, locator or base listed as a page object: %+v", po)
		}
	}
	po := skPO(t, k, "page-object:e2e/area.spec.ts")
	if m := skMethod(t, po, "seed"); strings.Join(m.Params, ",") != "page,rows,label" {
		t.Fatalf("params = %v, want generic type arguments skipped", m.Params)
	}
	for _, m := range po.Methods {
		if m.Name == "gotoArea" {
			t.Fatalf("a nested helper declared in two scopes resolved by guess: %+v", m)
		}
	}
	tt := skTest(t, k, "opens settings")
	// gotoArea is declared twice, so its call stays unresolved; the value assert is kept.
	if skKinds(tt.Steps) != "opaque:unresolved-call,assert" {
		t.Fatalf("steps = %s", skKinds(tt.Steps))
	}
	if a := tt.Steps[1]; a.Target != "value" || a.Matcher != "toEqual" || a.Expected != "[]" || a.Locator != nil {
		t.Fatalf("value assert = %+v", a)
	}

	// With one declaration, the spec-local helper inlines and the template binds its slot.
	k = extractSiteKnowledge(skSrc(map[string]string{"e2e/area.spec.ts": `import { test, expect, type Page } from '@playwright/test';
const BASE = '/web';
test.describe('area', () => {
  async function gotoArea(page: Page, route: string): Promise<void> {
    await page.goto(` + "`${BASE}/area/${route}`" + `);
  }
  test('opens settings', async ({ page }) => {
    await gotoArea(page, 'settings');
    await expect(page).toHaveURL(` + "`${BASE}/area/settings`" + `);
  });
});
`}), nil, nil)
	po = skPO(t, k, "page-object:e2e/area.spec.ts")
	if m := skMethod(t, po, "gotoArea"); m.Actions[0].Target != "/web/area/${route}" || strings.Join(m.Actions[0].Slots, ",") != "route" {
		t.Fatalf("template visit = %+v", m.Actions[0])
	}
	tt = skTest(t, k, "opens settings")
	if skKinds(tt.Steps) != "visit,assert" || tt.Steps[0].Target != "/web/area/settings" || tt.Steps[0].Slots != nil || !tt.Complete {
		t.Fatalf("bound template = %s %+v", skKinds(tt.Steps), tt.Steps)
	}
	if tt.Steps[1].Target != "page" || tt.Steps[1].Expected != "/web/area/settings" {
		t.Fatalf("page assert = %+v", tt.Steps[1])
	}
}

func TestSKHV0_023_StrictStepDecoderRefusesOutsideTheClosedIR(t *testing.T) {
	k := extractSiteKnowledge(skSrc(map[string]string{
		"e2e/a.spec.ts": `import { test, expect } from '@playwright/test';
test('a', async ({ page }) => {
  await page.goto('/a');
  await page.getByLabel('Name').fill('x');
  await page.keyboard.press('Enter');
  await expect(page.getByText('Saved')).toBeVisible();
});
`,
	}), nil, nil)
	for _, s := range skTest(t, k, "a").Steps {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := DecodeSKStep(b); err != nil || got.Kind != s.Kind {
			t.Fatalf("emitted step %s did not round-trip: %v", b, err)
		}
	}
	for _, bad := range []string{
		`{"kind":"drag"}`,
		`{"kind":"opaque","reason":"guess"}`,
		`{"kind":"click","reason":"dynamic-value"}`,
		`{"kind":"click","colour":"red"}`,
		`{"kind":"click"}{"kind":"click"}`,
	} {
		if _, err := DecodeSKStep([]byte(bad)); err == nil || !strings.Contains(err.Error(), "refused") {
			t.Fatalf("DecodeSKStep(%s) = %v, want a refusal (SKH-V0-023)", bad, err)
		}
	}
}
