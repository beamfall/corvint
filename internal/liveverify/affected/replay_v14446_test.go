// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestRubyFixturesOfANewGoPackageAreNoUnindexedSource_V14446 replays the shape of PR 85
// (run 41714153849): a new Go package whose tests run Ruby fixtures below its own testdata
// directory. In a Go-module root the Ruby plugin indexes nothing below testdata
// (RUBY-AFFECTED-V0-041) but still claims *.rb, so every fixture was UNINDEXED_SOURCE_PATH and
// the CI selector fell back to the whole run. Under AFP-V0-2440 each is UNOWNED_DIRTY_PATH and the
// hosting package is selected. The negative control drops go.mod: no Go unit hosts the directory,
// the Ruby plugin indexes the files as its own, and nothing is reclassified.
func TestRubyFixturesOfANewGoPackageAreNoUnindexedSource_V14446(t *testing.T) {
	files := map[string]string{
		"go.mod":                    "module example.test/m\n\ngo 1.27\n",
		"internal/rubylinecov/c.go": "package rubylinecov\n\nfunc Collect() {}\n",
		"internal/rubylinecov/c_test.go": "package rubylinecov\n\nimport \"testing\"\n\n" +
			"func TestCollect(t *testing.T) { _ = \"testdata/minitest\"; Collect() }\n",
		"internal/rubylinecov/testdata/minitest/lib/calc.rb":       "class Calc\n  def add(a, b) = a + b\nend\n",
		"internal/rubylinecov/testdata/minitest/test/calc_test.rb": "require 'minitest/autorun'\nrequire_relative '../lib/calc'\nclass CalcTest < Minitest::Test\n  def test_add = assert_equal(3, Calc.new.add(1, 2))\nend\n",
		"internal/rubylinecov/testdata/rspec/spec/calc_spec.rb":    "RSpec.describe 'calc' do\n  it('adds') { expect(1 + 2).to eq(3) }\nend\n",
		"internal/rubylinecov/testdata/minitest/bin/tool":          "#!/usr/bin/env ruby\nputs 1\n",
		"lib/app.rb":       "class App\nend\n",
		"test/app_test.rb": "require 'minitest/autorun'\nrequire_relative '../lib/app'\nclass AppTest < Minitest::Test\n  def test_app = assert(App)\nend\n",
	}
	fixtures := []string{
		"internal/rubylinecov/testdata/minitest/lib/calc.rb",
		"internal/rubylinecov/testdata/minitest/test/calc_test.rb",
		"internal/rubylinecov/testdata/rspec/spec/calc_spec.rb",
	}
	plan := func(t *testing.T, withGoModule bool) affected.Plan {
		root := t.TempDir()
		for name, body := range files {
			if name == "go.mod" && !withGoModule {
				continue
			}
			if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		graph, err := affected.Build(context.Background(), root, languages.All()...)
		if err != nil {
			t.Fatal(err)
		}
		return affected.Select(graph, append(append([]string{}, fixtures...), "internal/rubylinecov/testdata/minitest/bin/tool"))
	}
	t.Run("a Go package hosts the fixtures", func(t *testing.T) {
		p := plan(t, true)
		reasons := map[string]string{}
		for _, unknown := range p.Unknown {
			if unknown.Reason == affected.UnknownUnindexedSourcePath {
				t.Errorf("plan has %s %s", unknown.Reason, unknown.Detail)
			}
			if unknown.Reason == affected.UnknownUnownedDirtyPath {
				reasons[unknown.Detail] = unknown.Hint
			}
		}
		for _, fixture := range fixtures {
			if hint, ok := reasons[fixture]; !ok || !strings.Contains(hint, "ruby") || !strings.Contains(hint, "AFP-V0-2440") {
				t.Errorf("%s: want UNOWNED_DIRTY_PATH with the ruby fixture hint, got %q (present %v)", fixture, hint, ok)
			}
		}
		if hint, ok := reasons["internal/rubylinecov/testdata/minitest/bin/tool"]; !ok || hint != "" {
			t.Errorf("the unclaimed bin/tool must stay a plain UNOWNED_DIRTY_PATH, got %q (present %v)", hint, ok)
		}
		var selected []string
		for _, selection := range p.Selected {
			selected = append(selected, selection.UnitID)
		}
		if !slices.Contains(selected, "go:example.test/m/internal/rubylinecov") {
			t.Errorf("the hosting Go package is not selected: %v", selected)
		}
	})
	t.Run("negative control: no Go module, the fixtures are Ruby's own", func(t *testing.T) {
		p := plan(t, false)
		for _, unknown := range p.Unknown {
			if strings.Contains(unknown.Hint, "AFP-V0-2440") {
				t.Errorf("reclassified without a hosting Go unit: %+v", unknown)
			}
		}
	})
}
