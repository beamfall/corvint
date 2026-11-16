// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
)

// NGI-V0-005, LGP-V0-8600: explicit reader wiring does not widen the frozen
// MCP language allowlist. Swift and unknown future forms still fail closed.
func TestProfileImpactRetainsClosedLanguageSet(t *testing.T) {
	root := makeHostileRepository(t, "impact admission")
	for profile, constructor := range profileRegistries {
		t.Run("profile="+profile, func(t *testing.T) {
			registry, err := constructor(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"Sources/Core/Ledger.swift", "Sources/Core/Ledger.future"} {
				_, callErr := registry.Call(context.Background(), bridge.ToolImpact, []byte(`{"paths":["`+path+`"]}`))
				if callErr == nil || callErr.Code != "invalid-arguments" {
					t.Fatalf("%s: expected frozen admission refusal, got %v", path, callErr)
				}
			}
		})
	}
}
