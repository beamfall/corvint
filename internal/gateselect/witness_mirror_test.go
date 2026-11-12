// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAttributingWitnessMirrorsThePlanner pins the witness kinds and the
// existence-reader path Select spells for itself (RNC-V1-004, AFP-V0-980) to
// the planner's constants, so keeping Select free of the affected engine
// cannot let them drift.
func TestAttributingWitnessMirrorsThePlanner(t *testing.T) {
	for _, kind := range []string{affected.WitnessPathLiteralReader, affected.WitnessPathGlobReader, affected.WitnessDeclaredReadScope, affected.WitnessEnclosingPackage} {
		if !attributingWitness[kind] {
			t.Errorf("attributingWitness lacks the planner's %s", kind)
		}
	}
	if len(attributingWitness) != 5 || !attributingWitness["DECLARED_CONSUMER"] {
		t.Errorf("attributingWitness = %v, want the four planner kinds and DECLARED_CONSUMER", attributingWitness)
	}
	if existenceReadersPath != affected.ExistenceReadersPath {
		t.Errorf("existenceReadersPath = %q, want %q", existenceReadersPath, affected.ExistenceReadersPath)
	}
	if witnessDeclaredExistenceReader != affected.WitnessDeclaredExistenceReader {
		t.Errorf("witnessDeclaredExistenceReader = %q, want %q", witnessDeclaredExistenceReader, affected.WitnessDeclaredExistenceReader)
	}
}
