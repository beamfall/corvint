package testsource

import "testing"

// issue1358Helpers is the helper shape GitHub issue #1358 reports, with its comments kept: a
// multi-line def that returns a value and never yields, beside a yielding, a block_given? and an
// &block-forwarding helper.
const issue1358Helpers = `  def read_data            # returns a value, never yields
    StringIO.new(build_csv)
  end
  def read_yielding
    yield StringIO.new(build_csv)
  end
  def read_checked
    return to_enum(:read_checked) unless block_given?
    yield StringIO.new(build_csv)
  end
  def read_forwarding(&block)
    with_csv(&block)
  end
`

// issue1358Examples are the issue's minimal example, its rewrite without the block, and one
// example per clean helper.
const issue1358Examples = `  it "exports only rows in range" do
    read_data do |data|    # block discarded
      expect(data.string.lines.size).to eq 2   # never runs
    end
  end
  it "exports only rows in range, rewritten" do
    data = read_data; expect(data.string.lines.size).to eq 2
  end
  it "yields" do
    read_yielding { |data| expect(data.string.lines.size).to eq 2 }
  end
  it "checks block_given?" do
    read_checked { |data| expect(data.string.lines.size).to eq 2 }
  end
  it "forwards the block" do
    read_forwarding { |data| expect(data.string.lines.size).to eq 2 }
  end
`

// TestRSpecIssue1358NonYieldingHelper pins GitHub issue #1358 against TSA-V0-084 (V1-1866,
// V1-2125): the issue's block handed to a helper that never yields is RB-BLOCK-NEVER-YIELDED with
// the helper's definition location, whether the helper is defined in the spec file or in a
// shared_context of a support file included into the group; the rewrite without the block and
// the yield, block_given? and &block helpers stay clean. A support module helper abstains.
func TestRSpecIssue1358NonYieldingHelper(t *testing.T) {
	t.Run("helper in the spec file", func(t *testing.T) {
		spec := "RSpec.describe Export do\n" + issue1358Helpers + issue1358Examples + "end\n"
		got := rbBrief(t, "spec/export_spec.rb", spec, bound(nil))
		wantExamples(t, got, map[int][]string{
			15: {"RB-BLOCK-NEVER-YIELDED invalid spec/export_spec.rb:16 spec/export_spec.rb:2"},
			20: {}, 23: {}, 26: {}, 29: {},
		})
	})
	t.Run("helper in a shared_context support file", func(t *testing.T) {
		context := "RSpec.shared_context 'export io' do\n" + issue1358Helpers + "end\n"
		spec := "RSpec.describe Export do\n  include_context 'export io'\n" + issue1358Examples + "end\n"
		got := rbBrief(t, "spec/export_spec.rb", spec, bound(map[string]string{"spec/support/export_io.rb": context}))
		wantExamples(t, got, map[int][]string{
			3: {"RB-BLOCK-NEVER-YIELDED invalid spec/export_spec.rb:4 spec/support/export_io.rb:2"},
			8: {}, 11: {}, 14: {}, 17: {},
		})
	})
	// A support module mixed in by config.include is not followed (TSA-V0-088): its never-yielding
	// def makes the block call abstain with the def as the named cause instead of a verdict.
	t.Run("helper in a support module", func(t *testing.T) {
		module := "module ExportHelpers\n" + issue1358Helpers + "end\n"
		spec := "RSpec.describe Export do\n" + issue1358Examples + "end\n"
		got := rbBrief(t, "spec/export_spec.rb", spec, bound(map[string]string{"spec/support/export_helpers.rb": module}))
		wantExamples(t, got, map[int][]string{
			2: {"RB-HELPER-UNRESOLVED abstained spec/export_spec.rb:3 spec/support/export_helpers.rb:2"},
			7: {}, 10: {}, 13: {}, 16: {},
		})
	})
}
