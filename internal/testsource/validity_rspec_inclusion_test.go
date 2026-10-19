package testsource

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

const widgetExamples = `RSpec.shared_examples 'a widget' do
  it 'reads the data' do
    read_data { |data| expect(data).to be_valid }
  end
  it 'has a name' do
    expect(widget.name).to eq('w')
  end
end
`

const widgetSpec = `require 'rails_helper'
RSpec.describe Widget do
  context 'stub reader' do
    def read_data; build_widget; end
    it_behaves_like 'a widget'
  end
  context 'yielding reader' do
    def read_data
      yield build_widget
    end
    include_examples 'a widget'
  end
end
`

const widgetShared = "spec/support/widget_examples.rb"

// inclusion is a qualified read of widgetExamples through widgetSpec, with the support set bound
// to the shared file alone; edit adjusts it before the read.
func inclusion(t *testing.T, edit func(*testvalidity.RSpecInclusion)) (testvalidity.ExampleSource, string) {
	t.Helper()
	q := testvalidity.RSpecInclusion{
		SpecPath: "spec/widget_spec.rb", SpecText: widgetSpec, IncludeLine: 5,
		SharedPath: widgetShared, SharedText: widgetExamples, ExampleLine: 2,
		Support: bound(map[string]string{widgetShared: widgetExamples}),
	}
	if edit != nil {
		edit(&q)
	}
	return RSpecInclusionSource(q)
}

func project(source testvalidity.ExampleSource, ref string) testvalidity.Projection {
	return testvalidity.ApplyExampleSource(testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, source, ref)
}

// TestRSpecInclusionEvaluatesEachSiteIndependently pins TSA-V0-089 and TSA-V0-090: the same shared
// example read through two inclusions gets each site's own verdict, under a qualified reference
// that keeps both paths, while the bare shared example stays SHARED-EXAMPLE-ONLY.
func TestRSpecInclusionEvaluatesEachSiteIndependently(t *testing.T) {
	source, ref := inclusion(t, nil)
	if want := "spec/widget_spec.rb:5>spec/support/widget_examples.rb:2"; ref != want {
		t.Fatalf("ref = %q, want %q", ref, want)
	}
	if source.Read != testvalidity.ExampleSourceLocated || len(source.Findings) != 1 {
		t.Fatalf("source = %+v, want one finding", source)
	}
	f := source.Findings[0]
	if got := fmt.Sprintf("%s %s %s %s %s", f.Rule, f.Severity, f.Test, f.Location, f.Cause); got != testvalidity.RuleRBBlockNeverYielded+" invalid "+ref+" spec/support/widget_examples.rb:3 spec/widget_spec.rb:4" {
		t.Errorf("finding = %s", got)
	}
	if p := project(source, ref); p.Hygiene.State != testvalidity.HygieneIneligible {
		t.Errorf("through the stub reader: %+v, want INELIGIBLE", p.Hygiene)
	}

	source, ref = inclusion(t, func(q *testvalidity.RSpecInclusion) { q.IncludeLine = 11 })
	if ref != "spec/widget_spec.rb:11>spec/support/widget_examples.rb:2" || source.Read != testvalidity.ExampleSourceLocated || len(source.Findings) != 0 || source.Expectations != 1 {
		t.Fatalf("ref %q source %+v, want located, clean, one expectation", ref, source)
	}
	if p := project(source, ref); p.Hygiene.State != testvalidity.HygieneEligible {
		t.Errorf("through the yielding reader: %+v, want ELIGIBLE", p.Hygiene)
	}

	bare, _ := RSpecExampleSourcesWithSupport("spec/widget_shared_spec.rb", widgetExamples, bound(nil))
	if rules := sourceRules(bare[2]); !slices.Contains(rules, testvalidity.RuleSharedExampleOnly) {
		t.Errorf("bare shared example rules = %v, want %s", rules, testvalidity.RuleSharedExampleOnly)
	}
}

func sourceRules(e testvalidity.ExampleSource) []string {
	out := []string{}
	for _, f := range e.Findings {
		out = append(out, f.Rule)
	}
	return out
}

// TestRSpecInclusionSameFile pins TSA-V0-089's `same` form: a shared group defined in the including
// spec, with a customization block whose let overrides the shared body's.
func TestRSpecInclusionSameFile(t *testing.T) {
	spec := `RSpec.shared_examples 'a widget' do
  let(:name) { nil }
  it 'has a name' do
    expect(name).to eq('w')
  end
end
RSpec.describe Widget do
  it_behaves_like 'a widget' do
    let(:name) { 'w' }
  end
end
`
	source, ref := RSpecInclusionSource(testvalidity.RSpecInclusion{SpecPath: "spec/widget_spec.rb", SpecText: spec, IncludeLine: 8, SharedPath: "spec/widget_spec.rb", SharedText: spec, ExampleLine: 3, Support: bound(nil)})
	if ref != "spec/widget_spec.rb:8>same:3" || source.Read != testvalidity.ExampleSourceLocated || len(source.Findings) != 0 {
		t.Fatalf("ref %q source %+v, want located and clean", ref, source)
	}
	if !RSpecQualifiedRef.MatchString(ref) {
		t.Errorf("%q does not match the qualified reference syntax", ref)
	}
}

// TestRSpecInclusionRefusals pins TSA-V0-091: each inclusion the reader cannot bind to one site and
// one definition refuses with its own reason, never a verdict.
func TestRSpecInclusionRefusals(t *testing.T) {
	nested := "RSpec.shared_context 'outer' do\n  " + strings.ReplaceAll(strings.TrimSuffix(widgetExamples, "\n"), "\n", "\n  ") + "\nend\n"
	including := strings.Replace(widgetExamples, "  it 'has a name' do", "  include_context 'names'\n  it 'has a name' do", 1)
	looped := strings.Replace(widgetExamples, "  it 'reads the data' do\n    read_data { |data| expect(data).to be_valid }\n  end\n",
		"  %w[a b].each do |n|\n    it \"reads #{n}\" do\n      read_data { |data| expect(data).to be_valid }\n    end\n  end\n", 1)
	cases := []struct {
		name string
		edit func(*testvalidity.RSpecInclusion)
		want string
	}{
		{"no include on the line", func(q *testvalidity.RSpecInclusion) { q.IncludeLine = 3 }, testvalidity.InclusionNotFound},
		{"include in a hook", func(q *testvalidity.RSpecInclusion) {
			q.SpecText = strings.Replace(widgetSpec, "    it_behaves_like 'a widget'\n", "    before do\n      include_examples 'a widget'\n    end\n", 1)
			q.IncludeLine = 6
		}, testvalidity.InclusionInExampleOrHook},
		{"loop-generated include with a block", func(q *testvalidity.RSpecInclusion) {
			q.SpecText = strings.Replace(widgetSpec, "    it_behaves_like 'a widget'\n", "    [1, 2].each do |n|\n      it_behaves_like 'a widget' do\n        let(:size) { n }\n      end\n    end\n", 1)
			q.IncludeLine = 6
		}, testvalidity.InclusionLoopGenerated},
		{"loop-generated include", func(q *testvalidity.RSpecInclusion) {
			q.SpecText = strings.Replace(widgetSpec, "    it_behaves_like 'a widget'\n", "    [1, 2].each do |n|\n      it_behaves_like 'a widget'\n    end\n", 1)
			q.IncludeLine = 6
		}, testvalidity.InclusionLoopGenerated},
		{"non-literal name", func(q *testvalidity.RSpecInclusion) {
			q.SpecText = strings.Replace(widgetSpec, "it_behaves_like 'a widget'", "it_behaves_like WIDGET_EXAMPLES", 1)
		}, testvalidity.InclusionNameNotLiteral},
		{"no definition", func(q *testvalidity.RSpecInclusion) {
			q.SpecText = strings.Replace(widgetSpec, "it_behaves_like 'a widget'", "it_behaves_like 'a gadget'", 1)
		}, testvalidity.SharedGroupNotFound},
		{"two definitions", func(q *testvalidity.RSpecInclusion) {
			q.Support.Files["spec/support/other.rb"] = widgetExamples
		}, testvalidity.SharedGroupAmbiguous},
		{"defined inside a shared group", func(q *testvalidity.RSpecInclusion) {
			q.SharedText, q.ExampleLine = nested, 3
			q.Support.Files[widgetShared] = nested
		}, testvalidity.SharedGroupNested},
		{"shared group includes another", func(q *testvalidity.RSpecInclusion) {
			q.SharedText = including
			q.Support.Files[widgetShared] = including
		}, testvalidity.SharedGroupNested},
		{"support not bound", func(q *testvalidity.RSpecInclusion) {
			q.Support = testvalidity.RubySupport{Unbound: "rspec-helpers-not-bound"}
		}, testvalidity.SharedGroupUnbound},
		{"line is not an example", func(q *testvalidity.RSpecInclusion) { q.ExampleLine = 3 }, testvalidity.InclusionExampleNotInGroup},
		{"loop-generated example", func(q *testvalidity.RSpecInclusion) {
			q.SharedText, q.ExampleLine = looped, 3
			q.Support.Files[widgetShared] = looped
		}, testvalidity.InclusionExampleLoop},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source, ref := inclusion(t, c.edit)
			if source.Read != c.want {
				t.Fatalf("read = %q, want %q", source.Read, c.want)
			}
			if p := project(source, ref); p.Hygiene.State != testvalidity.HygieneAbstained || p.Hygiene.Reason != c.want {
				t.Errorf("hygiene = %+v, want ABSTAINED %s", p.Hygiene, c.want)
			}
		})
		seen[c.want] = true
	}
	if len(seen) != 10 {
		t.Errorf("%d distinct refusal codes, want 10", len(seen))
	}
}

// TestRSpecInclusionReceiptJoin pins TSA-V0-092: a receipt's scoped id walks to the inclusion it ran
// through, nested by it_behaves_like or inline by include_examples, and a mismatched id, a
// mismatched description or a missing one abstains.
func TestRSpecInclusionReceiptJoin(t *testing.T) {
	cases := []struct {
		id, description string
		example         int
		wantRef, want   string
	}{
		{"./spec/widget_spec.rb[1:1:1:1]", "Widget stub reader behaves like a widget reads the data", 2, "spec/widget_spec.rb:5>spec/support/widget_examples.rb:2", testvalidity.ExampleSourceLocated},
		{"./spec/widget_spec.rb[1:1:1:2]", "Widget stub reader behaves like a widget has a name", 5, "spec/widget_spec.rb:5>spec/support/widget_examples.rb:5", testvalidity.ExampleSourceLocated},
		{"./spec/widget_spec.rb[1:2:1]", "Widget yielding reader reads the data", 2, "spec/widget_spec.rb:11>spec/support/widget_examples.rb:2", testvalidity.ExampleSourceLocated},
		{"./spec/widget_spec.rb[1:2:2]", "Widget yielding reader has a name", 5, "spec/widget_spec.rb:11>spec/support/widget_examples.rb:5", testvalidity.ExampleSourceLocated},
		{"./spec/widget_spec.rb[1:1:1:2]", "", 2, "", testvalidity.InclusionIDUnplaced},
		{"./spec/widget_spec.rb[1:3]", "", 2, "", testvalidity.InclusionIDUnplaced},
		{"./spec/widget_spec.rb[2:1]", "", 2, "", testvalidity.InclusionIDUnplaced},
		{"./spec/widget_spec.rb[1:1:1:1]", "Widget stub reader reads the data", 2, "", testvalidity.InclusionDescriptionMismatch},
		{"./spec/widget_spec.rb[1:2:2]", "", 5, "", testvalidity.InclusionDescriptionMismatch},
	}
	for _, c := range cases {
		t.Run(c.id+" "+c.description, func(t *testing.T) {
			path, ids, ok := testvalidity.ParseRSpecScopedID(c.id)
			if !ok || path != "spec/widget_spec.rb" {
				t.Fatalf("testvalidity.ParseRSpecScopedID(%q) = %q %v %v", c.id, path, ids, ok)
			}
			source, ref := inclusion(t, func(q *testvalidity.RSpecInclusion) {
				q.IncludeLine, q.ScopedID, q.Description, q.ExampleLine = 0, ids, c.description, c.example
			})
			if source.Read != c.want || c.wantRef != "" && ref != c.wantRef {
				t.Fatalf("read %q ref %q, want %q %q", source.Read, ref, c.want, c.wantRef)
			}
		})
	}
	for _, bad := range []string{"spec/widget_spec.rb:5", "./spec/x_spec.rb[]", "./spec/x_spec.rb[0:1]", "./spec/x_spec.rb[1:a]"} {
		if _, _, ok := testvalidity.ParseRSpecScopedID(bad); ok {
			t.Errorf("testvalidity.ParseRSpecScopedID(%q) accepted", bad)
		}
	}
}

// TestRSpecInclusionCustomizationOverride pins TSA-V0-088 and TSA-V0-089 for a helper the shared
// body defines and the customization block redefines, directly or through an included context: the
// two definitions sit in different files with no line order to compare, so the read abstains
// instead of judging either, whether the shared def never yields or never calls its block.
func TestRSpecInclusionCustomizationOverride(t *testing.T) {
	direct := "RSpec.describe Widget do\n  it_behaves_like 'a widget' do\n    def read_data\n      yield build_widget\n    end\n  end\nend\n"
	included := "RSpec.describe Widget do\n  it_behaves_like 'a widget' do\n    include_context 'yielding reader'\n  end\nend\n"
	reader := "RSpec.shared_context 'yielding reader' do\n  def read_data\n    yield build_widget\n  end\nend\n"
	cases := []struct{ name, spec, def string }{
		{"def override, shared def never yields", direct, "  def read_data; build_widget; end\n"},
		{"context override, shared def never calls its block", included, "  def read_data(&block)\n    build_widget\n  end\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The shared def follows the examples, on a later line than the customization block's.
			shared := strings.Replace(widgetExamples, "  end\nend\n", "  end\n"+c.def+"end\n", 1)
			source, ref := inclusion(t, func(q *testvalidity.RSpecInclusion) {
				q.SpecText, q.IncludeLine, q.SharedText = c.spec, 2, shared
				q.Support.Files[widgetShared] = shared
				q.Support.Files["spec/support/yielding_reader.rb"] = reader
			})
			if source.Read != testvalidity.ExampleSourceLocated {
				t.Fatalf("read = %q, want located", source.Read)
			}
			if p := project(source, ref); p.Hygiene.State != testvalidity.HygieneAbstained || p.Hygiene.Reason != testvalidity.RuleRBHelperUnresolved {
				t.Errorf("hygiene = %+v (findings %v), want ABSTAINED %s", p.Hygiene, sourceRules(source), testvalidity.RuleRBHelperUnresolved)
			}
		})
	}
}
