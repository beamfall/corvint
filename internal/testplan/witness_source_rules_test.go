package testplan

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testsource"
	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// TestWitnessRefusesMemoizedBlockExpectation: an otherwise passing witness whose example carries a
// MEMOIZED-BLOCK-EXPECTATION finding is refused as ineligible, while an advisory
// ISOLATED-IN-SERIAL finding leaves a passing witness reusable (TSA-V0-051, TSA-V0-047).
func TestWitnessRefusesMemoizedBlockExpectation(t *testing.T) {
	spec := `RSpec.describe WidgetSync do
  subject { described_class.new.call }
  before { subject }
  it 'creates an entry' do
    expect { subject }.to change(Entry, :count).by(1)
  end
end
`
	rspec := testsource.ValidityFindings("spec/widget_sync_spec.rb", spec)
	serial := testsource.ValidityFindings("e2e/s.spec.ts", "test.describe.serial('s', () => { test('t @isolated', async () => {}); });\n")
	if len(rspec.Findings) != 1 || len(serial.Findings) != 1 {
		t.Fatalf("findings %+v %+v", rspec, serial)
	}
	refused := e2eDocument("memoized", "", testvalidity.ApplySourceFindings(passing(), rspec.Findings))
	advisory := e2eDocument("serial", "", testvalidity.ApplySourceFindings(passing(), serial.Findings))
	if _, got := judgeWitness(Witness{TestID: "memoized", Basis: "declared"}, []testvaliditydoc.Document{refused, advisory}); got != WitnessIneligible {
		t.Errorf("memoized witness: %q, want %q", got, WitnessIneligible)
	}
	if _, got := judgeWitness(Witness{TestID: "serial", Basis: "declared"}, []testvaliditydoc.Document{refused, advisory}); got != "" {
		t.Errorf("advisory witness refused: %q", got)
	}
}
