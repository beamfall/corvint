package ruby

import (
	"context"
	"testing"

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
	if len(selectedWitnesses(committed)) != 2 || !equal(result.Frontier, []string{FrontierRelativeLoad}) {
		t.Fatalf("selected=%v frontier=%v", committed.SelectedTests(), result.Frontier)
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
		if !contains(result.Frontier, FrontierRailsAutoload) {
			t.Fatalf("%s: rails-autoload frontier missing: %v", name, result.Frontier)
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
	if contains(result.Frontier, FrontierRailsAutoload) {
		t.Fatalf("resolved associations widened scope: %v", result.Frontier)
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
	if !contains(result.Frontier, FrontierDynamicLoad) {
		t.Fatalf("foreign glob did not widen: %v", result.Frontier)
	}
}
