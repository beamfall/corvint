// SPDX-License-Identifier: Apache-2.0

package runnernames

import "fmt"

// RunnerPiece is one selection capability, declared per runner (RSP-V0-001).
type RunnerPiece struct {
	Present bool
	Reason  string
}

// RunnerPieces is internal metadata; it does not extend existing runner wires.
type RunnerPieces struct {
	Inventory     RunnerPiece
	TestEvidence  RunnerPiece
	Qualification RunnerPiece
}

// AbsentRunnerPieces explicitly keeps each legacy runner on the current path.
func AbsentRunnerPieces(runner string) RunnerPieces {
	return RunnerPieces{
		Inventory:     RunnerPiece{Reason: runner + ": inventory not implemented"},
		TestEvidence:  RunnerPiece{Reason: runner + ": per-test evidence not implemented"},
		Qualification: RunnerPiece{Reason: runner + ": selection not qualified"},
	}
}

// FullReason never treats a declaration as qualification. Qualification is
// supplied by the core, not by the plugin (RSP-V0-001).
func (p RunnerPieces) FullReason(qualified bool) string {
	for _, item := range []struct {
		name  string
		piece RunnerPiece
	}{
		{"inventory", p.Inventory}, {"test-evidence", p.TestEvidence}, {"qualification", p.Qualification},
	} {
		if !item.piece.Present || item.piece.Reason != "" {
			return fmt.Sprintf("piece-unavailable:%s:%s", item.name, item.piece.Reason)
		}
	}
	if !qualified {
		return "runner-not-qualified"
	}
	return ""
}
