package ruby

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// writeIssue1130 writes the #1130 Rails fixture (the same files as
// internal/contextindex/testdata/ruby-autoload-1130), plus config/boot.rb when
// booted, so its require_relative resolves.
func writeIssue1130(t *testing.T, booted bool) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, ".rspec", "--require rails_helper\n")
	write(t, root, "Gemfile", "source \"https://rubygems.org\"\ngem \"rails\"\ngem \"rspec-rails\"\n")
	write(t, root, "app/api_clients/foo_client.rb", "class FooClient\n  def fetch(id)\n    { id: id }\n  end\nend\n")
	write(t, root, "app/services/bar/baz_service.rb", "module Bar\n  class BazService\n    def call(id)\n      FooClient.new.fetch(id)\n    end\n  end\nend\n")
	write(t, root, "config/application.rb", "require_relative \"boot\"\n\nmodule Fixture\n  class Application < Rails::Application\n  end\nend\n")
	if booted {
		write(t, root, "config/boot.rb", "ENV[\"BUNDLE_GEMFILE\"] ||= File.expand_path(\"../Gemfile\", __dir__)\n")
	}
	write(t, root, "spec/api_clients/foo_client_spec.rb", "require \"rails_helper\"\n\nRSpec.describe FooClient do\n  it_behaves_like \"a fetcher\"\nend\n")
	write(t, root, "spec/services/bar/baz_service_spec.rb", "require \"rails_helper\"\n\nRSpec.describe Bar::BazService do\n  it \"fetches\" do\n    expect(described_class.new.call(1)).to eq(id: 1)\n  end\nend\n")
	write(t, root, "spec/rails_helper.rb", "require \"spec_helper\"\nDir[Rails.root.join(\"spec/support/**/*.rb\")].sort.each { |f| require f }\n")
	write(t, root, "spec/spec_helper.rb", "RSpec.configure do |config|\nend\n")
	write(t, root, "spec/support/helper.rb", "module SupportHelper\nend\n")
	write(t, root, "spec/support/shared_examples/fetches.rb", "RSpec.shared_examples \"a fetcher\" do\n  it \"fetches\" do\n    expect(described_class.new.fetch(1)).to eq(id: 1)\n  end\nend\n")
	return root
}

func plan(t *testing.T, root string, dirty ...string) affected.Plan {
	t.Helper()
	graph, err := affected.Build(context.Background(), root, New())
	if err != nil {
		t.Fatal(err)
	}
	return affected.Select(graph, dirty)
}

func selectedWitnesses(selected affected.Plan) map[string]string {
	kinds := map[string]string{}
	for _, selection := range selected.Selected {
		for _, test := range selection.Tests {
			kinds[test] = selection.Witness.Kind
		}
	}
	return kinds
}

// TestConstantEdgeSelectsAutoloadedDependentSpec_RUBYAFFECTEDV0009 is the
// #1130 acceptance: a change to FooClient selects its own spec and, through
// the advisory constant edge from Bar::BazService, the service's spec, while a
// change to the service no longer selects FooClient's spec.
func TestConstantEdgeSelectsAutoloadedDependentSpec_RUBYAFFECTEDV0009(t *testing.T) {
	const fooSpec, bazSpec = "spec/api_clients/foo_client_spec.rb", "spec/services/bar/baz_service_spec.rb"
	root := writeIssue1130(t, true)
	got := plan(t, root, "app/api_clients/foo_client.rb")
	kinds := selectedWitnesses(got)
	if got.Scope != affected.ScopeBounded || len(kinds) != 2 {
		t.Fatalf("scope=%s selected=%v unknown=%v", got.Scope, got.SelectedTests(), got.Unknown)
	}
	if kinds[fooSpec] != affected.WitnessDependency || kinds[bazSpec] != affected.WitnessAdvisoryConstant {
		t.Fatalf("witnesses=%v", kinds)
	}
	narrowed := plan(t, root, "app/services/bar/baz_service.rb")
	if tests := narrowed.SelectedTests(); narrowed.Scope != affected.ScopeBounded || len(tests) != 1 || tests[0] != bazSpec {
		t.Fatalf("scope=%s selected=%v unknown=%v", narrowed.Scope, tests, narrowed.Unknown)
	}
	again := plan(t, root, "app/api_clients/foo_client.rb")
	if again.GraphDigest != got.GraphDigest || len(again.Selected) != len(got.Selected) {
		t.Fatalf("plans differ: %s %s", again.GraphDigest, got.GraphDigest)
	}

	// The fixture as committed lacks config/boot.rb: the unresolved
	// require_relative still widens, but rails-autoload no longer does.
	committed := plan(t, writeIssue1130(t, false), "app/api_clients/foo_client.rb")
	result, err := New().Units(context.Background(), writeIssue1130(t, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(selectedWitnesses(committed)) != 2 || len(result.Frontier) != 0 || !raised(result, FrontierRelativeLoad) || raised(result, FrontierRailsAutoload) {
		t.Fatalf("selected=%v frontier=%v", committed.SelectedTests(), allFrontier(result))
	}
	service := unitByID(t, result, "ruby:source:app/services/bar/baz_service")
	if !equal(service.AdvisoryImports, []string{"ruby:source:app/api_clients/foo_client"}) {
		t.Fatalf("service advisory edges=%v", service.AdvisoryImports)
	}
}

// TestUnboundedConstantLookupKeepsRailsAutoloadFrontier_RUBYAFFECTEDV0010
// keeps the coarse frontier exactly where no edge can bound a constant.
func TestUnboundedConstantLookupKeepsRailsAutoloadFrontier_RUBYAFFECTEDV0010(t *testing.T) {
	cases := map[string]map[string]string{
		"constantize":  {"app/services/router.rb": "class Router\n  def route(name)\n    name.constantize.new\n  end\nend\n"},
		"const_get":    {"app/services/router.rb": "class Router\n  def route(name)\n    Object.const_get(name)\n  end\nend\n"},
		"polymorphic":  {"app/models/comment.rb": "class Comment\n  belongs_to :commentable, polymorphic: true\nend\n"},
		"unmapped":     {"app/models/comment.rb": "class Comment\n  has_many :widgets\nend\n"},
		"declaration":  {".corvint/ruby-autoload.json": "{\"schema\":\"ruby-autoload/0\",\"roots\":[\"app/models\"],\"roots\":[\"lib\"]}"},
		"inflection":   {"config/initializers/inflections.rb": "ActiveSupport::Inflector.inflections do |inflect|\n  inflect.acronym(name)\nend\n"},
		"nested rails": {"engines/shop/spec/dummy/config/application.rb": "class Application < Rails::Application\nend\n"},
	}
	for name, files := range cases {
		root := writeIssue1130(t, true)
		for relative, body := range files {
			write(t, root, relative, body)
		}
		result, err := New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !raised(result, FrontierRailsAutoload) {
			t.Fatalf("%s: rails-autoload frontier missing: %v", name, allFrontier(result))
		}
	}
	root := writeIssue1130(t, true)
	write(t, root, "app/models/order.rb", "class Order\nend\n")
	write(t, root, "app/models/customer.rb", "class Customer\n  has_many :orders, dependent: :destroy\nend\n")
	write(t, root, "app/models/invoice.rb", "class Invoice\n  belongs_to :buyer, class_name: \"Customer\"\nend\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if raised(result, FrontierRailsAutoload) {
		t.Fatalf("resolved associations widened scope: %v", allFrontier(result))
	}
	if !equal(unitByID(t, result, "ruby:source:app/models/customer").AdvisoryImports, []string{"ruby:source:app/models/order"}) ||
		!equal(unitByID(t, result, "ruby:source:app/models/invoice").AdvisoryImports, []string{"ruby:source:app/models/customer"}) {
		t.Fatalf("association edges missing: %+v", result.Units)
	}
}

// TestRailsBootAndRouteEdgesSelectTheirSpecs_RUBYAFFECTEDV0011 selects every
// spec on a change below config/, and a request spec on a controller change.
func TestRailsBootAndRouteEdgesSelectTheirSpecs_RUBYAFFECTEDV0011(t *testing.T) {
	root := writeIssue1130(t, true)
	write(t, root, "config/initializers/time.rb", "Time.zone_default = \"UTC\"\n")
	write(t, root, "app/controllers/fetches_controller.rb", "class FetchesController\nend\n")
	write(t, root, "spec/requests/fetches_spec.rb", "require \"rails_helper\"\n\nRSpec.describe \"fetches\", type: :request do\n  it \"lists\" do\n    get \"/fetches\"\n  end\nend\n")
	booted := plan(t, root, "config/initializers/time.rb")
	if kinds := selectedWitnesses(booted); booted.Scope != affected.ScopeBounded || len(kinds) != 3 || kinds["spec/requests/fetches_spec.rb"] != affected.WitnessAdvisoryConstant {
		t.Fatalf("scope=%s witnesses=%v unknown=%v", booted.Scope, kinds, booted.Unknown)
	}
	routed := plan(t, root, "app/controllers/fetches_controller.rb")
	if tests := routed.SelectedTests(); routed.Scope != affected.ScopeBounded || !equal(tests, []string{"spec/requests/fetches_spec.rb"}) {
		t.Fatalf("scope=%s selected=%v unknown=%v", routed.Scope, tests, routed.Unknown)
	}
}

// writeRouteDispatch writes two controllers with one controller spec each,
// a request spec naming the first controller by constant, and a request spec
// that names no controller (RUBY-AFFECTED-V0-049).
func writeRouteDispatch(t *testing.T) string {
	t.Helper()
	root := writeIssue1130(t, true)
	write(t, root, "app/controllers/application_controller.rb", "class ApplicationController\nend\n")
	write(t, root, "app/controllers/alpha_controller.rb", "class AlphaController < ApplicationController\nend\n")
	write(t, root, "app/controllers/beta_controller.rb", "class BetaController < ApplicationController\nend\n")
	write(t, root, "spec/controllers/alpha_controller_spec.rb", "require \"rails_helper\"\n\nRSpec.describe AlphaController, type: :controller do\nend\n")
	write(t, root, "spec/controllers/beta_controller_spec.rb", "require \"rails_helper\"\n\nRSpec.describe BetaController, type: :controller do\nend\n")
	write(t, root, "spec/requests/alpha_flow_spec.rb", "require \"rails_helper\"\n\nRSpec.describe AlphaController, type: :request do\n  it \"lists\" do\n    get \"/alpha\"\n  end\nend\n")
	write(t, root, "spec/requests/pages_spec.rb", "require \"rails_helper\"\n\nRSpec.describe \"pages\", type: :request do\n  it \"lists\" do\n    get \"/pages\"\n  end\nend\n")
	return root
}

// TestRouteDispatchEdgesCarryTheirOwnWitness_RUBYAFFECTEDV0049 keeps every
// route-dispatched spec selected on a controller change, but names the boot
// fan-out ROUTE_DISPATCH_BOOT unless a constant or load also reaches the spec.
func TestRouteDispatchEdgesCarryTheirOwnWitness_RUBYAFFECTEDV0049(t *testing.T) {
	root := writeRouteDispatch(t)
	got := plan(t, root, "app/controllers/alpha_controller.rb")
	want := map[string]string{
		"spec/controllers/alpha_controller_spec.rb": affected.WitnessDependency,
		"spec/requests/alpha_flow_spec.rb":          affected.WitnessAdvisoryConstant,
		"spec/controllers/beta_controller_spec.rb":  "ROUTE_DISPATCH_BOOT",
		"spec/requests/pages_spec.rb":               "ROUTE_DISPATCH_BOOT",
	}
	if kinds := selectedWitnesses(got); got.Scope != affected.ScopeBounded || !maps.Equal(kinds, want) {
		t.Fatalf("scope=%s witnesses=%v unknown=%v", got.Scope, kinds, got.Unknown)
	}
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	flow := unitByID(t, result, "ruby:rspec:spec/requests/alpha_flow_spec")
	if !slices.Contains(flow.AdvisoryImports, "ruby:source:app/controllers/alpha_controller") ||
		slices.Contains(flow.RouteDispatchImports, "ruby:source:app/controllers/alpha_controller") ||
		!slices.Contains(flow.RouteDispatchImports, "ruby:source:app/controllers/beta_controller") {
		t.Fatalf("alpha flow edges advisory=%v routed=%v", flow.AdvisoryImports, flow.RouteDispatchImports)
	}
	if service := unitByID(t, result, "ruby:rspec:spec/services/bar/baz_service_spec"); len(service.RouteDispatchImports) != 0 {
		t.Fatalf("a service spec carries route edges %v", service.RouteDispatchImports)
	}
}

// TestSuiteHelpersAndSupportSelectEverySpec_RUBYAFFECTEDV0012 keeps
// rails_helper and spec_helper source and selects every spec of the suite on a
// change to either or to anything below spec/support/.
func TestSuiteHelpersAndSupportSelectEverySpec_RUBYAFFECTEDV0012(t *testing.T) {
	root := writeIssue1130(t, true)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, helper := range []string{"ruby:source:spec/rails_helper", "ruby:source:spec/spec_helper"} {
		if unit := unitByID(t, result, helper); len(unit.Tests) != 0 {
			t.Fatalf("%s is a test unit: %+v", helper, unit)
		}
	}
	for _, dirty := range []string{"spec/rails_helper.rb", "spec/spec_helper.rb", "spec/support/helper.rb", "spec/support/shared_examples/fetches.rb"} {
		got := plan(t, root, dirty)
		if tests := got.SelectedTests(); got.Scope != affected.ScopeBounded || len(tests) != 2 {
			t.Fatalf("%s: scope=%s selected=%v unknown=%v", dirty, got.Scope, tests, got.Unknown)
		}
	}

	gem := t.TempDir()
	write(t, gem, "lib/widget.rb", "class Widget\nend\n")
	write(t, gem, "spec/spec_helper.rb", "require \"widget\"\nDir[File.join(__dir__, \"support\", \"**\", \"*.rb\")].each(&method(:require))\n")
	write(t, gem, "spec/support/matchers.rb", "module Matchers\nend\n")
	write(t, gem, "spec/widget_spec.rb", "require \"spec_helper\"\nRSpec.describe Widget do\n  it { true }\nend\n")
	write(t, gem, "spec/other_spec.rb", "require \"widget\"\nRSpec.describe \"other\" do\n  it { true }\nend\n")
	got := plan(t, gem, "spec/support/matchers.rb")
	if tests := got.SelectedTests(); got.Scope != affected.ScopeBounded || len(tests) != 2 {
		t.Fatalf("gem support: scope=%s selected=%v unknown=%v", got.Scope, tests, got.Unknown)
	}

	// A glob outside the suite's own support directory still widens.
	write(t, gem, "spec/spec_helper.rb", "require \"widget\"\nDir[File.join(__dir__, \"..\", \"lib\", \"**\", \"*.rb\")].each { |f| require f }\n")
	result, err = New().Units(context.Background(), gem)
	if err != nil {
		t.Fatal(err)
	}
	if !raised(result, FrontierDynamicLoad) {
		t.Fatalf("foreign glob did not widen: %v", allFrontier(result))
	}
}

// stockApplication is config/application.rb as Rails 7.0 `rails new`
// generates it. The 7.1 `config.autoload_lib` line is read under
// RUBY-AFFECTED-V0-042 and pinned by conformance/ruby-profile-v0.
const stockApplication = "require_relative \"boot\"\n\nrequire \"rails/all\"\n\n" +
	"# Require the gems listed in Gemfile, including any gems\n# you've limited to :test, :development, or :production.\n" +
	"Bundler.require(*Rails.groups)\n\nmodule Fixture\n  class Application < Rails::Application\n" +
	"    config.load_defaults 7.0\n  end\nend\n"

// TestStockBundlerRequireKeepsRailsAppBounded_RUBYAFFECTEDV0013: the stock
// gem-group load in config/application.rb is not a dynamic application load,
// so an app/models change in a generated Rails app stays BOUNDED, while any
// other dynamic require, or the same call elsewhere, keeps ruby:dynamic-load.
func TestStockBundlerRequireKeepsRailsAppBounded_RUBYAFFECTEDV0013(t *testing.T) {
	root := writeIssue1130(t, true)
	write(t, root, "config/application.rb", stockApplication)
	write(t, root, "app/models/widget.rb", "class Widget\n  def client = FooClient.new\nend\n")
	write(t, root, "spec/models/widget_spec.rb", "require \"rails_helper\"\n\nRSpec.describe Widget do\n  it \"builds\" do\n    expect(described_class.new.client).to be_a(FooClient)\n  end\nend\n")
	got := plan(t, root, "app/models/widget.rb")
	if tests := got.SelectedTests(); got.Scope != affected.ScopeBounded || !equal(tests, []string{"spec/models/widget_spec.rb"}) {
		t.Fatalf("scope=%s selected=%v unknown=%v", got.Scope, tests, got.Unknown)
	}
	for name, change := range map[string][2]string{
		"other groups":       {"config/application.rb", strings.Replace(stockApplication, "*Rails.groups)", "*Rails.groups(:assets))", 1)},
		"computed groups":    {"config/application.rb", strings.Replace(stockApplication, "*Rails.groups)", "*groups)", 1)},
		"conditional":        {"config/application.rb", strings.Replace(stockApplication, "Bundler.require", "  Bundler.require", 1)},
		"trailing statement": {"config/application.rb", strings.Replace(stockApplication, "*Rails.groups)", "*Rails.groups); require name", 1)},
		"other owner":        {"lib/boot_gems.rb", "Bundler.require(*Rails.groups)\n"},
		"computed require":   {"app/models/loader.rb", "class Loader\n  def load(name) = require(name)\nend\n"},
	} {
		variant := writeIssue1130(t, true)
		write(t, variant, "config/application.rb", stockApplication)
		write(t, variant, change[0], change[1])
		result, err := New().Units(context.Background(), variant)
		if err != nil {
			t.Fatal(err)
		}
		if !raised(result, FrontierDynamicLoad) {
			t.Fatalf("%s: dynamic load not raised: %v", name, allFrontier(result))
		}
	}
}

// writeIssue1475 writes the #1475 repository: autoload-convention sources
// with no Rails application, a three-hop constant chain, a namespaced
// constant, an acronym inflection and a spec that reaches none of them.
func writeIssue1475(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "Gemfile", "source \"https://rubygems.org\"\n")
	write(t, root, "app/models/alpha_account.rb", "class AlphaAccount\n  def balance; Alpha::Ledger.new.total; end\nend\n")
	write(t, root, "app/models/alpha/ledger.rb", "module Alpha\n  class Ledger\n    def total; 1; end\n  end\nend\n")
	write(t, root, "app/models/alpha/journal.rb", "module Alpha\n  class Journal\n    def total; Ledger.new.total; end\n  end\nend\n")
	write(t, root, "app/services/alpha_report.rb", "class AlphaReport\n  def call; AlphaAccount.new.balance; end\nend\n")
	write(t, root, "app/services/alpha_digest.rb", "class AlphaDigest\n  def call; AlphaReport.new.call; end\nend\n")
	write(t, root, "app/clients/api_client.rb", "class APIClient\n  def get; 1; end\nend\n")
	write(t, root, "app/services/beta_note.rb", "class BetaNote\n  def call; 2; end\nend\n")
	write(t, root, "config/initializers/inflections.rb", "ActiveSupport::Inflector.inflections(:en) do |inflect|\n  inflect.acronym \"API\"\nend\n")
	write(t, root, "spec/models/alpha_account_spec.rb", "RSpec.describe AlphaAccount do\n  it \"x\" do expect(AlphaAccount.new.balance).to eq(1) end\nend\n")
	write(t, root, "spec/services/alpha_report_spec.rb", "RSpec.describe AlphaReport do\n  it \"y\" do expect(AlphaReport.new.call).to eq(1) end\nend\n")
	write(t, root, "spec/digests/alpha_digest_spec.rb", "RSpec.describe AlphaDigest do\n  it \"z\" do expect(AlphaDigest.new.call).to eq(1) end\nend\n")
	write(t, root, "spec/journal_spec.rb", "RSpec.describe Alpha::Journal do\n  it \"j\" do expect(Alpha::Journal.new.total).to eq(1) end\nend\n")
	write(t, root, "spec/remote/client_spec.rb", "RSpec.describe APIClient do\n  it \"c\" do expect(APIClient.new.get).to eq(1) end\nend\n")
	write(t, root, "spec/other/beta_spec.rb", "RSpec.describe BetaNote do\n  it \"b\" do expect(BetaNote.new.call).to eq(2) end\nend\n")
	return root
}

// TestConventionAutoloadEdgesWithoutRailsApplication_RUBYAFFECTEDV0045 is the
// #1475 acceptance: without config/application.rb the autoload-convention
// constants impact maps still become advisory edges, transitively, through a
// namespaced constant and an acronym inflection; an unrelated spec stays a
// proven exclusion; and the disk and fs.FS sources the worktree, --base,
// --path and --snapshot modes read give the same plan.
func TestConventionAutoloadEdgesWithoutRailsApplication_RUBYAFFECTEDV0045(t *testing.T) {
	root := writeIssue1475(t)
	got := plan(t, root, "app/models/alpha/ledger.rb")
	want := map[string]string{
		"spec/models/alpha_account_spec.rb":  affected.WitnessAdvisoryConstant,
		"spec/services/alpha_report_spec.rb": affected.WitnessAdvisoryConstant,
		"spec/digests/alpha_digest_spec.rb":  affected.WitnessAdvisoryConstant,
		"spec/journal_spec.rb":               affected.WitnessAdvisoryConstant,
	}
	if kinds := selectedWitnesses(got); got.Scope != affected.ScopeBounded || len(got.Unknown) != 0 || !equalWitnesses(kinds, want) {
		t.Fatalf("scope=%s selected=%v unknown=%v", got.Scope, kinds, got.Unknown)
	}
	excluded := map[string]string{}
	for _, exclusion := range got.Excluded {
		excluded[exclusion.UnitID] = exclusion.Reason
	}
	for _, id := range []string{"ruby:rspec:spec/other/beta_spec", "ruby:rspec:spec/remote/client_spec"} {
		if excluded[id] != affected.ExcludedNoDependencyPath {
			t.Fatalf("%s not a proven exclusion: %v", id, excluded)
		}
	}

	issue := plan(t, root, "app/models/alpha_account.rb")
	if kinds := selectedWitnesses(issue); issue.Scope != affected.ScopeBounded ||
		kinds["spec/services/alpha_report_spec.rb"] != affected.WitnessAdvisoryConstant ||
		kinds["spec/digests/alpha_digest_spec.rb"] != affected.WitnessAdvisoryConstant || len(kinds) != 3 {
		t.Fatalf("scope=%s selected=%v unknown=%v", issue.Scope, kinds, issue.Unknown)
	}
	client := plan(t, root, "app/clients/api_client.rb")
	if tests := client.SelectedTests(); len(tests) != 1 || tests[0] != "spec/remote/client_spec.rb" {
		t.Fatalf("acronym inflection: selected=%v unknown=%v", tests, client.Unknown)
	}

	graph, err := affected.BuildFS(newImmutableTree(t, root), New())
	if err != nil {
		t.Fatal(err)
	}
	fromFS := affected.Select(graph, []string{"app/models/alpha/ledger.rb"})
	if fromFS.GraphDigest != got.GraphDigest || !equalWitnesses(selectedWitnesses(fromFS), want) {
		t.Fatalf("fs.FS plan differs: %s %s %v", fromFS.GraphDigest, got.GraphDigest, selectedWitnesses(fromFS))
	}
}

// TestConventionAutoloadFailsClosed_RUBYAFFECTEDV0045 keeps the
// RUBY-AFFECTED-V0-010 bounds outside Rails: a dynamic constant lookup or an
// invalid declaration raises ruby:rails-autoload, and a library with no
// app/<dir> root or declaration gets no constant edges.
func TestConventionAutoloadFailsClosed_RUBYAFFECTEDV0045(t *testing.T) {
	dynamic := writeIssue1475(t)
	write(t, dynamic, "app/services/beta_note.rb", "class BetaNote\n  def call; params[:kind].constantize; end\nend\n")
	invalid := writeIssue1475(t)
	write(t, invalid, ".corvint/ruby-autoload.json", "{\"schema\":\"ruby-autoload/0\",\"roots\":[]}\n")
	for name, root := range map[string]string{"dynamic": dynamic, "invalid-declaration": invalid} {
		result, err := New().Units(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if !raised(result, FrontierRailsAutoload) {
			t.Fatalf("%s: rails-autoload frontier missing: %v", name, allFrontier(result))
		}
		// A dynamic lookup bears on its own unit's closure; an invalid
		// declaration on every plan (RUBY-AFFECTED-V0-046, RUBY-AFFECTED-V0-047).
		changed := "app/models/alpha_account.rb"
		if name == "dynamic" {
			changed = "app/services/beta_note.rb"
		}
		if got := plan(t, root, changed); got.Scope == affected.ScopeBounded {
			t.Fatalf("%s: plan stayed BOUNDED: %v", name, got.Unknown)
		}
	}
	site := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierRailsAutoload + ":app/services/beta_note.rb:2"}
	if got := plan(t, dynamic, "app/services/beta_note.rb"); len(got.Unknown) == 0 || got.Unknown[0] != site {
		t.Fatalf("dynamic site not located: %v", got.Unknown)
	}

	library := t.TempDir()
	write(t, library, "lib/widget.rb", "class Widget\n  def size; Gadget.new.size; end\nend\n")
	write(t, library, "lib/gadget.rb", "class Gadget\n  def size; 1; end\nend\n")
	write(t, library, "spec/widget_spec.rb", "require \"widget\"\nRSpec.describe Widget do\n  it \"w\" do expect(Widget.new.size).to eq(1) end\nend\n")
	result, err := New().Units(context.Background(), library)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range result.Units {
		if len(unit.AdvisoryImports) != 0 {
			t.Fatalf("library unit %s carries constant edges %v", unit.ID, unit.AdvisoryImports)
		}
	}
}

// TestRailsSignalsWithoutApplicationStayUnbounded_RUBYAFFECTEDV0045 keeps
// RUBY-AFFECTED-V0-010's absent-application bound: a Rails test with no root
// config/application.rb still raises ruby:rails-autoload when convention
// edges exist.
func TestRailsSignalsWithoutApplicationStayUnbounded_RUBYAFFECTEDV0045(t *testing.T) {
	root := writeIssue1475(t)
	write(t, root, "test/models/alpha_account_test.rb", "class AlphaAccountTest < ActiveSupport::TestCase\n  test \"x\" do\n    assert AlphaAccount.new\n  end\nend\n")
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !raised(result, FrontierRailsAutoload) {
		t.Fatalf("rails-autoload frontier missing: %v", allFrontier(result))
	}
}

// immutableTree is root as the bounded fs.FS the --base, --path and
// --snapshot modes read from a Git tree.
type immutableTree struct{ fstest.MapFS }

func (tree immutableTree) OpenBounded(name string, limit int) (fs.File, error) {
	file, ok := tree.MapFS[name]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if len(file.Data) > limit {
		return nil, affected.ErrWalkLimit
	}
	return tree.Open(name)
}

func newImmutableTree(t *testing.T, root string) immutableTree {
	t.Helper()
	tree := fstest.MapFS{}
	err := fs.WalkDir(os.DirFS(root), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(os.DirFS(root), name)
		tree[name] = &fstest.MapFile{Data: data, Mode: 0o644}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return immutableTree{tree}
}

func equalWitnesses(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for test, kind := range want {
		if got[test] != kind {
			return false
		}
	}
	return true
}
