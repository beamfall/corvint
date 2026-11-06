package main

import (
	"strings"
	"testing"
)

// AMAP-V0-211: the help documents the incremental flags and what each kind of edit re-derives.
// The build behaviour itself is TestAMAPV0211BuildFromCLI in internal/command/flows.
func TestAMAPV0211BuildFromHelp(t *testing.T) {
	if help := runCatalogHelp(t, "flows", "appmap", "build", "--help"); !strings.Contains(help, "--from MAP") || !strings.Contains(help, "--verify") ||
		!strings.Contains(help, "AMAP-V0-245") || !strings.Contains(help, "index-admission-changed") {
		t.Fatalf("appmap build help lacks the incremental build: %s", help)
	}
}
