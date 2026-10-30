package testsource

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const seedLegacySpec = `RSpec.describe Order do
  let(:customer) { create(:customer, tier: :gold) }

  before do
    create_list(:line_item, 3, order: order, amount: 10)
  end

  let(:order) { create(:order, customer: customer, status: 'open') }

  it 'totals the line items' do
    expect(order.line_items.sum(&:amount)).to eq(30)
    expect(order.line_items.count).to eq(3)
  end

  it 'is unrelated' do
    create(:refund)
  end
end
`

const seedNewSpec = `RSpec.describe Order do
  let(:customer) { create(:customer) }
  let(:order) { create(:order, customer: customer, status: 'open') }

  it 'totals the line items' do
    create(:line_item, order: order, amount: 10)
    expect(order.line_items.sum(&:amount)).to eq(10)
    expect(order.line_items.count).to eq(1)
  end
end
`

// seedSameSpec seeds the legacy shape through other spellings: FactoryBot., paren-less calls,
// let!, split list calls and reordered attributes.
const seedSameSpec = `RSpec.describe Order do
  let!(:order) { FactoryBot.create(:order, status: "open", customer: customer) }
  let(:customer) { create :customer, tier: :gold }

  it 'totals the line items' do
    FactoryBot.create_list :line_item, 2, order: order, amount: 10
    create(:line_item, amount: 10, order: order)
    expect(order.line_items.sum(&:amount)).to eq(30)
  end
end
`

const seedUnsupportedSpec = `RSpec.describe Order do
  before(:all) { create(:customer) }

  it 'totals the line items' do
    order = Order.create!(status: 'open')
    expect(order.total).to eq(0)
  end
end
`

const seedFixtureSpec = `RSpec.describe User do
  fixtures :users

  it 'counts users' do
    expect(User.count).to eq(2)
  end
end

describe 'erb' do
  fixtures :accounts
  it 'reads accounts' do
    expect(Account.count).to eq(1)
  end
end

describe 'missing' do
  fixtures(:missing)
  it 'reads nothing' do
    expect(1).to eq(1)
  end
end
`

const seedUsersYAML = `# committed fixtures
alice:
  name: Alice
  admin: true
bob:
  name: "Bob"
  profile:
    bio: hi
`

const seedUserSpec = `RSpec.describe User do
  it 'counts users' do
    create(:user, name: 'Alice', admin: true)
    expect(User.count).to eq(1)
  end
end
`

const seedLegacyJS = `import { factory } from './factory';

describe('cart', () => {
  beforeEach(async () => {
    await factory.createList('item', 2, { price: 5 });
  });

  it('totals items', async () => {
    const cart = await factory.create('cart', { currency: 'EUR' });
    expect(cart.items.reduce((a, i) => a + i.price, 0)).toBe(10);
  });
});

describe('other', () => {
  beforeEach(() => { factory.create('coupon'); });
  it('is unrelated', () => { expect(1).toBe(1); });
});
`

const seedNewJS = `import { build, create, createList } from './factory';

test('totals items', async () => {
  const cart = await create('cart');
  const [item] = await createList('item', 1, { price: 10 });
  expect(cart.items.reduce((a, i) => a + i.price, 0)).toBe(10);
});

test('inserts', async () => {
  await db.insert('item', { price: 10 });
  expect(1).toBe(1);
});
`

func seedRepo(t *testing.T) Repo {
	t.Helper()
	root := t.TempDir()
	commitFiles(t, root, map[string]string{
		"spec/legacy/orders_spec.rb":       seedLegacySpec,
		"spec/new/orders_spec.rb":          seedNewSpec,
		"spec/new/same_spec.rb":            seedSameSpec,
		"spec/new/unsupported_spec.rb":     seedUnsupportedSpec,
		"spec/legacy/users_spec.rb":        seedFixtureSpec,
		"spec/fixtures/users.yml":          seedUsersYAML,
		"test/fixtures/accounts.yml":       "one:\n  name: <%= 'x' %>\n",
		"spec/new/users_spec.rb":           seedUserSpec,
		"test/legacy/cart.test.ts":         seedLegacyJS,
		"test/new/cart.test.ts":            seedNewJS,
		"docs/readme.md":                   "# readme\n",
		"spec/legacy/plain_helper_spec.rb": "RSpec.describe X do\n  it 'x' do\n    expect(1).to eq(1)\n  end\nend\n",
	})
	return OpenRepo(context.Background(), root)
}

func seedRun(t *testing.T, repo Repo, links ...SeedLink) []SeedParity {
	t.Helper()
	report, err := SourceValidity(repo, SourceValidityOptions{Links: links})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 0 || len(report.Tests) != 0 || len(report.SeedParity) != len(links) {
		t.Fatalf("report %+v", report)
	}
	return report.SeedParity
}

// seedJSON renders v as the report does, with < and > unescaped for readable expectations.
func seedJSON(v any) string {
	data, _ := json.Marshal(v)
	return strings.NewReplacer(`\u003c`, "<", `\u003e`, ">").Replace(string(data))
}

func seedRules(p SeedParity) string {
	rules := []string{}
	for _, f := range p.Findings {
		if f.Severity != testvalidity.SeverityAdvisory || f.Test != p.New {
			return "non-advisory or misattributed finding " + f.Rule
		}
		rules = append(rules, f.Rule)
	}
	return strings.Join(rules, ",")
}

// TestTSAV0101SeedShapeRSpec: the RSpec side reads FactoryBot create/create_list in the example
// body, its before hook and the lazy lets that scope invokes, normalised to kinds, summed counts
// and explicitly set attributes; another example's setup is not read (TSA-V0-101, TSA-V0-102).
func TestTSAV0101SeedShapeRSpec(t *testing.T) {
	p := seedRun(t, seedRepo(t), SeedLink{New: "spec/new/orders_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"})[0]
	legacy := seedJSON(p.Sides[0])
	want := `{"side":"legacy","test":"spec/legacy/orders_spec.rb:10","analysed":true,"kinds":[` +
		`{"kind":"customer","count":1,"attributes":[{"name":"tier","values":["\"gold\""]}]},` +
		`{"kind":"line_item","count":3,"attributes":[{"name":"amount","values":["10"]},{"name":"order","values":["<expr>"]}]},` +
		`{"kind":"order","count":1,"attributes":[{"name":"customer","values":["<expr>"]},{"name":"status","values":["\"open\""]}]}]}`
	if legacy != want {
		t.Fatalf("legacy side\n got %s\nwant %s", legacy, want)
	}
	if s := p.Sides[1]; s.Side != "new" || !s.Analysed || len(s.Kinds) != 3 || s.Kinds[0].Count != 1 || len(s.Kinds[0].Attributes) != 0 || s.Kinds[1].Count != 1 {
		t.Errorf("new side %+v", s)
	}
}

// TestTSAV0104SeedCountDivergence: an N vs M count difference names both counts and the
// asserting lines on each side that read the kind, ordered before differences no assertion reads
// (TSA-V0-104, TSA-V0-103).
func TestTSAV0104SeedCountDivergence(t *testing.T) {
	p := seedRun(t, seedRepo(t), SeedLink{New: "spec/new/orders_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"})[0]
	if p.Status != testvalidity.SeedParityStatusDiverges || seedRules(p) != "SEED-PARITY-DIVERGES,SEED-PARITY-DIVERGES,SEED-AGGREGATE-SINGLE-MEMBER,SEED-AGGREGATE-SINGLE-MEMBER" {
		t.Fatalf("status %s rules %s %+v", p.Status, seedRules(p), p.Findings)
	}
	f := p.Findings[0]
	if f.Location != "spec/new/orders_spec.rb:6" || f.Cause != "spec/legacy/orders_spec.rb:5" ||
		f.Message != "legacy seeds 3 line_item, new seeds 1; asserting lines reading line_item: legacy spec/legacy/orders_spec.rb:11, spec/legacy/orders_spec.rb:12; new spec/new/orders_spec.rb:7, spec/new/orders_spec.rb:8" {
		t.Errorf("count finding %+v", f)
	}
}

// TestTSAV0105SeedExplicitVersusDefault: an attribute set explicitly on one side and left at its
// default on the other is reported; differing literal values are reported (TSA-V0-105).
func TestTSAV0105SeedExplicitVersusDefault(t *testing.T) {
	repo := seedRepo(t)
	p := seedRun(t, repo, SeedLink{New: "spec/new/orders_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"})[0]
	if f := p.Findings[1]; f.Location != "spec/new/orders_spec.rb:2" || f.Cause != "spec/legacy/orders_spec.rb:2" ||
		!strings.HasPrefix(f.Message, `customer.tier is set explicitly on the legacy side ("gold") and left at its default on the new side;`) {
		t.Errorf("explicit/default finding %+v", f)
	}
	js := seedRun(t, repo, SeedLink{New: "test/new/cart.test.ts:3", Legacy: "test/legacy/cart.test.ts:8"})[0]
	messages := []string{}
	for _, f := range js.Findings {
		messages = append(messages, f.Rule+" "+f.Message)
	}
	got := strings.Join(messages, "\n")
	for _, want := range []string{
		"SEED-PARITY-DIVERGES legacy seeds 2 item, new seeds 1; asserting lines reading item: legacy test/legacy/cart.test.ts:10; new test/new/cart.test.ts:6",
		"SEED-PARITY-DIVERGES item.price differs: legacy 5, new 10;",
		`SEED-PARITY-DIVERGES cart.currency is set explicitly on the legacy side ("EUR") and left at its default on the new side;`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Index(got, "item.price") > strings.Index(got, "cart.currency") {
		t.Errorf("an assertion-read difference must order first:\n%s", got)
	}
}

// TestTSAV0106SeedAggregateSingleMember: a group-aggregate assertion reading a kind one side seeds
// once is reported on both sides (TSA-V0-106).
func TestTSAV0106SeedAggregateSingleMember(t *testing.T) {
	repo := seedRepo(t)
	for _, c := range []struct {
		link      SeedLink
		locations []string
		cause     string
	}{
		{SeedLink{New: "spec/new/orders_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"}, []string{"spec/legacy/orders_spec.rb:11", "spec/new/orders_spec.rb:7"}, "spec/new/orders_spec.rb:6"},
		{SeedLink{New: "test/new/cart.test.ts:3", Legacy: "test/legacy/cart.test.ts:8"}, []string{"test/legacy/cart.test.ts:10", "test/new/cart.test.ts:6"}, "test/new/cart.test.ts:5"},
	} {
		p := seedRun(t, repo, c.link)[0]
		got := []string{}
		for _, f := range p.Findings {
			if f.Rule == testvalidity.RuleSeedAggregateSingleMember {
				if f.Cause != c.cause || !strings.Contains(f.Message, "the new side seeds only one") {
					t.Errorf("aggregate finding %+v", f)
				}
				got = append(got, f.Location)
			}
		}
		if strings.Join(got, ",") != strings.Join(c.locations, ",") {
			t.Errorf("%v: aggregate locations %v, want %v", c.link, got, c.locations)
		}
	}
}

// TestTSAV0103SeedIdenticalShapesParity: identical normalised shapes written differently report
// parity with no finding (TSA-V0-103).
func TestTSAV0103SeedIdenticalShapesParity(t *testing.T) {
	p := seedRun(t, seedRepo(t), SeedLink{New: "spec/new/same_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"})[0]
	if p.Status != testvalidity.SeedParityStatusParity || len(p.Findings) != 0 {
		t.Fatalf("parity %s %+v sides %+v", p.Status, p.Findings, p.Sides)
	}
}

// TestTSAV0107SeedNotAnalysed: an unsupported form or unresolved scope abstains naming the side
// and line; parity and divergence are never reported when either side was not analysed
// (TSA-V0-107).
func TestTSAV0107SeedNotAnalysed(t *testing.T) {
	repo := seedRepo(t)
	ps := seedRun(t, repo,
		SeedLink{New: "spec/new/unsupported_spec.rb:4", Legacy: "spec/legacy/orders_spec.rb:10"},
		SeedLink{New: "spec/legacy/orders_spec.rb:10", Legacy: "spec/new/unsupported_spec.rb:4"},
		SeedLink{New: "test/new/cart.test.ts:9", Legacy: "docs/readme.md:1"},
		SeedLink{New: "spec/new/orders_spec.rb:7", Legacy: "test/legacy/cart.test.ts:99"},
	)
	for k, want := range [][]string{
		{"new", "seed-scope-unresolved", "spec/new/unsupported_spec.rb:2"},
		{"legacy", "seed-scope-unresolved", "spec/new/unsupported_spec.rb:2"},
		{"legacy", "seed-file-unsupported", "docs/readme.md:1", "new", "seed-form-unsupported", "test/new/cart.test.ts:10"},
		{"legacy", "seed-example-not-found", "test/legacy/cart.test.ts:99", "new", "seed-example-not-found", "spec/new/orders_spec.rb:7"},
	} {
		p := ps[k]
		if p.Status != testvalidity.SeedParityStatusNotAnalysed || len(p.Findings) != len(want)/3 {
			t.Fatalf("link %d: %s %+v", k, p.Status, p.Findings)
		}
		for j, f := range p.Findings {
			side, reason, at := want[3*j], want[3*j+1], want[3*j+2]
			if f.Rule != testvalidity.RuleSeedParityNotAnalysed || f.Location != at ||
				!strings.HasPrefix(f.Message, "the "+side+" side's setup was not analysed ("+reason+" at "+at+")") {
				t.Errorf("link %d finding %d %+v", k, j, f)
			}
		}
		for _, s := range p.Sides {
			if !s.Analysed && len(s.Kinds) != 0 {
				t.Errorf("an unanalysed side lists kinds %+v", s)
			}
		}
	}
	// Only the first abstention of a side is named: Order.create! (line 5) follows before(:all).
	if ps[0].Sides[1].Reason != "seed-scope-unresolved" {
		t.Errorf("sides %+v", ps[0].Sides)
	}
	for _, src := range []struct{ name, text, reason, at string }{
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    order = Order.create!(status: 'open')\n  end\nend\n", "seed-form-unsupported", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    create(:order, **attrs)\n  end\nend\n", "seed-attributes-not-literal", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    create_list(:order, n)\n  end\nend\n", "seed-count-not-literal", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    create(kind)\n  end\nend\n", "seed-kind-not-literal", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    create(:order) do |o|\n      o.x = 1\n    end\n  end\nend\n", "seed-form-unsupported", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    seed_orders\n  end\nend\n", "seed-form-unsupported", "spec/a_spec.rb:3"},
		{"spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    expect(1).to eq(1)\n  end\n  end\nend\n", "seed-source-uncertain", "spec/a_spec.rb:2"},
		{"a.test.ts", "test('a', () => {\n  createList('order', n);\n});\n", "seed-count-not-literal", "a.test.ts:2"},
		{"a.test.ts", "test('a', () => {\n  create('order', { ...base });\n});\n", "seed-attributes-not-literal", "a.test.ts:2"},
		{"a.test.ts", "test('a', () => {\n  create(kind);\n});\n", "seed-kind-not-literal", "a.test.ts:2"},
		{"a.test.ts", "beforeEach(seedOrders);\ntest('a', () => {\n  expect(1).toBe(1);\n});\n", "seed-form-unsupported", "a.test.ts:1"},
		{"a.test.ts", "const order = createOrder();\ntest('a', () => {\n  expect(order).toBeDefined();\n});\n", "seed-scope-unresolved", "a.test.ts:1"},
		{"a.test.ts", "test('a', () => {\n  expect(1).toBe(1);\n", "seed-source-uncertain", "a.test.ts:1"},
	} {
		line := 2
		if strings.HasPrefix(src.text, "test(") {
			line = 1
		}
		if strings.HasPrefix(src.text, "beforeEach") || strings.HasPrefix(src.text, "const") {
			line = 2
		}
		x, err := extractSeed(src.name, src.text, line, func(string) ([]byte, bool, error) { return nil, false, nil })
		if err != nil || x.reason != src.reason || x.at != src.at {
			t.Errorf("%q: %+v %v, want %s at %s", src.text, x, err, src.reason, src.at)
		}
	}
}

// TestTSAV0101SeedYAMLFixtures: `fixtures :table` reads each record of the committed YAML file as
// one seed with its attributes; ERB and a missing file abstain (TSA-V0-101, TSA-V0-109).
func TestTSAV0101SeedYAMLFixtures(t *testing.T) {
	repo := seedRepo(t)
	ps := seedRun(t, repo,
		SeedLink{New: "spec/new/users_spec.rb:2", Legacy: "spec/legacy/users_spec.rb:4"},
		SeedLink{New: "spec/new/users_spec.rb:2", Legacy: "spec/legacy/users_spec.rb:11"},
		SeedLink{New: "spec/new/users_spec.rb:2", Legacy: "spec/legacy/users_spec.rb:18"},
	)
	legacy := seedJSON(ps[0].Sides[0].Kinds)
	if legacy != `[{"kind":"user","count":2,"attributes":[{"name":"admin","values":["true"]},{"name":"name","values":["\"Alice\"","\"Bob\""]},{"name":"profile","values":["<expr>"]}]}]` {
		t.Errorf("fixture shape %s", legacy)
	}
	got := []string{}
	for _, f := range ps[0].Findings {
		got = append(got, f.Message)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{"legacy seeds 2 user, new seeds 1; asserting lines reading user: legacy spec/legacy/users_spec.rb:5; new spec/new/users_spec.rb:4",
		`user.name differs: legacy "Alice", "Bob", new "Alice";`, "user.profile is set explicitly on the legacy side (<expr>)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	if s := ps[1].Sides[0]; s.Analysed || s.Reason != "seed-fixture-unsupported" || s.Location != "spec/legacy/users_spec.rb:10" {
		t.Errorf("erb fixture side %+v", s)
	}
	if s := ps[2].Sides[0]; s.Analysed || s.Reason != "seed-fixture-unavailable" || s.Location != "spec/legacy/users_spec.rb:17" {
		t.Errorf("missing fixture side %+v", s)
	}
	for _, text := range []string{"a:\n\tname: x\n", "a:\n  <<: *base\n", "a:\n  name: &n x\n", "- a\n", "a: b\n", "  a: b\n"} {
		if _, ok := fixtureRecords(text); ok {
			t.Errorf("%q read", text)
		}
	}
	tables := make([]string, maxSeedFixtures+1)
	for k := range tables {
		tables[k] = ":t" + strings.Repeat("x", k)
	}
	read := func(string) ([]byte, bool, error) { return []byte("a:\n  b: 1\n"), true, nil }
	n := 0
	if _, reason, err := fixtureSeeds("fixtures "+strings.Join(tables, ", "), "x:1", read, &n); err != nil || reason != seedReasonFixtureBoundReached {
		t.Errorf("fixture bound: %s %v", reason, err)
	}
}

// TestTSAV0100SeedLinkArguments: a link reference that is not path:line refuses
// tests-invalid-arguments, an uncommitted file refuses tests-source-unavailable, more than
// MaxValidityFiles links refuses tests-bound-exceeded, and a link alone needs no --files
// (TSA-V0-100, TSA-V0-109).
func TestTSAV0100SeedLinkArguments(t *testing.T) {
	repo := seedRepo(t)
	many := make([]SeedLink, MaxValidityFiles+1)
	for k := range many {
		many[k] = SeedLink{New: "spec/new/orders_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"}
	}
	for _, c := range []struct {
		links []SeedLink
		code  string
	}{
		{[]SeedLink{{New: "spec/new/orders_spec.rb", Legacy: "spec/legacy/orders_spec.rb:10"}}, "tests-invalid-arguments"},
		{[]SeedLink{{New: "spec/new/orders_spec.rb:5", Legacy: "../x_spec.rb:1"}}, "tests-invalid-arguments"},
		{[]SeedLink{{New: "spec/new/orders_spec.rb:05", Legacy: "spec/legacy/orders_spec.rb:10"}}, "tests-invalid-arguments"},
		{[]SeedLink{{New: "spec/new/orders_spec.rb:0", Legacy: "spec/legacy/orders_spec.rb:10"}}, "tests-invalid-arguments"},
		{[]SeedLink{{New: "spec/new/missing_spec.rb:5", Legacy: "spec/legacy/orders_spec.rb:10"}}, "tests-source-unavailable"},
		{many, "tests-bound-exceeded"},
	} {
		if _, err := SourceValidity(repo, SourceValidityOptions{Links: c.links}); !hasCode(err, c.code) {
			t.Errorf("%v: %v, want %s", c.links[0], err, c.code)
		}
	}
}

// TestTSAV0108SeedParityAdvisoryOnly: seed parity adds no tests entry and changes no hygiene; a
// report without links has no seedParity member (TSA-V0-108), nor an assertionParity one
// (TSA-V0-677).
func TestTSAV0108SeedParityAdvisoryOnly(t *testing.T) {
	repo := seedRepo(t)
	without, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"test/new/cart.test.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	with, err := SourceValidity(repo, SourceValidityOptions{Files: []string{"test/new/cart.test.ts"},
		Links: []SeedLink{{New: "test/new/cart.test.ts:3", Legacy: "test/legacy/cart.test.ts:8"}}})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(without)
	with.SeedParity, with.AssertionParity = nil, nil
	b, _ := json.Marshal(with)
	if string(a) != string(b) || strings.Contains(string(a), "seedParity") || strings.Contains(string(a), "assertionParity") || with.Refused() {
		t.Errorf("seed parity changed the validity report:\n%s\n%s", a, b)
	}
}

// TestTSAV0102SeedNormalisation: a trait is attribute trait:NAME=true, nil and null are null, a
// symbol and a string normalise alike, a non-literal value is <expr>, a JS factory call's bound
// name and literal attributes are read, and camelCase kinds and keys become snake_case
// (TSA-V0-102).
func TestTSAV0102SeedNormalisation(t *testing.T) {
	none := func(string) ([]byte, bool, error) { return nil, false, nil }
	rb, err := extractSeed("spec/a_spec.rb", "RSpec.describe A do\n  it 'a' do\n    admin = build(:user, :admin, name: nil, role: :owner, plan: 'owner', age: helper)\n    expect(admin.role).to eq('owner')\n  end\nend\n", 2, none)
	if err != nil || rb.reason != "" {
		t.Fatalf("%+v %v", rb, err)
	}
	if got := seedJSON(rb.shape()); got != `[{"kind":"user","count":1,"attributes":[{"name":"age","values":["<expr>"]},{"name":"name","values":["null"]},{"name":"plan","values":["\"owner\""]},{"name":"role","values":["\"owner\""]},{"name":"trait:admin","values":["true"]}]}]` {
		t.Errorf("rspec shape %s", got)
	}
	if !rb.readsKind(rb.asserts[0], "user") {
		t.Errorf("the bound local admin must read user: %+v", rb.asserts)
	}
	js, _ := extractSeed("a.test.ts", "test('a', async () => {\n  const owner = await factory.build('AccountUser', { isAdmin: true, score: -1, note: null, at: now() });\n  expect(owner.isAdmin).toBe(true);\n});\n", 1, none)
	if js.reason != "" {
		t.Fatalf("%+v", js)
	}
	if got := seedJSON(js.shape()); got != `[{"kind":"account_user","count":1,"attributes":[{"name":"at","values":["<expr>"]},{"name":"is_admin","values":["true"]},{"name":"note","values":["null"]},{"name":"score","values":["-1"]}]}]` {
		t.Errorf("js shape %s", got)
	}
	if !js.readsKind(js.asserts[0], "account_user") || !js.readsAttr("is_admin") {
		t.Errorf("js asserts %+v", js.asserts)
	}
}

func seedRepoWith(t *testing.T, files map[string]string) Repo {
	t.Helper()
	root := t.TempDir()
	commitFiles(t, root, files)
	return OpenRepo(context.Background(), root)
}

const seedPlainSpec = `RSpec.describe User do
  it 'counts' do
    create(:user)
    expect(User.count).to eq(1)
  end
end
`

const seedConditionalSpec = `RSpec.describe User do
  let(:make) { -> { create(:user) } }

  it 'modifier' do
    create(:user) if enabled
    expect(User.count).to eq(1)
  end

  it 'lambda' do
    make
    expect(User.count).to eq(1)
  end

  it 'block' do
    if enabled
      create(:user)
    end
    expect(User.count).to eq(1)
  end

  it 'after a block' do
    if enabled
      puts 'x'
    end
    create(:user)
    expect(User.count).to eq(1)
  end
end
`

const seedConditionalJS = `test('if', async () => {
  if (enabled) create('user');
  expect(1).toBe(1);
});

test('deferred', async () => {
  const make = () => create('user');
  expect(1).toBe(1);
});

test('short-circuit', async () => {
  enabled && create('user');
  expect(1).toBe(1);
});

test('after a block', async () => {
  if (enabled) { console.log(); }
  const user = await create('user');
  expect(user).toBeTruthy();
});
`

// TestTSAV0107SeedConditionalAbstains: a factory call in a condition, a short-circuit operand or
// a block or callback that may never run abstains with seed-execution-unresolved, naming the
// line; a call after a closed block still reads (TSA-V0-101, TSA-V0-107; Codex review of #1178).
func TestTSAV0107SeedConditionalAbstains(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/plain_spec.rb": seedPlainSpec, "spec/cond_spec.rb": seedConditionalSpec, "test/cond.test.ts": seedConditionalJS,
	})
	ps := seedRun(t, repo,
		SeedLink{New: "spec/cond_spec.rb:4", Legacy: "spec/plain_spec.rb:2"},
		SeedLink{New: "spec/cond_spec.rb:9", Legacy: "spec/plain_spec.rb:2"},
		SeedLink{New: "spec/cond_spec.rb:14", Legacy: "spec/plain_spec.rb:2"},
		SeedLink{New: "test/cond.test.ts:1", Legacy: "spec/plain_spec.rb:2"},
		SeedLink{New: "test/cond.test.ts:6", Legacy: "spec/plain_spec.rb:2"},
		SeedLink{New: "test/cond.test.ts:11", Legacy: "spec/plain_spec.rb:2"},
	)
	for k, at := range []string{"spec/cond_spec.rb:5", "spec/cond_spec.rb:2", "spec/cond_spec.rb:16", "test/cond.test.ts:2", "test/cond.test.ts:7", "test/cond.test.ts:12"} {
		s := ps[k].Sides[1]
		if ps[k].Status != testvalidity.SeedParityStatusNotAnalysed || s.Analysed || s.Reason != "seed-execution-unresolved" || s.Location != at {
			t.Errorf("%d: status %s side %+v, want abstention at %s", k, ps[k].Status, s, at)
		}
	}
	for _, link := range []SeedLink{{New: "spec/cond_spec.rb:21", Legacy: "spec/plain_spec.rb:2"}, {New: "test/cond.test.ts:16", Legacy: "spec/plain_spec.rb:2"}} {
		if p := seedRun(t, repo, link)[0]; p.Status != testvalidity.SeedParityStatusParity {
			t.Errorf("%v: a call after a closed block must read: %s %+v", link, p.Status, p.Sides)
		}
	}
}

// TestTSAV0107SeedFixtureOversizeNoFallback: a committed fixture file over the bound abstains with
// seed-fixture-bound-exceeded instead of reading another directory's file of the same table
// (TSA-V0-107, TSA-V0-109; Codex review of #1178).
func TestTSAV0107SeedFixtureOversizeNoFallback(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/users_spec.rb":      "RSpec.describe User do\n  fixtures :users\n  it 'counts' do\n    expect(User.count).to eq(1)\n  end\nend\n",
		"spec/plain_spec.rb":      seedPlainSpec,
		"spec/fixtures/users.yml": "alice:\n  name: " + strings.Repeat("a", MaxFileBytes) + "\n",
		"test/fixtures/users.yml": "alice:\n  name: Alice\n",
	})
	p := seedRun(t, repo, SeedLink{New: "spec/users_spec.rb:3", Legacy: "spec/plain_spec.rb:2"})[0]
	if s := p.Sides[1]; p.Status != testvalidity.SeedParityStatusNotAnalysed || s.Analysed || s.Reason != "seed-fixture-bound-exceeded" || s.Location != "spec/users_spec.rb:2" {
		t.Fatalf("status %s side %+v", p.Status, s)
	}
}

// TestTSAV0103SeedExprVersusLiteralUncertain: an attribute whose value one side computes and the
// other sets to a different literal is uncertain, never parity; equal unknown values on both sides
// stay parity (TSA-V0-103, TSA-V0-105; Codex review of #1178).
func TestTSAV0103SeedExprVersusLiteralUncertain(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/literal_spec.rb": "RSpec.describe Order do\n  it 'opens' do\n    create(:order, status: 'open')\n    expect(Order.count).to eq(1)\n  end\nend\n",
		"spec/expr_spec.rb":    "RSpec.describe Order do\n  it 'opens' do\n    create(:order, status: current_status)\n    expect(Order.count).to eq(1)\n  end\nend\n",
	})
	p := seedRun(t, repo, SeedLink{New: "spec/expr_spec.rb:2", Legacy: "spec/literal_spec.rb:2"})[0]
	if p.Status != testvalidity.SeedParityStatusUncertain || seedRules(p) != "SEED-PARITY-UNCERTAIN" ||
		!strings.HasPrefix(p.Findings[0].Message, `order.status cannot be compared: legacy "open", new <expr>, and <expr> is a value the reading does not know;`) {
		t.Fatalf("status %s rules %s %+v", p.Status, seedRules(p), p.Findings)
	}
	if p := seedRun(t, repo, SeedLink{New: "spec/expr_spec.rb:2", Legacy: "spec/expr_spec.rb:2"})[0]; p.Status != testvalidity.SeedParityStatusParity {
		t.Errorf("equal unknown values: %s %+v", p.Status, p.Findings)
	}
}

// TestTSAV0101SeedNamedSubject: `subject(:user) { ... }` used through is_expected resolves to the
// named subject's body (TSA-V0-101; Codex review of #1178).
func TestTSAV0101SeedNamedSubject(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/subject_spec.rb": "RSpec.describe User do\n  subject(:user) { create(:user, admin: true) }\n\n  it { is_expected.to be_admin }\nend\n",
		"spec/plain_spec.rb":   "RSpec.describe User do\n  it 'is admin' do\n    user = create(:user, admin: true)\n    expect(user).to be_admin\n  end\nend\n",
	})
	p := seedRun(t, repo, SeedLink{New: "spec/subject_spec.rb:4", Legacy: "spec/plain_spec.rb:2"})[0]
	if got := seedJSON(p.Sides[1].Kinds); p.Status != testvalidity.SeedParityStatusParity ||
		got != `[{"kind":"user","count":1,"attributes":[{"name":"admin","values":["true"]}]}]` {
		t.Fatalf("status %s new kinds %s", p.Status, got)
	}
}

// TestTSAV0101SeedOverriddenEagerLet: an inner let overriding an outer let! runs only the inner
// body (TSA-V0-101; Codex review of #1178).
func TestTSAV0101SeedOverriddenEagerLet(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/override_spec.rb": `RSpec.describe User do
  let!(:user) { create(:user, admin: false) }

  context 'admin' do
    let(:user) { create(:user, admin: true) }

    it 'is admin' do
      expect(user).to be_admin
    end
  end
end
`,
	})
	p := seedRun(t, repo, SeedLink{New: "spec/override_spec.rb:7", Legacy: "spec/override_spec.rb:7"})[0]
	if got := seedJSON(p.Sides[1].Kinds); got != `[{"kind":"user","count":1,"attributes":[{"name":"admin","values":["true"]}]}]` {
		t.Fatalf("new kinds %s", got)
	}
}

// TestTSAV0101SeedYAMLQuotedComment: a comment after a quoted fixture scalar is dropped; a `#`
// inside the quotes is kept (TSA-V0-101; Codex review of #1178).
func TestTSAV0101SeedYAMLQuotedComment(t *testing.T) {
	for in, want := range map[string]string{
		`"Alice" # owner`: `"Alice"`, `'O''Neil' # c`: `'O''Neil'`, `"a # b"`: `"a # b"`,
		`"a\" # b" # c`: `"a\" # b"`, `"Alice"#x`: `"Alice"#x`, `plain # c`: `plain`,
	} {
		if got := yamlStripComment(in); got != want {
			t.Errorf("yamlStripComment(%q) = %q, want %q", in, got, want)
		}
	}
	repo := seedRepoWith(t, map[string]string{
		"spec/users_spec.rb":      "RSpec.describe User do\n  fixtures :users\n  it 'counts' do\n    expect(User.count).to eq(1)\n  end\nend\n",
		"spec/plain_spec.rb":      "RSpec.describe User do\n  it 'counts' do\n    create(:user, name: 'Alice')\n    expect(User.count).to eq(1)\n  end\nend\n",
		"spec/fixtures/users.yml": "alice:\n  name: \"Alice\" # the owner\n",
	})
	if p := seedRun(t, repo, SeedLink{New: "spec/users_spec.rb:3", Legacy: "spec/plain_spec.rb:2"})[0]; p.Status != testvalidity.SeedParityStatusParity {
		t.Fatalf("status %s %+v sides %s", p.Status, p.Findings, seedJSON(p.Sides))
	}
}

// TestTSAV0106SeedShouldAggregate: a `.should` chain keeps its receiver, so a group aggregate
// asserted through should is reported against a single-member seed (TSA-V0-106; Codex review of
// #1178).
func TestTSAV0106SeedShouldAggregate(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"test/legacy.test.js": "test('sums', async () => {\n  const items = await createList('item', 2, { price: 5 });\n  items.reduce((a, i) => a + i.price, 0).should.equal(10);\n});\n",
		"test/new.test.js":    "test('sums', async () => {\n  const items = await createList('item', 1, { price: 10 });\n  items.reduce((a, i) => a + i.price, 0).should.equal(10);\n});\n",
	})
	p := seedRun(t, repo, SeedLink{New: "test/new.test.js:1", Legacy: "test/legacy.test.js:1"})[0]
	got := []string{}
	for _, f := range p.Findings {
		if f.Rule == testvalidity.RuleSeedAggregateSingleMember {
			got = append(got, f.Location)
		}
	}
	if strings.Join(got, ",") != "test/legacy.test.js:3,test/new.test.js:3" {
		t.Fatalf("aggregate locations %v in %+v", got, p.Findings)
	}
}

const seedHelperLegacySpec = `RSpec.describe Order do
  def make_order
    create(:order, total: 5)
  end

  it 'totals' do
    create(:customer)
    make_order
    expect(Order.count).to eq(1)
  end
end
`

const seedHelperNewSpec = `RSpec.describe Order do
  def make_order(total)
    create(:order, total: 7)
  end

  let(:order) { make_order(7) }

  it 'totals' do
    create(:customer)
    order
    expect(Order.count).to eq(1)
  end

  it 'calls twice' do
    create(:customer)
    make_order(1)
    make_order(2)
    expect(Order.count).to eq(2)
  end

  it 'calls a support helper' do
    create(:customer)
    prepare_orders
    expect(Order.count).to eq(1)
  end
end
`

const seedHelperJS = `test('totals', async () => {
  await create('customer');
  await makeOrder();
  expect(1).toBe(1);
});
`

// TestTSAV0644SeedHelpers: a first-party helper the setup calls is followed into its factory
// calls when it runs once unconditionally, so two tests with identical visible factory calls but
// different helper-seeded records no longer report parity; a helper called twice, a helper the
// file does not define and a JS/TS helper call abstain with seed-helper-unresolved naming the
// helper and line (TSA-V0-644; V1-1908).
func TestTSAV0644SeedHelpers(t *testing.T) {
	repo := seedRepoWith(t, map[string]string{
		"spec/legacy_spec.rb": seedHelperLegacySpec, "spec/new_spec.rb": seedHelperNewSpec, "test/helper.test.ts": seedHelperJS,
	})
	followed := seedRun(t, repo, SeedLink{New: "spec/new_spec.rb:8", Legacy: "spec/legacy_spec.rb:6"})[0]
	if followed.Status != testvalidity.SeedParityStatusDiverges {
		t.Fatalf("helper-seeded records must be compared, got %s %+v", followed.Status, followed.Sides)
	}
	if len(followed.Findings) == 0 || !strings.Contains(followed.Findings[0].Message, "order.total differs: legacy 5, new 7") {
		t.Errorf("findings %+v, want the helper-seeded total divergence", followed.Findings)
	}
	for _, c := range []struct {
		link         SeedLink
		helper, line string
	}{
		{SeedLink{New: "spec/new_spec.rb:14", Legacy: "spec/legacy_spec.rb:6"}, "make_order", "spec/new_spec.rb:16"},
		{SeedLink{New: "spec/new_spec.rb:21", Legacy: "spec/legacy_spec.rb:6"}, "prepare_orders", "spec/new_spec.rb:23"},
		{SeedLink{New: "test/helper.test.ts:1", Legacy: "spec/legacy_spec.rb:6"}, "makeOrder", "test/helper.test.ts:3"},
	} {
		p := seedRun(t, repo, c.link)[0]
		s := p.Sides[1]
		if p.Status != testvalidity.SeedParityStatusNotAnalysed || s.Reason != "seed-helper-unresolved" || s.Helper != c.helper || s.Location != c.line {
			t.Errorf("%v: status %s side %+v, want seed-helper-unresolved %s at %s", c.link, p.Status, s, c.helper, c.line)
			continue
		}
		if len(p.Findings) != 1 || !strings.Contains(p.Findings[0].Message, "calling "+c.helper+" at "+c.line) {
			t.Errorf("%v: findings %+v, want the helper named", c.link, p.Findings)
		}
	}
}
