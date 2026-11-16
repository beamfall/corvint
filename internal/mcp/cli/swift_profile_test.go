// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/swiftimpact"
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

// LGP-V0-8600: inspect the actual startup constructors. Frozen MCP admission
// prevents a Swift call, so inspect the private dependency without adding a
// production accessor or injecting a reader from this test.
func TestProfileConstructorsInstallSwiftReader(t *testing.T) {
	root := makeHostileRepository(t, "Swift reader startup")
	for _, profile := range []string{"", "flows"} {
		t.Run("profile="+profile, func(t *testing.T) {
			constructor := profileRegistries[profile]
			if constructor == nil {
				t.Fatal("missing profile constructor")
			}
			registry, err := constructor(root)
			if err != nil {
				t.Fatal(err)
			}
			reader := reflect.ValueOf(registry).Elem().FieldByName("swiftPMReader")
			if !reader.IsValid() || reader.Kind() != reflect.Interface || reader.IsNil() {
				t.Fatal("startup constructor did not install a Swift reader")
			}
			if got, want := reader.Elem().Type(), reflect.TypeOf(swiftimpact.Reader{}); got != want {
				t.Fatalf("startup reader = %v, want %v", got, want)
			}
		})
	}
}
