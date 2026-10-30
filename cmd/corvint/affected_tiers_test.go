package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
)

// AFP-V0-162: a change to one package puts it in tier 0, its direct importer
// and a same-parent sibling dependent in tier 1, and a transitive dependent
// in tier 2, without changing the selection; the --full document is unchanged.
func TestAFPV0162AffectedTiersRankTheFullSelection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                "module example.com/tiers\n\ngo 1.27.0\n",
		"x/a/core/core.go":      "package core\n\nfunc Value() int { return 1 }\n",
		"x/a/core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"x/b/mid/mid.go":        "package mid\n\nimport \"example.com/tiers/x/a/core\"\n\nfunc Value() int { return core.Value() }\n",
		"x/b/mid/mid_test.go":   "package mid\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"x/c/leaf/leaf.go":      "package leaf\n\nimport \"example.com/tiers/x/b/mid\"\n\nfunc Value() int { return mid.Value() }\n",
		"x/c/leaf/leaf_test.go": "package leaf\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
		"x/a/peer/peer.go":      "package peer\n\nimport \"example.com/tiers/x/b/mid\"\n\nfunc Value() int { return mid.Value() }\n",
		"x/a/peer/peer_test.go": "package peer\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { _ = Value() }\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"}, {"add", "."}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	appendFile(t, filepath.Join(root, "x", "a", "core", "core.go"), "\n// edited\n")

	receipt, _, stderr, code := runAffectedCLI(t, root)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	tiers, ok := receipt["tiers"].(map[string]any)
	if !ok {
		t.Fatalf("default document has no tiers member: %v", receipt)
	}
	want := map[string]any{
		"go:example.com/tiers/x/a/core": float64(0),
		"go:example.com/tiers/x/b/mid":  float64(1),
		"go:example.com/tiers/x/a/peer": float64(1),
		"go:example.com/tiers/x/c/leaf": float64(2),
	}
	if !reflect.DeepEqual(tiers["units"], want) {
		t.Fatalf("tiers.units = %v, want %v", tiers["units"], want)
	}
	if !reflect.DeepEqual(tiers["counts"], []any{float64(1), float64(2), float64(1)}) || tiers["summary"] != "tier0=1 tier1=2 tier2=1" {
		t.Fatalf("tier counts %v summary %v", tiers["counts"], tiers["summary"])
	}
	selected := receipt["plan"].(map[string]any)["selected"].([]any)
	if len(selected) != len(want) {
		t.Fatalf("tiers must rank the whole selection: %d selected, %d tiered", len(selected), len(want))
	}
	full, _, stderr, code := runAffectedCLI(t, root, "--full")
	if code != 0 {
		t.Fatalf("--full exit %d: %s", code, stderr)
	}
	if _, present := full["tiers"]; present {
		t.Fatal("the affected-plan/0 --full document must stay unchanged")
	}
}

func TestAFPV0162EmptySelectionOmitsTiers(t *testing.T) {
	if tiers := tierAffectedSelections(nil); tiers != nil {
		t.Fatalf("empty selection tiers = %v, want nil", tiers)
	}
}

// AFP-V0-162: a changed package directly under a top-level directory makes
// no siblings, so internal/* does not flood tier 1.
func TestAFPV0162TopLevelParentHasNoSiblings(t *testing.T) {
	tiers := tierAffectedSelections([]affected.Selection{
		{UnitID: "go:example.com/m/internal/changed", Witness: affected.Witness{Kind: affected.WitnessDirectSource, DirtyPath: "internal/changed/a.go", Via: []string{"go:example.com/m/internal/changed"}}},
		{UnitID: "go:example.com/m/internal/other", Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "internal/changed/a.go", Via: []string{"go:example.com/m/internal/changed", "go:example.com/m/internal/mid", "go:example.com/m/internal/other"}}},
	})
	if tiers.Units["go:example.com/m/internal/other"] != 2 || tiers.Summary != "tier0=1 tier1=0 tier2=1" {
		t.Fatalf("tiers = %+v", tiers)
	}
}

// RUBY-AFFECTED-V0-049 (GH #1565): a controller change ranks the spec that
// names it by constant in tier 1 and leaves the route-dispatch fan-out to
// another controller's spec selected in tier 2, witnessed ROUTE_DISPATCH_BOOT.
func TestRubyRouteDispatchSpecsAreTierTwo_RUBYAFFECTEDV0049(t *testing.T) {
	t.Parallel()
	tree := map[string]string{
		"Gemfile":               "source \"https://rubygems.org\"\ngem \"rails\"\ngem \"rspec-rails\"\n",
		".rspec":                "--require rails_helper\n",
		"config/application.rb": "module Fixture\n  class Application < Rails::Application\n  end\nend\n",
		"app/controllers/application_controller.rb": "class ApplicationController\nend\n",
		"app/controllers/alpha_controller.rb":       "class AlphaController < ApplicationController\nend\n",
		"app/controllers/beta_controller.rb":        "class BetaController < ApplicationController\nend\n",
		"spec/rails_helper.rb":                      "RSpec.configure do |config|\nend\n",
		"spec/requests/alpha_flow_spec.rb":          "require \"rails_helper\"\n\nRSpec.describe AlphaController, type: :request do\n  it \"lists\" do\n    get \"/alpha\"\n  end\nend\n",
		"spec/requests/beta_flow_spec.rb":           "require \"rails_helper\"\n\nRSpec.describe BetaController, type: :request do\n  it \"lists\" do\n    get \"/beta\"\n  end\nend\n",
		"spec/requests/pages_spec.rb":               "require \"rails_helper\"\n\nRSpec.describe \"pages\", type: :request do\n  it \"lists\" do\n    get \"/pages\"\n  end\nend\n",
	}
	root := t.TempDir()
	for name, body := range tree {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, ruby.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"app/controllers/alpha_controller.rb"})
	tiers := tierAffectedSelections(plan.Selected)
	if plan.Scope != affected.ScopeBounded || tiers == nil {
		t.Fatalf("scope=%s unknown=%v selected=%v", plan.Scope, plan.Unknown, plan.SelectedTests())
	}
	kinds := map[string]string{}
	for _, selection := range plan.Selected {
		kinds[selection.UnitID] = selection.Witness.Kind
	}
	const alpha, beta, pages = "ruby:rspec:spec/requests/alpha_flow_spec", "ruby:rspec:spec/requests/beta_flow_spec", "ruby:rspec:spec/requests/pages_spec"
	if tiers.Units[alpha] != 1 || kinds[alpha] != affected.WitnessAdvisoryConstant {
		t.Fatalf("describing spec tier=%d kind=%s; tiers=%+v", tiers.Units[alpha], kinds[alpha], tiers)
	}
	for _, id := range []string{beta, pages} {
		tier, selected := tiers.Units[id]
		if !selected || tier != 2 || kinds[id] != "ROUTE_DISPATCH_BOOT" {
			t.Fatalf("%s selected=%v tier=%d kind=%s; tiers=%+v", id, selected, tier, kinds[id], tiers)
		}
	}
}
