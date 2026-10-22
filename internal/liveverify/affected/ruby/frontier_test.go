package ruby

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// writeIssue1519 extends the #1130 Rails fixture with one located site of
// each per-unit kind: a conditional load of a repository file and a
// define_method.
func writeIssue1519(t *testing.T) string {
	t.Helper()
	root := writeIssue1130(t, true)
	write(t, root, "app/models/report.rb", "class Report\n  [:a, :b].each { |name| define_method(name) {} }\nend\n")
	write(t, root, "app/services/exporter.rb", "class Exporter\n  def call\n    require \"report_format\" if ENV[\"FORMAT\"]\n  end\nend\n")
	write(t, root, "lib/report_format.rb", "module ReportFormat\nend\n")
	write(t, root, "spec/models/report_spec.rb", "require \"rails_helper\"\n\nRSpec.describe Report do\n  it \"kinds\" do\n    expect(Report.new).to be_truthy\n  end\nend\n")
	write(t, root, "spec/services/exporter_spec.rb", "require \"rails_helper\"\n\nRSpec.describe Exporter do\n  it \"calls\" do\n    expect(Exporter.new.call).to be_nil\n  end\nend\n")
	return root
}

func frontierUnknown(detail string) affected.Unknown {
	return affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: detail}
}

func hasUnknown(got affected.Plan, want affected.Unknown) bool {
	for _, unknown := range got.Unknown {
		if unknown == want {
			return true
		}
	}
	return false
}

// TestLocatedFrontierCountsOnlyCrossedSites_RUBYAFFECTEDV0047 is the #1519
// acceptance: every site carries its path and line, a change whose reverse
// closure crosses no site is BOUNDED with its tier-1 selection, and a change
// that crosses one stays UNKNOWN naming exactly that site.
func TestLocatedFrontierCountsOnlyCrossedSites_RUBYAFFECTEDV0047(t *testing.T) {
	root := writeIssue1519(t)
	result, err := New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	conditional := FrontierConditionalLoad + ":app/services/exporter.rb:3"
	definition := FrontierDynamicTestDefinition + ":app/models/report.rb:2"
	if len(result.Frontier) != 0 || !equal(unitByID(t, result, "ruby:source:app/services/exporter").Frontier, []string{conditional}) ||
		!equal(unitByID(t, result, "ruby:source:app/models/report").Frontier, []string{definition}) {
		t.Fatalf("sites not located per unit: %v", allFrontier(result))
	}

	uncrossed := plan(t, root, "app/api_clients/foo_client.rb")
	kinds := selectedWitnesses(uncrossed)
	if uncrossed.Scope != affected.ScopeBounded || len(uncrossed.Unknown) != 0 || len(kinds) != 2 ||
		kinds["spec/api_clients/foo_client_spec.rb"] != affected.WitnessDependency || kinds["spec/services/bar/baz_service_spec.rb"] != affected.WitnessAdvisoryConstant {
		t.Fatalf("uncrossed: scope=%s selected=%v unknown=%v", uncrossed.Scope, kinds, uncrossed.Unknown)
	}
	// AFP-V0-381: the plan still counts the repository's sites it does not cross.
	if !maps.Equal(uncrossed.FrontierSites, issue1519Sites()) {
		t.Fatalf("uncrossed frontierSites=%v", uncrossed.FrontierSites)
	}

	// The site's own change and a change to what it loads both cross it.
	for _, changed := range []string{"app/services/exporter.rb", "lib/report_format.rb"} {
		crossed := plan(t, root, changed)
		if crossed.Scope != affected.ScopeUnknown || len(crossed.Unknown) != 1 || crossed.Unknown[0] != frontierUnknown(conditional) ||
			selectedWitnesses(crossed)["spec/services/exporter_spec.rb"] == "" {
			t.Fatalf("%s: scope=%s selected=%v unknown=%v", changed, crossed.Scope, crossed.SelectedTests(), crossed.Unknown)
		}
	}
	model := plan(t, root, "app/models/report.rb")
	if model.Scope != affected.ScopeUnknown || len(model.Unknown) != 1 || model.Unknown[0] != frontierUnknown(definition) {
		t.Fatalf("model: scope=%s unknown=%v", model.Scope, model.Unknown)
	}
}

// TestRepositoryWideLoadSitesStayPluginWideAndLocated_RUBYAFFECTEDV0046
// keeps every site whose reach no closure bounds plugin-wide, located, and
// bounded per reason: a dynamic constant lookup, a computed load, a test with
// no visible edge, load-path manipulation and unmodelled autoload
// configuration. A valid autoload declaration stands in for autoload-root
// configuration, and a frontier declaration resolves a named site.
func TestRepositoryWideLoadSitesStayPluginWideAndLocated_RUBYAFFECTEDV0046(t *testing.T) {
	root := writeIssue1519(t)
	for name, site := range map[string]struct{ path, body, detail string }{
		"constantize": {"app/services/lookup.rb", "class Lookup\n  def call(name)\n    name.constantize.new\n  end\nend\n", FrontierRailsAutoload + ":app/services/lookup.rb:3"},
		"computed":    {"app/services/plugins.rb", "class Plugins\n  def fetch(name)\n    require \"plugins/#{name}\"\n  end\nend\n", FrontierDynamicLoad + ":app/services/plugins.rb:3"},
		"edgeless":    {"spec/smoke_spec.rb", "RSpec.describe \"smoke\" do\n  it(\"boots\") { expect(1).to eq(1) }\nend\n", FrontierImplicitTestDependency + ":spec/smoke_spec.rb"},
	} {
		repo := writeIssue1519(t)
		write(t, repo, site.path, site.body)
		got := plan(t, repo, "app/api_clients/foo_client.rb")
		if got.Scope != affected.ScopeUnknown || len(got.Unknown) != 1 || got.Unknown[0] != frontierUnknown(site.detail) {
			t.Fatalf("%s: scope=%s unknown=%v", name, got.Scope, got.Unknown)
		}
		write(t, repo, FrontierDeclarationPath, `{"schema":"ruby-frontiers/0","resolved":["`+site.detail+`"]}`)
		if got = plan(t, repo, "app/api_clients/foo_client.rb"); got.Scope != affected.ScopeBounded || !maps.Equal(got.FrontierSites, issue1519Sites()) {
			t.Fatalf("%s declared: scope=%s unknown=%v frontierSites=%v", name, got.Scope, got.Unknown, got.FrontierSites)
		}
	}
	write(t, root, "spec/spec_helper.rb", "$LOAD_PATH.unshift(File.expand_path(\"../lib\", __dir__))\nRSpec.configure do |config|\nend\n")
	got := plan(t, root, "app/api_clients/foo_client.rb")
	if got.Scope != affected.ScopeUnknown || !hasUnknown(got, frontierUnknown(FrontierDynamicLoad+":spec/spec_helper.rb:1")) {
		t.Fatalf("load path: scope=%s unknown=%v", got.Scope, got.Unknown)
	}

	configured := writeIssue1519(t)
	write(t, configured, "config/application.rb", "require_relative \"boot\"\n\nmodule Fixture\n  class Application < Rails::Application\n    config.autoload_paths << Rails.root.join(\"lib\")\n  end\nend\n")
	got = plan(t, configured, "app/api_clients/foo_client.rb")
	if got.Scope != affected.ScopeUnknown || len(got.Unknown) != 1 || got.Unknown[0] != frontierUnknown(FrontierRailsAutoload+":config/application.rb:5") {
		t.Fatalf("autoload config: scope=%s unknown=%v", got.Scope, got.Unknown)
	}
	write(t, configured, ".corvint/ruby-autoload.json", "{\"schema\":\"ruby-autoload/0\",\"roots\":[\"app/api_clients\",\"app/models\",\"app/services\",\"lib\"]}\n")
	if got = plan(t, configured, "app/api_clients/foo_client.rb"); got.Scope != affected.ScopeBounded {
		t.Fatalf("declared autoload config: scope=%s unknown=%v", got.Scope, got.Unknown)
	}

	sites := map[string]bool{}
	for index := 1; index <= maxSitesPerReason+3; index++ {
		sites[fmt.Sprintf("%s:lib/file%02d.rb:1", FrontierDynamicLoad, index)] = true
	}
	sites[FrontierUnreadableSource+":lib/broken.rb"] = true
	bounded := boundSites(sites)
	if len(bounded) != maxSitesPerReason+2 || !contains(bounded, FrontierDynamicLoad+":+3") || !contains(bounded, FrontierUnreadableSource+":lib/broken.rb") {
		t.Fatalf("bounded=%v", bounded)
	}
}

// TestFrontierDeclarationResolvesNamedSites_RUBYAFFECTEDV0048 resolves
// declared sites, by exact line or by reason and path, and fails closed on a
// declaration it cannot read.
func TestFrontierDeclarationResolvesNamedSites_RUBYAFFECTEDV0048(t *testing.T) {
	root := writeIssue1519(t)
	write(t, root, FrontierDeclarationPath, `{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:app/services/exporter.rb:3","ruby:dynamic-test-definition:app/models/report.rb"]}`)
	for _, changed := range []string{"app/services/exporter.rb", "app/models/report.rb"} {
		if got := plan(t, root, changed); got.Scope != affected.ScopeBounded || len(got.Unknown) != 0 {
			t.Fatalf("%s: scope=%s unknown=%v", changed, got.Scope, got.Unknown)
		}
	}
	// A declared line that no longer matches resolves nothing.
	write(t, root, FrontierDeclarationPath, `{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:app/services/exporter.rb:4"]}`)
	if got := plan(t, root, "app/services/exporter.rb"); got.Scope != affected.ScopeUnknown {
		t.Fatalf("stale line resolved the site: %v", got.Unknown)
	}

	invalid := frontierUnknown(FrontierDeclarationInvalid + ":" + FrontierDeclarationPath)
	for name, body := range map[string]string{
		"unknown-member":   `{"schema":"ruby-frontiers/0","resolved":[],"extra":1}`,
		"schema":           `{"schema":"ruby-frontiers/1","resolved":[]}`,
		"missing-resolved": `{"schema":"ruby-frontiers/0"}`,
		"undeclarable":     `{"schema":"ruby-frontiers/0","resolved":["ruby:unreadable-source:lib/x.rb"]}`,
		"path":             `{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:../x.rb:1"]}`,
		"duplicate":        `{"schema":"ruby-frontiers/0","resolved":["ruby:conditional-load:a.rb","ruby:conditional-load:a.rb"]}`,
		"duplicate-member": `{"schema":"ruby-frontiers/0","schema":"ruby-frontiers/0","resolved":[]}`,
		"not-json":         `{`,
	} {
		write(t, root, FrontierDeclarationPath, body)
		got := plan(t, root, "app/api_clients/foo_client.rb")
		if got.Scope != affected.ScopeUnknown || !hasUnknown(got, invalid) {
			t.Fatalf("%s: scope=%s unknown=%v", name, got.Scope, got.Unknown)
		}
	}
	if !strings.HasPrefix(FrontierDeclarationInvalid, "ruby:") {
		t.Fatal("declaration reason outside the plugin namespace")
	}
}

// issue1519Sites is the AFP-V0-381 frontier site count of writeIssue1519: its
// two per-unit sites, and no declared site.
func issue1519Sites() map[string]int {
	return map[string]int{FrontierConditionalLoad: 1, FrontierDynamicTestDefinition: 1}
}
