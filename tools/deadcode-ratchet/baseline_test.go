// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"testing"
)

// wantBaselineEntries is the number of entries in baseline.json. It changes
// only in the commit that changes the baseline (DCR-V0-004). V1-4801 recorded
// 139 functions on 2026-11-06; the six-configuration report measured 149 at
// origin/main 4c3c20333 on 2026-11-11.
const wantBaselineEntries = 149

//go:embed baseline.json
var committedBaseline []byte

// DCR-V0-004, V1-4801 criterion 3: the committed baseline decodes strictly
// and holds exactly the asserted number of entries.
func TestCommittedBaselineCount(t *testing.T) {
	entries, err := decodeBaseline(committedBaseline)
	if err != nil {
		t.Fatalf("baseline.json: %v", err)
	}
	if len(entries) != wantBaselineEntries {
		t.Fatalf("baseline.json has %d entries, want %d; change wantBaselineEntries in the same commit as the baseline", len(entries), wantBaselineEntries)
	}
}
