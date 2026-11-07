// SPDX-License-Identifier: Apache-2.0
package main

import (
	json "encoding/json/v2"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TSP-V0-1062: both JSON projections and text preserve the declaration
// responsible for test-source fan-out.
func TestDeclaredScopeExplanation_TSPV01062(t *testing.T) {
	selection := affected.Selection{UnitID: "go:reader", Tests: []string{"reader_test.go"}, Witness: affected.Witness{
		Kind: affected.WitnessDeclaredReadScope, DirtyPath: "pkg/new_test.go", Via: []string{"go:reader"},
		ReadScope: &affected.ReadScopeCause{Declaration: affected.ReadScopesPath, Entry: "pkg/"},
	}}
	compact, _ := compactAffectedSelections([]affected.Selection{selection})
	for _, value := range []any{selection, compact[0]} {
		raw, err := json.Marshal(value)
		if err != nil || !strings.Contains(string(raw), `"readScope":{"declaration":".corvint/test-read-scopes.json","entry":"pkg/"}`) {
			t.Fatalf("scope projection: %s %v", raw, err)
		}
	}
	receipt := affectedCompactReceipt{}
	receipt.Plan.Selected = compact
	text := renderAffectedText(receipt)
	if !strings.Contains(string(text), "declaration .corvint/test-read-scopes.json entry pkg/") {
		t.Fatalf("scope text: %s", text)
	}
}
