package testsource

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// exampleBrief renders each located example as "line: RULE severity location cause" lines, in
// declaring-line order, plus its expectation count.
func exampleBrief(t *testing.T, name, text string) map[int][]string {
	t.Helper()
	examples, reason := RSpecExampleSources(name, text)
	if reason != "" {
		t.Fatalf("%s: uncertain %q", name, reason)
	}
	out := map[int][]string{}
	for line, e := range examples {
		if e.Read != testvalidity.ExampleSourceLocated {
			t.Fatalf("%s:%d read %q", name, line, e.Read)
		}
		rows := []string{fmt.Sprintf("expectations=%d", e.Expectations)}
		for _, f := range e.Findings {
			rows = append(rows, fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
		}
		out[line] = rows
	}
	return out
}

func wantExamples(t *testing.T, got, want map[int][]string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("examples\n%v\nwant\n%v", got, want)
	}
}

const hygieneSpec = `RSpec.describe Widget do
  it 'saves' do
    expect(subject.save).to be(true)
    expect(subject).to be_valid
  end
  xit 'is x-skipped' do
    expect(1).to eq(1)
  end
  it 'has skip metadata', :skip do
  end
  it 'has pending key', pending: 'later' do
  end
  it 'has skip false', skip: false do
    expect(2).to eq(2)
  end
  it 'skips in the body' do
    skip 'not ready'
  end
  it 'skips conditionally' do
    skip 'windows' if Gem.win_platform?
    expect(3).to eq(3)
  end
  it 'mentions :skip and focus: true in its title' do
    expect('a :skip').to be_a(String)
  end
  fit 'is focused' do
    expect(4).to eq(4)
  end
  it 'has focus metadata', focus: true do
    is_expected.to be_a(Widget)
  end
  xdescribe 'disabled group' do
    it 'inherits the skip' do
      expect(5).to eq(5)
    end
  end
  context 'focused group', :focus do
    it 'inherits the focus' do
      expect(6).to eq(6)
    end
  end
  pending 'is pending' do
    expect(7).to eq(8)
  end
  specify { expect(9).to eq(9) }
end
`

// TestRSpecExampleSkippedAndFocused: x-prefixed, skip and pending declarators, skip/pending/focus
// metadata, an unconditional body skip, and xdescribe or focused ancestor groups are invalid
// RSPEC-SKIPPED and RSPEC-FOCUSED findings naming the triggering line; `skip: false` and the words
// inside a title are not, and a conditional skip abstains RSPEC-CONDITIONAL-SKIP instead
// (TSA-V0-062, TSA-V0-063, TSA-V0-064, TSA-V0-450).
func TestRSpecExampleSkippedAndFocused(t *testing.T) {
	p := "spec/models/widget_spec.rb"
	at := func(n int) string { return fmt.Sprintf("%s:%d", p, n) }
	skipped := func(line, cause int) string { return "RSPEC-SKIPPED invalid " + at(line) + " " + at(cause) }
	focused := func(line, cause int) string { return "RSPEC-FOCUSED invalid " + at(line) + " " + at(cause) }
	wantExamples(t, exampleBrief(t, p, hygieneSpec), map[int][]string{
		2:  {"expectations=2"},
		6:  {"expectations=1", skipped(6, 6)},
		9:  {"expectations=0", skipped(9, 9)},
		11: {"expectations=0", skipped(11, 11)},
		13: {"expectations=1"},
		16: {"expectations=0", skipped(16, 17)},
		19: {"expectations=1", "RSPEC-CONDITIONAL-SKIP abstained " + at(20) + " " + at(19)},
		23: {"expectations=1"},
		26: {"expectations=1", focused(26, 26)},
		29: {"expectations=1", focused(29, 29)},
		33: {"expectations=1", skipped(33, 32)},
		38: {"expectations=1", focused(38, 37)},
		42: {"expectations=1", skipped(42, 42)},
		45: {"expectations=1"},
	})
}

const skipConfigSpec = `RSpec.describe Widget do
  it 'skips under an env flag' do
    skip 'no network' unless ENV['NETWORK']
    expect(1).to eq(1)
  end
  it 'skips inside a block' do
    if ENV['CI']
      pending 'flaky on CI'
    end
    expect(2).to eq(2)
  end
  it 'is slow', :slow do
    expect(3).to eq(3)
  end
  context 'features', type: :feature do
    it 'inherits the excluded type' do
      expect(4).to eq(4)
    end
  end
  it 'is a model', type: :model do
    expect(5).to eq(5)
  end
  it 'is not slow', slow: false do
    expect(6).to eq(6)
  end
  it 'has a skipped body and is slow', :slow do
    skip 'later'
  end
end
`

const skipConfigHelper = `RSpec.configure do |config|
  # config.filter_run_excluding :commented
  config.filter_run_excluding :slow unless ENV['SLOW']
  config.filter_run_excluding type: :feature, broken: false
end
`

// TestRSpecConditionalAndConfigSkips pins TSA-V0-450 (V1-1837): a skip or pending statement with a
// trailing if or unless, or inside a nested block, abstains RSPEC-CONDITIONAL-SKIP; metadata a
// committed support file's filter_run_excluding names abstains RSPEC-CONFIG-EXCLUDED, matched on
// the example or an ancestor group and on the filtered value; an unconditional skip stays
// RSPEC-SKIPPED alone, and without a bound support set nothing is config-excluded.
func TestRSpecConditionalAndConfigSkips(t *testing.T) {
	p := "spec/models/widget_spec.rb"
	at := func(n int) string { return fmt.Sprintf("%s:%d", p, n) }
	conditional := func(line, example int) string {
		return "RSPEC-CONDITIONAL-SKIP abstained " + at(line) + " " + at(example)
	}
	excluded := func(example, cause int) string {
		return fmt.Sprintf("RSPEC-CONFIG-EXCLUDED abstained %s spec/spec_helper.rb:%d", at(example), cause)
	}
	examples, reason := RSpecExampleSourcesWithSupport(p, skipConfigSpec, testvalidity.RubySupport{Files: map[string]string{"spec/spec_helper.rb": skipConfigHelper}})
	if reason != "" {
		t.Fatalf("uncertain %q", reason)
	}
	got := map[int][]string{}
	for line, e := range examples {
		rows := []string{}
		for _, f := range e.Findings {
			rows = append(rows, fmt.Sprintf("%s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause))
		}
		got[line] = rows
	}
	wantExamples(t, got, map[int][]string{
		2:  {conditional(3, 2)},
		6:  {conditional(8, 6)},
		12: {excluded(12, 3)},
		16: {excluded(16, 4)},
		20: {},
		23: {},
		26: {"RSPEC-SKIPPED invalid " + at(26) + " " + at(27)},
	})
	p16 := testvalidity.ApplySourceFindings(testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible}}, examples[16].Findings)
	if p16.Hygiene.State != testvalidity.HygieneAbstained || p16.Hygiene.Reason != testvalidity.RuleRSpecConfigExcluded {
		t.Errorf("config-excluded hygiene = %+v, want ABSTAINED %s", p16.Hygiene, testvalidity.RuleRSpecConfigExcluded)
	}
	for line, rows := range exampleBrief(t, p, skipConfigSpec) {
		for _, row := range rows {
			if strings.HasPrefix(row, "RSPEC-CONFIG-EXCLUDED") {
				t.Errorf("%d: config exclusion without a support set: %s", line, row)
			}
		}
	}
}

// TestRSpecMetadataForms pins TSA-V0-450's metadata reading: a hash-literal argument is metadata,
// not a block; a hash-rocket pair keeps its value; and a value that is not a symbol, true, false,
// nil or a number (a constant, lambda or call) is unknown (""), so the filter matches on its key.
func TestRSpecMetadataForms(t *testing.T) {
	for _, c := range []struct {
		line string
		want map[string]string
	}{
		{"it 'x', {slow: true} do", map[string]string{"slow": "true"}},
		{"it 'x', { :type => :feature, :slow => true } do", map[string]string{"type": ":feature", "slow": "true"}},
		{"it 'x', type: FEATURE_TYPE do", map[string]string{"type": ""}},
		{"it('x', type: :model) { expect(1).to eq(1) }", map[string]string{"type": ":model"}},
		{"it 'x', :slow, retries: 3 do", map[string]string{"slow": "true", "retries": "3"}},
	} {
		if got := rspecMetadataPairs(c.line); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: pairs = %v, want %v", c.line, got, c.want)
		}
	}
	helper := `RSpec.configure do |config|
  config.filter_run_excluding({slow: true})
  config.filter_run_excluding :type => :feature
  config.filter_run_excluding kind: FEATURE_KIND
  config.filter_run_excluding speed: lambda { |v| v > 2 }
  config.filter_run_excluding tier: ->(v) { v == :gold }
end
`
	got := []string{}
	for _, x := range rspecExclusions(map[string]string{"spec/spec_helper.rb": helper}) {
		got = append(got, fmt.Sprintf("%s=%q %s", x.key, x.value, x.cause))
	}
	want := []string{`slow="true" spec/spec_helper.rb:2`, `type=":feature" spec/spec_helper.rb:3`, `kind="" spec/spec_helper.rb:4`,
		`speed="" spec/spec_helper.rb:5`, `tier="" spec/spec_helper.rb:6`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("exclusions = %v, want %v", got, want)
	}
	spec := `RSpec.describe Widget do
  it 'is slow', {slow: true} do
    expect(1).to eq(1)
  end
  it 'is a feature', type: :feature do
    expect(2).to eq(2)
  end
  it 'is a model', type: :model do
    expect(3).to eq(3)
  end
  it 'has a computed kind', kind: :anything do
    expect(4).to eq(4)
  end
end
`
	examples, reason := RSpecExampleSourcesWithSupport("spec/widget_spec.rb", spec, bound(map[string]string{"spec/spec_helper.rb": helper}))
	if reason != "" {
		t.Fatalf("uncertain %q", reason)
	}
	causes := map[int]string{}
	for line, e := range examples {
		for _, f := range e.Findings {
			if f.Rule == testvalidity.RuleRSpecConfigExcluded {
				causes[line] = f.Cause
			}
		}
	}
	if want := map[int]string{2: "spec/spec_helper.rb:2", 5: "spec/spec_helper.rb:3", 11: "spec/spec_helper.rb:4"}; !reflect.DeepEqual(causes, want) {
		t.Errorf("config-excluded causes = %v, want %v", causes, want)
	}
}

// TestRSpecInclusionConfigExcluded pins TSA-V0-450 for a shared example read through its
// inclusion: a committed filter_run_excluding applies to it as to a direct example.
func TestRSpecInclusionConfigExcluded(t *testing.T) {
	spec := `RSpec.shared_examples 'a widget' do
  it 'has a name', :slow do
    expect(name).to eq('w')
  end
end
RSpec.describe Widget do
  it_behaves_like 'a widget'
end
`
	helper := "RSpec.configure do |config|\n  config.filter_run_excluding :slow\nend\n"
	source, _ := RSpecInclusionSource(testvalidity.RSpecInclusion{SpecPath: "spec/widget_spec.rb", SpecText: spec, IncludeLine: 7,
		SharedPath: "spec/widget_spec.rb", SharedText: spec, ExampleLine: 2, Support: bound(map[string]string{"spec/spec_helper.rb": helper})})
	if source.Read != testvalidity.ExampleSourceLocated {
		t.Fatalf("source %+v, want located", source)
	}
	if !slices.ContainsFunc(source.Findings, func(f testvalidity.SourceFinding) bool {
		return f.Rule == testvalidity.RuleRSpecConfigExcluded && f.Cause == "spec/spec_helper.rb:2"
	}) {
		t.Errorf("findings %+v, want %s from spec/spec_helper.rb:2", source.Findings, testvalidity.RuleRSpecConfigExcluded)
	}
}

const sharedSpec = `RSpec.shared_examples 'a persisted record' do
  it 'persists' do
    expect(record).to be_persisted
  end
end

RSpec.describe Order do
  it_behaves_like 'a persisted record'
  shared_context 'with stock' do
    context 'nested' do
      it 'counts' do
        expect(stock).to eq(1)
      end
    end
  end
  it 'totals' do
    expect(Order.new.total).to eq(0)
  end
end
`

// TestRSpecExampleSharedOnly: an example declared inside a shared example group or shared context
// is an invalid SHARED-EXAMPLE-ONLY finding naming the innermost shared group; an example in the
// including group is not (TSA-V0-065).
func TestRSpecExampleSharedOnly(t *testing.T) {
	p := "spec/models/order_spec.rb"
	wantExamples(t, exampleBrief(t, p, sharedSpec), map[int][]string{
		2:  {"expectations=1", "SHARED-EXAMPLE-ONLY invalid " + p + ":2 " + p + ":1"},
		11: {"expectations=1", "SHARED-EXAMPLE-ONLY invalid " + p + ":11 " + p + ":9"},
		16: {"expectations=1"},
	})
}

const stubSpec = `RSpec.describe Invoice do
  it 'sends' do
    expect(described_class).to receive(:deliver)
    described_class.new.send!
  end
  it 'stubs another class' do
    allow(Mailer).to receive(:deliver)
    expect(described_class.new).to be_valid
  end
  it 'only names described_class in a string' do
    expect('allow(described_class).to receive(:x)').to be_a(String)
  end
  context 'stubbed by a hook' do
    before do
      allow(described_class).to receive(:rate).and_return(0)
    end
    context 'deeper' do
      it 'computes' do
        expect(described_class.new.total).to eq(0)
        expect(described_class.new.tax).to eq(0)
      end
    end
  end
  context 'stubbed by let!' do
    let!(:frozen) { allow_any_instance_of(described_class).to receive(:now) }
    it 'reads' do
    end
  end
end
`

// TestRSpecExampleDescribedClassStub: a message stub or spy on described_class in the example body,
// or in a before or let! hook on its group chain, is an advisory DESCRIBED-CLASS-STUBBED finding at
// the stub line caused by the example or the hook; a stub on another class or inside a string is
// not; expectations are counted in the example body only (TSA-V0-066, TSA-V0-067).
func TestRSpecExampleDescribedClassStub(t *testing.T) {
	p := "spec/models/invoice_spec.rb"
	stub := func(location, cause int) string {
		return fmt.Sprintf("DESCRIBED-CLASS-STUBBED advisory %s:%d %s:%d", p, location, p, cause)
	}
	wantExamples(t, exampleBrief(t, p, stubSpec), map[int][]string{
		2:  {"expectations=1", stub(3, 2)},
		6:  {"expectations=1"},
		10: {"expectations=1"},
		18: {"expectations=2", stub(15, 14)},
		26: {"expectations=0", stub(25, 25)},
	})
	examples, _ := RSpecExampleSources(p, stubSpec)
	if got := strings.Join(examples[18].ExpectationAnchors, ","); got != p+":19,"+p+":20" {
		t.Errorf("expectation anchors %q", got)
	}
}

// TestRSpecExampleSourcesCarryValidityFindings: an example's facts carry the TSA-V0-048
// MEMOIZED-BLOCK-EXPECTATION findings that name it, before the example rules (TSA-V0-062).
func TestRSpecExampleSourcesCarryValidityFindings(t *testing.T) {
	p := "spec/services/widget_sync_spec.rb"
	examples, reason := RSpecExampleSources(p, memoizedSpec)
	if reason != "" {
		t.Fatalf("uncertain %q", reason)
	}
	e := examples[12]
	if len(e.Findings) != 1 || e.Findings[0].Rule != testvalidity.RuleMemoizedBlockExpectation || e.Findings[0].Test != p+":12" {
		t.Errorf("example 12 findings %+v", e.Findings)
	}
}

// TestRSpecExampleSourcesAbstain: a file that is not a `_spec.rb` file, or that the TSA-V0-048
// reader cannot balance, yields no examples and the reader's reason (TSA-V0-062).
func TestRSpecExampleSourcesAbstain(t *testing.T) {
	if got, reason := RSpecExampleSources("spec/widget.rb", hygieneSpec); len(got) != 0 || reason != "not-an-rspec-file" {
		t.Errorf("non-spec: %d %q", len(got), reason)
	}
	if got, reason := RSpecExampleSources("spec/widget_spec.rb", "RSpec.describe Widget do\n  it 'x' do\n"); len(got) != 0 || reason == "" {
		t.Errorf("unbalanced: %d %q", len(got), reason)
	}
}
