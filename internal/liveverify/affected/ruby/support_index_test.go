package ruby

import (
	"maps"
	"strings"
	"testing"
)

// referenceSupportEdges is the per-test scan supportEdges replaced: every
// unit is checked against every test.
func referenceSupportEdges(item observation, units map[string]string) map[string]bool {
	edges := map[string]bool{}
	if item.kind == frameworkSource {
		return edges
	}
	prefix, _, ok := testRelative(item.relative)
	if !ok {
		return edges
	}
	suite := strings.TrimPrefix(item.relative, prefix)
	suite = prefix + suite[:strings.Index(suite, "/")+1]
	helpers := map[string]bool{suite + "spec_helper.rb": true, suite + "rails_helper.rb": true, suite + "test_helper.rb": true}
	for relative, id := range units {
		if relative != item.relative && strings.HasPrefix(id, "ruby:source:") &&
			(helpers[relative] || strings.HasPrefix(relative, suite+"support/")) {
			edges[id] = true
		}
	}
	return edges
}

// referenceBootEdges is the per-test scan bootEdges replaced.
func referenceBootEdges(ids map[string]string, item observation) (boot, routes map[string]bool) {
	boot, routes = map[string]bool{}, map[string]bool{}
	routed := routeDispatched(item)
	for relative, id := range ids {
		if relative == item.relative || !strings.HasPrefix(id, "ruby:source:") {
			continue
		}
		switch {
		case strings.HasPrefix(relative, "config/"):
			boot[id] = true
		case routed && (strings.HasPrefix(relative, "app/controllers/") || strings.HasPrefix(relative, "app/helpers/")):
			routes[id] = true
		}
	}
	return boot, routes
}

// RUBY-AFFECTED-V0-050: the suite and boot candidates are listed once per
// observation, and every test still receives exactly the edges the per-test
// scan gave it: helpers and support of its own suite only (nested engine
// suites included), never its own file, never a non-source unit, and route
// edges only when route-dispatched.
func TestMemoizedSupportAndBootEdgesMatchPerTestScan_RUBYAFFECTEDV0050(t *testing.T) {
	source := func(relative string) observation {
		return observation{relative: relative, kind: frameworkSource, id: "ruby:source:" + strings.TrimSuffix(relative, ".rb")}
	}
	test := func(relative string, kind framework, text string) observation {
		return observation{relative: relative, kind: kind, id: "ruby:" + string(kind) + ":" + strings.TrimSuffix(relative, ".rb"), text: text}
	}
	observed := []observation{
		source("config/application.rb"), source("config/initializers/time.rb"), source("config/routes.rb"),
		source("app/controllers/application_controller.rb"), source("app/controllers/alpha_controller.rb"),
		source("app/helpers/alpha_helper.rb"), source("app/models/widget.rb"),
		source("spec/spec_helper.rb"), source("spec/rails_helper.rb"), source("spec/support/matchers.rb"),
		source("spec/support/shared/fetches.rb"), source("test/test_helper.rb"), source("test/support/login.rb"),
		source("engines/billing/spec/spec_helper.rb"), source("engines/billing/spec/support/money.rb"),
		test("spec/models/widget_spec.rb", frameworkRSpec, "RSpec.describe Widget do\nend\n"),
		test("spec/requests/alpha_spec.rb", frameworkRSpec, "RSpec.describe \"alpha\" do\nend\n"),
		test("spec/services/pay_spec.rb", frameworkRSpec, "RSpec.describe Pay, type: :request do\nend\n"),
		test("spec/support/self_check_spec.rb", frameworkRSpec, "RSpec.describe \"support\" do\nend\n"),
		test("spec/widget_spec.rb", frameworkRSpec, "RSpec.describe Widget do\nend\n"),
		test("test/models/widget_test.rb", frameworkRailsTest, "class WidgetTest < ActiveSupport::TestCase\nend\n"),
		test("test/controllers/alpha_controller_test.rb", frameworkRailsTest, "class AlphaControllerTest\nend\n"),
		test("engines/billing/spec/models/invoice_spec.rb", frameworkRSpec, "RSpec.describe Invoice do\nend\n"),
		test("config/initializers/boot_spec.rb", frameworkRSpec, "RSpec.describe \"boot\" do\nend\n"),
	}
	paths := indexPaths(observed)
	autoload := &railsAutoload{ids: indexPaths(observed)}
	support := supportIndex{}
	// Twice, so the second pass reads only memoized candidates.
	for pass := range 2 {
		for _, item := range observed {
			if got, want := support.supportEdges(item, paths), referenceSupportEdges(item, paths); !maps.Equal(got, want) {
				t.Fatalf("pass %d %s support edges %v, want %v", pass, item.relative, got, want)
			}
			if item.kind == frameworkSource {
				continue
			}
			boot, routes := autoload.bootEdges(item)
			wantBoot, wantRoutes := referenceBootEdges(autoload.ids, item)
			if !maps.Equal(boot, wantBoot) || !maps.Equal(routes, wantRoutes) {
				t.Fatalf("pass %d %s boot %v routes %v, want %v %v", pass, item.relative, boot, routes, wantBoot, wantRoutes)
			}
		}
	}
	// The fixture exercises each case the memo must preserve.
	selfCheck := support.supportEdges(test("spec/support/self_check_spec.rb", frameworkRSpec, ""), paths)
	engine := support.supportEdges(observed[22], paths)
	boot, routed := autoload.bootEdges(observed[16])
	_, unrouted := autoload.bootEdges(observed[15])
	_, typed := autoload.bootEdges(observed[17])
	if len(selfCheck) != 4 || len(engine) != 2 || len(boot) != 3 || len(routed) != 3 || len(unrouted) != 0 || len(typed) != 3 {
		t.Fatalf("fixture lost a case: self=%v engine=%v boot=%v routed=%v unrouted=%v typed=%v", selfCheck, engine, boot, routed, unrouted, typed)
	}
}
