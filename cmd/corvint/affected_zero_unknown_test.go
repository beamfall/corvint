// SPDX-License-Identifier: Apache-2.0
package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AZU-V0-006 and AZU-V0-007: the default compact plan retains the full plan's
// coverage gaps and runnable consumer argv; the witness chain is unchanged.
func TestAZUV0006And007CompactPlan(t *testing.T) {
	run := []string{"bash", "script/web-flows-gate"}
	plan := affected.Plan{
		Uncovered: []affected.Unknown{{Reason: affected.UnknownNoSelectableTest, Detail: "go:gap"}},
		Selected:  []affected.Selection{{UnitID: "declared-consumer:web/e2e.mjs", Tests: []string{"web/e2e.mjs"}, Kind: "DECLARED_CONSUMER", Run: run, Witness: affected.Witness{Kind: "DECLARED_CONSUMER", DirtyPath: "cmd/tool/main.go", Via: []string{"go:tool", "declared-consumer:web/e2e.mjs"}}}},
	}
	compact, err := compactAffected(affectedReceipt{Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(compact.Plan.Uncovered, plan.Uncovered) {
		t.Fatal(compact.Plan.Uncovered)
	}
	text := renderAffectedText(compact)
	if !strings.Contains(text, "uncovered NO_SELECTABLE_TEST go:gap") || !strings.Contains(text, "run ") || !strings.Contains(text, "script/web-flows-gate") {
		t.Fatal(text)
	}
	selection := compact.Plan.Selected[0]
	if selection.Kind != "DECLARED_CONSUMER" || !reflect.DeepEqual(selection.Run, run) || selection.TestCount != 1 || selection.Witness.Kind != "DECLARED_CONSUMER" || selection.Witness.ViaLength != 2 {
		t.Fatalf("selection=%+v", selection)
	}
}
