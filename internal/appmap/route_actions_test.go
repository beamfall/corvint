package appmap

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const railsManifest = `{"schema":"application-map-manifest/0","app":"shop","routers":[{"path":"config/routes.rb","dialect":"rails-routes/0"}],` +
	`"tests":{"root":"e2e","specs":["e2e/specs"],"page_objects":["e2e/pages"]}}`

const railsRoutesFixture = `Rails.application.routes.draw do
  root "home#index"
  resources :orders, only: [:index, :show, :create, :update]
  resources :invoices, except: %i[destroy]
  resources :reports, only: [:show] do
    member do
      get :preview
    end
  end
  get "status", to: "status#show"
  get "dynamic", to: DynamicApp
  namespace :admin do
    resources :users, only: [:index, :destroy]
  end
  post "test_setup/orders", to: "test_setup#orders"
  resources :widgets, only: [:index]
  resources :magic, only: [:index]
  get "old", to: redirect("/new")
  get "pages/about"
end
`

// railsFixtureFiles is a small Rails application exercising every action verdict and the branch
// query's boundaries and setup writers.
func railsFixtureFiles() map[string]string {
	return map[string]string{
		"appmap.json":      railsManifest,
		"config/routes.rb": railsRoutesFixture,
		"app/controllers/application_controller.rb":  "class ApplicationController < ActionController::Base\n  include Authentication\nend\n",
		"app/controllers/concerns/authentication.rb": "module Authentication\n  extend ActiveSupport::Concern\n\n  included do\n    before_action :authenticate\n  end\n\n  def authenticate\n  end\nend\n",
		"app/controllers/home_controller.rb":         "class HomeController < ApplicationController\n  def index\n    render :index\n  end\nend\n",
		"app/controllers/orders_controller.rb": `class OrdersController < ApplicationController
  def index
  end

  def show
    @order = Order.find(params[:id])
    flash[:note] = "draft" if params[:preview] && current_user.admin?
  end

  def update
    @order = Order.find(params[:id])
    if @order.shipped?
      head :conflict
    end
    check_lock
  end

  private

  def check_lock
    head :locked if @order.locked
  end

  def create
  end
end
`,
		"app/controllers/base_invoices_controller.rb": "class BaseInvoicesController < ApplicationController\n  def show\n  end\n\n  def new\n  end\n\n  def edit\n  end\nend\n",
		"app/controllers/invoices_controller.rb":      "class InvoicesController < BaseInvoicesController\n  include Exportable\n\n  def index\n  end\nend\n",
		"app/controllers/concerns/exportable.rb":      "module Exportable\n  extend ActiveSupport::Concern\n\n  included do\n    def update\n    end\n  end\n\n  def create\n  end\n\n  class_methods do\n    def destroy\n    end\n  end\nend\n",
		"app/controllers/reports_controller.rb":       "class ReportsController < ApplicationController\n  def show\n  end\nend\n",
		"app/views/reports/preview.html.erb":          "<h1>Preview</h1>\n",
		"app/controllers/status_controller.rb":        "class StatusController < ApplicationController\n  define_method(:show) { head :ok }\nend\n",
		"app/controllers/admin/users_controller.rb":   "module Admin\n  class UsersController < Admin::BaseController\n    def index\n    end\n  end\nend\n",
		"app/controllers/test_setup_controller.rb":    "class TestSetupController < ApplicationController\n  def orders\n    seed_order\n    head :created\n  end\n\n  private\n\n  def seed_order\n    Order.create!(shipped: true, paid: params[:paid])\n  end\nend\n",
		"app/controllers/magic_controller.rb":         "class Wizard\nend\n",
		"app/controllers/pages_controller.rb":         "class PagesController < ApplicationController\nend\n",
		"app/views/pages/about.html.erb":              "<p>About</p>\n",
		"app/models/application_record.rb":            "class ApplicationRecord < ActiveRecord::Base\n  self.abstract_class = true\nend\n",
		"app/models/order.rb": `class Order < ApplicationRecord
  def refundable?
    return false unless paid
    true
  end

  def stale?
    return true if archived_at.nil?
    false
  end
end
`,
		"spec/factories/orders.rb":     "FactoryBot.define do\n  factory :order do\n    paid { false }\n\n    trait :shipped do\n      shipped { true }\n    end\n  end\nend\n",
		"spec/fixtures/orders.yml":     "one:\n  paid: true\n  locked: false\n",
		"spec/models/order_spec.rb":    "RSpec.describe Order do\n  it \"refunds\" do\n    expect(Order.new.refundable?).to be false\n  end\nend\n",
		"spec/requests/orders_spec.rb": "RSpec.describe \"orders\" do\n  it \"updates\" do\n    patch \"/orders/1\", params: {}\n  end\nend\n",
		"e2e/specs/orders.spec.ts":     "test('orders', async ({ page }) => {\n  await page.goto('/orders/1');\n});\n",
		"e2e/pages/orders.page.ts":     "export const orders = '/orders';\n",
	}
}

func railsFixture(t *testing.T, edit func(map[string]string)) (string, *Map) {
	t.Helper()
	files := railsFixtureFiles()
	if edit != nil {
		edit(files)
	}
	root := t.TempDir()
	for name, text := range files {
		writeFile(t, root, name, text)
	}
	gitTest(t, root, "init", "-q")
	rev := commitAll(t, root, "rails fixture")
	return root, build(t, root, rev)
}

type actionsDoc struct {
	Schema            string                `json:"schema"`
	Rule              string                `json:"rule"`
	RoutersChecked    []string              `json:"routers_checked"`
	RoutersNotChecked int                   `json:"routers_not_checked"`
	Routes            int                   `json:"routes"`
	Defined           int                   `json:"defined"`
	Missing           int                   `json:"missing"`
	Abstained         int                   `json:"abstained"`
	NotApplicable     int                   `json:"not_applicable_total"`
	Findings          []actionFinding       `json:"findings"`
	Abstentions       []actionAbstention    `json:"abstentions"`
	NA                []actionNotApplicable `json:"not_applicable"`
	Omitted           map[string]int        `json:"omitted"`
}

func checkActions(t *testing.T, root string, m *Map, o Options) (actionsDoc, map[string]any, *ActionsReport) {
	t.Helper()
	o.Root = root
	r, err := CheckActions(context.Background(), m, o)
	if err != nil {
		t.Fatal(err)
	}
	var doc actionsDoc
	if err := json.Unmarshal(r.Document, &doc); err != nil {
		t.Fatalf("%v\n%s", err, r.Document)
	}
	return doc, decode(t, r.Document), r
}

func abstentionKeys(d actionsDoc) []string {
	out := []string{}
	for _, a := range d.Abstentions {
		out = append(out, strings.Join([]string{a.Declaration, a.Controller, a.Action, a.Reason, a.At}, "|"))
	}
	sort.Strings(out)
	return out
}

func notApplicableKeys(raw map[string]any) []string {
	out := []string{}
	items, _ := raw["not_applicable"].([]any)
	for _, it := range items {
		m := it.(map[string]any)
		out = append(out, m["declaration"].(string)+"|"+m["reason"].(string))
	}
	sort.Strings(out)
	return out
}

// AMAP-V0-100 AMAP-V0-106: actions reads the map revision only, writes nothing, reports the one
// missing action with its declaration, handler and action and refuses bad revisions and budgets.
func TestAMAPV0100ActionsReadOnlyContract(t *testing.T) {
	root, m := railsFixture(t, nil)
	writeFile(t, root, "app/controllers/orders_controller.rb", "class OrdersController < ApplicationController\nend\n")
	doc, _, r := checkActions(t, root, m, Options{Full: true})
	if doc.Schema != ActionsSchema || doc.Rule != RuleRouteActionMissing || len(doc.RoutersChecked) != 1 || doc.RoutersNotChecked != 0 {
		t.Fatalf("%+v", doc)
	}
	want := actionFinding{Rule: RuleRouteActionMissing, Declaration: "config/routes.rb:3", Verb: "POST", Path: "/orders",
		Controller: "OrdersController", HandlerFile: "app/controllers/orders_controller.rb", Action: "create", Basis: "non-public-method"}
	if r.Findings != 1 || doc.Missing != 1 || !reflect.DeepEqual(doc.Findings, []actionFinding{want}) {
		t.Fatalf("findings %+v", doc.Findings)
	}
	if doc.Routes != doc.Defined+doc.Missing+countRouteAbstentions(doc) {
		t.Fatalf("every compiled route must be defined, missing or abstained: %+v", doc)
	}
	if status := gitTest(t, root, "status", "--porcelain", "--ignored"); status != "M app/controllers/orders_controller.rb" {
		t.Fatalf("mutated repo: %q", status)
	}
	if _, err := CheckActions(context.Background(), m, Options{Root: t.TempDir()}); codeOf(err) != "appmap-invalid-revision" {
		t.Fatalf("foreign root: %v", err)
	}
	if _, err := CheckActions(context.Background(), m, Options{Root: root, Budget: 10}); codeOf(err) == "" {
		t.Fatalf("budget below floor must refuse")
	}
	alt := *m
	alt.Manifest.Blob = strings.Repeat("0", 40)
	if _, err := CheckActions(context.Background(), &alt, Options{Root: root}); codeOf(err) != "appmap-invalid-revision" {
		t.Fatalf("manifest anchor mismatch: %v", err)
	}
}

func countRouteAbstentions(d actionsDoc) int {
	n := 0
	for _, a := range d.Abstentions {
		if a.Controller != "" {
			n++
		}
	}
	return n
}

// AMAP-V0-100: client routers are listed as not checked rather than resolved to server actions.
func TestAMAPV0100ClientRoutersNotChecked(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["appmap.json"] = strings.Replace(railsManifest, `"routers":[`, `"routers":[{"path":"web/routes.jsx","dialect":"react-router/0"},`, 1)
		f["web/routes.jsx"] = "export const routes = [{ path: '/home', element: <Home /> }];\n"
	})
	_, raw, _ := checkActions(t, root, m, Options{Full: true})
	if raw["routers_not_checked"] != float64(1) || !reflect.DeepEqual(notApplicableKeys(raw), []string{"config/routes.rb:11|rack-endpoint", "config/routes.rb:18|redirect", "web/routes.jsx|no-server-handler"}) {
		t.Fatalf("%v %v", raw["routers_not_checked"], notApplicableKeys(raw))
	}
}

// AMAP-V0-101 AMAP-V0-102 AMAP-V0-103 AMAP-V0-104 AMAP-V0-105: every route resolves to a handler;
// only:/except: removals are honoured; ancestors, concerns and templates define actions; doubt
// abstains with a named reason; redirects are not applicable.
func TestAMAPV0101ActionVerdicts(t *testing.T) {
	root, m := railsFixture(t, nil)
	doc, raw, _ := checkActions(t, root, m, Options{Full: true})
	if doc.Missing != 1 || doc.Findings[0].Action != "create" || doc.Findings[0].Controller != "OrdersController" {
		t.Fatalf("findings %+v", doc.Findings)
	}
	// root, orders x4, invoices x6 (except destroy), reports show+preview, status, admin users x2,
	// test_setup, widgets, magic, pages/about.
	if doc.Routes != 20 || doc.Defined != 15 {
		t.Fatalf("routes %d defined %d: %s", doc.Routes, doc.Defined, abstentionKeys(doc))
	}
	want := []string{
		"config/routes.rb:10|StatusController|show|dynamic-definition|app/controllers/status_controller.rb:2",
		"config/routes.rb:13|Admin::UsersController|destroy|unresolved-ancestor|app/controllers/admin/users_controller.rb:2 Admin::BaseController",
		"config/routes.rb:16|WidgetsController|index|handler-file-not-found|app/controllers/widgets_controller.rb",
		"config/routes.rb:17|MagicController|index|handler-class-not-found|app/controllers/magic_controller.rb",
	}
	if got := abstentionKeys(doc); !reflect.DeepEqual(got, want) {
		t.Fatalf("abstentions\n got %q\nwant %q", got, want)
	}
	if got := notApplicableKeys(raw); !reflect.DeepEqual(got, []string{"config/routes.rb:11|rack-endpoint", "config/routes.rb:18|redirect"}) {
		t.Fatalf("not applicable %q", got)
	}
}

// AMAP-V0-101: route compilation covers namespaces, singular resources, nested resources and the
// handler-less declaration.
func TestAMAPV0101RouteCompilation(t *testing.T) {
	p := &routeActionParser{}
	src := "scope module: :api do\n  resource :profile, only: [:show, :edit]\n  resources :posts, controller: :articles, only: [] do\n    resources :comments, only: :create\n    get :search, on: :collection\n  end\nend\nget \"health\"\nresource :boss, only: :show\nresources :hats, only: dynamic_list\n"
	p.parse("config/routes.rb", []byte(src), actionScope{}, 0)
	got := []string{}
	for _, r := range p.routes {
		got = append(got, r.verb+" "+r.path+" "+r.ctrl+"#"+r.action)
	}
	want := []string{"GET /profile/edit api/profiles#edit", "GET /profile api/profiles#show",
		"POST /posts/:post_id/comments api/comments#create", "GET /posts/search api/articles#search"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes\n got %q\nwant %q\n%+v", got, want, p.abstain)
	}
	reasons := []string{}
	for _, n := range p.abstain {
		reasons = append(reasons, n.file+":"+strconv.Itoa(n.line)+"|"+n.reason)
	}
	if !reflect.DeepEqual(reasons, []string{"config/routes.rb:8|no-handler", "config/routes.rb:9|unsupported-rails-inflection", "config/routes.rb:10|non-literal-rails-resource"}) {
		t.Fatalf("abstentions %q", reasons)
	}
}

// AMAP-V0-106: the document fits the budget with findings before abstentions and an omitted count.
func TestAMAPV0106ActionsBudget(t *testing.T) {
	root, m := railsFixture(t, nil)
	_, raw, r := checkActions(t, root, m, Options{Budget: 700})
	if len(r.Document) > 700 || raw["omitted"] == nil {
		t.Fatalf("budget not honoured (%d bytes): %s", len(r.Document), r.Document)
	}
	if strings.Index(string(r.Document), `"findings"`) > strings.Index(string(r.Document), `"abstentions"`) {
		t.Fatalf("findings must precede abstentions: %s", r.Document)
	}
}

// AMAP-V0-107: a Ruby handler over the per-file bound is not read and abstains.
func TestAMAPV0107OversizedHandler(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["app/controllers/home_controller.rb"] = "class HomeController < ApplicationController\n  def index\n  end\nend\n#" + strings.Repeat("x", maxTestFileBytes) + "\n"
	})
	doc, _, _ := checkActions(t, root, m, Options{Full: true})
	found := false
	for _, a := range doc.Abstentions {
		found = found || a.Controller == "HomeController" && a.Reason == "handler-file-unreadable"
	}
	if !found {
		t.Fatalf("%s", abstentionKeys(doc))
	}
}

// AMAP-V0-101 AMAP-V0-103 AMAP-V0-105: an unresolved module, a class cycle and an unterminated
// handler abstain with their reasons, and each draw failure abstains at its declaration.
func TestAMAPV0105ChainDoubtAndDraws(t *testing.T) {
	root, m := railsFixture(t, func(f map[string]string) {
		f["app/controllers/widgets_controller.rb"] = "class WidgetsController < ApplicationController\n  include Missing\nend\n"
		f["app/controllers/magic_controller.rb"] = "class MagicController < OtherMagic\nend\n"
		f["app/controllers/other_magic.rb"] = "class OtherMagic < MagicController\nend\n"
		f["app/controllers/status_controller.rb"] = "class StatusController < ApplicationController\n  def show\n    if ready\n  end\n"
		f["config/routes.rb"] = strings.Replace(railsRoutesFixture, "  get \"pages/about\"\n",
			"  get \"pages/about\"\n  draw :absent\n  draw :extra\n  draw :extra\n  draw some_name\n", 1)
		f["config/routes/extra.rb"] = "draw :deeper\nget \"extra\", to: \"home#index\"\n"
	})
	doc, _, _ := checkActions(t, root, m, Options{Full: true})
	got := abstentionKeys(doc)
	want := []string{
		"config/routes.rb:10|StatusController|show|unsupported-ruby-syntax|app/controllers/status_controller.rb:",
		"config/routes.rb:13|Admin::UsersController|destroy|unresolved-ancestor|app/controllers/admin/users_controller.rb:2 Admin::BaseController",
		"config/routes.rb:16|WidgetsController|index|unresolved-module|app/controllers/widgets_controller.rb:2 Missing",
		"config/routes.rb:17|MagicController|index|ancestor-depth-exceeded|app/controllers/magic_controller.rb:1",
		"config/routes.rb:20|||draw-unavailable|config/routes.rb:20",
		"config/routes.rb:22|||repeated-draw|config/routes.rb:22",
		"config/routes.rb:23|||non-literal-draw|config/routes.rb:23",
		"config/routes/extra.rb:1|||draw-unavailable|config/routes/extra.rb:1", // nested draws are followed (AMAP-V0-353)
	}
	if len(got) != len(want) {
		t.Fatalf("abstentions\n got %q\nwant %q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Fatalf("abstentions\n got %q\nwant %q", got, want)
		}
	}
	if doc.Routes != 21 {
		t.Fatalf("routes %d: the drawn file's route must compile", doc.Routes)
	}
}

// routeVerdict is one route's verdict: defined, missing:<basis> or abstained:<reason>.
func routeVerdict(d actionsDoc, controller, action string) string {
	for _, f := range d.Findings {
		if f.Controller == controller && f.Action == action {
			return "missing:" + f.Basis
		}
	}
	for _, a := range d.Abstentions {
		if a.Controller == controller && a.Action == action {
			return "abstained:" + a.Reason
		}
	}
	return "defined"
}

// AMAP-V0-101: a leading-slash controller leaves its namespace; a resource update maps PATCH and
// PUT.
func TestAMAPV0101AbsoluteControllersAndUpdateVerbs(t *testing.T) {
	p := &routeActionParser{}
	src := "namespace :admin do\n  get \"health\", to: \"/health#show\"\n  resources :things, controller: \"/things\", only: :index\n  resources :orders, only: :update\n  resource :profile, only: :update\nend\n"
	p.parse("config/routes.rb", []byte(src), actionScope{}, 0)
	got := []string{}
	for _, r := range p.routes {
		got = append(got, r.verb+" "+r.path+" "+r.ctrl+"#"+r.action)
	}
	want := []string{"GET /admin/health health#show", "GET /admin/things things#index",
		"PATCH|PUT /admin/orders/:id admin/orders#update", "PATCH|PUT /admin/profile admin/profiles#update"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes\n got %q\nwant %q\n%+v", got, want, p.abstain)
	}
}

// AMAP-V0-102 AMAP-V0-103 AMAP-V0-104 AMAP-V0-105: Ruby lookup order decides each verdict: a `::`
// reference is top-level, a later fallback or conditional definition abstains, `include A, B`
// searches A first, inherited visibility changes apply, reopened classes join and ancestor view
// prefixes hold implicit templates.
func TestAMAPV0103LookupOrderCases(t *testing.T) {
	cases := []struct {
		name, controller, action, want string
		files                          map[string]string
	}{
		{"absolute-superclass", "Admin::UsersController", "destroy", "defined", map[string]string{
			"app/controllers/admin/users_controller.rb": "module Admin\n  class UsersController < ::BaseController\n    def index\n    end\n  end\nend\n",
			"app/controllers/admin/base_controller.rb":  "module Admin\n  class BaseController < ApplicationController\n  end\nend\n",
			"app/controllers/base_controller.rb":        "class BaseController < ApplicationController\n  def destroy\n  end\nend\n",
		}},
		{"absolute-include", "Admin::UsersController", "destroy", "defined", map[string]string{
			"app/controllers/admin/users_controller.rb":   "module Admin\n  class UsersController < ApplicationController\n    include ::Destroyer\n\n    def index\n    end\n  end\nend\n",
			"app/controllers/concerns/admin/destroyer.rb": "module Admin\n  module Destroyer\n  end\nend\n",
			"app/controllers/concerns/destroyer.rb":       "module Destroyer\n  def destroy\n  end\nend\n",
		}},
		{"private-with-action-missing", "ReportsController", "show", "abstained:dynamic-definition", map[string]string{
			"app/controllers/reports_controller.rb": "class ReportsController < ApplicationController\n  def action_missing(name)\n  end\n\n  private\n\n  def show\n  end\nend\n",
		}},
		{"conditional-definition", "ReportsController", "show", "abstained:dynamic-definition", map[string]string{
			"app/controllers/reports_controller.rb": "class ReportsController < ApplicationController\n  if Rails.env.test?\n    def show\n    end\n  end\nend\n",
		}},
		{"include-argument-order", "ReportsController", "show", "defined", map[string]string{
			"app/controllers/reports_controller.rb":     "class ReportsController < ApplicationController\n  include FirstAction, SecondAction\nend\n",
			"app/controllers/concerns/first_action.rb":  "module FirstAction\n  def show\n  end\nend\n",
			"app/controllers/concerns/second_action.rb": "module SecondAction\n  private\n\n  def show\n  end\nend\n",
		}},
		{"inherited-made-private", "ReportsController", "show", "missing:non-public-method", map[string]string{
			"app/controllers/reports_controller.rb":      "class ReportsController < BaseReportsController\n  private :show\nend\n",
			"app/controllers/base_reports_controller.rb": "class BaseReportsController < ApplicationController\n  def show\n  end\nend\n",
		}},
		{"inherited-made-public", "ReportsController", "show", "defined", map[string]string{
			"app/controllers/reports_controller.rb":      "class ReportsController < BaseReportsController\n  public :show\nend\n",
			"app/controllers/base_reports_controller.rb": "class BaseReportsController < ApplicationController\n  private\n\n  def show\n  end\nend\n",
		}},
		{"reopened-class", "ReportsController", "show", "defined", map[string]string{
			"app/controllers/reports_controller.rb": "class ReportsController < ApplicationController\nend\n\nclass ReportsController\n  def show\n  end\nend\n",
		}},
		{"ancestor-view-prefix", "ReportsController", "preview", "defined", map[string]string{
			"app/controllers/reports_controller.rb":      "class ReportsController < BaseReportsController\n  def show\n  end\nend\n",
			"app/controllers/base_reports_controller.rb": "class BaseReportsController < ApplicationController\nend\n",
			"app/views/reports/preview.html.erb":         "",
			"app/views/base_reports/preview.html.erb":    "<h1>Preview</h1>\n",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, m := railsFixture(t, func(f map[string]string) {
				for name, text := range tc.files {
					if text == "" {
						delete(f, name)
						continue
					}
					f[name] = text
				}
			})
			doc, _, _ := checkActions(t, root, m, Options{Full: true})
			if got := routeVerdict(doc, tc.controller, tc.action); got != tc.want {
				t.Fatalf("%s#%s = %s, want %s\n%q", tc.controller, tc.action, got, tc.want, abstentionKeys(doc))
			}
		})
	}
}
