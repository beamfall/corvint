package appmap

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

type branchesDoc struct {
	Schema          string         `json:"schema"`
	Limits          string         `json:"limits"`
	Branches        []branchRow    `json:"branches"`
	NoBranch        []noBranchRow  `json:"no_branch"`
	BranchCount     int            `json:"-"`
	TestFilesRead   int            `json:"test_files_read"`
	TestFilesUnread int            `json:"test_files_unread"`
	SetupEndpoints  int            `json:"setup_endpoints"`
	Factories       int            `json:"factories_and_traits"`
	Fixtures        int            `json:"fixtures"`
	Omitted         map[string]int `json:"omitted"`
}

var branchCites = []string{"app/controllers/orders_controller.rb:5-22", "app/models/order.rb:1-10", "app/controllers/home_controller.rb:1-5"}

func queryBranches(t *testing.T, root string, m *Map, o Options, cites ...string) (branchesDoc, *BranchesReport) {
	t.Helper()
	o.Root = root
	r, err := QueryBranches(context.Background(), m, cites, o)
	if err != nil {
		t.Fatal(err)
	}
	var doc branchesDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil {
		t.Fatalf("%v\n%s", err, r.Document)
	}
	return doc, r
}

// branchKey summarises one branch: location, kind, method, the three boundary statuses with
// their first test and basis, and the setup status with each input's writers.
func branchKey(b branchRow) string {
	parts := []string{b.At, b.Kind, b.Constant + "#" + b.Method}
	for _, bd := range b.Boundaries {
		s := bd.Boundary + "=" + bd.Status
		if len(bd.Tests) > 0 {
			s += "(" + bd.Tests[0].Test + " " + bd.Tests[0].Basis + ")"
		}
		if bd.Status == "none" && bd.Note != "none at "+bd.Boundary {
			s += "!note"
		}
		parts = append(parts, s)
	}
	if b.Setup == nil {
		return strings.Join(parts, " | ")
	}
	setup := "setup=" + b.Setup.Status
	for _, in := range b.Setup.Inputs {
		names := []string{}
		for _, w := range in.Writers {
			names = append(names, w.Kind+":"+w.Name)
		}
		setup += " " + in.Expr + "<-[" + strings.Join(names, ", ") + "]"
	}
	for _, u := range b.Setup.Unresolved {
		setup += " ?" + u.Expr + ":" + u.Reason
	}
	return strings.Join(append(parts, setup), " | ")
}

// AMAP-V0-110 AMAP-V0-116: branches reads the map revision only, writes nothing, carries the
// fixed limits sentence and refuses bad cites, revisions and budgets with owned codes.
func TestAMAPV0110BranchesReadOnlyContract(t *testing.T) {
	root, m := railsFixture(t, nil)
	writeFile(t, root, "app/models/order.rb", "class Order\nend\n")
	doc, _ := queryBranches(t, root, m, Options{Full: true}, branchCites...)
	if doc.Schema != BranchesSchema || doc.Limits != BranchesLimits || !strings.Contains(doc.Limits, "static naming evidence") || len(doc.Branches) != 5 {
		t.Fatalf("%+v", doc)
	}
	if status := gitTest(t, root, "status", "--porcelain", "--ignored"); status != "M app/models/order.rb" {
		t.Fatalf("mutated repo: %q", status)
	}
	for _, cites := range [][]string{nil, {"../x.rb:1"}, {"/app/models/order.rb:1"}, {"app/models/order.rb:1", "app/models/order.rb:1"},
		{"app/models/order.rb"}, {"app/models/order.rb:0"}, {"app/models/order.rb:3-2"}, {"app/models/order.rb:1-401"},
		{"spec/fixtures/orders.yml:1"}, {"app/models/absent.rb:1"}, {"app/models/order.rb:40"},
		strings.Split(strings.Repeat("app/models/order.rb:1,", maxCites+1), ",")[:maxCites+1]} {
		if _, err := QueryBranches(context.Background(), m, cites, Options{Root: root}); codeOf(err) != "appmap-invalid-query" {
			t.Errorf("cites %q: %v", cites, err)
		}
	}
	if _, err := QueryBranches(context.Background(), m, branchCites, Options{Root: root, Budget: 10}); codeOf(err) == "" {
		t.Fatalf("budget below floor must refuse")
	}
	if _, err := QueryBranches(context.Background(), m, branchCites, Options{Root: t.TempDir()}); codeOf(err) != "appmap-invalid-revision" {
		t.Fatalf("foreign root: %v", err)
	}
}

// AMAP-V0-111 AMAP-V0-112 AMAP-V0-113 AMAP-V0-114 AMAP-V0-115: each cited branch lists its
// reaching tests by boundary with explicit "none at" notes, and an unreached boundary carries the
// guard's inputs joined to setup endpoints, factories, traits and fixtures.
func TestAMAPV0111BranchReachAndSetup(t *testing.T) {
	root, m := railsFixture(t, nil)
	doc, _ := queryBranches(t, root, m, Options{Full: true}, branchCites...)
	got := []string{}
	for _, b := range doc.Branches {
		got = append(got, branchKey(b))
	}
	want := []string{
		"app/controllers/orders_controller.rb:7 | modifier-if | OrdersController#show | unit=none | request=none | " +
			"e2e=reached(e2e/specs/orders.spec.ts:2 routed-action-visit) | setup=unresolved-guard ?params:request-input ?current_user.admin?:unresolved-receiver",
		"app/controllers/orders_controller.rb:12 | if | OrdersController#update | unit=none | " +
			"request=reached(spec/requests/orders_spec.rb:3 routed-action-literal-path) | e2e=possible(e2e/specs/orders.spec.ts:2 same-controller-screen) | " +
			"setup=candidates Order#shipped<-[endpoint:POST /test_setup/orders, trait:trait :shipped (factory :order)]",
		"app/controllers/orders_controller.rb:21 | modifier-if | OrdersController#check_lock | unit=none | " +
			"request=reached(spec/requests/orders_spec.rb:3 called-by-routed-action-literal-path) | e2e=possible(e2e/specs/orders.spec.ts:2 same-controller-screen) | " +
			"setup=candidates Order#locked<-[fixture:fixtures orders]",
		"app/models/order.rb:3 | modifier-unless | Order#refundable? | unit=reached(spec/models/order_spec.rb:3 names-constant-and-method) | request=none | e2e=none | " +
			"setup=candidates Order#paid<-[endpoint:POST /test_setup/orders, factory:factory :order, fixture:fixtures orders]",
		"app/models/order.rb:8 | modifier-if | Order#stale? | unit=none | request=none | e2e=none | setup=none-found Order#archived_at<-[]",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("branches\n got %q\nwant %q", got, want)
	}
	if !reflect.DeepEqual(doc.NoBranch, []noBranchRow{{Cite: "app/controllers/home_controller.rb:1-5", Reason: "no-branch"}}) {
		t.Fatalf("no_branch %+v", doc.NoBranch)
	}
	if b := doc.Branches[1]; b.Guard != "@order.shipped?" || b.Boundaries[0].Note != "none at unit" {
		t.Fatalf("%+v", b)
	}
	if doc.SetupEndpoints != 1 || doc.Factories != 2 || doc.Fixtures != 1 {
		t.Fatalf("index counts %+v", doc)
	}
}

// AMAP-V0-113 AMAP-V0-115: an unresolved guard is never "none-found", even with candidates, and
// unsupported expressions and request inputs are named.
func TestAMAPV0113UnresolvedGuards(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["app/models/order.rb"] = "class Order < ApplicationRecord\n  def odd?\n    return 1 if paid && customer.account.active\n    return 2 if lookup(paid)\n    return 3 if ENV.fetch(\"X\") == paid\n    case paid\n    when true then 4\n    end\n  end\nend\n"
	})
	doc, _ := queryBranches(t, root, m, Options{Full: true}, "app/models/order.rb:3-7")
	got := []string{}
	for _, b := range doc.Branches {
		if b.Setup == nil || b.Setup.Status == "none-found" {
			t.Fatalf("unresolved guard read as no setup: %+v", b)
		}
		got = append(got, branchKey(b)[strings.Index(branchKey(b), "setup="):])
	}
	want := []string{
		"setup=unresolved-guard Order#paid<-[endpoint:POST /test_setup/orders, factory:factory :order, fixture:fixtures orders] ?customer.account.active:unsupported-expression",
		"setup=unresolved-guard Order#paid<-[endpoint:POST /test_setup/orders, factory:factory :order, fixture:fixtures orders] ?lookup:unsupported-expression",
		"setup=unresolved-guard Order#paid<-[endpoint:POST /test_setup/orders, factory:factory :order, fixture:fixtures orders] ?ENV.fetch:request-input",
		"setup=unresolved-guard ?case subject:unsupported-expression",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setup\n got %q\nwant %q", got, want)
	}
}

// AMAP-V0-112: callbacks are reached by every routed action of their controller; a request helper
// reaches its named route.
func TestAMAPV0112CallbackAndHelperReach(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["app/controllers/reports_controller.rb"] = "class ReportsController < ApplicationController\n  before_action :guard\n\n  def show\n  end\n\n  private\n\n  def guard\n    head :forbidden unless @report.visible\n  end\nend\n"
		f["app/models/report.rb"] = "class Report < ApplicationRecord\nend\n"
		f["spec/requests/reports_spec.rb"] = "describe \"reports\" do\n  it \"shows\" do\n    get report_path(1)\n  end\nend\n"
	})
	doc, _ := queryBranches(t, root, m, Options{Full: true}, "app/controllers/reports_controller.rb:10")
	if len(doc.Branches) != 1 {
		t.Fatalf("%+v", doc)
	}
	if got := branchKey(doc.Branches[0]); !strings.Contains(got, "request=reached(spec/requests/reports_spec.rb:3 callback-path-helper)") ||
		!strings.Contains(got, "setup=none-found Report#visible<-[]") {
		t.Fatalf("%s", got)
	}
}

// AMAP-V0-116: the document fits the budget with an omitted count; a secret-shaped guard prints
// guard_redacted and no guard text.
func TestAMAPV0116BudgetAndRedaction(t *testing.T) {
	secret := "password=" + strings.Repeat("Zx9", 8)
	if !secretscreen.MatchString(secret) {
		t.Fatal("fixture guard is not secret-shaped")
	}
	root, m := railsFixture(t, func(f map[string]string) {
		f["app/models/order.rb"] = "class Order < ApplicationRecord\n  def ok?\n    return true if token == \"" + secret + "\"\n  end\nend\n"
	})
	doc, r := queryBranches(t, root, m, Options{Full: true}, "app/models/order.rb:3")
	if len(doc.Branches) != 1 || !doc.Branches[0].GuardRedacted || doc.Branches[0].Guard != "" || strings.Contains(string(r.Document), secret) {
		t.Fatalf("%s", r.Document)
	}
	root, m = railsFixture(t, nil)
	_, r = queryBranches(t, root, m, Options{Budget: 1500}, branchCites...)
	if len(r.Document) > 1500 || strings.Contains(string(r.Document), `"omitted":{"branches":0`) || !strings.Contains(string(r.Document), `"omitted"`) {
		t.Fatalf("budget not honoured (%d bytes): %s", len(r.Document), r.Document)
	}
}

// AMAP-V0-112: a callback reaches only the actions its registration applies to after only:,
// except: and skips; a conditional registration leaves its reach possible.
func TestAMAPV0112CallbackRestrictions(t *testing.T) {
	cases := []struct{ register, skip, want string }{
		{"before_action :guard, only: :preview", "", "request=none"},
		{"before_action :guard", "skip_before_action :guard, only: :show", "request=none"},
		{"before_action :guard, except: :preview", "", "request=reached(spec/requests/reports_spec.rb:3 callback-path-helper)"},
		{"before_action :guard, only: %i[show]", "", "request=reached(spec/requests/reports_spec.rb:3 callback-path-helper)"},
		{"before_action :guard, only: [:show], if: :ready?", "", "request=possible(spec/requests/reports_spec.rb:3 conditional-callback-path-helper)"},
		{"before_action :guard", "skip_before_action :guard, unless: :ready?", "request=possible(spec/requests/reports_spec.rb:3 conditional-callback-path-helper)"},
	}
	for _, tc := range cases {
		t.Run(tc.register+" "+tc.skip, func(t *testing.T) {
			root, m := railsFixture(t, func(f map[string]string) {
				f["app/controllers/reports_controller.rb"] = "class ReportsController < ApplicationController\n  " + tc.register + "\n  " + tc.skip +
					"\n  def show\n  end\n\n  private\n\n  def guard\n    head :forbidden unless @report.visible\n  end\nend\n"
				f["spec/requests/reports_spec.rb"] = "describe \"reports\" do\n  it \"shows\" do\n    get report_path(1)\n  end\nend\n"
			})
			doc, _ := queryBranches(t, root, m, Options{Full: true}, "app/controllers/reports_controller.rb:10")
			if len(doc.Branches) != 1 || !strings.Contains(branchKey(doc.Branches[0]), " | "+tc.want+" | ") {
				t.Fatalf("want %s\n%+v", tc.want, doc.Branches)
			}
		})
	}
}

// AMAP-V0-112: a request or visit inside a comment or string literal reaches nothing.
func TestAMAPV0112CommentedCallsIgnored(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["spec/requests/orders_spec.rb"] = "RSpec.describe \"orders\" do\n  it \"updates\" do\n    # patch \"/orders/1\"\n    note = \"patch '/orders/1'\"\n  end\nend\n"
		f["e2e/specs/orders.spec.ts"] = "test('orders', async ({ page }) => {\n  // await page.goto('/orders/1');\n  const s = \"page.goto('/orders/1')\";\n});\n"
	})
	doc, _ := queryBranches(t, root, m, Options{Full: true}, "app/controllers/orders_controller.rb:7-12")
	if len(doc.Branches) != 2 {
		t.Fatalf("%+v", doc.Branches)
	}
	for _, b := range doc.Branches {
		if key := branchKey(b); !strings.Contains(key, "| request=none | e2e=none |") {
			t.Fatalf("commented call reached: %s", key)
		}
	}
}

// AMAP-V0-114: nested factories inherit their ancestors' attributes in every run, and a quoted
// class option names the model while an unread one binds to none.
func TestAMAPV0114FactoryInheritanceAndClass(t *testing.T) {
	src := "FactoryBot.define do\n  factory :order do\n    paid { true }\n    factory :a do\n      factory :b do\n        factory :c do\n          factory :d do\n            factory :e do\n              rush { true }\n            end\n          end\n        end\n      end\n    end\n  end\n" +
		"  factory :admin, class: \"User\" do\n    role { \"admin\" }\n  end\n  factory :ghost, class: model_for(:ghost) do\n    role { \"none\" }\n  end\nend\n"
	for run := 0; run < 50; run++ {
		x := &setupWriterIndex{factories: map[string]*setupWriter{}, traits: map[string]*setupWriter{}}
		x.readFactories("spec/factories/orders.rb", []byte(src))
		names := []string{}
		for _, w := range x.writersOf("Order", "paid") {
			names = append(names, w.Name)
		}
		if want := []string{"factory :order", "factory :a", "factory :b", "factory :c", "factory :d", "factory :e"}; !reflect.DeepEqual(names, want) {
			t.Fatalf("run %d: paid writers %q, want %q", run, names, want)
		}
		if w := x.writersOf("User", "role"); len(w) != 1 || w[0].Name != "factory :admin" {
			t.Fatalf("quoted class: %+v", w)
		}
		if x.factories["ghost"].Model != "?" {
			t.Fatalf("unread class bound to %q", x.factories["ghost"].Model)
		}
	}
}

// AMAP-V0-107 AMAP-V0-116: one read budget covers every application root a command loads, and
// branches loads only the roots its cites need.
func TestAMAPV0116SharedReadBudget(t *testing.T) {
	saved := newAppReadBudget
	defer func() { newAppReadBudget = saved }()
	newAppReadBudget = func() *appReadBudget { return &appReadBudget{files: maxTestFiles, bytes: 60_000} }
	engine := func(pad int) func(map[string]string) {
		return func(f map[string]string) {
			f["appmap.json"] = strings.Replace(railsManifest, `"routers":[`, `"routers":[{"path":"engine/config/routes.rb","dialect":"rails-routes/0"},`, 1)
			f["engine/config/routes.rb"] = "Rails.application.routes.draw do\n  get \"ping\", to: \"ping#show\"\nend\n"
			f["engine/app/controllers/ping_controller.rb"] = "class PingController < ActionController::Base\n  def show\n  end\nend\n#" + strings.Repeat("x", pad) + "\n"
			f["app/controllers/padding_controller.rb"] = "class PaddingController < ApplicationController\nend\n#" + strings.Repeat("x", 40_000) + "\n"
		}
	}
	root, m := railsFixture(t, engine(40_000))
	if _, err := CheckActions(context.Background(), m, Options{Root: root}); codeOf(err) != "appmap-bound-exceeded" {
		t.Fatalf("two roots within the budget each but not together: %v", err)
	}
	root, m = railsFixture(t, engine(70_000))
	doc, _ := queryBranches(t, root, m, Options{Full: true}, "app/models/order.rb:3")
	if len(doc.Branches) != 1 {
		t.Fatalf("%+v", doc)
	}
	if _, err := QueryBranches(context.Background(), m, []string{"engine/app/controllers/ping_controller.rb:2"}, Options{Root: root}); codeOf(err) != "appmap-bound-exceeded" {
		t.Fatalf("the cited engine root exceeds the budget: %v", err)
	}
}
