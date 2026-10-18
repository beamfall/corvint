package testsource

import (
	"fmt"
	"reflect"
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
// RSPEC-SKIPPED and RSPEC-FOCUSED findings naming the triggering line; `skip: false`, a
// conditional skip and the words inside a title are not (TSA-V0-062, TSA-V0-063, TSA-V0-064).
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
		19: {"expectations=1"},
		23: {"expectations=1"},
		26: {"expectations=1", focused(26, 26)},
		29: {"expectations=1", focused(29, 29)},
		33: {"expectations=1", skipped(33, 32)},
		38: {"expectations=1", focused(38, 37)},
		42: {"expectations=1", skipped(42, 42)},
		45: {"expectations=1"},
	})
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
