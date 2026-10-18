package testsource

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// issue1140Spec is the GitHub #1140 helper: the assertion's block only references the proc.
const issue1140Spec = `RSpec.describe Notifier do
  def expect_no_email_sent(&block)
    assert_no_enqueued_jobs do
      block          # references the proc; never calls it
    end
  end

  subject { described_class.new(user) }

  it 'does not send when muted' do
    expect_no_email_sent { subject.notify! }
  end

  it 'does not send twice' do
    subject.notify!
    expect_no_email_sent do
      subject.notify!
    end
  end

  it 'sends' do
    expect { subject.notify! }.to have_enqueued_job
  end
end
`

// TestBlockNotCalledIssueExample: the #1140 helper is reported at its assertion with its def as
// the cause, and each example that passes it a block inherits the finding at its call line
// (TSA-V0-053, TSA-V0-054).
func TestBlockNotCalledIssueExample(t *testing.T) {
	p := "spec/models/notifier_spec.rb"
	r := ValidityFindings(p, issue1140Spec)
	if r.Uncertain {
		t.Fatalf("uncertain: %q", r.Reason)
	}
	wantFindings(t, "issue 1140", r,
		"BLOCK-NOT-CALLED invalid "+p+":3 "+p+":2 expect_no_email_sent",
		"BLOCK-NOT-CALLED invalid "+p+":11 "+p+":2 does not send when muted",
		"BLOCK-NOT-CALLED invalid "+p+":16 "+p+":2 does not send twice")
	if f := r.Findings[0]; f.Test != p+":2" || !strings.Contains(f.Message, "assertion at "+p+":3") {
		t.Errorf("helper finding %+v", f)
	}
	if f := r.Findings[1]; f.Test != p+":10" || !strings.Contains(f.Message, "helper at "+p+":2") {
		t.Errorf("caller finding %+v", f)
	}
	// A call that passes no block does not inherit the finding.
	noBlock := ValidityFindings(p, strings.Replace(issue1140Spec, "expect_no_email_sent { subject.notify! }", "expect_no_email_sent", 1))
	wantFindings(t, "no block", noBlock,
		"BLOCK-NOT-CALLED invalid "+p+":3 "+p+":2 expect_no_email_sent",
		"BLOCK-NOT-CALLED invalid "+p+":16 "+p+":2 does not send twice")
}

// TestBlockNotCalledForms: one-line brace, `.itself`, paren-less arguments, a multi-line brace,
// a vacuous branch beside a calling one, a helper that never calls its block, a block_given?
// helper that never yields and a file-level helper are reported; a caller is resolved through
// its own group chain (TSA-V0-053, TSA-V0-054).
func TestBlockNotCalledForms(t *testing.T) {
	spec := `def quietly
  return unless block_given?
  Rails.logger.silence
end

RSpec.describe Mailer do
  def no_change(&action)
    expect { action }.not_to change(Email, :count)
  end

  def same_count(&action)
    assert_no_difference('Email.count') { action.itself }
  end

  def no_jobs(&blk)
    assert_no_enqueued_jobs only: Mailer::Job do
      blk
    end
  end

  def performing(&blk)
    perform_enqueued_jobs {
      blk
    }
  end

  def maybe(strict, &blk)
    if strict
      refute_emails { blk }
    else
      blk.call
    end
  end

  def stubbed(&blk)
    allow(Mailer).to receive(:deliver)
  end

  it 'is quiet' do
    quietly { expect(Email.count).to eq(0) }
  end

  context 'override' do
    def stubbed(&blk)
      blk.call
    end

    it 'calls the good one' do
      stubbed { Mailer.deliver }
    end
  end

  it('stubs') { stubbed(&-> { Mailer.deliver }) }
end
`
	p := "spec/mailer_spec.rb"
	r := ValidityFindings(p, spec)
	if r.Uncertain {
		t.Fatalf("uncertain: %q", r.Reason)
	}
	wantFindings(t, "forms", r,
		"BLOCK-NOT-CALLED invalid "+p+":1 "+p+":1 quietly",
		"BLOCK-NOT-CALLED invalid "+p+":40 "+p+":1 is quiet",
		"BLOCK-NOT-CALLED invalid "+p+":8 "+p+":7 no_change",
		"BLOCK-NOT-CALLED invalid "+p+":12 "+p+":11 same_count",
		"BLOCK-NOT-CALLED invalid "+p+":16 "+p+":15 no_jobs",
		"BLOCK-NOT-CALLED invalid "+p+":22 "+p+":21 performing",
		"BLOCK-NOT-CALLED invalid "+p+":29 "+p+":27 maybe",
		"BLOCK-NOT-CALLED invalid "+p+":35 "+p+":35 stubbed",
		"BLOCK-NOT-CALLED invalid "+p+":53 "+p+":35 stubs")
	if !strings.Contains(r.Findings[0].Message, "never yields") || !strings.Contains(r.Findings[7].Message, "never calls it") {
		t.Errorf("messages %q / %q", r.Findings[0].Message, r.Findings[7].Message)
	}
}

// TestBlockNotCalledNegatives: a block forwarded with &block, block.call, yield, instance_exec
// and instance_eval, a helper that calls the block on every path, super, a bare block returned
// at the def's own level, a ternary use, a string interpolation, a `def self.`, a block_given?
// refusal, a def inside a class or module, an anonymous & or `...` parameter and a def with no
// block are not reported (TSA-V0-055).
func TestBlockNotCalledNegatives(t *testing.T) {
	spec := `RSpec.describe Mailer do
  def forwarded(&block)
    assert_no_enqueued_jobs(&block)
  end

  def called(&block)
    assert_no_enqueued_jobs { block.call }
  end

  def yielded(&block)
    assert_no_enqueued_jobs do
      yield
    end
  end

  def execed(&block)
    expect { instance_exec(&block) }.not_to change(Email, :count)
  end

  def evaled(&block)
    assert_no_emails { instance_eval(&block) }
  end

  def every_path(strict, &block)
    if strict
      Mailer.strictly { block.call }
    else
      block.call
    end
  end

  def inherited(&block)
    super
  end

  def returned(&block)
    block
  end

  def anonymous(&)
    assert_no_enqueued_jobs { 1 }
  end

  def forwarding(...)
    assert_no_enqueued_jobs { 1 }
  end

  def plain
    create(:user)
  end

  def ternary(fast, &block)
    expect { fast ? block : nil }.not_to raise_error
  end

  def interpolated(&block)
    assert_no_emails { log("#{block.call}") }
  end

  def self.macro(&block)
    it('runs') { expect { block }.not_to raise_error }
  end

  def refuses
    raise ArgumentError, 'no block' if block_given?
    create(:user)
  end

  module Helpers
    def wrapped(&block)
      expect { block }.not_to raise_error
    end
  end

  class FakeClient
    def with_retry(&block)
    end
  end

  it 'uses them' do
    forwarded { Mailer.deliver }
    called { Mailer.deliver }
    yielded { Mailer.deliver }
    execed { deliver }
    evaled { deliver }
    every_path(true) { Mailer.deliver }
    returned { Mailer.deliver }
    wrapped { Mailer.deliver }
    ternary(true) { Mailer.deliver }
    interpolated { Mailer.deliver }
    plain
  end
end
`
	r := ValidityFindings("spec/n_spec.rb", spec)
	wantFindings(t, "negatives", r)
	if r.Uncertain {
		t.Errorf("negatives: uncertain %q", r.Reason)
	}
}

// TestBlockNotCalledAbstainsAndVerdict: an unbalanced helper makes the report uncertain with no
// finding (TSA-V0-050, TSA-V0-055), and a caller's finding applied to an eligible projection
// makes hygiene INELIGIBLE with reason block-not-called (TSA-V0-056).
func TestBlockNotCalledAbstainsAndVerdict(t *testing.T) {
	unbalanced := strings.Replace(issue1140Spec, "    end\n  end\n", "    end\n", 1)
	if r := ValidityFindings("spec/u_spec.rb", unbalanced); !r.Uncertain || r.Reason != validityRSpecUnbalanced || len(r.Findings) != 0 {
		t.Errorf("unbalanced: %+v", r)
	}
	deep := "RSpec.describe X do\n  def h(&block)\n" + strings.Repeat("    1.times do\n", maxRSpecDepth+1) +
		strings.Repeat("    end\n", maxRSpecDepth+1) + "  end\nend\n"
	if r := ValidityFindings("spec/d_spec.rb", deep); !r.Uncertain || r.Reason != validityRSpecTooDeep || len(r.Findings) != 0 {
		t.Errorf("deep helper: %+v", r)
	}
	r := ValidityFindings("spec/n_spec.rb", issue1140Spec)
	eligible := testvalidity.Projection{Hygiene: testvalidity.Axis{State: testvalidity.HygieneEligible, Anchors: []string{}}}
	got := testvalidity.ApplySourceFindings(eligible, r.Findings[1:2])
	if got.Hygiene.State != testvalidity.HygieneIneligible || got.Hygiene.Reason != testvalidity.HygieneReasonBlockNotCalled ||
		strings.Join(got.Hygiene.Anchors, " ") != "spec/n_spec.rb:11 spec/n_spec.rb:2" {
		t.Errorf("verdict %+v", got.Hygiene)
	}
}
