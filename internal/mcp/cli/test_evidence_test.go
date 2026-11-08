// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"slices"
	"testing"
)

func TestRSPV0051MCPOptIn(t *testing.T) {
	root, _, _ := followRootsFixture(t)
	named := toolNames(listedTools(t, []string{"--root", root, "--tool-profile", "test-evidence"}))
	if !slices.Contains(named, "corvint.test_evidence") {
		t.Fatal(named)
	}
	defaults := toolNames(listedTools(t, []string{"--root", root}))
	if slices.Contains(defaults, "corvint.test_evidence") {
		t.Fatal("default surface changed")
	}
	_, responses, stderr := legacySession(t, []string{"--root", root, "--tool-profile", "test-evidence"}, toolCall(2, "corvint.test_evidence", map[string]any{"path": "source.go"}), toolCall(3, "corvint.test_evidence", map[string]any{"path": "../outside"}))
	if stderr != "" {
		t.Fatal(stderr)
	}
	result := resultOf(t, responses[1])
	structured, ok := result["structuredContent"].(map[string]any)
	if !ok || structured["status"] != "UNKNOWN" {
		t.Fatal(result)
	}
	if responses[2]["error"] == nil {
		t.Fatal(responses[2])
	}
}
