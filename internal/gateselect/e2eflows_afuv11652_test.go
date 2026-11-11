// SPDX-License-Identifier: Apache-2.0

package gateselect

import (
	"encoding/json"
	"testing"
)

// AFU-V1-1652: the driver admits a well-formed flow-binding entry beside the
// binary consumer entry, because the consumer it concerns stays selected or
// named; a malformed detail is not admitted.
func TestE2EFlowUnknownAdmitted_AFUV11652(t *testing.T) {
	for _, tc := range []struct {
		reason, detail string
		ok             bool
	}{
		{"E2E_UNMAPPED_CHANGE", "internal/lib/lib.go", true},
		{"E2E_UNMAPPED_CHANGE", "../lib.go", false},
		{"E2E_UNMAPPED_CHANGE", "", false},
		{"E2E_UNMAPPED_CHANGE", "a\\b.go", false},
		{"E2E_UNMAPPED_CHANGE", "lib.go\ncorvint-test: PASS", false},
		{"E2E_FLOWS_INVALID", ".corvint/e2e-flows.json", true}, //corvint:fixture-path
		{"E2E_FLOWS_INVALID", ".corvint/other.json", false},
		{"UNSELECTED_BINARY_CONSUMER", "web/e2e.mjs", false},
	} {
		if _, ok := E2EFlowUnknown(tc.reason, tc.detail); ok != tc.ok {
			t.Errorf("%s %q: admitted=%v, want %v", tc.reason, tc.detail, ok, tc.ok)
		}
	}
	var plan Receipt
	if err := json.Unmarshal([]byte(`{"plan":{"scope":"UNKNOWN","unknown":[{"reason":"E2E_UNMAPPED_CHANGE","detail":"internal/lib/lib.go"},{"reason":"E2E_FLOWS_INVALID","detail":".corvint/e2e-flows.json"}]}}`), &plan); err != nil { //corvint:fixture-path
		t.Fatal(err)
	}
	if reason, lines := unknownNarrowing(plan, &repositoryIndex{}, t.TempDir()); reason != "" || len(lines) != 0 {
		t.Fatalf("narrowing refused: %q %v", reason, lines)
	}
	if err := json.Unmarshal([]byte(`{"plan":{"scope":"UNKNOWN","unknown":[{"reason":"E2E_UNMAPPED_CHANGE","detail":"../lib.go"}]}}`), &plan); err != nil {
		t.Fatal(err)
	}
	if reason, _ := unknownNarrowing(plan, &repositoryIndex{}, t.TempDir()); reason == "" {
		t.Fatal("malformed flow entry narrowed")
	}
}
