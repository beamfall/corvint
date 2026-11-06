// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"strings"
	"testing"
)

// AZU-V0-9900: FULL preserves explanations from both gate-only slices and
// independently reached whole packages without mutating the caller's slices.
func TestOwnershipFullReasons_AZU_V0_9900(t *testing.T) {
	input := fixtureInput()
	input.GoSelection = "FALLBACK UNINDEXED_SOURCE_PATH gone/only.go"
	const selected = "selected: TestOwnershipAuditRatchet_AZU_V0_002 via gone/only.go (AZU-V0-9900)"
	const mustRun = "whole: MUST_RUN (TestOwnershipAuditRatchet_AZU_V0_002 via gone/only.go (AZU-V0-9900))"
	input.GoSlices = map[string]GoSlice{
		"example.test/m/a": {Pattern: "^(TestOwnershipAuditRatchet_AZU_V0_002)$", Reason: selected, FullReason: "whole: FULL; required " + selected},
		"example.test/m/b": {Reason: mustRun, FullReason: mustRun},
		"example.test/m/c": {Reason: "whole: DIRECT_SOURCE_CHANGE via c/c.go"},
	}
	manifest, err := Build(input)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Scope != ScopeFull || len(manifest.Units) != len(input.Universe) {
		t.Fatalf("not FULL: %+v", manifest)
	}
	for _, unit := range manifest.Units {
		if unit.Slice != SliceWhole || unit.Pattern != "" || unit.Flag != "" {
			t.Fatalf("FULL narrowed: %+v", unit)
		}
		switch unit.Unit {
		case "go:example.test/m/a":
			if !strings.Contains(unit.Reason, selected) {
				t.Fatalf("lost selected explanation: %+v", unit)
			}
		case "go:example.test/m/b":
			if unit.Reason != mustRun {
				t.Fatalf("lost whole explanation: %+v", unit)
			}
		case "go:example.test/m/c":
			if unit.Reason != "" {
				t.Fatalf("unrelated fallback receipt changed: %+v", unit)
			}
		}
	}
	if input.GoSlices["example.test/m/a"].Pattern == "" {
		t.Fatal("Build mutated the input slice")
	}
}
