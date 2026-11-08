// SPDX-License-Identifier: Apache-2.0
package runnerqualification

import (
	"context"
	"testing"

	s "github.com/Beamfall/corvint/internal/runnerselection"
)

// RSP-V0-033/034: file classes must exercise core operations, not trusted labels.
func TestRSPV0034MislabeledPopulations(t *testing.T) {
	for _, piece := range []string{"core-file", "fixture-mutation"} {
		t.Run(piece, func(t *testing.T) {
			req := fixture()
			req.Declaration.Classes, req.Merges[0].Classes = []s.Class{s.Add}, []s.Class{s.Add}
			p := Population{Complete: true}
			for _, op := range []string{"listed-add", "lookup-add"} {
				p.Mutants = append(p.Mutants, Mutant{ID: op, Piece: piece, Operator: op, Class: s.Add, Edits: []Edit{{"existing", []byte("a"), []byte("b")}}})
			}
			mustBlock(t, Run(context.Background(), req, &fixtureDriver{population: func(s.Class) Population { return p }}), "core-operator-class-invalid")
		})
	}
	for _, tc := range []struct {
		name    string
		target  FileTarget
		corrupt func(*Mutant)
	}{
		{"byte-insertion", FileTarget{Path: "data", Bytes: []byte("ab"), Class: s.DataEdit, Kind: "data"}, func(m *Mutant) { m.Edits[0].After = []byte("abc") }},
		{"truncate-replacement", FileTarget{Path: "data", Bytes: []byte("ab"), Class: s.DataEdit, Kind: "data"}, func(m *Mutant) { m.Operator = "truncate"; m.Edits[0].After = []byte("z") }},
		{"delete-replacement", FileTarget{Path: "old", Bytes: []byte("a"), Class: s.DeleteRename, Kind: "delete"}, func(m *Mutant) { m.Edits[0].After = []byte("b") }},
		{"rename-content-drift", FileTarget{Path: "old", Bytes: []byte("a"), Class: s.DeleteRename, Kind: "rename", Destination: "new", DestinationAbsent: true}, func(m *Mutant) { m.Edits[1].After = []byte("b") }},
		{"exec-add", FileTarget{Path: "tool", Bytes: []byte("a"), Class: s.ExecTarget, Kind: "exec"}, func(m *Mutant) { m.Edits[0].Before = nil }},
		{"config-delete", FileTarget{Path: "config", Bytes: []byte("a"), Class: s.RunnerConfig, Kind: "configuration"}, func(m *Mutant) { m.Edits[0].After = nil }},
		{"unknown-form", FileTarget{Path: "tool", Bytes: []byte("a"), Class: s.ExecTarget, Kind: "exec"}, func(m *Mutant) { m.Operator = "future-file-operation" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := FilePopulation([]FileTarget{tc.target})
			tc.corrupt(&p.Mutants[0])
			if err := ValidatePopulation(fixture().Declaration, tc.target.Class, p); err == nil || err.Error() != "core-operator-class-invalid" {
				t.Fatalf("mislabeled form admitted: %v", err)
			}
		})
	}
}
