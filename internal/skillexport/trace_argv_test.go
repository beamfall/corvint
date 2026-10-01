package skillexport

import (
	"bytes"
	"github.com/Beamfall/corvint/internal/trace"
	"testing"
)

// LTA-V0-013: exported argv remains labelled JSON, including fence-like data.
func TestExportTypedArgv(t *testing.T) {
	record, err := trace.NewRecord(trace.Input{Revision: testRevision, Task: "typed export", Outcome: "passed", VerificationArgv: [][]string{{"printf", "a b", "", "```", ";"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := ExportRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	raw := skill.Files[ReferenceFile]
	if !bytes.Contains(raw, []byte("argv:\n\n    [\"printf\",\"a b\",\"\",\"```\",\";\"]\n")) {
		t.Fatalf("literal display: %s", raw)
	}
	record.TypedVerification[0].Argv[1] = "changed"
	if _, err := ExportRecord(record); err == nil {
		t.Fatal("tampered row exported")
	}
}
