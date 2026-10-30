package cli

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateTailLines bounds the output lines a failed gate run quotes
// (TCD-V0-165); the full output stays retained as evidence (TCD-V0-166).
const gateTailLines = 20

// gateResultValue is the `gateResult` of a gate run that executed and
// committed its gate in this call, or null for a reuse, a replay, a refusal
// or an undecodable record (TCD-V0-165). A result that did not pass quotes
// a bounded, secret-screened tail of its captured output and the line that
// names the failure; the message, if any, is the warning that says so.
func gateResultValue(run *store.GateExecution) (wire.Value, string) {
	if run == nil {
		return wire.Null(), ""
	}
	g, err := snapshot.DecodeGateResult(run.Record)
	if err != nil || len(g.Evidence) == 0 {
		return wire.Null(), ""
	}
	out := g.Evidence[0]
	path := "evidence/" + string(out.Sha256)
	o := wire.NewObject().
		Set("gateId", wire.String(g.GateID)).
		Set("state", wire.String(g.State)).
		Set("outcomeClass", wire.String(g.OutcomeClass)).
		Set("exitCode", nullableCount(g.ExitCode)).
		Set("signal", nullableString(g.Signal)).
		Set("output", wire.ObjectValue(wire.NewObject().
			Set("sha256", wire.String(string(out.Sha256))).
			Set("bytes", wire.String(string(out.Bytes))).
			Set("path", wire.String(path))))
	if g.State == "PASSED" {
		o.Set("failingLine", wire.Null()).Set("tail", wire.Null())
		return wire.ObjectValue(o), ""
	}
	tail, failing := obligation.FailureTail(string(run.Output), gateTailLines)
	o.Set("failingLine", stringOrNull(failing)).Set("tail", wire.Strings(tail))
	exit := "no exit code"
	if g.ExitCode != nil {
		exit = "exit " + string(*g.ExitCode)
	}
	if failing == "" {
		failing = "no output"
	}
	return wire.ObjectValue(o), fmt.Sprintf("gate %s %s (%s, %s): %s; full output retained at %s (TCD-V0-165)", g.GateID, g.State, g.OutcomeClass, exit, failing, path)
}

func nullableString(s *string) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(*s)
}
