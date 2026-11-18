// SPDX-License-Identifier: Apache-2.0
package gateselect

import "testing"

func TestRuntimeEdgeDegradationAdmission_CINV1942(t *testing.T) {
	detail := `runtime:unresolved-edge: tests="a_test.go" runs="new:unit": runs names no unit; reason="runtime tool"`
	if !AllowedUnknown("LANGUAGE_FRONTIER", detail) {
		t.Fatal("conservatively widened runtime edge refused")
	}
	for _, value := range []string{"runtime:anything", "runtime:unresolved-edge", "go:unresolved-edge: tests=x"} {
		if AllowedUnknown("LANGUAGE_FRONTIER", value) {
			t.Fatalf("unrelated frontier admitted: %q", value)
		}
	}
	if AllowedUnknown("UNINDEXED_SOURCE_PATH", detail) {
		t.Fatal("unrelated unknown admitted")
	}
}
