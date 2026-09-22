package ruby

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func TestOwnsOnlyRubySourcePaths(t *testing.T) {
	language := New()
	for _, relative := range []string{"lib/a.rb", "spec/a_spec.rb", "features/steps.rb"} {
		if !language.Owns(relative) {
			t.Fatalf("%s should be owned", relative)
		}
	}
	for _, relative := range []string{"features/a.feature", "config/a.yml", "lib/a.py", "Gemfile"} {
		if language.Owns(relative) {
			t.Fatalf("%s must not be owned", relative)
		}
	}
}

func TestUnitsDetectsConcurrentFrameworksAndRunnerIdentities(t *testing.T) {
	root := t.TempDir()
	write(t, root, "config/application.rb", "class Application < Rails::Application\nend\n")
	write(t, root, "app/models/account.rb", "class Account\nend\n")
	write(t, root, "spec/spec_helper.rb", "RSpec.configure do |config|\nend\n")
	write(t, root, "spec/runnable_helper.rb", "RSpec.describe \"helper\" do\n  it { expect(true).to be(true) }\nend\n")
	write(t, root, "spec/models/account_spec.rb", "require \"spec_helper\"\nRSpec.describe Account do\n  it { is_expected.to be_truthy }\nend\n")
	write(t, root, "spec/features/login_spec.rb", "require \"capybara/rspec\"\nRSpec.describe \"login\", type: :feature do\n  it { visit \"/\" }\nend\n")
	write(t, root, "test/test_helper.rb", "require_relative \"../config/application\"\n")
	write(t, root, "test/models/account_test.rb", "require \"test_helper\"\nclass AccountTest < ActiveSupport::TestCase\n  def test_valid\n  end\nend\n")
	write(t, root, "test/system/accounts_test.rb", "require \"application_system_test_case\"\nclass AccountsTest < ApplicationSystemTestCase\n  driven_by :selenium\n  def test_index\n  end\nend\n")
	write(t, root, "tools/standalone_spec.rb", "require \"minitest/autorun\"\ndescribe \"standalone\" do\n  it \"works\" do\n  end\nend\n")
	write(t, root, "tools/standalone_test.rb", "require \"minitest/autorun\"\nclass StandaloneTest < Minitest::Test\n  def test_value\n  end\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"spec/models/account_spec.rb":  "ruby:rspec:spec/models/account_spec",
		"spec/features/login_spec.rb":  "ruby:rspec:spec/features/login_spec",
		"spec/runnable_helper.rb":      "ruby:rspec:spec/runnable_helper",
		"test/models/account_test.rb":  "ruby:rails-test:test/models/account_test",
		"test/system/accounts_test.rb": "ruby:rails-test:test/system/accounts_test",
		"tools/standalone_spec.rb":     "ruby:minitest:tools/standalone_spec",
		"tools/standalone_test.rb":     "ruby:minitest:tools/standalone_test",
	}
	for testPath, id := range want {
		unit := unitByID(t, result, id)
		if len(unit.Tests) != 1 || unit.Tests[0] != testPath {
			t.Fatalf("%s tests=%v", id, unit.Tests)
		}
	}
	for _, helper := range []string{"ruby:source:spec/spec_helper", "ruby:source:test/test_helper"} {
		unit := unitByID(t, result, helper)
		if len(unit.Tests) != 0 || len(unit.Sources) != 1 {
			t.Fatalf("helper %s misclassified: %+v", helper, unit)
		}
	}
	if !contains(result.Frontier, FrontierRailsAutoload) {
		t.Fatalf("Rails autoload frontier missing: %v", result.Frontier)
	}
}

func TestFrameworkDetectionIgnoresCommentAndHeredocExamples(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/commented.rb", "# class Example < Test::Unit::TestCase\n#   def test_example\n#   end\n")
	write(t, root, "lib/documentation.rb", "EXAMPLE = <<~RUBY\nRSpec.describe Example do\n  it { true }\nend\nRUBY\n")
	write(t, root, "checks/append_then_spec.rb", "values << \"example\"\nRSpec.describe Example do\n  it { true }\nend\n")
	write(t, root, "spec/string_fixture_spec.rb", "require \"spec_helper\"\nRSpec.describe \"fixture\" do\n  let(:payload) { 'require \"minitest/autorun\"; test/unit' } # Minitest::Test\n  let(:template) do\n    %Q{\n      class Wrong < Minitest::Test\n      require \"test/unit\"\n    }\n  end\n  it { expect(payload).not_to be_empty }\nend\n")
	write(t, root, "lib/percent_documentation.rb", "EXAMPLE = %q!\nRSpec.describe Example do\n  it { true }\nend\n!\n")
	write(t, root, "spec/spec_helper.rb", "require \"rspec/core\"\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ruby:source:lib/commented", "ruby:source:lib/documentation", "ruby:source:lib/percent_documentation"} {
		unit := unitByID(t, result, id)
		if len(unit.Tests) != 0 || len(unit.Sources) != 1 {
			t.Fatalf("documentation %s misclassified: %+v", id, unit)
		}
	}
	if unit := unitByID(t, result, "ruby:rspec:checks/append_then_spec"); len(unit.Tests) != 1 {
		t.Fatalf("append operator hid a runnable spec: %+v", unit)
	}
	if unit := unitByID(t, result, "ruby:rspec:spec/string_fixture_spec"); len(unit.Tests) != 1 {
		t.Fatalf("fixture strings changed the selected runner: %+v", unit)
	}
}

func TestStandaloneMinitestClassAndSpecDSLAreRunnableFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/widget.rb", "class Widget\nend\n")
	write(t, root, "test/widget_test.rb", "require \"minitest/autorun\"\nrequire \"widget\"\nclass WidgetTest < Minitest::Test\n  def test_value\n  end\nend\n")
	write(t, root, "checks/widget_specification.rb", "require \"minitest/autorun\"\nrequire \"widget\"\ndescribe Widget do\n  it \"works\" do\n  end\nend\n")
	write(t, root, "legacy/test-widget.rb", "require \"test/unit\"\nrequire \"widget\"\nclass WidgetTest < Test::Unit::TestCase\n  def test_value\n  end\nend\n")
	write(t, root, "minispec/spec_helper.rb", "require \"minitest/autorun\"\n")
	write(t, root, "minispec/widget_spec.rb", "require_relative \"spec_helper\"\nrequire \"widget\"\ndescribe Widget do\n  it \"works\" do\n  end\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"ruby:minitest:test/widget_test", "ruby:minitest:checks/widget_specification", "ruby:minitest:legacy/test-widget", "ruby:minitest:minispec/widget_spec"} {
		if len(unitByID(t, result, id).Tests) != 1 {
			t.Fatalf("%s is not a runnable test unit", id)
		}
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"lib/widget.rb"})
	if got := plan.SelectedTests(); !equal(got, []string{"checks/widget_specification.rb", "legacy/test-widget.rb", "minispec/widget_spec.rb", "test/widget_test.rb"}) {
		t.Fatalf("selected=%v", got)
	}
}

func TestLoadsResolveOnlyUniqueRepositoryTargets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/core.rb", "VALUE = 1\n")
	write(t, root, "lib/relative.rb", "require_relative \"core\"\n")
	write(t, root, "app/core.rb", "VALUE = 2\n")
	write(t, root, "lib/compound.rb", "require \"./lib/core\"; require_relative \"relative\"\n")
	write(t, root, "lib/guarded.rb", "enabled && require(\"./lib/core\")\n")
	write(t, root, "lib/either.rb", "require(\"./lib/core\") || require_relative(\"relative\")\n")
	write(t, root, "lib/postfix.rb", "require \"./lib/core\" if enabled\n")
	write(t, root, "lib/load_path.rb", "$:.unshift(File.expand_path(\"../vendor\", __dir__))\n")
	write(t, root, "components/shop/lib/shop/item.rb", "ITEM = 1\n")
	write(t, root, "components/shop/lib/shop/order.rb", "require \"shop/item\"\n")
	write(t, root, "spec/relative_spec.rb", "require \"relative\"\nRSpec.describe \"relative\" do\n  it { expect(true).to be(true) }\nend\n")
	write(t, root, "spec/ambiguous_spec.rb", "require \"core\"\nRSpec.describe \"ambiguous\" do\n  it { expect(true).to be(true) }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	relative := unitByID(t, result, "ruby:source:lib/relative")
	if !contains(relative.Imports, "ruby:source:lib/core") {
		t.Fatalf("require_relative edge missing: %v", relative.Imports)
	}
	if !contains(result.Frontier, FrontierAmbiguousLoad) {
		t.Fatalf("ambiguous load frontier missing: %v", result.Frontier)
	}
	compound := unitByID(t, result, "ruby:source:lib/compound")
	for _, dependency := range []string{"ruby:source:lib/core", "ruby:source:lib/relative"} {
		if !contains(compound.Imports, dependency) {
			t.Fatalf("compound load %s missing: %v", dependency, compound.Imports)
		}
	}
	guarded := unitByID(t, result, "ruby:source:lib/guarded")
	if !contains(guarded.Imports, "ruby:source:lib/core") {
		t.Fatalf("embedded load edge missing: %v", guarded.Imports)
	}
	either := unitByID(t, result, "ruby:source:lib/either")
	for _, dependency := range []string{"ruby:source:lib/core", "ruby:source:lib/relative"} {
		if !contains(either.Imports, dependency) {
			t.Fatalf("alternative load %s missing: %v", dependency, either.Imports)
		}
	}
	if !contains(result.Frontier, FrontierConditionalLoad) || !contains(result.Frontier, FrontierDynamicLoad) {
		t.Fatalf("conditional or load-path uncertainty missing: %v", result.Frontier)
	}
	nested := unitByID(t, result, "ruby:source:components/shop/lib/shop/order")
	if !contains(nested.Imports, "ruby:source:components/shop/lib/shop/item") {
		t.Fatalf("nested source-root edge missing: %v", nested.Imports)
	}
}

func TestUnknownDiscoverySurfacesNeverBecomeSilentExclusions(t *testing.T) {
	root := t.TempDir()
	write(t, root, "spec/generated_spec.rb", "RSpec.describe \"generated\" do\n  [1, 2].each do |value|\n    define_method(\"test_#{value}\") {}\n  end\nend\n")
	write(t, root, "spec/dynamic_spec.rb", "target = ENV.fetch(\"TARGET\")\nrequire target\nRSpec.describe \"dynamic\" do\nend\n")
	write(t, root, "spec/conditional_spec.rb", "if ENV[\"OPTIONAL\"]\n  require \"optional\"\nend\nRSpec.describe \"conditional\" do\nend\n")
	write(t, root, "features/login.feature", "Feature: login\n  Scenario: success\n    Given a user\n")
	write(t, root, "features/step_definitions/login_steps.rb", "Given(\"a user\") do\nend\n")
	write(t, root, "generated.rb", "Class.new(Minitest::Test) do\n  [1, 2].each { |value| define_method(\"test_#{value}\") {} }\nend\n")
	write(t, root, "generated_examples.rb", "RSpec.describe \"generated\" do\n  3.times do\n    it(\"works\") {}\n  end\n  cases.each_with_index { |value, index| public_send(:it, value) {} }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{
		FrontierDynamicLoad,
		FrontierConditionalLoad,
		FrontierDynamicTestDefinition,
		FrontierCucumberOwnership,
	} {
		if !contains(result.Frontier, reason) {
			t.Fatalf("frontier %s missing from %v", reason, result.Frontier)
		}
	}
	for _, unit := range result.Units {
		if contains(unit.Tests, "features/login.feature") || contains(unit.Sources, "features/login.feature") {
			t.Fatalf("Cucumber feature was silently claimed: %+v", unit)
		}
	}
	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"spec/generated_spec.rb"})
	if plan.Scope != affected.ScopeUnknown {
		t.Fatalf("scope=%s unknown=%v", plan.Scope, plan.Unknown)
	}
}

func TestHelperFrameworkHintsIgnoreCommentsAndSurfaceConflicts(t *testing.T) {
	root := t.TempDir()
	write(t, root, "spec/minitest_helper.rb", "# RSpec.describe is documentation\nrequire \"minitest/autorun\"\n")
	write(t, root, "spec/minimal_spec.rb", "require_relative \"minitest_helper\"\ndescribe \"minimal\" do\n  it { true }\nend\n")
	write(t, root, "spec/conflicted_helper.rb", "require \"minitest/autorun\"\nrequire \"rspec/core\"\n")
	write(t, root, "spec/conflicted_spec.rb", "require_relative \"conflicted_helper\"\ndescribe \"conflicted\" do\n  it { true }\nend\n")
	write(t, root, "spec/indirect_helper.rb", "require_relative \"../support/minitest_setup\"\n")
	write(t, root, "support/minitest_setup.rb", "require \"minitest/autorun\"\n")
	write(t, root, "spec/indirect_spec.rb", "require_relative \"indirect_helper\"\ndescribe \"indirect\" do\n  it { true }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(unitByID(t, result, "ruby:minitest:spec/minimal_spec").Tests) != 1 {
		t.Fatalf("commented RSpec example overrode Minitest helper")
	}
	if len(unitByID(t, result, "ruby:minitest:spec/indirect_spec").Tests) != 1 {
		t.Fatalf("transitive helper did not preserve Minitest runner")
	}
	if !contains(result.Frontier, FrontierAmbiguousFramework) {
		t.Fatalf("mixed helper frameworks were not surfaced: %v", result.Frontier)
	}
}

func TestRailsLayoutDoesNotOverrideExplicitTestUnitRunner(t *testing.T) {
	root := t.TempDir()
	write(t, root, "config/application.rb", "class Application < Rails::Application\nend\n")
	write(t, root, "test/legacy_test.rb", "require \"test/unit\"\nclass LegacyTest < Test::Unit::TestCase\n  def test_value\n  end\nend\n")
	write(t, root, "checks/standalone.rb", "require \"minitest/autorun\"\ndescribe \"standalone\" do\n  it { true }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(unitByID(t, result, "ruby:minitest:test/legacy_test").Tests) != 1 {
		t.Fatalf("Rails layout overrode explicit Test::Unit runner: %+v", result.Units)
	}
	if len(unitByID(t, result, "ruby:minitest:checks/standalone").Tests) != 1 {
		t.Fatalf("Rails layout overrode standalone Minitest DSL: %+v", result.Units)
	}
}

func TestHelperInferenceFollowsRepositoryLibAliases(t *testing.T) {
	root := t.TempDir()
	write(t, root, "spec/spec_helper.rb", "require \"support/rspec_setup\"\n")
	write(t, root, "lib/support/rspec_setup.rb", "require \"rspec/core\"\n")
	write(t, root, "spec/unusual_test.rb", "require \"spec_helper\"\ndescribe \"unusual\" do\n  it { true }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(unitByID(t, result, "ruby:rspec:spec/unusual_test").Tests) != 1 {
		t.Fatalf("lib-alias helper chain lost RSpec runner: %+v", result.Units)
	}
}

func TestLoadPathRequireReachesRunnableHelper(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/widget.rb", "class Widget\nend\n")
	write(t, root, "lib/gadget.rb", "class Gadget\nend\n")
	write(t, root, "test/test_helper.rb", "require \"minitest/autorun\"\nrequire \"widget\"\nclass WidgetTestCase < Minitest::Test\nend\n")
	write(t, root, "test/gadget_test.rb", "require \"test_helper\"\nrequire \"gadget\"\nclass GadgetTest < WidgetTestCase\n  def test_builds\n  end\nend\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"lib/widget.rb"})
	if !contains(plan.SelectedTests(), "test/gadget_test.rb") {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestRakeTestTaskLiteralLoadPathResolvesRequire(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Rakefile", "require 'rake/testtask'\nRake::TestTask.new do |t|\n  t.libs << 'src'\nend\n")
	write(t, root, "src/gadget.rb", "class Gadget\nend\n")
	write(t, root, "lib/other.rb", "class Other\nend\n")
	write(t, root, "test/gadget_test.rb", "require 'minitest/autorun'\nrequire 'gadget'\nrequire 'other'\nclass GadgetTest < Minitest::Test\n  def test_builds\n  end\nend\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/gadget.rb"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "test/gadget_test.rb" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
}

func TestRakeTestTaskDynamicLoadPathRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Rakefile", "Rake::TestTask.new do |t|\n  t.libs << ENV.fetch('TEST_LIB')\nend\n")
	write(t, root, "src/gadget.rb", "class Gadget\nend\n")
	write(t, root, "lib/other.rb", "class Other\nend\n")
	write(t, root, "test/gadget_test.rb", "require 'minitest/autorun'\nrequire 'gadget'\nrequire 'other'\nclass GadgetTest < Minitest::Test\n  def test_builds\n  end\nend\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/gadget.rb"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierDynamicLoad}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 1 || plan.Unknown[0] != want {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestRakeTestTaskEscapedLoadPathRaisesFrontier(t *testing.T) {
	root := t.TempDir()
	write(t, root, "Rakefile", `Rake::TestTask.new do |t|
  t.libs << "\x73rc"
end
`)
	write(t, root, "src/gadget.rb", "class Gadget\nend\n")
	write(t, root, "lib/other.rb", "class Other\nend\n")
	write(t, root, "test/gadget_test.rb", "require 'minitest/autorun'\nrequire 'gadget'\nrequire 'other'\nclass GadgetTest < Minitest::Test\n  def test_builds\n  end\nend\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"src/gadget.rb"})
	want := affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: FrontierDynamicLoad}
	if plan.Scope != affected.ScopeUnknown || len(plan.Unknown) != 1 || plan.Unknown[0] != want {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, plan.SelectedTests(), plan.Unknown)
	}
}

func TestPostfixLiteralLoadIndependentlyWidensScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/core.rb", "VALUE = 1\n")
	write(t, root, "lib/postfix.rb", "require \"./lib/core\" if enabled\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierConditionalLoad) {
		t.Fatalf("postfix load was treated as unconditional: %v", result.Frontier)
	}
	if !contains(unitByID(t, result, "ruby:source:lib/postfix").Imports, "ruby:source:lib/core") {
		t.Fatalf("postfix literal edge was omitted")
	}
}

func TestRailsHelperKeepsRSpecRunnerAndNestedRailsWidensScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "engines/shop/test/dummy/config/application.rb", "class Application < Rails::Application\nend\n")
	write(t, root, "engines/shop/spec/rails_helper.rb", "require \"config/environment\"\nRSpec.configure do |config|\nend\n")
	write(t, root, "engines/shop/spec/system/checkout_spec.rb", "require \"rails_helper\"\nRSpec.context \"checkout\" do\n  it { true }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(unitByID(t, result, "ruby:rspec:engines/shop/spec/system/checkout_spec").Tests) != 1 {
		t.Fatalf("Rails helper changed the RSpec runner: %+v", result.Units)
	}
	if !contains(result.Frontier, FrontierRailsAutoload) {
		t.Fatalf("nested Rails application did not widen scope: %v", result.Frontier)
	}
}

func TestParenthesizedAutoloadAndImportedGeneratedTestsWidenScope(t *testing.T) {
	root := t.TempDir()
	write(t, root, "support/generated_cases.rb", "Class.new(BaseTest) { define_method(name) { true } }\n")
	write(t, root, "spec/generated_spec.rb", "require_relative \"../support/generated_cases\"\nRSpec.describe \"generated\" do\n  it { true }\nend\n")
	write(t, root, "lib/loader.rb", "autoload(:Widget, \"widget\")\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{FrontierDynamicLoad, FrontierDynamicTestDefinition} {
		if !contains(result.Frontier, reason) {
			t.Fatalf("frontier %s missing from %v", reason, result.Frontier)
		}
	}
}

func TestReflectionDrivenLoopHasIndependentUnknownRegression(t *testing.T) {
	root := t.TempDir()
	write(t, root, "checks/generated.rb", "cases.each_with_index do |value, index|\n  public_send(:test, value) { index }\nend\n")

	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Frontier, FrontierDynamicTestDefinition) {
		t.Fatalf("reflection-driven loop was silently omitted: %v", result.Frontier)
	}
}

// TestRealRepositoryDiscoveryMatchesConventionalFiles is the opt-in real-corpus
// smoke gate. CORVINT_RUBY_EXPECTED_TEST_FILES is supplied by an independent
// framework-specific count; conventional paths are checked for omissions here,
// while nonconventional cases remain part of the external corpus audit.
func TestRealRepositoryDiscoveryMatchesConventionalFiles(t *testing.T) {
	root := os.Getenv("CORVINT_RUBY_REPOSITORY")
	if root == "" {
		t.Skip("CORVINT_RUBY_REPOSITORY is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := affected.SourceFiles(root, func(name string) bool {
		return strings.HasSuffix(name, ".rb")
	})
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]bool)
	for _, relative := range files {
		base := filepath.Base(relative)
		insideTest := strings.HasPrefix(relative, "test/")
		if strings.HasSuffix(base, "_spec.rb") || strings.HasSuffix(base, "_test.rb") || insideTest && (strings.HasPrefix(base, "test_") || strings.HasPrefix(base, "test-") || strings.HasPrefix(base, "spec_")) {
			actual[relative] = true
		}
	}
	result, err := New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]bool)
	for _, unit := range result.Units {
		for _, relative := range unit.Tests {
			discovered[relative] = true
		}
	}
	missing := make([]string, 0)
	extra := make([]string, 0)
	for relative := range actual {
		if !discovered[relative] {
			missing = append(missing, relative)
		}
	}
	for relative := range discovered {
		if !actual[relative] {
			extra = append(extra, relative)
		}
	}
	t.Logf("nonconventional discovered files=%v", extra)
	expected := len(actual)
	if value := os.Getenv("CORVINT_RUBY_EXPECTED_TEST_FILES"); value != "" {
		expected, err = strconv.Atoi(value)
		if err != nil {
			t.Fatalf("CORVINT_RUBY_EXPECTED_TEST_FILES: %v", err)
		}
	}
	t.Logf("ruby real corpus %s: discovered=%d independently-actual=%d ratio=%.1f%% frontier=%v",
		root, len(discovered), expected, ratio(len(discovered), expected), result.Frontier)
	if len(missing) != 0 {
		t.Fatalf("adapter missed conventional test files: %v", missing)
	}
}

func ratio(discovered, actual int) float64 {
	if actual == 0 {
		return 100
	}
	return 100 * float64(discovered) / float64(actual)
}

func unitByID(t *testing.T, result affected.Result, id string) affected.Unit {
	t.Helper()
	for _, unit := range result.Units {
		if unit.ID == id {
			return unit
		}
	}
	t.Fatalf("unit %s missing from %+v", id, result.Units)
	return affected.Unit{}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func equal(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func write(t *testing.T, root, relative, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHelperDeclaringOnlyATestBaseClassStaysSource(t *testing.T) {
	root := t.TempDir()
	write(t, root, "lib/widget.rb", "class Widget\nend\n")
	write(t, root, "test/test_helper.rb", "require \"minitest/autorun\"\nrequire \"widget\"\nclass WidgetTestCase < Minitest::Test\nend\n")
	write(t, root, "test/widget_test.rb", "require \"test_helper\"\nclass WidgetTest < WidgetTestCase\n  def test_builds\n  end\nend\n")

	graph, err := affected.Build(root, New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"lib/widget.rb"})
	if got := plan.SelectedTests(); plan.Scope != affected.ScopeBounded || len(got) != 1 || got[0] != "test/widget_test.rb" {
		t.Fatalf("scope=%s selected=%v unknown=%v", plan.Scope, got, plan.Unknown)
	}
	declared := t.TempDir()
	write(t, declared, "test/declared_helper.rb", "class HelperTest < ActiveSupport::TestCase\n  test \"runs\" do\n  end\nend\n")
	graph, err = affected.Build(declared, New())
	if err != nil {
		t.Fatal(err)
	}
	if plan := affected.Select(graph, []string{"test/declared_helper.rb"}); !contains(plan.SelectedTests(), "test/declared_helper.rb") {
		t.Fatalf("helper with declarative tests was not runnable: scope=%s selected=%v", plan.Scope, plan.SelectedTests())
	}
}
