// SPDX-License-Identifier: Apache-2.0
package main

import (
	"os"
	"strings"
	"testing"
)

// CAP-V0-8800: introductory guidance must expose offline discovery without
// attributing live schemas or configured availability to the offline catalog.
func TestCAPV08800IntroductoryMCPDiscovery(t *testing.T) {
	src, err := os.ReadFile("../../docs/CAPABILITIES.md")
	if err != nil {
		t.Fatal(err)
	}
	intro, _, found := strings.Cut(string(src), "\n## Start here\n")
	if !found {
		t.Fatal("capability catalog introduction boundary missing")
	}
	intro = strings.Join(strings.Fields(intro), " ")
	for _, claim := range []string{
		"Use `corvint help --json` → `mcpTools` for offline MCP tool names, summaries, server/profile selection and role conditions.",
		"MCP tools do not accept CLI help flags.",
		"The configured server's live `tools/list` supplies input schemas and configured availability;",
		"the offline catalog does not assert that a tool is enabled there.",
	} {
		if !strings.Contains(intro, claim) {
			t.Errorf("capability catalog introduction missing discovery guidance: %q", claim)
		}
	}
}
