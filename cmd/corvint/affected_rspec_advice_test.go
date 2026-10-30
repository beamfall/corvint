package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// rspecAdviceRepository commits a small Rails-shaped RSpec repository (the
// GitHub #1130 reproduction) and then edits one source path.
func rspecAdviceRepository(t *testing.T, withGemfile bool) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".rspec":                                "--require rails_helper\n",
		"app/api_clients/foo_client.rb":         "class FooClient\n  def fetch = :ok\nend\n",
		"app/services/bar/baz_service.rb":       "module Bar\n  class BazService\n  end\nend\n",
		"spec/api_clients/foo_client_spec.rb":   "require \"rails_helper\"\n\nRSpec.describe FooClient do\n  it { expect(FooClient.new.fetch).to eq(:ok) }\nend\n",
		"spec/services/bar/baz_service_spec.rb": "require \"rails_helper\"\n\nRSpec.describe Bar::BazService do\n  it { expect(Bar::BazService.new).to be_truthy }\nend\n",
		"spec/rails_helper.rb":                  "require \"spec_helper\"\n",
		"spec/spec_helper.rb":                   "RSpec.configure do |config|\nend\n",
	}
	if withGemfile {
		files["Gemfile"] = "source \"https://rubygems.org\"\ngem \"rspec-rails\"\n"
	}
	for name, body := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	edited := filepath.Join(root, "app", "api_clients", "foo_client.rb")
	if err := os.WriteFile(edited, []byte("class FooClient\n  def fetch = :changed\nend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestAFPV0066AdviceListsSelectedRSpecFiles: AFP-V0-066. A dirty Ruby source
// whose plan selects an RSpec unit yields one advisory `bundle exec rspec`
// check over the selected spec files.
func TestAFPV0066AdviceListsSelectedRSpecFiles(t *testing.T) {
	t.Parallel()
	root := rspecAdviceRepository(t, true)
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	checks := affectedChecks(t, receipt)
	if len(checks) != 1 {
		t.Fatalf("expected one advisory RSpec check: %v", checks)
	}
	check := checks[0]
	if check["command"] != "bundle exec rspec './spec/api_clients/foo_client_spec.rb'" ||
		check["kind"] != adviceKindAdvisory || check["source"] != adviceSourcePlan {
		t.Fatalf("advisory RSpec check = %v", check)
	}
	if !strings.Contains(check["reason"].(string), "1 RSpec unit(s)") || !strings.Contains(check["reason"].(string), "Gemfile") {
		t.Fatalf("reason must name the selection and the Gemfile: %v", check)
	}
	unknown := affectedAdviceUnknown(t, receipt)
	for _, line := range unknown {
		if strings.HasPrefix(line, "NO_ADVISORY_RSPEC_COMMAND") {
			t.Fatalf("a runnable RSpec check must not also report it is missing: %v", unknown)
		}
	}
}

// TestAFPV0066NoRSpecCommandWithoutGemfile: AFP-V0-066. Without a root
// Gemfile the same selection yields no command and one unknown line.
func TestAFPV0066NoRSpecCommandWithoutGemfile(t *testing.T) {
	t.Parallel()
	root := rspecAdviceRepository(t, false)
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if checks := affectedChecks(t, receipt); len(checks) != 0 {
		t.Fatalf("no command may be offered without a Gemfile: %v", checks)
	}
	if unknown := affectedAdviceUnknown(t, receipt); !slices.Contains(unknown, adviceNoRSpecGemfile) {
		t.Fatalf("unknown must name the missing Gemfile: %v", unknown)
	}
}

// TestAFPV0066RSpecAdviceOrderBoundAndScope: AFP-V0-066. The RSpec check
// follows the Go advisory check, quotes and sorts its files, ignores
// non-RSpec units, and is replaced by an unknown line past the file bound.
func TestAFPV0066RSpecAdviceOrderBoundAndScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source \"https://rubygems.org\"\ngem \"rspec\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Selected: []affected.Selection{
		{UnitID: "go:example.com/fixture/core", Tests: []string{"core/core_test.go"}},
		{UnitID: "ruby:rspec:spec/z_spec", Tests: []string{"spec/z_spec.rb"}},
		{UnitID: "ruby:minitest:test/a_test", Tests: []string{"test/a_test.rb"}},
		{UnitID: "ruby:rspec:spec/it's_spec", Tests: []string{"spec/it's_spec.rb"}},
	}}
	provider := affectedGoProvider{Packages: []string{"example.com/fixture/core"}, State: providerStateRunnable}
	advice := compileAffectedAdvice(root, nil, plan, provider)
	if len(advice.Checks) != 2 || !strings.HasPrefix(advice.Checks[0].Command, adviceAdvisoryGoTest) {
		t.Fatalf("the Go advisory check must come first and stay unchanged: %v", advice.Checks)
	}
	want := `bundle exec rspec './spec/it'\''s_spec.rb' './spec/z_spec.rb'`
	if advice.Checks[1].Command != want {
		t.Fatalf("RSpec command = %q, want %q", advice.Checks[1].Command, want)
	}
	if !strings.Contains(advice.Checks[1].Reason, "2 RSpec unit(s)") {
		t.Fatalf("reason must count RSpec units only: %q", advice.Checks[1].Reason)
	}

	over := affected.Plan{}
	for index := 0; index <= adviceMaxRSpecFiles; index++ {
		name := fmt.Sprintf("spec/f%04d_spec", index)
		over.Selected = append(over.Selected, affected.Selection{UnitID: "ruby:rspec:" + name, Tests: []string{name + ".rb"}})
	}
	bounded := compileAffectedAdvice(root, nil, over, affectedGoProvider{State: providerStateEmpty})
	if len(bounded.Checks) != 0 {
		t.Fatalf("a selection past the bound must not yield a command: %v", bounded.Checks)
	}
	if !slices.Contains(bounded.Unknown, "NO_ADVISORY_RSPEC_COMMAND: 1025 selected spec files exceed the 1024-file bound") {
		t.Fatalf("unknown must name the bound: %v", bounded.Unknown)
	}

	none := compileAffectedAdvice(root, nil, affected.Plan{Selected: plan.Selected[:1]}, provider)
	for _, line := range none.Unknown {
		if strings.HasPrefix(line, "NO_ADVISORY_RSPEC_COMMAND") {
			t.Fatalf("a plan with no RSpec unit adds no RSpec line: %v", none.Unknown)
		}
	}
}

// TestAFPV0066DashLeadingSpecFileIsNeverAnOption: AFP-V0-066. Every selected
// spec file is passed as a `./` path, so a file named like an RSpec option
// stays an operand.
func TestAFPV0066DashLeadingSpecFileIsNeverAnOption(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("source \"https://rubygems.org\"\ngem \"rspec\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := affected.Plan{Selected: []affected.Selection{
		{UnitID: "ruby:rspec:--format=broken_spec", Tests: []string{"--format=broken_spec.rb"}},
		{UnitID: "ruby:rspec:-r/tmp/x_spec", Tests: []string{"-r/tmp/x_spec.rb"}},
	}}
	advice := compileAffectedAdvice(root, nil, plan, affectedGoProvider{State: providerStateEmpty})
	if len(advice.Checks) != 1 {
		t.Fatalf("expected one advisory RSpec check: %v", advice.Checks)
	}
	want := "bundle exec rspec './--format=broken_spec.rb' './-r/tmp/x_spec.rb'"
	if advice.Checks[0].Command != want {
		t.Fatalf("RSpec command = %q, want %q", advice.Checks[0].Command, want)
	}
	for _, word := range strings.Fields(strings.TrimPrefix(advice.Checks[0].Command, adviceAdvisoryRSpec+" ")) {
		if !strings.HasPrefix(word, "'./") {
			t.Fatalf("operand %q could be read as an option", word)
		}
	}
}

// TestAFPV0066RSpecAdviceNeedsDeclaredRSpecGem: AFP-V0-066 (V1-1828). The
// advisory command needs rspec, rspec-core or rspec-rails declared in the root
// Gemfile or Gemfile.lock by the NGI-V0-052 check; a Minitest repository with a
// spec/ directory gets the no-rspec-gem line instead.
func TestAFPV0066RSpecAdviceNeedsDeclaredRSpecGem(t *testing.T) {
	t.Parallel()
	plan := affected.Plan{Selected: []affected.Selection{{UnitID: "ruby:rspec:spec/a_spec", Tests: []string{"spec/a_spec.rb"}}}}
	cases := []struct {
		name, gemfile, lock string
		runnable            bool
	}{
		{"minitest with spec dir", "source \"https://rubygems.org\"\ngem \"minitest\"\n", "", false},
		{"rspec-rails in Gemfile", "gem 'rspec-rails', group: :test\n", "", true},
		{"rspec-core in lockfile", "gemspec\n", "GEM\n  specs:\n    rspec-core (3.13.0)\n\nDEPENDENCIES\n  minitest\n", true},
		{"rspec only as a nested requirement", "gemspec\n", "GEM\n  specs:\n    rails (7.1.0)\n      rspec-core (>= 3)\n", false},
		{"commented gem line", "# gem \"rspec\"\n", "", false},
		{"oversized Gemfile", "gem \"rspec\"\n" + strings.Repeat("#\n", adviceMaxGemManifestBytes), "", false},
	}
	for _, test := range cases {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "spec"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte(test.gemfile), 0o644); err != nil {
			t.Fatal(err)
		}
		if test.lock != "" {
			if err := os.WriteFile(filepath.Join(root, "Gemfile.lock"), []byte(test.lock), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		advice := compileAffectedAdvice(root, nil, plan, affectedGoProvider{State: providerStateEmpty})
		if runnable := len(advice.Checks) == 1; runnable != test.runnable {
			t.Fatalf("%s: checks=%v unknown=%v", test.name, advice.Checks, advice.Unknown)
		}
		if !test.runnable && !slices.Contains(advice.Unknown, adviceNoRSpecGem) {
			t.Fatalf("%s: unknown must name the missing rspec gem: %v", test.name, advice.Unknown)
		}
	}
}

// TestAFPV0066SymlinkedGemManifestDeclaresNothing: a Gemfile.lock that is a
// symlink, even to a manifest declaring rspec, is never read (AFP-V0-066).
func TestAFPV0066SymlinkedGemManifestDeclaresNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "Gemfile.lock")
	if err := os.WriteFile(target, []byte("GEM\n  specs:\n    rspec-core (3.13.0)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Gemfile"), []byte("gemspec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "Gemfile.lock")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if adviceDeclaresRSpecGem(root) {
		t.Fatal("a symlinked Gemfile.lock must declare nothing")
	}
}

// TestAFPV0009NoGoAdviceGapWithoutGoUnits: AFP-V0-009 (V1-2076). A repository
// whose graph has no Go unit reports no NO_ADVISORY_GO_COMMAND line for its
// empty Go selection; a Go repository keeps the line (the consequence use case
// missing-anchors).
func TestAFPV0009NoGoAdviceGapWithoutGoUnits(t *testing.T) {
	t.Parallel()
	root := rspecAdviceRepository(t, true)
	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if state := receipt["provider"].(map[string]any)["go"].(map[string]any)["state"]; state != providerStateEmpty {
		t.Fatalf("provider.go.state = %v, want %s", state, providerStateEmpty)
	}
	for _, line := range affectedAdviceUnknown(t, receipt) {
		if strings.HasPrefix(line, "NO_ADVISORY_GO_COMMAND") {
			t.Fatalf("a repository without Go units must not report a Go advice gap: %q", line)
		}
	}
}
