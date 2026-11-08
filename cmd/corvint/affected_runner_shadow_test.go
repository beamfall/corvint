// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestRSPV0040AffectedReceiptCarriesNoShadow pins the 2026-11-08 decision that the
// advisory runner-selection shadow never enters the default, full or compact
// `affected` receipt: AFP-V0-009 closes the advice members, and the shadow stays
// reachable only through the opt-in test-evidence reader (RSP-V0-051).
func TestRSPV0040AffectedReceiptCarriesNoShadow(t *testing.T) {
	root := affectedFixtureRepository(t)
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	for _, extra := range [][]string{nil, {"--full"}} {
		args := append([]string{"--base", base}, extra...)
		receipt, _, stderr, code := runAffectedArguments(t, root, args...)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if _, ok := receipt["advice"].(map[string]any)["runner_selection_shadow"]; ok {
			t.Fatalf("%v: affected advice carries the runner-selection shadow", extra)
		}
	}
	full, err := compileAffected(context.Background(), affectedInvocation{Root: root, Base: base, NoCache: true})
	if err != nil {
		t.Fatal(err)
	}
	compact, err := compactAffected(full)
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"full": full, "compact": compact} {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte("runner_selection")) {
			t.Fatalf("%s receipt carries a runner-selection member", name)
		}
	}
}
