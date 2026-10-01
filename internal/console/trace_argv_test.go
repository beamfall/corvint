package console

import (
	"github.com/Beamfall/corvint/internal/trace"
	"strings"
	"testing"
)

// LAC-V0-038: a typed row is validated and displayed as literal labelled JSON.
func TestConsoleTypedArgv(t *testing.T) {
	t.Run("LAC-V0-038 typed literal trace display", func(t *testing.T) {
		revision := strings.Repeat("a", 40)
		record, err := trace.NewRecord(trace.Input{Revision: revision, Task: "typed task", Outcome: "passed", VerificationArgv: [][]string{{"printf", "a b", "", "<script>", ";"}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		edge := traceEdge("trace.jsonl", revision, 1, string(raw), Axes{})
		if edge.Gap != "" || edge.Target != record.TraceID || !strings.Contains(edge.Detail, `argv ["printf","a b","","<script>",";"]`) {
			t.Fatalf("edge: %+v", edge)
		}
		corrupt := strings.Replace(string(raw), "typed task", "tampered task", 1)
		if traceEdge("trace.jsonl", revision, 1, corrupt, Axes{}).Gap != GapUnsupported {
			t.Fatal("invalid v2 identity admitted")
		}

	})
}
