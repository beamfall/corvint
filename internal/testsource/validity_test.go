package testsource

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// brief renders findings as "rule test cause" lines for comparison.
func brief(r ValidityReport) []string {
	out := []string{}
	for _, f := range r.Findings {
		out = append(out, fmt.Sprintf("%s %s %s %s %s", f.Rule, f.Severity, f.Location, f.Cause, f.Title))
	}
	return out
}

func wantFindings(t *testing.T, name string, r ValidityReport, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := brief(r); !reflect.DeepEqual(got, want) {
		t.Errorf("%s: findings\n%q\nwant\n%q", name, got, want)
	}
}

// TestIsolatedInSerialForms: an isolated test under a configure({mode:'serial'}) describe, under
// test.describe.serial, and under a file-level configure is reported advisory, naming the outermost
// serial ancestor's file:line (TSA-V0-045, TSA-V0-047).
func TestIsolatedInSerialForms(t *testing.T) {
	configure := `test.describe('alpha', () => {
  test.describe.configure({ mode: 'serial' });
  test('alpha-01 existing', async () => { expect(1).toBe(1); });
  test.describe('beta', () => {
    test('beta-01 new row', { annotation: [{ type: 'isolated', description: 'own seed' }] }, async () => {
      expect(2).toBe(2);
    });
  });
});
`
	r := ValidityFindings("e2e/alpha.spec.ts", configure)
	wantFindings(t, "configure", r, "ISOLATED-IN-SERIAL advisory e2e/alpha.spec.ts:5 e2e/alpha.spec.ts:1 beta-01 new row")
	if r.Uncertain || !strings.Contains(r.Findings[0].Message, "move the test out of the serial suite") {
		t.Errorf("configure: %+v", r)
	}

	serial := `import { test, expect } from '@playwright/test';
test.describe('gamma', () => {
  test.describe.serial('delta', () => {
    test('delta-01', async () => { expect(1).toBe(1); });
    test('delta-02 @isolated', async () => { expect(1).toBe(1); });
    test('delta-03', { tag: ['@fast', '@isolated'] }, async () => { expect(1).toBe(1); });
    test('delta-04', { annotation: { type: 'isolated' } }, async () => { expect(1).toBe(1); });
  });
});
`
	wantFindings(t, "describe.serial", ValidityFindings("e2e/gamma.spec.ts", serial),
		"ISOLATED-IN-SERIAL advisory e2e/gamma.spec.ts:5 e2e/gamma.spec.ts:3 delta-02 @isolated",
		"ISOLATED-IN-SERIAL advisory e2e/gamma.spec.ts:6 e2e/gamma.spec.ts:3 delta-03",
		"ISOLATED-IN-SERIAL advisory e2e/gamma.spec.ts:7 e2e/gamma.spec.ts:3 delta-04")

	fileLevel := `test.describe.configure({ mode: 'serial' });
test.describe.serial('epsilon', () => {
  test.describe('zeta', { annotation: { type: 'isolated' } }, () => {
    test('zeta-01', async () => { expect(1).toBe(1); });
  });
});
`
	wantFindings(t, "file-level, describe annotation", ValidityFindings("e2e/epsilon.test.js", fileLevel),
		"ISOLATED-IN-SERIAL advisory e2e/epsilon.test.js:4 e2e/epsilon.test.js:1 zeta-01")
}

// TestIsolatedInSerialNegatives: a test without an isolation annotation under a serial suite, and
// an isolated test outside any serial suite, are not reported; a mode that is not a literal makes
// the file uncertain rather than reported (TSA-V0-046).
func TestIsolatedInSerialNegatives(t *testing.T) {
	cases := map[string]string{
		"not isolated under serial": `test.describe('alpha', () => {
  test.describe.configure({ mode: 'serial' });
  test('alpha-01', async () => { expect(1).toBe(1); });
  test('alpha-02', { annotation: { type: 'issue', description: 'isolated' } }, async () => { expect(1).toBe(1); });
});
`,
		"isolated, not serial": `test.describe('beta', () => {
  test.describe.configure({ mode: 'parallel' });
  test('beta-01', { annotation: [{ type: 'isolated' }] }, async () => { expect(1).toBe(1); });
});
test.describe('gamma', () => {
  test('gamma-01 @isolated', async () => { expect(1).toBe(1); });
});
`,
		"serial sibling": `test.describe('delta', () => {
  test.describe.configure({ mode: 'serial' });
  test('delta-01', async () => { expect(1).toBe(1); });
});
test.describe('epsilon', () => {
  test('epsilon-01 @isolated', async () => { expect(1).toBe(1); });
});
`,
	}
	for name, text := range cases {
		r := ValidityFindings("e2e/n.spec.ts", text)
		wantFindings(t, name, r)
		if r.Uncertain {
			t.Errorf("%s: uncertain %q", name, r.Reason)
		}
	}
	computed := `const MODE = 'serial';
test.describe('alpha', () => {
  test.describe.configure({ mode: MODE });
  test('alpha-01 @isolated', async () => { expect(1).toBe(1); });
});
`
	r := ValidityFindings("e2e/c.spec.ts", computed)
	wantFindings(t, "computed mode", r)
	if !r.Uncertain || r.Reason != validitySerialNotLiteral {
		t.Errorf("computed mode: %+v", r)
	}
	if r := ValidityFindings("e2e/u.spec.ts", "test.describe('a', () => { test('b @isolated', async () => {\n"); !r.Uncertain || r.Reason != validitySourceUnparsed {
		t.Errorf("unbalanced: %+v", r)
	}
}

const memoizedSpec = `require 'rails_helper'

RSpec.describe WidgetSync do
  subject { described_class.new(widget).call }

  let(:widget) { create(:widget) }
  let(:ledger) { build(:ledger) }

  describe 'subject memoized by before' do
    before { subject }

    it 'creates an entry' do
      expect { subject }.to change(Entry, :count).by(1)
    end
  end

  context 'let memoized by an outer before' do
    before(:each) do
      ledger.save!
    end

    context 'inner' do
      it 'records the ledger' do
        expect { ledger }.not_to raise_error
      end
    end
  end

  context 'named subject' do
    subject(:runner) { described_class.new(widget) }

    let!(:prepared) { runner.prepare }

    it { expect { runner }.to change { Entry.count } }
  end

  context 'let! invokes itself' do
    let!(:audit) { create(:audit) }

    it 'logs' do
      expect do
        audit
      end.to change(Log, :count)
    end
  end

  context 'sibling before' do
    before { ledger.save! }

    it('runs its own setup') { expect(ledger).to be_persisted }
  end

  context 'no memoizing ancestor' do
    it 'performs' do
      expect { described_class.new.perform }.to change(Entry, :count)
      expect { ledger }.to change(Entry, :count)
    end
  end

  context 'before all is not memoizing' do
    before(:all) { ledger }
    before { widget = build(:widget); widget.valid? }

    it 'checks' do
      expect { ledger }.to change(Entry, :count)
      expect { widget }.to change(Entry, :count)
    end
  end
end
`

// TestMemoizedBlockExpectationFindings: `expect { NAME }` for a subject, a named subject, a let and
// a let! already invoked by a before or let! of the same or an outer group is an invalid finding
// naming that hook's file:line (TSA-V0-048, TSA-V0-049).
func TestMemoizedBlockExpectationFindings(t *testing.T) {
	r := ValidityFindings("spec/services/widget_sync_spec.rb", memoizedSpec)
	if r.Uncertain {
		t.Fatalf("uncertain: %q", r.Reason)
	}
	p := "spec/services/widget_sync_spec.rb"
	wantFindings(t, "memoized", r,
		"MEMOIZED-BLOCK-EXPECTATION invalid "+p+":13 "+p+":10 creates an entry",
		"MEMOIZED-BLOCK-EXPECTATION invalid "+p+":24 "+p+":18 records the ledger",
		"MEMOIZED-BLOCK-EXPECTATION invalid "+p+":34 "+p+":32 ",
		"MEMOIZED-BLOCK-EXPECTATION invalid "+p+":41 "+p+":38 logs")
	f := r.Findings[0]
	if f.Test != p+":12" || !strings.Contains(f.Message, "before at "+p+":10") {
		t.Errorf("first finding %+v", f)
	}
	if !strings.Contains(r.Findings[2].Message, "let! at "+p+":32") {
		t.Errorf("named subject finding %+v", r.Findings[2])
	}
}

// TestMemoizedBlockExpectationNegatives: a before in a sibling group, `expect { described_class.new.perform }`,
// a name no hook on the chain invokes, a before(:all) hook and a local that shadows a let are not
// reported (TSA-V0-049); the fixture's contexts after line 45 hold those cases.
func TestMemoizedBlockExpectationNegatives(t *testing.T) {
	for _, f := range ValidityFindings("spec/w_spec.rb", memoizedSpec).Findings {
		var line int
		fmt.Sscanf(f.Location[strings.LastIndex(f.Location, ":")+1:], "%d", &line)
		if line > 45 {
			t.Errorf("negative case reported: %+v", f)
		}
	}
	sibling := `RSpec.describe Widget do
  let(:widget) { create(:widget) }
  context 'a' do
    before { widget }
    it('a') { expect(widget).to be_valid }
  end
  context 'b' do
    it 'b' do
      expect { widget }.to change(Widget, :count).by(1)
    end
  end
end
`
	r := ValidityFindings("spec/models/widget_spec.rb", sibling)
	wantFindings(t, "sibling", r)
	if r.Uncertain {
		t.Errorf("sibling: uncertain %q", r.Reason)
	}
	if r := ValidityFindings("spec/models/widget_spec.rb", strings.Replace(sibling, "  context 'a' do\n    before { widget }", "  before { widget }\n  context 'a' do", 1)); len(r.Findings) != 1 || r.Findings[0].Cause != "spec/models/widget_spec.rb:3" {
		t.Errorf("hoisted before: %+v", r)
	}
}

// TestMemoizedBlockExpectationAbstains: unbalanced blocks, nesting beyond the bound and an
// oversized file make the report uncertain with no finding (TSA-V0-050).
func TestMemoizedBlockExpectationAbstains(t *testing.T) {
	unbalanced := "RSpec.describe X do\n  before { subject }\n  it 'x' do\n    expect { subject }.to change(Y, :count)\n"
	if r := ValidityFindings("spec/x_spec.rb", unbalanced); !r.Uncertain || r.Reason != validityRSpecUnbalanced || len(r.Findings) != 0 {
		t.Errorf("unbalanced: %+v", r)
	}
	if r := ValidityFindings("spec/x_spec.rb", "end\n"); !r.Uncertain || r.Reason != validityRSpecUnbalanced {
		t.Errorf("stray end: %+v", r)
	}
	deep := strings.Repeat("context 'c' do\n", maxRSpecDepth+1) + strings.Repeat("end\n", maxRSpecDepth+1)
	if r := ValidityFindings("spec/x_spec.rb", deep); !r.Uncertain || r.Reason != validityRSpecTooDeep {
		t.Errorf("deep: %+v", r)
	}
	if r := ValidityFindings("spec/x_spec.rb", strings.Repeat("#", MaxFileBytes+1)); !r.Uncertain || r.Reason != validitySourceTooLarge {
		t.Errorf("large: %+v", r)
	}
}

// TestValidityFindingsDispatchAndVerdict: only JS/TS test files and `_spec.rb` files are read, the
// report is deterministic, and an RSpec finding applied to an eligible projection makes it
// INELIGIBLE while an ISOLATED-IN-SERIAL finding leaves it unchanged (TSA-V0-047, TSA-V0-051,
// TSA-V0-052).
func TestValidityFindingsDispatchAndVerdict(t *testing.T) {
	for _, name := range []string{"spec/support/helpers.rb", "e2e/helpers.ts", "README.md"} {
		if r := ValidityFindings(name, memoizedSpec); len(r.Findings) != 0 || r.Uncertain {
			t.Errorf("%s: %+v", name, r)
		}
	}
	a := ValidityFindings("spec/w_spec.rb", memoizedSpec)
	b := ValidityFindings("spec/w_spec.rb", memoizedSpec)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("report is not deterministic")
	}
	eligible := testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible, Anchors: []string{}}}
	got := testvalidity.ApplySourceFindings(eligible, a.Findings[:1])
	if got.Hygiene.State != testvalidity.HygieneIneligible || got.Hygiene.Reason != testvalidity.HygieneReasonMemoizedBlockExpectation ||
		!reflect.DeepEqual(got.Hygiene.Anchors, []string{"spec/w_spec.rb:13", "spec/w_spec.rb:10"}) {
		t.Errorf("verdict %+v", got.Hygiene)
	}
	serial := ValidityFindings("e2e/s.spec.ts", "test.describe.serial('s', () => { test('t @isolated', async () => {}); });\n")
	if len(serial.Findings) != 1 || !reflect.DeepEqual(testvalidity.ApplySourceFindings(eligible, serial.Findings), eligible) {
		t.Errorf("advisory changed the verdict: %+v", serial)
	}
}

// TestIsolatedInSerialTagAndQuotedKey: an `@isolated` string that is an annotation's description,
// not a tag, does not make a test isolated, and a quoted `'mode'` key still makes its describe
// serial (TSA-V0-045, TSA-V0-046).
func TestIsolatedInSerialTagAndQuotedKey(t *testing.T) {
	described := `test.describe.serial('alpha', () => {
  test('alpha-01', { annotation: { type: 'issue', description: '@isolated' } }, async () => { expect(1).toBe(1); });
});
`
	wantFindings(t, "description text", ValidityFindings("e2e/d.spec.ts", described))
	quoted := `test.describe('beta', () => {
  test.describe.configure({ 'mode': 'serial' });
  test('beta-01', { tag: '@isolated' }, async () => { expect(1).toBe(1); });
});
`
	r := ValidityFindings("e2e/q.spec.ts", quoted)
	wantFindings(t, "quoted key", r, "ISOLATED-IN-SERIAL advisory e2e/q.spec.ts:3 e2e/q.spec.ts:1 beta-01")
	if r.Uncertain {
		t.Errorf("quoted key: uncertain %q", r.Reason)
	}
}

// TestMemoizedBlockExpectationOverrides: distinct named subjects are distinct helpers, and a let!
// overridden by a nested let no longer runs its body, so neither is reported; without the
// override the let! body's subject call is reported (TSA-V0-049).
func TestMemoizedBlockExpectationOverrides(t *testing.T) {
	named := `RSpec.describe Widget do
  subject(:original) { described_class.new }
  before { original }
  context 'replaced' do
    subject(:replacement) { described_class.create }
    it 'creates' do
      expect { replacement }.to change(Widget, :count).by(1)
      expect { subject }.to change(Widget, :count).by(1)
    end
  end
end
`
	wantFindings(t, "named subjects", ValidityFindings("spec/n_spec.rb", named))
	eager := `RSpec.describe Widget do
  subject { described_class.create }
  let!(:setup) { subject }
  context 'lazy' do
    let(:setup) { nil }
    it 'creates' do
      expect { subject }.to change(Widget, :count).by(1)
    end
  end
end
`
	wantFindings(t, "overridden let!", ValidityFindings("spec/e_spec.rb", eager))
	kept := strings.Replace(eager, "    let(:setup) { nil }\n", "    let(:other) { nil }\n", 1)
	wantFindings(t, "kept let!", ValidityFindings("spec/e_spec.rb", kept),
		"MEMOIZED-BLOCK-EXPECTATION invalid spec/e_spec.rb:7 spec/e_spec.rb:3 creates")
}

// TestMemoizedBlockExpectationPercentStrings: a name inside a percent literal is not an
// invocation (TSA-V0-048).
func TestMemoizedBlockExpectationPercentStrings(t *testing.T) {
	spec := `RSpec.describe Widget do
  subject { described_class.create }
  before do
    log(%q(subject), %Q[subject], %w{subject other}, %i<subject>, %(subject))
  end
  it 'creates' do
    expect { subject }.to change(Widget, :count).by(1)
  end
end
`
	r := ValidityFindings("spec/p_spec.rb", spec)
	wantFindings(t, "percent strings", r)
	if r.Uncertain {
		t.Errorf("percent strings: uncertain %q", r.Reason)
	}
}

// TestMemoizedBlockExpectationDepthInsideExample: nesting beyond the bound inside an example body
// makes the report uncertain with no finding (TSA-V0-050).
func TestMemoizedBlockExpectationDepthInsideExample(t *testing.T) {
	spec := "RSpec.describe Widget do\n  subject { described_class.create }\n  before { subject }\n  it 'creates' do\n" +
		"    expect { subject }.to change(Widget, :count).by(1)\n" +
		strings.Repeat("    1.times do\n", maxRSpecDepth+1) + strings.Repeat("    end\n", maxRSpecDepth+1) +
		"  end\nend\n"
	if r := ValidityFindings("spec/d_spec.rb", spec); !r.Uncertain || r.Reason != validityRSpecTooDeep || len(r.Findings) != 0 {
		t.Errorf("deep example: %+v", r)
	}
}
