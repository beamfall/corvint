package testsource

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// rbBrief renders the RB-* findings of each located example as "RULE severity location cause"
// rows, keyed by declaring line; an example with none maps to an empty list.
func rbBrief(t *testing.T, name, text string, support testvalidity.RubySupport) map[int][]string {
	t.Helper()
	examples, reason := RSpecExampleSourcesWithSupport(name, text, support)
	if reason != "" {
		t.Fatalf("%s: uncertain %q", name, reason)
	}
	out := map[int][]string{}
	for line, e := range examples {
		rows := []string{}
		for _, f := range e.Findings {
			if strings.HasPrefix(f.Rule, "RB-") {
				rows = append(rows, fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
			}
		}
		out[line] = rows
	}
	return out
}

func bound(files map[string]string) testvalidity.RubySupport {
	return testvalidity.RubySupport{Files: files}
}

const neverYieldContext = `RSpec.shared_context 'report io' do
  def read_data; build_report_io; end
  def read_yielding
    yield build_report_io
  end
  def read_calling(&block)
    block.call(build_report_io)
  end
end
`

const neverYieldSpec = `require 'rails_helper'
RSpec.describe Report do
  include_context 'report io'
  it 'reads the header' do
    read_data do |data|
      expect(data).to include('header')
    end
  end
  it 'yields the header' do
    read_yielding { |data| expect(data).to include('header') }
  end
  it 'calls the block' do
    read_calling do |data|
      expect(data).to include('header')
    end
  end
  context 'with a local override' do
    def read_data
      yield build_report_io
    end
    it 'reads through the override' do
      read_data { |data| expect(data).to be_present }
    end
  end
end
`

// TestRSpecBlockNeverYieldedAcrossFiles pins TSA-V0-084 and TSA-V0-088: a helper from an included
// shared context in a support file that takes no block and never yields makes the example
// INELIGIBLE; a yielding, block.call or locally overriding helper stays clean.
func TestRSpecBlockNeverYieldedAcrossFiles(t *testing.T) {
	got := rbBrief(t, "spec/report_spec.rb", neverYieldSpec, bound(map[string]string{"spec/support/report_io.rb": neverYieldContext}))
	wantExamples(t, got, map[int][]string{
		4:  {"RB-BLOCK-NEVER-YIELDED invalid spec/report_spec.rb:5 spec/support/report_io.rb:2"},
		9:  {},
		12: {},
		21: {},
	})
	examples, _ := RSpecExampleSourcesWithSupport("spec/report_spec.rb", neverYieldSpec, bound(map[string]string{"spec/support/report_io.rb": neverYieldContext}))
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible}, Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, examples[4], "spec/report_spec.rb:4")
	if p.Hygiene.State != testvalidity.HygieneIneligible || p.Hygiene.Reason != testvalidity.RuleRBBlockNeverYielded {
		t.Errorf("hygiene = %+v, want INELIGIBLE %s", p.Hygiene, testvalidity.RuleRBBlockNeverYielded)
	}
}

// TestRSpecBlockNeverYieldedSameFile pins TSA-V0-084 for a one-line def in the spec file itself.
func TestRSpecBlockNeverYieldedSameFile(t *testing.T) {
	spec := `RSpec.describe Report do
  def read_data; build_report_io; end
  it 'reads' do
    read_data { |data| expect(data).to be_present }
  end
end
`
	got := rbBrief(t, "spec/report_spec.rb", spec, bound(nil))
	wantExamples(t, got, map[int][]string{3: {"RB-BLOCK-NEVER-YIELDED invalid spec/report_spec.rb:4 spec/report_spec.rb:2"}})
}

// TestRSpecHelperAbstentions pins TSA-V0-088: a shared context the reader cannot bind, one with
// two definitions, a non-literal include, and a support module def the reader does not follow
// each abstain with a named reason instead of flagging, and the projection becomes ABSTAINED.
func TestRSpecHelperAbstentions(t *testing.T) {
	spec := `RSpec.describe Report do
  include_context 'report io'
  it 'reads' do
    read_data { |data| expect(data).to be_present }
  end
end
`
	module := "module ReportHelpers\n  def read_data; build_report_io; end\nend\n"
	cases := []struct {
		name    string
		spec    string
		support testvalidity.RubySupport
		want    string
	}{
		{"unbound support", spec, testvalidity.RubySupport{Unbound: "rspec-helpers-not-bound"}, "RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:4 spec/report_spec.rb:2"},
		{"context not found", spec, bound(map[string]string{"spec/support/m.rb": module}), "RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:4 spec/report_spec.rb:2"},
		{"two definitions", spec, bound(map[string]string{"spec/support/a.rb": neverYieldContext, "spec/support/b.rb": neverYieldContext}), "RB-HELPER-AMBIGUOUS abstained spec/report_spec.rb:4 spec/report_spec.rb:2"},
		{"non-literal include", strings.Replace(spec, "'report io'", "CONTEXT_NAME", 1), bound(map[string]string{"spec/support/report_io.rb": neverYieldContext}), "RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:4 spec/report_spec.rb:2"},
		{"module def", strings.Replace(spec, "  include_context 'report io'\n", "", 1), bound(map[string]string{"spec/support/m.rb": module}), "RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:3 spec/support/m.rb:2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rbBrief(t, "spec/report_spec.rb", c.spec, c.support)
			var rows []string
			for _, r := range got {
				rows = append(rows, r...)
			}
			if len(rows) != 1 || rows[0] != c.want {
				t.Fatalf("rows = %q, want [%q]", rows, c.want)
			}
			examples, _ := RSpecExampleSourcesWithSupport("spec/report_spec.rb", c.spec, c.support)
			for line, e := range examples {
				p := testvalidity.ApplyExampleSource(testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, e, fmt.Sprintf("spec/report_spec.rb:%d", line))
				if p.Hygiene.State != testvalidity.HygieneAbstained || !strings.HasPrefix(p.Hygiene.Reason, "RB-HELPER-") {
					t.Errorf("hygiene = %+v, want ABSTAINED RB-HELPER-*", p.Hygiene)
				}
			}
		})
	}
}

// TestRSpecChangeOnIdentity pins TSA-V0-085: a negated change over a reloaded record is advisory,
// since a reload returning a mutable object can still change its hash, and leaves the example
// ELIGIBLE; change { record.reload.attr } and a positive change stay clean.
func TestRSpecChangeOnIdentity(t *testing.T) {
	spec := `RSpec.describe Widget do
  it 'does not touch the record' do
    expect { described_class.call(record) }.not_to change { record.reload }
  end
  it 'does not touch the name' do
    expect { described_class.call(record) }.not_to change { record.reload.name }
  end
  it 'does not touch the ivar' do
    expect { run }.to_not change { @widget.reload }
  end
end
`
	got := rbBrief(t, "spec/widget_spec.rb", spec, bound(nil))
	wantExamples(t, got, map[int][]string{
		2: {"RB-CHANGE-ON-IDENTITY advisory spec/widget_spec.rb:3 spec/widget_spec.rb:2"},
		5: {},
		8: {"RB-CHANGE-ON-IDENTITY advisory spec/widget_spec.rb:9 spec/widget_spec.rb:8"},
	})
	examples, _ := RSpecExampleSourcesWithSupport("spec/widget_spec.rb", spec, bound(nil))
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, examples[2], "spec/widget_spec.rb:2")
	if p.Hygiene.State != testvalidity.HygieneEligible {
		t.Errorf("hygiene = %+v, want ELIGIBLE", p.Hygiene)
	}
}

// TestRSpecAssignmentInPredicate pins TSA-V0-086: a satisfy block ending in an assignment of a
// truthy literal is INELIGIBLE; a comparison, or an assignment of an expression that can be false
// (`valid = x.even?`, `x.date = Date.today`), in brace or do form, stays clean.
func TestRSpecAssignmentInPredicate(t *testing.T) {
	spec := `RSpec.describe Widget do
  it 'assigns' do
    expect(row).to satisfy { |x| x["date"] = "today" }
  end
  it 'compares' do
    expect(row).to satisfy { |x| x["date"] == Date.today }
  end
  it 'assigns in a do block' do
    expect(row).to satisfy do |x|
      x.date ||= :today
    end
  end
  it 'compares in a do block' do
    expect(row).to satisfy do |x|
      x.date <= Date.today
    end
  end
  it 'assigns a predicate' do
    expect(value).to satisfy { |x| valid = x.even? }
  end
  it 'assigns an expression' do
    expect(row).to satisfy do |x|
      x.date = Date.today
    end
  end
end
`
	got := rbBrief(t, "spec/widget_spec.rb", spec, bound(nil))
	wantExamples(t, got, map[int][]string{
		2:  {"RB-ASSIGNMENT-IN-PREDICATE invalid spec/widget_spec.rb:3 spec/widget_spec.rb:2"},
		5:  {},
		8:  {"RB-ASSIGNMENT-IN-PREDICATE invalid spec/widget_spec.rb:9 spec/widget_spec.rb:8"},
		13: {},
		18: {},
		21: {},
	})
}

const reportSpec = `RSpec.describe Report do
  include_context 'report io'
  it 'reads' do
    read_data { |data| expect(data).to be_present }
  end
end
`

// TestRSpecResolutionAbstains pins TSA-V0-088 against definitions the reader cannot order or see:
// an incomplete support set, a nested include in the shared context, and a helper no read file
// defines while the support set is unbound abstain; the later of two defs in one context wins.
func TestRSpecResolutionAbstains(t *testing.T) {
	yielding := "RSpec.shared_context 'report io' do\n  def read_data; build_report_io; end\n  def read_data\n    yield build_report_io\n  end\nend\n"
	outer := "RSpec.shared_context 'inner' do\n  def read_data\n    yield build_report_io\n  end\nend\nRSpec.shared_context 'report io' do\n  def read_data; build_report_io; end\n  include_context 'inner'\nend\n"
	noInclude := strings.Replace(reportSpec, "  include_context 'report io'\n", "", 1)
	cases := []struct {
		name    string
		spec    string
		support testvalidity.RubySupport
		want    []string
	}{
		{"unread support file", reportSpec, bound(map[string]string{"spec/support/report_io.rb": neverYieldContext, "spec/support/broken.rb": "RSpec.shared_context 'report io' do\n"}), []string{"RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:4 spec/report_spec.rb:2"}},
		{"later def in one context wins", reportSpec, bound(map[string]string{"spec/support/report_io.rb": yielding}), nil},
		{"nested include in the context", reportSpec, bound(map[string]string{"spec/support/report_io.rb": outer}), []string{"RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:4 spec/support/report_io.rb:8"}},
		{"unknown helper, unbound support", noInclude, testvalidity.RubySupport{Unbound: "rspec-helpers-not-bound"}, []string{"RB-HELPER-UNRESOLVED abstained spec/report_spec.rb:3 spec/report_spec.rb:3"}},
		{"unknown helper, bound support", noInclude, bound(nil), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rows []string
			for _, r := range rbBrief(t, "spec/report_spec.rb", c.spec, c.support) {
				rows = append(rows, r...)
			}
			if fmt.Sprint(rows) != fmt.Sprint(c.want) {
				t.Fatalf("rows = %q, want %q", rows, c.want)
			}
		})
	}
}

// TestRSpecHelperForms pins TSA-V0-084 for helper and example shapes: a default argument that
// yields is read as yielding, an endless def is a never-yielding helper that keeps the reader
// balanced, and a one-line example's block call is read.
func TestRSpecHelperForms(t *testing.T) {
	spec := `RSpec.describe Report do
  context 'default argument' do
    def read_data(value = yield); value; end
    it 'reads' do
      read_data { |data| expect(data).to be_present }
    end
  end
  context 'endless def' do
    def read_data = build_report_io
    it 'reads' do
      read_data { |data| expect(data).to be_present }
    end
  end
  context 'one-line example' do
    def read_data; build_report_io; end
    it('reads') { read_data { |data| expect(data).to be_present } }
  end
end
`
	got := rbBrief(t, "spec/report_spec.rb", spec, bound(nil))
	wantExamples(t, got, map[int][]string{
		4:  {},
		10: {"RB-BLOCK-NEVER-YIELDED invalid spec/report_spec.rb:11 spec/report_spec.rb:9"},
		16: {"RB-BLOCK-NEVER-YIELDED invalid spec/report_spec.rb:16 spec/report_spec.rb:15"},
	})
}

// TestRSpecRaiseErrorOnlyIsAdvisory pins TSA-V0-087: an example whose only expectation is
// not_to raise_error gets an advisory finding and stays ELIGIBLE; a title naming errors, or a
// second expectation, gets none.
func TestRSpecRaiseErrorOnlyIsAdvisory(t *testing.T) {
	spec := `RSpec.describe Importer do
  it 'imports the rows' do
    expect { described_class.import(file) }.not_to raise_error
  end
  it 'does not raise on a blank file' do
    expect { described_class.import(blank) }.not_to raise_error
  end
  it 'imports and counts' do
    expect { described_class.import(file) }.not_to raise_error
    expect(Row.count).to eq(2)
  end
end
`
	got := rbBrief(t, "spec/importer_spec.rb", spec, bound(nil))
	wantExamples(t, got, map[int][]string{
		2: {"RB-RAISE-ERROR-ONLY advisory spec/importer_spec.rb:2 spec/importer_spec.rb:2"},
		5: {},
		8: {},
	})
	examples, _ := RSpecExampleSourcesWithSupport("spec/importer_spec.rb", spec, bound(nil))
	p := testvalidity.ApplyExampleSource(testvalidity.Projection{Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured}}, examples[2], "spec/importer_spec.rb:2")
	if p.Hygiene.State != testvalidity.HygieneEligible {
		t.Errorf("hygiene = %+v, want ELIGIBLE", p.Hygiene)
	}
}
