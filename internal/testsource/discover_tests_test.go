package testsource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const discoverPlaywrightSpec = `import { test, expect } from './fixtures';
import { LoginPage } from '../pages/login';
import { seed } from '@app/seed';
import * as helpers from './helpers.js';

test.beforeEach(async ({ page }) => {
  await page.goto('/home');
});

test.describe('cart @smoke', () => {
  test.afterEach(async ({ page }) => {
    await expect(page).toHaveURL(/cart/);
  });

  test('adds an item', { tag: ['@fast'], annotation: { type: 'issue', description: 'GH-1' } }, async ({ page, request, account }) => {
    const login = new LoginPage(page);
    await login.open();
    await page.goto('/cart');
    const res = await request.post('/api/items', { data: {} });
    expect(res.ok()).toBeTruthy();
    await expect(page.locator('.item')).not.toHaveCount(0);
    helpers.check(page);
    test.skip(process.env.CI === '1', 'flaky on CI');
  });

  test('computed', async ({ page }) => {
    const mod = await import(name);
    await page.goto(base + '/x');
    expect.soft(page).toBeTruthy();
  });
});
`

const discoverPlaywrightFixtures = `import { test as base } from '@playwright/test';
export const test = base.extend({ account: async ({}, use) => { await use({}); } });
export { expect } from '@playwright/test';
`

const discoverRSpecSpec = `require 'rails_helper'
require_relative 'support/helpers'

RSpec.shared_examples 'a priced item' do
  it 'has a price' do
    expect(subject.price).to be > 0
  end
end

RSpec.describe Cart, type: :request do
  before do
    visit '/cart'
  end

  after(:all) { expect(Cart.count).to eq(0) }

  it 'adds an item', :slow, priority: 'high', owner: owner_name do
    post '/cart/items', params: { id: 1 }
    expect(response).to have_http_status(:created)
    expect { Cart.add(1) }.to change(Cart, :count).by(1)
    expect do
      Cart.clear
    end.not_to raise_error
    is_expected.to be_valid
    skip 'not yet' if ENV['CI']
  end

  it_behaves_like 'a priced item'
  include_examples shared_name

  context 'when empty' do
    xit 'shows nothing' do
      get path_for(:cart)
      expect(page).not_to have_css('.item')
    end
  end
end
`

func discoverFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	commit := commitFiles(t, root, map[string]string{
		"tests/cart.spec.ts":      discoverPlaywrightSpec,
		"tests/fixtures.ts":       discoverPlaywrightFixtures,
		"tests/helpers.ts":        "export function check(p) {}\n",
		"pages/login.ts":          "export class LoginPage {}\n",
		"spec/cart_spec.rb":       discoverRSpecSpec,
		"spec/support/helpers.rb": "module Helpers; end\n",
	})
	return root, commit
}

func discover(t *testing.T, root, runner, commit, path string) TestDiscovery {
	t.Helper()
	data, err := BuildTestDiscovery(context.Background(), TestDiscoveryInput{Root: root, Runner: runner, Revision: commit, Path: path})
	if err != nil {
		t.Fatalf("BuildTestDiscovery(%s): %v", runner, err)
	}
	var out TestDiscovery
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func corpusCode(err error, code string) bool {
	var typed *doccorpus.Error
	return errors.As(err, &typed) && typed.Code == code
}

func discoveredExample(t *testing.T, d TestDiscovery, line int) DiscoveredExample {
	t.Helper()
	for _, e := range d.Examples {
		if e.Line == line {
			return e
		}
	}
	t.Fatalf("no example at line %d in %+v", line, d.Examples)
	return DiscoveredExample{}
}

func discoveryReasons(d TestDiscovery) []string {
	out := []string{}
	for _, u := range d.Unknown {
		out = append(out, u.Reason)
	}
	return out
}

// checkAnchors verifies every anchor in a discovery document against the committed blob: the
// revision, the blob and file digests, and the span digest of exactly its lines.
func checkAnchors(t *testing.T, root, commit string, data []byte) int {
	t.Helper()
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	blobs := map[string][]byte{}
	count := 0
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if span, ok := v["span_sha256"].(string); ok {
				count++
				path := v["path"].(string)
				data, ok := blobs[path]
				if !ok {
					data = []byte(gitTest(t, root, "show", commit+":"+path) + "\n")
					blobs[path] = data
				}
				lines := bytes.SplitAfter(data, []byte("\n"))
				start, end := int(v["start_line"].(float64)), int(v["end_line"].(float64))
				if v["revision"] != commit || v["blob"] != blobAt(t, root, commit, path) || start < 1 || end < start || end > len(lines) {
					t.Fatalf("anchor %v does not pin the committed blob", v)
				}
				whole, part := sha256.Sum256(data), sha256.Sum256(bytes.Join(lines[start-1:end], nil))
				if v["sha256"] != hex.EncodeToString(whole[:]) || span != hex.EncodeToString(part[:]) || v["authority"] != "external-provider" {
					t.Fatalf("anchor %v digests do not match lines %d-%d", v, start, end)
				}
			}
			for _, x := range v {
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(doc)
	return count
}

// TestDCXV1370DiscoverTestsDeterministicAnchored pins DCX-V1-370: one generated, unreviewed
// document per runner, pinned to the full revision, byte-identical across runs, with every fact
// anchored to its committed lines; invalid input is refused with a typed code.
func TestDCXV1370DiscoverTestsDeterministicAnchored(t *testing.T) {
	root, commit := discoverFixture(t)
	for _, runner := range []string{"playwright", "rspec"} {
		input := TestDiscoveryInput{Root: root, Runner: runner, Revision: commit, Path: "."}
		first, err := BuildTestDiscovery(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		second, err := BuildTestDiscovery(context.Background(), input)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("%s: second run differs: %v", runner, err)
		}
		if n := checkAnchors(t, root, commit, first); n < 20 {
			t.Fatalf("%s: only %d anchors", runner, n)
		}
		d := discover(t, root, runner, commit, ".")
		if d.Schema != TestDiscoverySchema || d.Trust != "generated" || d.Revision != commit || d.Runner != runner || len(d.Files) != 1 || len(d.Limitations) == 0 {
			t.Fatalf("%s: document = %+v", runner, d)
		}
		for _, e := range d.Examples {
			if e.Review != "unreviewed" || e.Read != discoveryLocated {
				t.Fatalf("%s: example %s review/read = %q %q", runner, e.ID, e.Review, e.Read)
			}
		}
	}
	scoped := discover(t, root, "playwright", commit, "tests/cart.spec.ts")
	if len(scoped.Files) != 1 || scoped.Files[0].Path != "tests/cart.spec.ts" {
		t.Fatalf("path scope = %+v", scoped.Files)
	}
	for name, input := range map[string]TestDiscoveryInput{
		"runner":        {Root: root, Runner: "jest", Revision: commit, Path: "."},
		"short":         {Root: root, Runner: "rspec", Revision: commit[:12], Path: "."},
		"escape":        {Root: root, Runner: "rspec", Revision: commit, Path: "../x"},
		"no test files": {Root: root, Runner: "rspec", Revision: commit, Path: "pages"},
	} {
		if _, err := BuildTestDiscovery(context.Background(), input); !corpusCode(err, "corpus-refused") {
			t.Fatalf("%s: err = %v, want corpus-refused", name, err)
		}
	}
	missing := strings.Repeat("0", 40)
	if _, err := BuildTestDiscovery(context.Background(), TestDiscoveryInput{Root: root, Runner: "rspec", Revision: missing, Path: "."}); !corpusCode(err, "corpus-invalid-revision") {
		t.Fatalf("unknown revision: %v", err)
	}
}

// TestDCXV1371DiscoverTestsIdentityJoins pins DCX-V1-371: an RSpec example's identity is the key
// tests example-join gives the same example, a Playwright example's full name and file form the
// js-title-path key once a receipt supplies the project, and an id the source does not fix is an
// unknown with its reason.
func TestDCXV1371DiscoverTestsIdentityJoins(t *testing.T) {
	root := t.TempDir()
	loop := "RSpec.describe Calc do\n  [1, 2].each do |n|\n    it \"handles #{n}\" do\n      expect(n).to be_positive\n    end\n  end\nend\n"
	commit := commitFiles(t, root, map[string]string{"spec/calc_spec.rb": calcBefore, "tests/cart.spec.ts": cartA, "spec/loop_spec.rb": loop})
	ruby := discoveredExample(t, discover(t, root, "rspec", commit, "spec/calc_spec.rb"), 2)
	joined := joinOne(t, root, ExampleJoinOptions{Revision: commit, Ruby: calcDocument(t, root, commit, 2)})
	if ruby.Identity.Kind != ExampleIdentityRSpec || ruby.Identity.Example != joined.Examples[0].Example || ruby.Identity.FullName != "Calc adds" {
		t.Fatalf("ruby identity %+v, join key %q", ruby.Identity, joined.Examples[0].Example)
	}
	js := discoveredExample(t, discover(t, root, "playwright", commit, "."), 2)
	if js.Identity.Kind != ExampleIdentityJS || js.Identity.Unknown != discoverReasonProjectNotStatic || js.Identity.Example != "" {
		t.Fatalf("js identity = %+v", js.Identity)
	}
	receipt := jsReceipt(t, root, commit, cartA)
	receipt.Tests[0].FullName = js.Identity.FullName
	record := joinOne(t, root, ExampleJoinOptions{Revision: commit, JS: receipt}).Examples[0]
	if want := js.Identity.File + " > chromium > " + js.Identity.FullName; record.Example != want || record.IdentityKind != js.Identity.Kind {
		t.Fatalf("js join key %q, want %q", record.Example, want)
	}
	looped := discover(t, root, "rspec", commit, "spec/loop_spec.rb")
	if e := discoveredExample(t, looped, 3); e.Identity.Unknown != discoverReasonScopedLoop || e.TitleUnknown != discoverReasonTitleNotLiteral {
		t.Fatalf("loop example identity = %+v title %q", e.Identity, e.TitleUnknown)
	}
	fixture, fcommit := discoverFixture(t)
	d := discover(t, fixture, "rspec", fcommit, ".")
	if e := discoveredExample(t, d, 17); e.Identity.Example != "spec/cart_spec.rb[1:1]" {
		t.Fatalf("placed example = %+v", e.Identity)
	}
	if e := discoveredExample(t, d, 32); e.Identity.Unknown != discoverReasonScopedInline || e.Identity.FullName != "Cart when empty shows nothing" {
		t.Fatalf("after a computed inline include = %+v", e.Identity)
	}
	if e := discoveredExample(t, d, 5); e.Identity.Unknown != discoverReasonSharedDefinition || e.TitleUnknown != discoverReasonSharedDefinition {
		t.Fatalf("shared definition example = %+v", e)
	}
	// An inline inclusion adds one slot per runtime child of the shared body: a loop there makes
	// the count, and every later sibling's id, unknown; a nested inline inclusion adds its own.
	shared := map[string]string{
		"spec/shared_loop_spec.rb":  "RSpec.describe Calc do\n  shared_examples 'looped' do\n    [1, 2].each do |n|\n      it \"handles #{n}\" do\n        expect(n).to be_positive\n      end\n    end\n  end\n\n  include_examples 'looped'\n  it 'follows' do\n    expect(1).to eq(1)\n  end\nend\n",
		"spec/shared_nest_spec.rb":  "RSpec.describe Calc do\n  shared_examples 'inner' do\n    it('a') { expect(1).to eq(1) }\n    it('b') { expect(2).to eq(2) }\n  end\n  shared_examples 'outer' do\n    include_examples 'inner'\n  end\n\n  include_examples 'outer'\n  it 'follows' do\n    expect(1).to eq(1)\n  end\nend\n",
		"spec/shared_cycle_spec.rb": "RSpec.describe Calc do\n  shared_examples 'self' do\n    include_examples 'self'\n  end\n  include_examples 'self'\n  it 'follows' do\n    expect(1).to eq(1)\n  end\nend\n",
	}
	scommit := commitFiles(t, root, shared)
	if e := discoveredExample(t, discover(t, root, "rspec", scommit, "spec/shared_loop_spec.rb"), 11); e.Identity.Unknown != discoverReasonScopedLoop || e.Identity.Example != "" {
		t.Fatalf("after an inline include with a loop = %+v", e.Identity)
	}
	if e := discoveredExample(t, discover(t, root, "rspec", scommit, "spec/shared_nest_spec.rb"), 11); e.Identity.Unknown != "" || e.Identity.Example != "spec/shared_nest_spec.rb[1:3]" {
		t.Fatalf("after a nested inline include = %+v", e.Identity)
	}
	if e := discoveredExample(t, discover(t, root, "rspec", scommit, "spec/shared_cycle_spec.rb"), 6); e.Identity.Unknown != discoverReasonScopedInline || e.Identity.Example != "" {
		t.Fatalf("after a cyclic inline include = %+v", e.Identity)
	}
}

type expectationRow struct {
	matcher, receiver string
	negated           bool
	lines             [2]int
}

func expectationRows(e DiscoveredExample) []expectationRow {
	out := []expectationRow{}
	for _, x := range e.Expectations {
		out = append(out, expectationRow{x.Matcher, x.Receiver, x.Negated, [2]int{x.Evidence.Start, x.Evidence.End}})
	}
	return out
}

// TestDCXV1372DiscoverTestsExpectations pins DCX-V1-372: each expectation's matcher, negation,
// receiver and line span, including multi-line RSpec block expectations; a shape not read is an
// expectation with matcher-not-read or assertion-shape-not-read, never dropped.
func TestDCXV1372DiscoverTestsExpectations(t *testing.T) {
	root, commit := discoverFixture(t)
	js := discoveredExample(t, discover(t, root, "playwright", commit, "."), 15)
	want := []expectationRow{{"toBeTruthy", "res.ok()", false, [2]int{20, 20}}, {"toHaveCount", `page.locator(".item")`, true, [2]int{21, 21}}}
	if got := expectationRows(js); !slices.Equal(got, want) {
		t.Fatalf("js expectations = %+v", got)
	}
	ruby := discoveredExample(t, discover(t, root, "rspec", commit, "."), 17)
	want = []expectationRow{{"have_http_status", "response", false, [2]int{19, 19}}, {"change", "{ Cart.add(1) }", false, [2]int{20, 20}},
		{"raise_error", "block", true, [2]int{21, 23}}, {"be_valid", "subject", false, [2]int{24, 24}}}
	if got := expectationRows(ruby); !slices.Equal(got, want) {
		t.Fatalf("ruby expectations = %+v", got)
	}
	odd := commitFiles(t, root, map[string]string{
		"spec/odd_spec.rb":  "RSpec.describe Odd do\n  it 'checks' do\n    expect(x).to\n    assert_equal 1, y\n    value.should_not eq(2)\n  end\nend\n",
		"tests/odd.spec.ts": "import { test, expect } from '@playwright/test';\ntest('odd', async () => {\n  expect(a)[m](1);\n  assert.notEqual(a, b);\n});\n",
	})
	rows := expectationRows(discoveredExample(t, discover(t, root, "rspec", odd, "spec/odd_spec.rb"), 2))
	if len(rows) != 3 || rows[0].matcher != "" || rows[1] != (expectationRow{"assert_equal", "1", false, [2]int{4, 4}}) || rows[2] != (expectationRow{"eq", "value", true, [2]int{5, 5}}) {
		t.Fatalf("odd ruby expectations = %+v", rows)
	}
	if x := discoveredExample(t, discover(t, root, "rspec", odd, "spec/odd_spec.rb"), 2).Expectations[0]; x.Unknown != discoverReasonMatcherUnread {
		t.Fatalf("unread matcher = %+v", x)
	}
	// A nested block expectation: each matcher is the one after the line closing its own block.
	nested := commitFiles(t, root, map[string]string{
		"spec/nested_spec.rb": "RSpec.describe Nested do\n  it 'checks' do\n    expect do\n      expect do\n        Y.create\n      end.to raise_error(A)\n      [1].each { |v| v }\n    end.to change(Y, :count)\n    expect do\n      x\n    end\n    y.call\n  end\nend\n",
	})
	rows = expectationRows(discoveredExample(t, discover(t, root, "rspec", nested, "spec/nested_spec.rb"), 2))
	if want := []expectationRow{{"change", "block", false, [2]int{3, 8}}, {"raise_error", "block", false, [2]int{4, 6}}}; len(rows) != 3 || !slices.Equal(rows[:2], want) || rows[2].matcher != "" {
		t.Fatalf("nested ruby expectations = %+v", rows)
	}
	jsOdd := discoveredExample(t, discover(t, root, "playwright", odd, "tests/odd.spec.ts"), 2)
	if len(jsOdd.Expectations) != 2 || jsOdd.Expectations[0].Unknown != discoverReasonMatcherUnread || jsOdd.Expectations[1].Matcher != "notEqual" || !jsOdd.Expectations[1].Negated || jsOdd.Expectations[1].Receiver != "a" {
		t.Fatalf("odd js expectations = %+v", jsOdd.Expectations)
	}
}

type navigationRow struct {
	kind, method, target, source, unknown string
	line                                  int
}

func navigationRows(e DiscoveredExample) []navigationRow {
	out := []navigationRow{}
	for _, n := range e.Navigations {
		out = append(out, navigationRow{n.Kind, n.Method, n.Target, n.Source, n.Unknown, n.Evidence.Start})
	}
	return out
}

// TestDCXV1373DiscoverTestsNavigations pins DCX-V1-373: goto, visit and request helpers in the
// example and its applicable hooks, with the method and a literal target, and target-not-literal
// for a computed one.
func TestDCXV1373DiscoverTestsNavigations(t *testing.T) {
	root, commit := discoverFixture(t)
	pw := discover(t, root, "playwright", commit, ".")
	want := []navigationRow{{"goto", "", "/home", "hook", "", 7}, {"goto", "", "/cart", "example", "", 18}, {"request", "POST", "/api/items", "example", "", 19}}
	if got := navigationRows(discoveredExample(t, pw, 15)); !slices.Equal(got, want) {
		t.Fatalf("js navigations = %+v", got)
	}
	if got := navigationRows(discoveredExample(t, pw, 26)); len(got) != 2 || got[1] != (navigationRow{"goto", "", "", "example", discoverReasonTargetNotLiteral, 28}) {
		t.Fatalf("computed goto = %+v", got)
	}
	rs := discover(t, root, "rspec", commit, ".")
	want = []navigationRow{{"visit", "", "/cart", "hook", "", 12}, {"request", "POST", "/cart/items", "example", "", 18}}
	if got := navigationRows(discoveredExample(t, rs, 17)); !slices.Equal(got, want) {
		t.Fatalf("ruby navigations = %+v", got)
	}
	if got := navigationRows(discoveredExample(t, rs, 32)); len(got) != 2 || got[1] != (navigationRow{"request", "GET", "", "example", discoverReasonTargetNotLiteral, 33}) {
		t.Fatalf("computed get = %+v", got)
	}
}

type reachRow struct {
	kind, name, specifier, target, state, reason string
}

func reachRows(e DiscoveredExample) []reachRow {
	out := []reachRow{}
	for _, r := range e.Reaches {
		out = append(out, reachRow{r.Kind, r.Name, r.Specifier, r.Target, r.State, r.Reason})
	}
	return out
}

// TestDCXV1374DiscoverTestsReaches pins DCX-V1-374: page objects, helpers and fixtures reached
// by static import, shared groups reached by inclusion and files by require_relative, each with
// its declaration anchor; a computed lookup, an alias and an unregistered shared group carry the
// setup-select reasons.
func TestDCXV1374DiscoverTestsReaches(t *testing.T) {
	root, commit := discoverFixture(t)
	pw := discover(t, root, "playwright", commit, ".")
	want := []reachRow{
		{"import", "LoginPage", "../pages/login", "pages/login.ts", "repository", ""},
		{"import", "expect", "./fixtures", "tests/fixtures.ts", "repository", ""},
		{"import", "helpers", "./helpers.js", "tests/helpers.ts", "repository", ""},
		{"fixture", "page", "@playwright/test", "", "builtin", ""},
		{"fixture", "request", "@playwright/test", "", "builtin", ""},
		{"fixture", "account", "./fixtures", "tests/fixtures.ts", "repository", discoverReasonFixtureBody},
	}
	first := discoveredExample(t, pw, 15)
	if got := reachRows(first); !slices.Equal(got, want) {
		t.Fatalf("js reaches = %+v", got)
	}
	for _, r := range first.Reaches {
		if r.Kind == "import" && (r.Declaration == nil || r.Declaration.Start > 4) {
			t.Fatalf("import reach without its import anchor: %+v", r)
		}
	}
	if got := reachRows(discoveredExample(t, pw, 26)); got[0] != (reachRow{"import", "", "name", "", "unresolved", discoverReasonComputedLookup}) {
		t.Fatalf("dynamic import = %+v", got)
	}
	alias := commitFiles(t, root, map[string]string{"tests/alias.spec.ts": "import { test } from '@playwright/test';\nimport { seed } from '@/seed';\ntest('a', async () => { seed(); });\n"})
	if got := reachRows(discoveredExample(t, discover(t, root, "playwright", alias, "tests/alias.spec.ts"), 3)); !slices.Equal(got, []reachRow{{"import", "seed", "@/seed", "", "unresolved", discoverReasonImportUnresolved}}) {
		t.Fatalf("alias reach = %+v", got)
	}
	rs := discover(t, root, "rspec", commit, ".")
	want = []reachRow{
		{"shared-example", "", "shared_name", "", "unresolved", discoverReasonComputedLookup},
		{"import", "", "rails_helper", "", "unresolved", discoverReasonLoadPath},
		{"import", "", "support/helpers", "spec/support/helpers.rb", "repository", ""},
	}
	if got := reachRows(discoveredExample(t, rs, 17)); !slices.Equal(got, want) {
		t.Fatalf("ruby reaches = %+v", got)
	}
	shared := commitFiles(t, root, map[string]string{"spec/shared_spec.rb": "RSpec.shared_context 'signed in' do\n  let(:user) { 1 }\nend\n\nRSpec.describe Shared do\n  include_context 'signed in'\n  it_behaves_like 'elsewhere'\n  it 'works' do\n    expect(user).to eq(1)\n  end\nend\n"})
	e := discoveredExample(t, discover(t, root, "rspec", shared, "spec/shared_spec.rb"), 8)
	got := reachRows(e)
	if len(got) != 1 || got[0] != (reachRow{"shared-context", "signed in", "", "spec/shared_spec.rb", "repository", ""}) || e.Reaches[0].Declaration == nil || e.Reaches[0].Declaration.Start != 1 {
		t.Fatalf("shared context reach = %+v", e.Reaches)
	}
	nested := commitFiles(t, root, map[string]string{"spec/nested_spec.rb": "RSpec.describe Nested do\n  it_behaves_like 'elsewhere' do\n    it 'customizes' do\n      expect(1).to eq(1)\n    end\n  end\nend\n"})
	if got := reachRows(discoveredExample(t, discover(t, root, "rspec", nested, "spec/nested_spec.rb"), 3)); !slices.Equal(got, []reachRow{{"shared-example", "elsewhere", "", "", "unresolved", discoverReasonRegistry}}) {
		t.Fatalf("unregistered shared group = %+v", got)
	}
}

// TestDCXV1375DiscoverTestsHooksTagsAnnotations pins DCX-V1-375: the hooks that apply to each
// example, outermost first, with scope and whether the hook body asserts, and the example's
// tags and annotations from its declaration, its groups and its runtime skip calls.
func TestDCXV1375DiscoverTestsHooksTagsAnnotations(t *testing.T) {
	root, commit := discoverFixture(t)
	js := discoveredExample(t, discover(t, root, "playwright", commit, "."), 15)
	hooks := fmt.Sprint(js.Hooks[0].Kind, js.Hooks[0].Scope, js.Hooks[0].Asserts, js.Hooks[1].Kind, js.Hooks[1].Asserts)
	if len(js.Hooks) != 2 || hooks != "beforeEacheachfalseafterEachtrue" {
		t.Fatalf("js hooks = %+v", js.Hooks)
	}
	if tags := []string{js.Tags[0].Name, js.Tags[1].Name}; len(js.Tags) != 2 || !slices.Equal(tags, []string{"@smoke", "@fast"}) {
		t.Fatalf("js tags = %+v", js.Tags)
	}
	if a := js.Annotations; len(a) != 2 || a[0] != (DiscoveredAnnotation{Type: "issue", Description: "GH-1", Evidence: a[0].Evidence}) ||
		a[1].Type != "skip" || !a[1].Conditional || a[1].Description != "flaky on CI" || a[1].Evidence.Start != 23 {
		t.Fatalf("js annotations = %+v", a)
	}
	rs := discover(t, root, "rspec", commit, ".")
	e := discoveredExample(t, rs, 17)
	if len(e.Hooks) != 2 || e.Hooks[0].Kind != "before" || e.Hooks[0].Scope != "each" || e.Hooks[0].Asserts || e.Hooks[1].Kind != "after" || e.Hooks[1].Scope != "all" || !e.Hooks[1].Asserts {
		t.Fatalf("ruby hooks = %+v", e.Hooks)
	}
	tags := []string{}
	for _, tag := range e.Tags {
		tags = append(tags, tag.Name+"="+tag.Value)
	}
	if !slices.Equal(tags, []string{"type=:request", "slow=true", "priority=high", "owner="}) {
		t.Fatalf("ruby tags = %v", tags)
	}
	if a := e.Annotations; len(a) != 1 || a[0].Type != "skip" || a[0].Description != "not yet" || !a[0].Conditional {
		t.Fatalf("ruby annotations = %+v", a)
	}
	if a := discoveredExample(t, rs, 32).Annotations; len(a) != 1 || a[0].Type != "skip" || a[0].Conditional || a[0].Evidence.Start != 32 {
		t.Fatalf("xit annotation = %+v", a)
	}
}

// TestDCXV1376DiscoverTestsUnknowns pins DCX-V1-376: what the reader cannot classify is an
// unknown with a reason (a non-literal metadata value, a file the reader cannot balance, a file
// over the byte bound, a fact list over its bound) rather than a guess or a silent drop.
func TestDCXV1376DiscoverTestsUnknowns(t *testing.T) {
	root, commit := discoverFixture(t)
	if got := discoveryReasons(discover(t, root, "rspec", commit, ".")); !slices.Contains(got, discoverReasonMetadata) {
		t.Fatalf("metadata unknown missing: %v", got)
	}
	many := strings.Builder{}
	many.WriteString("RSpec.describe Many do\n  it 'checks' do\n")
	for i := 0; i < maxDiscoveryFacts+3; i++ {
		fmt.Fprintf(&many, "    expect(%d).to eq(%d)\n", i, i)
	}
	many.WriteString("  end\nend\n")
	bad := commitFiles(t, root, map[string]string{
		"spec/broken_spec.rb":  "RSpec.describe Broken do\n  it 'never closes' do\n    expect(1).to eq(1)\n",
		"spec/many_spec.rb":    many.String(),
		"spec/huge_spec.rb":    "# " + strings.Repeat("x", MaxFileBytes) + "\n",
		"tests/broken.spec.ts": "test('x', async () => {\n  expect(`${a`).toBe(1);\n",
	})
	d := discover(t, root, "rspec", bad, "spec")
	reasons := discoveryReasons(d)
	for _, want := range []string{validityRSpecUnbalanced, discoverReasonFactBound, discoverReasonFileTooLarge} {
		if !slices.Contains(reasons, want) {
			t.Fatalf("reasons %v lack %s", reasons, want)
		}
	}
	reads := map[string]string{}
	for _, f := range d.Files {
		reads[f.Path] = f.Read
	}
	if reads["spec/broken_spec.rb"] != validityRSpecUnbalanced || reads["spec/huge_spec.rb"] != discoverReasonFileTooLarge {
		t.Fatalf("file reads = %v", reads)
	}
	if e := discoveredExample(t, d, 2); len(e.Expectations) != maxDiscoveryFacts {
		t.Fatalf("bounded expectations = %d", len(e.Expectations))
	}
	// A fact whose span cannot be anchored (a line over the 64 KiB excerpt bound) is dropped with
	// an explicit unknown, never emitted with a zero-valued anchor.
	long := strings.Repeat("y", 70<<10)
	wide := commitFiles(t, root, map[string]string{
		"tests/wide.spec.ts": "import { test, expect } from '@playwright/test';\ntest('wide', async () => {\n  expect(x).toBe('" + long + "');\n  expect(y).toBe(1);\n});\n",
		"spec/wide_spec.rb":  "RSpec.describe Wide do\n  it 'checks' do\n    expect(x).to eq('" + long + "')\n    expect(y).to eq(1)\n  end\nend\n",
	})
	for _, c := range []struct{ runner, path string }{{"playwright", "tests/wide.spec.ts"}, {"rspec", "spec/wide_spec.rb"}} {
		w := discover(t, root, c.runner, wide, c.path)
		e := discoveredExample(t, w, 2)
		if len(e.Expectations) != 1 || e.Expectations[0].Evidence.Start != 4 || !slices.Contains(discoveryReasons(w), discoverReasonFactUnanchored) {
			t.Fatalf("%s unanchorable expectation = %+v unknown %v", c.runner, e.Expectations, w.Unknown)
		}
		for _, x := range e.Expectations {
			if x.Evidence == (doccorpus.Anchor{}) {
				t.Fatalf("%s zero-valued anchor emitted: %+v", c.runner, x)
			}
		}
	}
	js := discover(t, root, "playwright", bad, "tests/broken.spec.ts")
	if len(js.Examples) != 0 || js.Files[0].Read != validitySourceUnparsed || !slices.Contains(discoveryReasons(js), validitySourceUnparsed) {
		t.Fatalf("unparsed js = %+v", js)
	}
}
