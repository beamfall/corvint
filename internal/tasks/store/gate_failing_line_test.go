package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0801_GateRunCarriesTheFailurePattern: a gate run that executed a
// gate with a policy failurePattern reports that pattern with its output, so
// the CLI's failingLine names the matched step; a gate without one reports
// nil and the default skips make's wrapper line (V1-3940).
func TestTCDV0801_GateRunCarriesTheFailurePattern(t *testing.T) {
	script := `printf "step tonemap: BAD frame 3\nFAIL summary\nmake: *** [gate] Error 2\n"; exit 2`
	patterned := commandGate("patterned", script, "30", false)
	patterned.Obj.Set("failurePattern", wire.String(`^step [a-z]+: BAD`))
	s := newLeaseStore(t, patterned, commandGate("plain", script, "30", false))
	claim, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)
	for gate, want := range map[string]string{"patterned": "step tonemap: BAD frame 3", "plain": "FAIL summary"} {
		report, err := s.gate(t, "gate-"+gate, gateOf(claim, gate), 2)
		if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted || report.GateExecuted == nil {
			t.Fatalf("%s: %+v %v", gate, report, err)
		}
		if (gate == "patterned") != (report.GateExecuted.FailurePattern != nil) {
			t.Fatalf("%s: pattern %v", gate, report.GateExecuted.FailurePattern)
		}
		if _, failing := obligation.FailureTailMatching(string(report.GateExecuted.Output), 20, report.GateExecuted.FailurePattern); failing != want {
			t.Fatalf("%s: failing %q, want %q", gate, failing, want)
		}
	}
}
